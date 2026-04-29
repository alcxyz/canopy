package ui

import "github.com/charmbracelet/lipgloss"

// Catppuccin Mocha palette
const (
	catMauve    = "#cba6f7"
	catRed      = "#f38ba8"
	catYellow   = "#f9e2af"
	catGreen    = "#a6e3a1"
	catBlue     = "#89b4fa"
	catText     = "#cdd6f4"
	catOverlay2 = "#9399b2"
	catOverlay0 = "#6c7086"
	catSurface2 = "#585b70"
)

var (
	TabStyle = lipgloss.NewStyle().Padding(0, 2)

	ActiveTabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Bold(true).
			Foreground(lipgloss.Color(catMauve)).
			Underline(true)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(catMauve)).
			PaddingLeft(1)

	DimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(catOverlay0))

	FilterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(catYellow))

	StatusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(catOverlay2))

	CountStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(catBlue))

	TypeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(catText))

	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(catSurface2)).
			Padding(1, 2)

	// Indicator styles
	OverdueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(catRed))
	DueSoonStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(catYellow))
	FreshStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(catGreen))
	RecentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(catBlue))
	AgingStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(catYellow))
	StaleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(catRed))

	// State colors (for use in stateColors map in view.go)
	StateTodoColor       = lipgloss.NewStyle().Foreground(lipgloss.Color(catSurface2))
	StateInProgressColor = lipgloss.NewStyle().Foreground(lipgloss.Color(catBlue))
	StateInReviewColor   = lipgloss.NewStyle().Foreground(lipgloss.Color(catYellow))
	StateDoneColor       = lipgloss.NewStyle().Foreground(lipgloss.Color(catGreen))
	StateClosedColor     = lipgloss.NewStyle().Foreground(lipgloss.Color(catOverlay0))
)
