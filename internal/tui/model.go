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

	listContent := m.renderList(listWidth)
	previewContent := m.renderPreview(previewWidth)

	leftPane := listPaneStyle.Width(listWidth).Height(contentHeight).Render(listContent)
	rightPane := previewPaneStyle.Width(previewWidth).Height(contentHeight).Render(previewContent)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, body, statusBar)
}

func (m Model) renderList(width int) string {
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

func (m Model) renderPreview(width int) string {
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

	right := statusBarRightStyle.Width(m.width - lipgloss.Width(keys) - 2).
		Render(fmt.Sprintf("%s  %s%s", count, status, msg))

	return statusBarStyle.Width(m.width).Render(lipgloss.JoinHorizontal(lipgloss.Top, keys, right))
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}
