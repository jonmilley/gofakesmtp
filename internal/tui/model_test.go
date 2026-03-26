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
	m := tui.New(ch, "", 80, 24)

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
	m := tui.New(ch, "", 80, 24)

	m = applyUpdate(t, m, tui.EmailReceivedMsg{Email: makeEmail("a@b.com", "First")})
	m = applyUpdate(t, m, tea.KeyMsg{Type: tea.KeyUp})

	if m.SelectedIndex() != 0 {
		t.Errorf("expected selection to stay at 0, got %d", m.SelectedIndex())
	}
}

func TestModel_DeleteEmail(t *testing.T) {
	ch := make(chan smtppkg.Email, 10)
	m := tui.New(ch, "", 80, 24)

	m = applyUpdate(t, m, tui.EmailReceivedMsg{Email: makeEmail("a@b.com", "First")})
	m = applyUpdate(t, m, tui.EmailReceivedMsg{Email: makeEmail("c@d.com", "Second")})
	m = applyUpdate(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})

	if m.EmailCount() != 1 {
		t.Errorf("expected 1 email after delete, got %d", m.EmailCount())
	}
}

func TestModel_StorageError_ShowsInStatusBar(t *testing.T) {
	ch := make(chan smtppkg.Email, 10)
	m := tui.New(ch, "", 80, 24)

	m = applyUpdate(t, m, tui.StorageErrMsg{Err: fmt.Errorf("disk full")})

	status := m.StatusMessage()
	if status == "" {
		t.Error("expected non-empty status message after storage error")
	}
}
