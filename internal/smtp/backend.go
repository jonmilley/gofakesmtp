// internal/smtp/backend.go
package smtp

import (
	"bytes"
	"fmt"
	"io"
	"net/mail"
	"strings"
	"time"

	"github.com/emersion/go-sasl"
	gosmtp "github.com/emersion/go-smtp"
)

// Config configures Backend behavior.
//
// Auth modes:
//   - Username == "" && !AllowAnyAuth: AUTH is not advertised; clients send mail directly.
//   - Username != "":                  AUTH PLAIN required; only the configured creds are accepted.
//   - AllowAnyAuth == true:            AUTH PLAIN advertised; any credentials accepted; clients
//     may also skip auth and send mail directly.
//
// Username/Password are ignored when AllowAnyAuth is true.
type Config struct {
	Username     string
	Password     string
	AllowAnyAuth bool
	SessionLog   io.Writer
}

// Backend implements go-smtp's Backend interface.
// It creates a new session per connection, each of which sends parsed
// emails onto ch when a message is fully received.
type Backend struct {
	ch  chan Email
	cfg Config
}

// NewBackend returns a Backend that sends received emails onto ch.
// See Config for the available auth modes and session logging.
func NewBackend(ch chan Email, cfg Config) *Backend {
	return &Backend{ch: ch, cfg: cfg}
}

// NewSession implements gosmtp.Backend.
func (b *Backend) NewSession(c *gosmtp.Conn) (gosmtp.Session, error) {
	s := &Session{
		ch:           b.ch,
		username:     b.cfg.Username,
		password:     b.cfg.Password,
		requireAuth:  b.cfg.Username != "" && !b.cfg.AllowAnyAuth,
		allowAnyAuth: b.cfg.AllowAnyAuth,
		sessionLog:   b.cfg.SessionLog,
	}
	if c != nil && c.Conn() != nil {
		s.remoteAddr = c.Conn().RemoteAddr().String()
	}
	s.logMarker("connect")
	return s, nil
}

// Session holds per-connection state: the envelope from/to addresses
// collected during the SMTP dialogue, plus the shared output channel.
type Session struct {
	ch           chan Email
	from         string
	to           []string
	username     string
	password     string
	requireAuth  bool
	allowAnyAuth bool
	authed       bool
	sessionLog   io.Writer
	remoteAddr   string
}

// AuthMechanisms advertises supported AUTH mechanisms. Returning nil
// causes go-smtp to omit the AUTH capability entirely.
func (s *Session) AuthMechanisms() []string {
	if !s.requireAuth && !s.allowAnyAuth {
		return nil
	}
	return []string{sasl.Plain}
}

// Auth handles AUTH PLAIN. When AllowAnyAuth is set, any credentials
// are accepted; otherwise they must match the configured username/password.
func (s *Session) Auth(mech string) (sasl.Server, error) {
	if !s.requireAuth && !s.allowAnyAuth {
		return nil, gosmtp.ErrAuthUnsupported
	}
	if mech != sasl.Plain {
		return nil, gosmtp.ErrAuthUnknownMechanism
	}
	return sasl.NewPlainServer(func(_, username, password string) error {
		if s.allowAnyAuth {
			s.authed = true
			s.logAuth(username, password, true)
			return nil
		}
		ok := username == s.username && password == s.password
		s.logAuth(username, password, ok)
		if !ok {
			return gosmtp.ErrAuthFailed
		}
		s.authed = true
		return nil
	}), nil
}

// Mail records the envelope sender.
func (s *Session) Mail(from string, _ *gosmtp.MailOptions) error {
	if s.requireAuth && !s.authed {
		return gosmtp.ErrAuthRequired
	}
	s.from = from
	return nil
}

// Rcpt records each envelope recipient.
func (s *Session) Rcpt(to string, _ *gosmtp.RcptOptions) error {
	if s.requireAuth && !s.authed {
		return gosmtp.ErrAuthRequired
	}
	s.to = append(s.to, to)
	return nil
}

// Data reads the full message, parses it, and sends it onto the channel.
func (s *Session) Data(r io.Reader) error {
	if s.requireAuth && !s.authed {
		return gosmtp.ErrAuthRequired
	}
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

// Logout writes a disconnect marker to the session log.
func (s *Session) Logout() error {
	s.logMarker("disconnect")
	return nil
}

func (s *Session) logMarker(event string) {
	if s.sessionLog == nil {
		return
	}
	fmt.Fprintf(s.sessionLog, "\n--- %s %s %s ---\n", time.Now().UTC().Format(time.RFC3339), event, s.remoteAddr)
}

func (s *Session) logAuth(username, password string, ok bool) {
	if s.sessionLog == nil {
		return
	}
	result := "ok"
	if !ok {
		result = "fail"
	}
	fmt.Fprintf(s.sessionLog, "\n--- %s auth-%s %s username=%q password=%q ---\n",
		time.Now().UTC().Format(time.RFC3339), result, s.remoteAddr, username, password)
}
