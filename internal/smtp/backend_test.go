// internal/smtp/backend_test.go
package smtp_test

import (
	"net"
	"net/smtp"
	"strings"
	"testing"
	"time"

	fakesmtp "github.com/emersion/go-smtp"
	oursmtp "github.com/jonmilley/gofakesmtp/internal/smtp"
)

func startTestServer(t *testing.T, ch chan oursmtp.Email) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	backend := oursmtp.NewBackend(ch)
	s := fakesmtp.NewServer(backend)
	s.Domain = "localhost"
	s.AllowInsecureAuth = true

	go func() {
		_ = s.Serve(ln)
	}()

	t.Cleanup(func() { s.Close() })
	return ln.Addr().String()
}

func TestBackend_ReceivesEmail(t *testing.T) {
	ch := make(chan oursmtp.Email, 10)
	addr := startTestServer(t, ch)

	msg := strings.Join([]string{
		"From: sender@example.com",
		"To: receiver@example.com",
		"Subject: Test Subject",
		"",
		"Hello, this is the body.",
	}, "\r\n")

	err := smtp.SendMail(addr, nil, "sender@example.com", []string{"receiver@example.com"}, []byte(msg))
	if err != nil {
		t.Fatalf("SendMail failed: %v", err)
	}

	select {
	case email := <-ch:
		if email.From != "sender@example.com" {
			t.Errorf("expected From=sender@example.com, got %q", email.From)
		}
		if len(email.To) != 1 || email.To[0] != "receiver@example.com" {
			t.Errorf("unexpected To: %v", email.To)
		}
		if email.Subject != "Test Subject" {
			t.Errorf("expected Subject=Test Subject, got %q", email.Subject)
		}
		if !strings.Contains(email.Body, "Hello, this is the body.") {
			t.Errorf("unexpected Body: %q", email.Body)
		}
		if email.ReceivedAt.IsZero() {
			t.Error("expected ReceivedAt to be set")
		}
		if len(email.Raw) == 0 {
			t.Error("expected Raw to be non-empty")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for email on channel")
	}
}

func TestBackend_MultipleRecipients(t *testing.T) {
	ch := make(chan oursmtp.Email, 10)
	addr := startTestServer(t, ch)

	msg := strings.Join([]string{
		"From: a@example.com",
		"To: b@example.com, c@example.com",
		"Subject: Multi",
		"",
		"Body",
	}, "\r\n")

	err := smtp.SendMail(addr, nil, "a@example.com", []string{"b@example.com", "c@example.com"}, []byte(msg))
	if err != nil {
		t.Fatalf("SendMail failed: %v", err)
	}

	select {
	case email := <-ch:
		if len(email.To) != 2 {
			t.Errorf("expected 2 recipients, got %d: %v", len(email.To), email.To)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for email on channel")
	}
}
