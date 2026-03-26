# gofakesmtp Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI tool that runs a local fake SMTP server with a Bubbletea TUI showing intercepted emails in a horizontal split layout.

**Architecture:** Channel-based: the SMTP backend parses incoming emails and sends them onto a buffered `chan Email`. The Bubbletea TUI listens on that channel via a `tea.Cmd` and updates its model on receipt. A separate storage package handles `.eml` file writes when `--output-dir` is set.

**Tech Stack:** Go 1.22+, `github.com/emersion/go-smtp`, `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`, `github.com/spf13/cobra`

---

## File Map

| File | Responsibility |
|------|---------------|
| `main.go` | Cobra CLI entry point; wires channel, SMTP server, TUI |
| `internal/smtp/email.go` | `Email` struct definition |
| `internal/smtp/backend.go` | `Backend` and `Session` implementations for `go-smtp` |
| `internal/smtp/backend_test.go` | Integration tests: start real listener, send via `net/smtp`, assert on channel |
| `internal/storage/storage.go` | `Save(email, dir) error` — writes `.eml` files |
| `internal/storage/storage_test.go` | Unit tests for file write, filename format, dir creation |
| `internal/tui/styles.go` | Lipgloss style definitions |
| `internal/tui/model.go` | Bubbletea `Model`, `Init`, `Update`, `View`; `waitForEmail()` cmd |
| `internal/tui/model_test.go` | Unit tests: call `Update()` with `tea.Msg` values, assert model state |

---

## Task 1: Project scaffold

**Files:**
- Create: `go.mod`
- Create: `main.go` (stub)

- [ ] **Step 1: Initialize the Go module**

```bash
cd /Users/jonmilley/projects/gofakesmtp
go mod init github.com/jonmilley/gofakesmtp
```

Expected: `go.mod` created with `module github.com/jonmilley/gofakesmtp` and current Go version.

- [ ] **Step 2: Add dependencies**

```bash
go get github.com/emersion/go-smtp@latest
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/spf13/cobra@latest
```

Expected: `go.sum` created, all four packages appear in `go.mod`.

- [ ] **Step 3: Create stub `main.go`**

```go
package main

import "fmt"

func main() {
	fmt.Println("gofakesmtp")
}
```

- [ ] **Step 4: Verify it builds**

```bash
go build ./...
```

Expected: no output, no errors.

- [ ] **Step 5: Commit**

```bash
git init
git add go.mod go.sum main.go
git commit -m "chore: initialize Go module with dependencies"
```

---

## Task 2: Email struct

**Files:**
- Create: `internal/smtp/email.go`

- [ ] **Step 1: Create the Email struct**

```go
// internal/smtp/email.go
package smtp

import "time"

// Email holds a parsed SMTP message received by the server.
type Email struct {
	From       string
	To         []string
	Subject    string
	Body       string
	Raw        []byte
	ReceivedAt time.Time
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./internal/smtp/...
```

Expected: no output, no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/smtp/email.go
git commit -m "feat: add Email struct"
```

---

## Task 3: Storage package

**Files:**
- Create: `internal/storage/storage.go`
- Create: `internal/storage/storage_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/storage/storage_test.go
package storage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	smtppkg "github.com/jonmilley/gofakesmtp/internal/smtp"
	"github.com/jonmilley/gofakesmtp/internal/storage"
)

