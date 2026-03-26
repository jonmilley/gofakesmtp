// internal/smtp/backend.go
package smtp

import (
	"bytes"
	"io"
	"net/mail"
	"strings"
	"time"

	gosmtp "github.com/emersion/go-smtp"
)

// Backend implements go-smtp's Backend interface.
// It creates a new session per connection, each of which sends parsed
// emails onto ch when a message is fully received.
type Backend struct {
	ch chan Email
}

// NewBackend returns a Backend that sends received emails onto ch.
func NewBackend(ch chan Email) *Backend {
	return &Backend{ch: ch}
}

// NewSession implements gosmtp.Backend.
func (b *Backend) NewSession(_ *gosmtp.Conn) (gosmtp.Session, error) {
	return &Session{ch: b.ch}, nil
}

// Session holds per-connection state: the envelope from/to addresses
// collected during the SMTP dialogue, plus the shared output channel.
type Session struct {
	ch   chan Email
	from string
	to   []string
}

// AuthPlain accepts all credentials (this is a fake server).
func (s *Session) AuthPlain(_, _ string) error { return nil }

// Mail records the envelope sender.
func (s *Session) Mail(from string, _ *gosmtp.MailOptions) error {
	s.from = from
	return nil
}

// Rcpt records each envelope recipient.
func (s *Session) Rcpt(to string, _ *gosmtp.RcptOptions) error {
	s.to = append(s.to, to)
	return nil
}

// Data reads the full message, parses it, and sends it onto the channel.
func (s *Session) Data(r io.Reader) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return err
	}

	bodyBytes, err := io.ReadAll(msg.Body)
	if err != nil {
		return err
	}

	email := Email{
		From:       s.from,
		To:         s.to,
		Subject:    msg.Header.Get("Subject"),
		Body:       strings.TrimSpace(string(bodyBytes)),
		Raw:        raw,
		ReceivedAt: time.Now(),
	}

	select {
	case s.ch <- email:
	default:
		// Channel full — drop the email rather than stalling the SMTP connection.
		// This should not occur in normal use (buffer is 100).
	}
	return nil
}

// Reset clears the session envelope state between messages.
func (s *Session) Reset() {
	s.from = ""
	s.to = nil
}

// Logout is a no-op.
func (s *Session) Logout() error { return nil }
