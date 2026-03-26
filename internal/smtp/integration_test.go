//go:build integration

package smtp_test

import (
	"net/smtp"
	"strings"
	"testing"
	"time"

	oursmtp "github.com/jonmilley/gofakesmtp/internal/smtp"
)

func TestIntegration_SendAndReceive(t *testing.T) {
	ch := make(chan oursmtp.Email, 10)
	addr := startTestServer(t, ch)

	body := "Line one of the integration test body.\r\nLine two."
	msg := strings.Join([]string{
		"From: integration@example.com",
		"To: recipient@example.com",
		"Subject: Integration test",
		"",
		body,
	}, "\r\n")

	if err := smtp.SendMail(addr, nil, "integration@example.com", []string{"recipient@example.com"}, []byte(msg)); err != nil {
		t.Fatalf("SendMail failed: %v", err)
	}

	select {
	case email := <-ch:
		if email.From != "integration@example.com" {
			t.Errorf("From: got %q, want %q", email.From, "integration@example.com")
		}
		if len(email.To) != 1 || email.To[0] != "recipient@example.com" {
			t.Errorf("To: got %v, want [recipient@example.com]", email.To)
		}
		if email.Subject != "Integration test" {
			t.Errorf("Subject: got %q, want %q", email.Subject, "Integration test")
		}
		wantBody := "Line one of the integration test body.\r\nLine two."
		if email.Body != wantBody {
			t.Errorf("Body: got %q, want %q", email.Body, wantBody)
		}
		if email.ReceivedAt.IsZero() {
			t.Error("ReceivedAt should not be zero")
		}
		if len(email.Raw) == 0 {
			t.Error("Raw should not be empty")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for email")
	}
}
