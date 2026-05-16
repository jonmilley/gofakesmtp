// internal/tui/model_test.go
package tui_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	m := tui.New(ch, "", "", 80, 24)

	email := makeEmail("sender@example.com", "Hello")
	updatedModel, _ := m.Update(tui.EmailReceivedMsg{Email: email})
	updated := updatedModel.(tui.Model)

	if updated.EmailCount() != 1 {
		t.Errorf("expected 1 email, got %d", updated.EmailCount())
	}
}

func applyUpdate(t *testing.T, m tui.Model, msg tea.Msg) tui.Model {
	t.Helper()
	result, _ := m.Update(msg)
	updated, ok := result.(tui.Model)
	if !ok {
		t.Fatalf("Update did not return a tui.Model")
	}
	return updated
}

func TestModel_NavigateDown(t *testing.T) {
	ch := make(chan smtppkg.Email, 10)
	m := tui.New(ch, "", "", 80, 24)

	m = applyUpdate(t, m, tui.EmailReceivedMsg{Email: makeEmail("a@b.com", "First")})
	m = applyUpdate(t, m, tui.EmailReceivedMsg{Email: makeEmail("c@d.com", "Second")})

	if m.SelectedIndex() != 0 {
		t.Errorf("expected initial selection at 0, got %d", m.SelectedIndex())
	}

	m = applyUpdate(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.SelectedIndex() != 1 {
		t.Errorf("expected selection at 1 after down, got %d", m.SelectedIndex())
	}
}

func TestModel_NavigateUpDoesNotGoNegative(t *testing.T) {
	ch := make(chan smtppkg.Email, 10)
	m := tui.New(ch, "", "", 80, 24)

	m = applyUpdate(t, m, tui.EmailReceivedMsg{Email: makeEmail("a@b.com", "First")})
	m = applyUpdate(t, m, tea.KeyMsg{Type: tea.KeyUp})

	if m.SelectedIndex() != 0 {
		t.Errorf("expected selection to stay at 0, got %d", m.SelectedIndex())
	}
}

func TestModel_DeleteEmail(t *testing.T) {
	ch := make(chan smtppkg.Email, 10)
	m := tui.New(ch, "", "", 80, 24)

	m = applyUpdate(t, m, tui.EmailReceivedMsg{Email: makeEmail("a@b.com", "First")})
	m = applyUpdate(t, m, tui.EmailReceivedMsg{Email: makeEmail("c@d.com", "Second")})
	m = applyUpdate(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})

	if m.EmailCount() != 1 {
		t.Errorf("expected 1 email after delete, got %d", m.EmailCount())
	}
}

