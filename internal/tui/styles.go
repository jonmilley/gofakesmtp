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
