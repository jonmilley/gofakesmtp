// internal/smtp/backend_test.go
package smtp_test

import (
	"bytes"
	"io"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"testing"
	"time"

	fakesmtp "github.com/emersion/go-smtp"
	oursmtp "github.com/jonmilley/gofakesmtp/internal/smtp"
)

type serverOpts struct {
	username     string
	password     string
	allowAnyAuth bool
	sessionLog   *syncBuffer
}

// syncBuffer is a goroutine-safe bytes.Buffer suitable for use as Debug writer.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func startTestServer(t *testing.T, ch chan oursmtp.Email) string {
	return startTestServerOpts(t, ch, serverOpts{})
}

func startTestServerOpts(t *testing.T, ch chan oursmtp.Email, opts serverOpts) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	var logWriter io.Writer
	if opts.sessionLog != nil {
		logWriter = opts.sessionLog
	}

	backend := oursmtp.NewBackend(ch, oursmtp.Config{
		Username:     opts.username,
		Password:     opts.password,
		AllowAnyAuth: opts.allowAnyAuth,
		SessionLog:   logWriter,
	})
	s := fakesmtp.NewServer(backend)
	s.Domain = "localhost"
	s.AllowInsecureAuth = true
	s.Debug = logWriter

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

func TestBackend_AuthRequired_AcceptsValidCredentials(t *testing.T) {
	ch := make(chan oursmtp.Email, 10)
	addr := startTestServerOpts(t, ch, serverOpts{username: "alice", password: "s3cret"})

	host, _, _ := net.SplitHostPort(addr)
	auth := smtp.PlainAuth("", "alice", "s3cret", host)

	msg := strings.Join([]string{
		"From: alice@example.com",
		"To: bob@example.com",
		"Subject: Authed",
		"",
		"hi",
	}, "\r\n")

	if err := smtp.SendMail(addr, auth, "alice@example.com", []string{"bob@example.com"}, []byte(msg)); err != nil {
		t.Fatalf("SendMail failed: %v", err)
	}

	select {
	case email := <-ch:
		if email.Subject != "Authed" {
			t.Errorf("expected Subject=Authed, got %q", email.Subject)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for email on channel")
	}
}

func TestBackend_AuthRequired_RejectsBadCredentials(t *testing.T) {
	ch := make(chan oursmtp.Email, 10)
	addr := startTestServerOpts(t, ch, serverOpts{username: "alice", password: "s3cret"})

	host, _, _ := net.SplitHostPort(addr)
	auth := smtp.PlainAuth("", "alice", "wrong", host)

	msg := []byte("From: a@b\r\nTo: c@d\r\nSubject: x\r\n\r\nbody\r\n")
	err := smtp.SendMail(addr, auth, "a@b", []string{"c@d"}, msg)
	if err == nil {
		t.Fatal("expected SendMail to fail with bad credentials, got nil")
	}
	if !strings.Contains(err.Error(), "535") {
		t.Errorf("expected 535 auth-failed error, got %v", err)
	}
}

func TestBackend_AuthRequired_RejectsUnauthenticatedMail(t *testing.T) {
	ch := make(chan oursmtp.Email, 10)
	addr := startTestServerOpts(t, ch, serverOpts{username: "alice", password: "s3cret"})

	msg := []byte("From: a@b\r\nTo: c@d\r\nSubject: x\r\n\r\nbody\r\n")
	err := smtp.SendMail(addr, nil, "a@b", []string{"c@d"}, msg)
	if err == nil {
		t.Fatal("expected SendMail without auth to fail, got nil")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("expected 502 auth-required error, got %v", err)
	}
}

func TestBackend_AllowAnyAuth_AcceptsArbitraryCredentials(t *testing.T) {
	ch := make(chan oursmtp.Email, 10)
	logBuf := &syncBuffer{}
	addr := startTestServerOpts(t, ch, serverOpts{allowAnyAuth: true, sessionLog: logBuf})

	host, _, _ := net.SplitHostPort(addr)
	auth := smtp.PlainAuth("", "anyone", "hunter2", host)

	msg := []byte("From: a@b\r\nTo: c@d\r\nSubject: AnyAuth\r\n\r\nbody\r\n")
	if err := smtp.SendMail(addr, auth, "a@b", []string{"c@d"}, msg); err != nil {
		t.Fatalf("SendMail with arbitrary creds failed: %v", err)
	}

	select {
	case email := <-ch:
		if email.Subject != "AnyAuth" {
			t.Errorf("expected Subject=AnyAuth, got %q", email.Subject)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for email on channel")
	}

	got := logBuf.String()
	for _, want := range []string{"AUTH PLAIN", `auth-ok`, `username="anyone"`, `password="hunter2"`} {
		if !strings.Contains(got, want) {
			t.Errorf("session log missing %q; log was:\n%s", want, got)
		}
	}
}

func TestBackend_AuthRequired_LogsPlaintextCredentials(t *testing.T) {
	ch := make(chan oursmtp.Email, 10)
	logBuf := &syncBuffer{}
	addr := startTestServerOpts(t, ch, serverOpts{
		username:   "alice",
		password:   "s3cret",
		sessionLog: logBuf,
	})

	host, _, _ := net.SplitHostPort(addr)

	// First, a failed login.
	failAuth := smtp.PlainAuth("", "alice", "wrong", host)
	msg := []byte("From: a@b\r\nTo: c@d\r\nSubject: x\r\n\r\nbody\r\n")
	if err := smtp.SendMail(addr, failAuth, "a@b", []string{"c@d"}, msg); err == nil {
		t.Fatal("expected SendMail with bad creds to fail")
	}

	// Then a successful one.
	okAuth := smtp.PlainAuth("", "alice", "s3cret", host)
	if err := smtp.SendMail(addr, okAuth, "a@b", []string{"c@d"}, msg); err != nil {
		t.Fatalf("SendMail with good creds failed: %v", err)
	}
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for email on channel")
	}

	got := logBuf.String()
	for _, want := range []string{
		`auth-fail`, `password="wrong"`,
		`auth-ok`, `password="s3cret"`,
		`username="alice"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("session log missing %q; log was:\n%s", want, got)
		}
	}
}

func TestBackend_AllowAnyAuth_AlsoAcceptsUnauthenticated(t *testing.T) {
	ch := make(chan oursmtp.Email, 10)
	addr := startTestServerOpts(t, ch, serverOpts{allowAnyAuth: true})

	msg := []byte("From: a@b\r\nTo: c@d\r\nSubject: NoAuth\r\n\r\nbody\r\n")
	if err := smtp.SendMail(addr, nil, "a@b", []string{"c@d"}, msg); err != nil {
		t.Fatalf("SendMail without auth failed: %v", err)
	}

	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for email on channel")
	}
}

func TestBackend_SessionLog_CapturesWireDialogue(t *testing.T) {
	ch := make(chan oursmtp.Email, 10)
	logBuf := &syncBuffer{}
	addr := startTestServerOpts(t, ch, serverOpts{sessionLog: logBuf})

	msg := []byte("From: a@b\r\nTo: c@d\r\nSubject: Logged\r\n\r\nbody\r\n")
	if err := smtp.SendMail(addr, nil, "a@b", []string{"c@d"}, msg); err != nil {
		t.Fatalf("SendMail failed: %v", err)
	}

	// Drain the email so the session has a chance to complete.
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for email")
	}

	// Allow Logout marker to be written.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(logBuf.String(), "disconnect") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	got := logBuf.String()
	for _, want := range []string{"connect", "disconnect", "MAIL FROM", "RCPT TO", "Subject: Logged"} {
		if !strings.Contains(got, want) {
			t.Errorf("session log missing %q\nlog:\n%s", want, got)
		}
	}
}