func TestSave_WritesFileWithCorrectContent(t *testing.T) {
	dir := t.TempDir()
	email := smtppkg.Email{
		From:       "sender@example.com",
		To:         []string{"receiver@example.com"},
		Subject:    "Hello",
		Body:       "World",
		Raw:        []byte("raw email bytes"),
		ReceivedAt: time.Date(2026, 3, 25, 14, 32, 0, 0, time.UTC),
	}

	err := storage.Save(email, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	filename := entries[0].Name()
	if !strings.HasSuffix(filename, ".eml") {
		t.Errorf("expected .eml extension, got %q", filename)
	}
	if !strings.Contains(filename, "2026-03-25T14-32-00Z") {
		t.Errorf("expected RFC3339 timestamp in filename, got %q", filename)
	}
	if !strings.Contains(filename, "sender@example.com") {
		t.Errorf("expected from address in filename, got %q", filename)
	}

	content, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "raw email bytes" {
		t.Errorf("expected raw bytes in file, got %q", string(content))
	}
}

func TestSave_CreatesDirectoryIfNotExists(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "subdir", "emails")
	email := smtppkg.Email{
		From:       "a@b.com",
		Raw:        []byte("data"),
		ReceivedAt: time.Now(),
	}

	err := storage.Save(email, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("expected dir to be created: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 file, got %d", len(entries))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/storage/... -v
```

Expected: compilation error — `storage` package doesn't exist yet.

- [ ] **Step 3: Implement `storage.Save`**

```go
// internal/storage/storage.go
package storage

import (
	"fmt"
	"os"
	"strings"

	smtppkg "github.com/jonmilley/gofakesmtp/internal/smtp"
)

// Save writes the raw bytes of email to a .eml file in dir.
// The filename format is: <RFC3339-timestamp>-<from>.eml
// with colons replaced by hyphens for filesystem compatibility.
// dir is created if it does not exist.
func Save(email smtppkg.Email, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	ts := strings.ReplaceAll(email.ReceivedAt.UTC().Format("2006-01-02T15-04-05Z"), ":", "-")
	filename := fmt.Sprintf("%s-%s.eml", ts, email.From)
	path := fmt.Sprintf("%s/%s", dir, filename)

	if err := os.WriteFile(path, email.Raw, 0644); err != nil {
		return fmt.Errorf("write email file: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/storage/... -v
```

Expected:
```
--- PASS: TestSave_WritesFileWithCorrectContent
--- PASS: TestSave_CreatesDirectoryIfNotExists
PASS
```

- [ ] **Step 5: Commit**

```bash
git add internal/storage/storage.go internal/storage/storage_test.go
git commit -m "feat: add storage.Save for writing .eml files"
```

---

## Task 4: SMTP backend

**Files:**
- Create: `internal/smtp/backend.go`
- Create: `internal/smtp/backend_test.go`

- [ ] **Step 1: Write the failing tests**

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/smtp/... -v
```

Expected: compilation error — `NewBackend` not defined yet.

- [ ] **Step 3: Implement the backend**

```go
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

	s.ch <- email
	return nil
}

// Reset clears the session envelope state between messages.
func (s *Session) Reset() {
	s.from = ""
	s.to = nil
}

// Logout is a no-op.
func (s *Session) Logout() error { return nil }
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/smtp/... -v
```

Expected:
```
--- PASS: TestBackend_ReceivesEmail
--- PASS: TestBackend_MultipleRecipients
PASS
```

- [ ] **Step 5: Commit**

```bash
git add internal/smtp/backend.go internal/smtp/backend_test.go
git commit -m "feat: add SMTP backend with channel-based email dispatch"
```

---

## Task 5: TUI styles

**Files:**
- Create: `internal/tui/styles.go`

- [ ] **Step 1: Create the styles file**

```go
// internal/tui/styles.go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Layout
	listPaneStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderRight(true).
			PaddingLeft(1).
			PaddingRight(1)

	previewPaneStyle = lipgloss.NewStyle().
				PaddingLeft(1).
				PaddingRight(1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			PaddingLeft(1).
			PaddingRight(1)

	// List items
	selectedItemStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("25")).
				Foreground(lipgloss.Color("117")).
				Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	// Preview
	headerLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Width(10)

	headerValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("117"))

	subjectStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("117")).
			Bold(true)

	bodyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	// Status bar indicators
	listeningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("76"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
)
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./internal/tui/...
```

Expected: no output, no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/styles.go
git commit -m "feat: add TUI lipgloss styles"
```

---

## Task 6: TUI model

**Files:**
- Create: `internal/tui/model.go`
- Create: `internal/tui/model_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/tui/model_test.go
package tui_test

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	smtppkg "github.com/jonmilley/gofakesmtp/internal/smtp"
	"github.com/jonmilley/gofakesmtp/internal/tui"
)

func makeEmail(from, subject string) smtppkg.Email {
	return smtppkg.Email{
		From:       from,
		To:         []string{"to@example.com"},
		Subject:    subject,
		Body:       "body text",
		Raw:        []byte("raw"),
		ReceivedAt: time.Now(),
	}
}

func TestModel_ReceivesEmail(t *testing.T) {
	ch := make(chan smtppkg.Email, 10)
	m := tui.New(ch, "", 80, 24)

	email := makeEmail("sender@example.com", "Hello")
	updatedModel, _ := m.Update(tui.EmailReceivedMsg{Email: email})
	updated := updatedModel.(tui.Model)

	if updated.EmailCount() != 1 {
		t.Errorf("expected 1 email, got %d", updated.EmailCount())
	}
}

func TestModel_NavigateDown(t *testing.T) {
	ch := make(chan smtppkg.Email, 10)
	m := tui.New(ch, "", 80, 24)

	m, _ = m.Update(tui.EmailReceivedMsg{Email: makeEmail("a@b.com", "First")}).(tui.Model)
	m, _ = m.Update(tui.EmailReceivedMsg{Email: makeEmail("c@d.com", "Second")}).(tui.Model)

	if m.SelectedIndex() != 0 {
		t.Errorf("expected initial selection at 0, got %d", m.SelectedIndex())
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}).(tui.Model)
	if m.SelectedIndex() != 1 {
		t.Errorf("expected selection at 1 after down, got %d", m.SelectedIndex())
	}
}

func TestModel_NavigateUpDoesNotGoNegative(t *testing.T) {
	ch := make(chan smtppkg.Email, 10)
	m := tui.New(ch, "", 80, 24)

	m, _ = m.Update(tui.EmailReceivedMsg{Email: makeEmail("a@b.com", "First")}).(tui.Model)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp}).(tui.Model)

	if m.SelectedIndex() != 0 {
		t.Errorf("expected selection to stay at 0, got %d", m.SelectedIndex())
	}
}

func TestModel_DeleteEmail(t *testing.T) {
	ch := make(chan smtppkg.Email, 10)
	m := tui.New(ch, "", 80, 24)

	m, _ = m.Update(tui.EmailReceivedMsg{Email: makeEmail("a@b.com", "First")}).(tui.Model)
	m, _ = m.Update(tui.EmailReceivedMsg{Email: makeEmail("c@d.com", "Second")}).(tui.Model)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")}).(tui.Model)

	if m.EmailCount() != 1 {
		t.Errorf("expected 1 email after delete, got %d", m.EmailCount())
	}
}

func TestModel_StorageError_ShowsInStatusBar(t *testing.T) {
	ch := make(chan smtppkg.Email, 10)
	m := tui.New(ch, "", 80, 24)

	m, _ = m.Update(tui.StorageErrMsg{Err: fmt.Errorf("disk full")}).(tui.Model)

	status := m.StatusMessage()
	if status == "" {
		t.Error("expected non-empty status message after storage error")
	}
}
```

Note: add `"fmt"` to imports in the test file.

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/tui/... -v
```

Expected: compilation error — `tui.New`, `tui.EmailReceivedMsg`, `tui.Model` not defined yet.

- [ ] **Step 3: Implement the TUI model**

```go
// internal/tui/model.go
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	smtppkg "github.com/jonmilley/gofakesmtp/internal/smtp"
	"github.com/jonmilley/gofakesmtp/internal/storage"
)

// EmailReceivedMsg is sent when the SMTP backend delivers a new email.
type EmailReceivedMsg struct {
	Email smtppkg.Email
}

// StorageErrMsg is sent when saving an email to disk fails.
type StorageErrMsg struct {
	Err error
}

// Model is the Bubbletea model for gofakesmtp.
type Model struct {
	emails        []smtppkg.Email
	selected      int
	ch            chan smtppkg.Email
	outputDir     string
	statusMessage string
	width         int
	height        int
}

// New creates a new Model. outputDir is empty when no --output-dir flag is set.
func New(ch chan smtppkg.Email, outputDir string, width, height int) Model {
	return Model{
		ch:        ch,
		outputDir: outputDir,
		width:     width,
		height:    height,
	}
}

// EmailCount returns the number of emails currently in the model.
func (m Model) EmailCount() int { return len(m.emails) }

// SelectedIndex returns the index of the currently selected email.
func (m Model) SelectedIndex() int { return m.selected }

// StatusMessage returns the current status bar message (e.g. error text).
func (m Model) StatusMessage() string { return m.statusMessage }

// waitForEmail blocks on the channel and returns the email as a tea.Msg.
func waitForEmail(ch chan smtppkg.Email) tea.Cmd {
	return func() tea.Msg {
		return EmailReceivedMsg{Email: <-ch}
	}
}

// Init starts listening for emails.
func (m Model) Init() tea.Cmd {
	return waitForEmail(m.ch)
}

// Update handles all incoming messages and key events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case EmailReceivedMsg:
		m.emails = append(m.emails, msg.Email)
		m.statusMessage = ""
		if m.outputDir != "" {
			if err := storage.Save(msg.Email, m.outputDir); err != nil {
				m.statusMessage = fmt.Sprintf("save error: %v", err)
			}
		}
		return m, waitForEmail(m.ch)

	case StorageErrMsg:
		m.statusMessage = fmt.Sprintf("save error: %v", msg.Err)

	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyCtrlC || msg.String() == "q":
			return m, tea.Quit

		case msg.Type == tea.KeyDown || msg.String() == "j":
			if m.selected < len(m.emails)-1 {
				m.selected++
			}

		case msg.Type == tea.KeyUp || msg.String() == "k":
			if m.selected > 0 {
				m.selected--
			}

		case msg.String() == "d":
			if len(m.emails) > 0 {
				m.emails = append(m.emails[:m.selected], m.emails[m.selected+1:]...)
				if m.selected >= len(m.emails) && m.selected > 0 {
					m.selected--
				}
			}
		}
	}

	return m, nil
}

// View renders the full TUI: left list pane + right preview pane + status bar.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	statusBarHeight := 1
	contentHeight := m.height - statusBarHeight

	listWidth := m.width / 3
	previewWidth := m.width - listWidth - 1 // -1 for border

	listContent := m.renderList(listWidth, contentHeight)
	previewContent := m.renderPreview(previewWidth, contentHeight)

	leftPane := listPaneStyle.Width(listWidth).Height(contentHeight).Render(listContent)
	rightPane := previewPaneStyle.Width(previewWidth).Height(contentHeight).Render(previewContent)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, body, statusBar)
}

func (m Model) renderList(width, height int) string {
	if len(m.emails) == 0 {
		return dimStyle.Render("Waiting for emails...")
	}

	var rows []string
	for i, email := range m.emails {
		from := truncate(email.From, width-14)
		subject := truncate(email.Subject, width-14)
		ts := email.ReceivedAt.Format("15:04:05")
		line := fmt.Sprintf("%-*s  %s\n%s", width-14, from, ts, subject)

		if i == m.selected {
			rows = append(rows, selectedItemStyle.Render(line))
		} else {
			rows = append(rows, normalItemStyle.Render(line))
		}
	}

	return strings.Join(rows, "\n")
}

func (m Model) renderPreview(width, height int) string {
	if len(m.emails) == 0 || m.selected >= len(m.emails) {
		return dimStyle.Render("No email selected.")
	}

	email := m.emails[m.selected]
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		headerLabelStyle.Render("Subject:"),
		subjectStyle.Render(email.Subject),
	) + "\n" +
		lipgloss.JoinHorizontal(lipgloss.Top,
			headerLabelStyle.Render("From:"),
			headerValueStyle.Render(email.From),
		) + "\n" +
		lipgloss.JoinHorizontal(lipgloss.Top,
			headerLabelStyle.Render("To:"),
			headerValueStyle.Render(strings.Join(email.To, ", ")),
		) + "\n\n"

	body := bodyStyle.Width(width - 2).Render(email.Body)
	return header + body
}

func (m Model) renderStatusBar() string {
	count := fmt.Sprintf("%d email(s)", len(m.emails))
	status := listeningStyle.Render("● listening")

	var msg string
	if m.statusMessage != "" {
		msg = "  " + errorStyle.Render(m.statusMessage)
	}

	keys := dimStyle.Render("↑↓/jk navigate  d delete  q quit")

	right := lipgloss.NewStyle().Width(m.width - lipgloss.Width(keys) - 2).Align(lipgloss.Right).
		Render(fmt.Sprintf("%s  %s%s", count, status, msg))

	return statusBarStyle.Width(m.width).Render(lipgloss.JoinHorizontal(lipgloss.Top, keys, right))
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/tui/... -v
```

Expected:
```
--- PASS: TestModel_ReceivesEmail
--- PASS: TestModel_NavigateDown
--- PASS: TestModel_NavigateUpDoesNotGoNegative
--- PASS: TestModel_DeleteEmail
--- PASS: TestModel_StorageError_ShowsInStatusBar
PASS
```

- [ ] **Step 6: Commit**

```bash
git add internal/tui/model.go internal/tui/model_test.go internal/tui/styles.go
git commit -m "feat: add Bubbletea TUI model with email list and preview"
```

---

## Task 7: Wire it all together in `main.go`

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Replace the stub `main.go` with the full Cobra CLI**

```go
package main

import (
	"crypto/tls"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	gosmtp "github.com/emersion/go-smtp"
	"github.com/spf13/cobra"

	oursmtp "github.com/jonmilley/gofakesmtp/internal/smtp"
	"github.com/jonmilley/gofakesmtp/internal/tui"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	var (
		port      int
		addr      string
		outputDir string
		tlsCert   string
		tlsKey    string
	)

	cmd := &cobra.Command{
		Use:   "gofakesmtp",
		Short: "A fake SMTP server with a terminal UI for testing email",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate TLS flags
			if (tlsCert == "") != (tlsKey == "") {
				return fmt.Errorf("--tls-cert and --tls-key must both be provided to enable STARTTLS")
			}

			ch := make(chan oursmtp.Email, 100)

			// Configure and start SMTP server
			backend := oursmtp.NewBackend(ch)
			smtpServer := gosmtp.NewServer(backend)
			smtpServer.Addr = fmt.Sprintf("%s:%d", addr, port)
			smtpServer.Domain = "localhost"
			smtpServer.AllowInsecureAuth = true

			if tlsCert != "" {
				cert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
				if err != nil {
					return fmt.Errorf("load TLS key pair: %w", err)
				}
				smtpServer.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
			}

			go func() {
				if err := smtpServer.ListenAndServe(); err != nil {
					// Server closed on quit — not an error worth logging.
				}
			}()

			// Start TUI
			model := tui.New(ch, outputDir, 0, 0)
			p := tea.NewProgram(model, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("TUI error: %w", err)
			}

			smtpServer.Close()
			return nil
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 2525, "SMTP port to listen on")
	cmd.Flags().StringVarP(&addr, "addr", "a", "127.0.0.1", "Address to bind to")
	cmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "Directory to auto-save received emails as .eml files")
	cmd.Flags().StringVar(&tlsCert, "tls-cert", "", "Path to TLS certificate file (enables STARTTLS)")
	cmd.Flags().StringVar(&tlsKey, "tls-key", "", "Path to TLS private key file (enables STARTTLS)")

	return cmd
}
```

- [ ] **Step 2: Build the binary**

```bash
go build -o gofakesmtp .
```

Expected: `gofakesmtp` binary created, no errors.

- [ ] **Step 3: Run all tests**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 4: Quick smoke test**

In one terminal:
```bash
./gofakesmtp
```

In another terminal:
```bash
go run - <<'EOF'
package main

import (
	"net/smtp"
	"strings"
)

func main() {
	msg := strings.Join([]string{
		"From: test@example.com",
		"To: you@example.com",
		"Subject: Smoke test",
		"",
		"Hello from the smoke test!",
	}, "\r\n")
	smtp.SendMail("127.0.0.1:2525", nil, "test@example.com", []string{"you@example.com"}, []byte(msg))
}
EOF
```

Expected: The email appears in the TUI left pane. Select it with arrow keys; preview shows subject, from, to, and body on the right.

- [ ] **Step 5: Commit**

```bash
git add main.go
git commit -m "feat: wire CLI, SMTP server, and TUI in main.go"
```

---

## Task 8: Add `.gitignore` and clean up

**Files:**
- Create: `.gitignore`

- [ ] **Step 1: Create `.gitignore`**

```
gofakesmtp
.superpowers/
*.eml
```

- [ ] **Step 2: Commit**

```bash
git add .gitignore
git commit -m "chore: add .gitignore"
```
