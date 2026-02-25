package ui

import "github.com/charmbracelet/lipgloss"

var (
	// focusedBorder wraps a focused pane in a bright rounded border.
	focusedBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))

	// blurredBorder wraps an unfocused pane in a dim rounded border.
	blurredBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	// titleStyle renders pane title bars.
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	// statusStyle renders the status/help bar at the bottom.
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1)

	// errorStyle renders error messages in red.
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	// runningStyle colours "running" status green.
	runningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	// exitedStyle colours "exited" status red.
	exitedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	// pausedStyle colours "paused" status yellow.
	pausedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	// createdStyle colours "created" status blue.
	createdStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
)