func TestModel_DeleteLastEmail_ClampsSelection(t *testing.T) {
	ch := make(chan smtppkg.Email, 10)
	m := tui.New(ch, "", "", 80, 24)

	m = applyUpdate(t, m, tui.EmailReceivedMsg{Email: makeEmail("a@b.com", "First")})
	m = applyUpdate(t, m, tui.EmailReceivedMsg{Email: makeEmail("c@d.com", "Second")})

	// Navigate to last item
	m = applyUpdate(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.SelectedIndex() != 1 {
		t.Fatalf("expected selection at 1, got %d", m.SelectedIndex())
	}

	// Delete the last item, selection should clamp to 0
	m = applyUpdate(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if m.EmailCount() != 1 {
		t.Errorf("expected 1 email after delete, got %d", m.EmailCount())
	}
	if m.SelectedIndex() != 0 {
		t.Errorf("expected selection clamped to 0, got %d", m.SelectedIndex())
	}
}

func TestModel_StorageError_ShowsInStatusBar(t *testing.T) {
	ch := make(chan smtppkg.Email, 10)
	m := tui.New(ch, "", "", 80, 24)

	m = applyUpdate(t, m, tui.StorageErrMsg{Err: fmt.Errorf("disk full")})

	status := m.StatusMessage()
	if status == "" {
		t.Error("expected non-empty status message after storage error")
	}
}

func keyMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// runLoadCmd opens the log view and runs the synchronous file-load cmd that
// `l` returns, returning the resulting message ready for re-application to the
// model. It deliberately ignores the second cmd in the batch (the periodic
// tick), which would block on a timer.
func runLoadCmd(t *testing.T, batchCmd tea.Cmd) tea.Msg {
	t.Helper()
	if batchCmd == nil {
		t.Fatal("expected a batched load+tick command from opening log view, got nil")
	}
	msg := batchCmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", msg)
	}
	if len(batch) == 0 {
		t.Fatal("expected at least one cmd in batch")
	}
	// loadLog is the first cmd in the batch (see model.go).
	return batch[0]()
}

func TestModel_StatusBar_HidesLogHint_WhenSessionLogNotConfigured(t *testing.T) {
	ch := make(chan smtppkg.Email, 10)

	noLog := tui.New(ch, "", "", 80, 24)
	if strings.Contains(noLog.View(), "l log") {
		t.Errorf("expected status bar to omit 'l log' hint when no session log; view:\n%s", noLog.View())
	}

	withLog := tui.New(ch, "", "/tmp/whatever.log", 80, 24)
	if !strings.Contains(withLog.View(), "l log") {
		t.Errorf("expected status bar to include 'l log' hint when session log configured; view:\n%s", withLog.View())
	}
}

func TestModel_OpenLog_WithoutSessionLog_ShowsStatusMessage(t *testing.T) {
	ch := make(chan smtppkg.Email, 10)
	m := tui.New(ch, "", "", 80, 24)

	m = applyUpdate(t, m, keyMsg("l"))

	if m.IsLogView() {
		t.Error("expected to remain in email view when no session log is configured")
	}
	if m.StatusMessage() == "" {
		t.Error("expected a status message explaining session log is not configured")
	}
}

func TestModel_OpenLog_LoadsAndDisplaysFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.log")
	contents := "line one\nline two\nline three\n"
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	ch := make(chan smtppkg.Email, 10)
	m := tui.New(ch, "", path, 80, 24)

	result, cmd := m.Update(keyMsg("l"))
	m = result.(tui.Model)
	if !m.IsLogView() {
		t.Fatal("expected to enter log view after pressing l")
	}

	loaded := runLoadCmd(t, cmd)
	m = applyUpdate(t, m, loaded)

	if got, want := m.LogLineCount(), 3; got != want {
		t.Errorf("expected %d log lines, got %d", want, got)
	}
	if !strings.Contains(m.View(), "line three") {
		t.Errorf("expected log view to render log content; got:\n%s", m.View())
	}
}

func TestModel_LogView_EscReturnsToEmailView(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.log")
	if err := os.WriteFile(path, []byte("hi\n"), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	ch := make(chan smtppkg.Email, 10)
	m := tui.New(ch, "", path, 80, 24)
	m = applyUpdate(t, m, keyMsg("l"))
	if !m.IsLogView() {
		t.Fatal("expected to be in log view after l")
	}

	m = applyUpdate(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.IsLogView() {
		t.Error("expected esc to return to email view")
	}
}

func TestModel_LogView_ScrollAndJumpControls(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.log")
	var b strings.Builder
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	ch := make(chan smtppkg.Email, 10)
	m := tui.New(ch, "", path, 80, 24)

	result, cmd := m.Update(keyMsg("l"))
	m = result.(tui.Model)
	loaded := runLoadCmd(t, cmd)
	m = applyUpdate(t, m, loaded)

	// followTail snaps initial scroll to the bottom.
	if m.LogScroll() == 0 {
		t.Errorf("expected initial scroll to be pinned to bottom, got 0 (lines=%d)", m.LogLineCount())
	}

	// 'g' jumps to top.
	m = applyUpdate(t, m, keyMsg("g"))
	if m.LogScroll() != 0 {
		t.Errorf("expected g to jump to 0, got %d", m.LogScroll())
	}

	// 'j' scrolls down by 1.
	m = applyUpdate(t, m, keyMsg("j"))
	if m.LogScroll() != 1 {
		t.Errorf("expected j to scroll to 1, got %d", m.LogScroll())
	}

	// 'k' scrolls up.
	m = applyUpdate(t, m, keyMsg("k"))
	if m.LogScroll() != 0 {
		t.Errorf("expected k to scroll back to 0, got %d", m.LogScroll())
	}

	// 'G' jumps to bottom.
	// max scroll = lines - viewportHeight; viewport = height - 2 = 22; lines = 50 → 28.
	m = applyUpdate(t, m, keyMsg("G"))
	if got, want := m.LogScroll(), 50-22; got != want {
		t.Errorf("expected G to land at %d, got %d", want, got)
	}
}
