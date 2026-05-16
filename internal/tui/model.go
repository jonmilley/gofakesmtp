// internal/tui/model.go
package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

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

// logLoadedMsg carries session log content read from disk.
type logLoadedMsg struct {
	lines []string
	err   error
}

// logTickMsg fires periodically while the log view is open.
type logTickMsg struct{}

// viewMode selects between the email list and the session log viewer.
type viewMode int

const (
	viewEmails viewMode = iota
	viewLog
)

const logRefreshInterval = time.Second

// Model is the Bubbletea model for gofakesmtp.
type Model struct {
	emails        []smtppkg.Email
	selected      int
	ch            chan smtppkg.Email
	outputDir     string
	sessionLog    string
	statusMessage string
	width         int
	height        int

	mode      viewMode
	logLines  []string
	logScroll int
	logErr    error
	// followTail keeps the log viewport pinned to the bottom across refreshes
	// until the user scrolls up.
	followTail bool
}

// New creates a new Model. outputDir and sessionLog are empty when the
// corresponding flags were not set.
func New(ch chan smtppkg.Email, outputDir, sessionLog string, width, height int) Model {
	return Model{
		ch:         ch,
		outputDir:  outputDir,
		sessionLog: sessionLog,
		width:      width,
		height:     height,
	}
}

// EmailCount returns the number of emails currently in the model.
func (m Model) EmailCount() int { return len(m.emails) }

// SelectedIndex returns the index of the currently selected email.
func (m Model) SelectedIndex() int { return m.selected }

// StatusMessage returns the current status bar message (e.g. error text).
func (m Model) StatusMessage() string { return m.statusMessage }

// IsLogView reports whether the model is showing the session log viewer.
func (m Model) IsLogView() bool { return m.mode == viewLog }

// LogLineCount returns the number of session log lines currently loaded.
func (m Model) LogLineCount() int { return len(m.logLines) }

// LogScroll returns the current scroll offset (0-indexed line) of the log viewer.
func (m Model) LogScroll() int { return m.logScroll }

// waitForEmail blocks on the channel and returns the email as a tea.Msg.
func waitForEmail(ch chan smtppkg.Email) tea.Cmd {
	return func() tea.Msg {
		return EmailReceivedMsg{Email: <-ch}
	}
}

func loadLog(path string) tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(path)
		if err != nil {
			return logLoadedMsg{err: err}
		}
		trimmed := strings.TrimRight(string(data), "\n")
		if trimmed == "" {
			return logLoadedMsg{lines: nil}
		}
		return logLoadedMsg{lines: strings.Split(trimmed, "\n")}
	}
}

func tickLog() tea.Cmd {
	return tea.Tick(logRefreshInterval, func(time.Time) tea.Msg { return logTickMsg{} })
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

	case logLoadedMsg:
		m.logLines = msg.lines
		m.logErr = msg.err
		if m.followTail {
			m.logScroll = m.maxLogScroll()
		}

	case logTickMsg:
		if m.mode != viewLog || m.sessionLog == "" {
			return m, nil
		}
		return m, tea.Batch(loadLog(m.sessionLog), tickLog())

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		if m.mode == viewLog {
			return m.updateLogView(msg)
		}
		return m.updateEmailView(msg)
	}

	return m, nil
}

func (m Model) updateEmailView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.String() == "q":
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

	case msg.String() == "l":
		if m.sessionLog == "" {
			m.statusMessage = "no session log configured (--session-log)"
			return m, nil
		}
		m.mode = viewLog
		m.followTail = true
		m.statusMessage = ""
		return m, tea.Batch(loadLog(m.sessionLog), tickLog())
	}

	return m, nil
}

func (m Model) updateLogView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.String() == "q":
		return m, tea.Quit

	case msg.Type == tea.KeyEsc, msg.String() == "l":
		m.mode = viewEmails
		return m, nil

	case msg.Type == tea.KeyDown || msg.String() == "j":
		if m.logScroll < m.maxLogScroll() {
			m.logScroll++
		}
		m.followTail = m.logScroll >= m.maxLogScroll()

	case msg.Type == tea.KeyUp || msg.String() == "k":
		if m.logScroll > 0 {
			m.logScroll--
		}
		m.followTail = false

	case msg.Type == tea.KeyPgDown, msg.String() == "f":
		m.logScroll += m.logViewportHeight()
		if m.logScroll > m.maxLogScroll() {
			m.logScroll = m.maxLogScroll()
		}
		m.followTail = m.logScroll >= m.maxLogScroll()

	case msg.Type == tea.KeyPgUp, msg.String() == "b":
		m.logScroll -= m.logViewportHeight()
		if m.logScroll < 0 {
			m.logScroll = 0
		}
		m.followTail = false

	case msg.String() == "g":
		m.logScroll = 0
		m.followTail = false

	case msg.String() == "G":
		m.logScroll = m.maxLogScroll()
		m.followTail = true

	case msg.String() == "r":
		m.followTail = true
		return m, loadLog(m.sessionLog)
	}

	return m, nil
}

// View renders the full TUI: left list pane + right preview pane + status bar,
// or the session log viewer when active.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	if m.mode == viewLog {
		return m.viewLog()
	}
	return m.viewEmails()
}

func (m Model) viewEmails() string {
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

func (m Model) viewLog() string {
	viewportHeight := m.logViewportHeight()

	title := logTitleStyle.Width(m.width).Render(fmt.Sprintf(" Session log: %s ", m.sessionLog))

	var content string
	switch {
	case m.logErr != nil:
		content = errorStyle.Render(fmt.Sprintf("error reading log: %v", m.logErr))
	case len(m.logLines) == 0:
		content = dimStyle.Render("(empty)")
	default:
		end := m.logScroll + viewportHeight
		if end > len(m.logLines) {
			end = len(m.logLines)
		}
		content = strings.Join(m.logLines[m.logScroll:end], "\n")
	}

	body := logPaneStyle.Width(m.width).Height(viewportHeight).Render(content)
	statusBar := m.renderLogStatusBar()
	return lipgloss.JoinVertical(lipgloss.Left, title, body, statusBar)
}

func (m Model) logViewportHeight() int {
	h := m.height - 2 // title + status bar
	if h < 1 {
		return 1
	}
	return h
}

func (m Model) maxLogScroll() int {
	max := len(m.logLines) - m.logViewportHeight()
	if max < 0 {
		return 0
	}
	return max
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

	keyHint := "↑↓/jk navigate  d delete  q quit"
	if m.sessionLog != "" {
		keyHint = "↑↓/jk navigate  d delete  l log  q quit"
	}
	keys := dimStyle.Render(keyHint)

	right := statusBarRightStyle.Width(m.width - lipgloss.Width(keys) - 2).
		Render(fmt.Sprintf("%s  %s%s", count, status, msg))

	return statusBarStyle.Width(m.width).Render(lipgloss.JoinHorizontal(lipgloss.Top, keys, right))
}

func (m Model) renderLogStatusBar() string {
	pos := fmt.Sprintf("line %d/%d", m.logScroll+1, len(m.logLines))
	if len(m.logLines) == 0 {
		pos = "line 0/0"
	}
	mode := dimStyle.Render("paused")
	if m.followTail {
		mode = listeningStyle.Render("● tailing")
	}

	keys := dimStyle.Render("↑↓/jk scroll  PgUp/PgDn page  g/G top/bot  r refresh  l/esc back  q quit")
	right := statusBarRightStyle.Width(m.width - lipgloss.Width(keys) - 2).
		Render(fmt.Sprintf("%s  %s", pos, mode))

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
