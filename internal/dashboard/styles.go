package dashboard

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("57")).
			Padding(0, 1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39"))

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	activeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	inactiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	staleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	noGitStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("37"))

	highStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	medStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	lowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	costStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220"))

	autoPilotOnStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("42"))

	autoPilotOffStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))

	runningTagStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("51"))

	blockedTagStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("196"))

	plannedTagStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("33"))

	logBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("236")).
				Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	cursorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62"))

	menuStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("57")).
			Padding(1, 2)

	menuTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("57")).
				Padding(0, 1)

	menuOptionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("39"))

	runningDot = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Render("●")

	progressFull = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Render("█")

	progressEmpty = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("░")

	contextFullGreen = lipgloss.NewStyle().
				Foreground(lipgloss.Color("42")).
				Render("█")

	contextFullYellow = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")).
				Render("█")

	contextFullRed = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true).
			Render("█")
)

func progressBar(percent float64, width int) string {
	if percent > 100 {
		percent = 100
	}
	if percent < 0 {
		percent = 0
	}
	filled := int(float64(width) * percent / 100)
	empty := width - filled

	bar := ""
	for i := 0; i < filled; i++ {
		bar += progressFull
	}
	for i := 0; i < empty; i++ {
		bar += progressEmpty
	}
	return bar
}

func contextBar(percent float64, width int) string {
	if percent > 100 {
		percent = 100
	}
	if percent < 0 {
		percent = 0
	}
	filled := int(float64(width) * percent / 100)
	empty := width - filled

	var fillChar string
	switch {
	case percent >= 80:
		fillChar = contextFullRed
	case percent >= 60:
		fillChar = contextFullYellow
	default:
		fillChar = contextFullGreen
	}

	bar := ""
	for i := 0; i < filled; i++ {
		bar += fillChar
	}
	for i := 0; i < empty; i++ {
		bar += progressEmpty
	}
	return bar
}
