// Package styles holds the Lip Gloss styles used by the UI.
package styles

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/jastbytes/gardenctl-tui/internal/gardener"
)

var (
	ColorGreen  = lipgloss.AdaptiveColor{Light: "#1a7f37", Dark: "#3fb950"}
	ColorYellow = lipgloss.AdaptiveColor{Light: "#9a6700", Dark: "#d29922"}
	ColorRed    = lipgloss.AdaptiveColor{Light: "#cf222e", Dark: "#f85149"}
	ColorGrey   = lipgloss.AdaptiveColor{Light: "#6e7781", Dark: "#8b949e"}
	ColorAccent = lipgloss.AdaptiveColor{Light: "#0969da", Dark: "#58a6ff"}
	ColorSelBg  = lipgloss.AdaptiveColor{Light: "#ddf4ff", Dark: "#1f3a5f"}

	App        = lipgloss.NewStyle().Padding(0, 1)
	Logo       = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	Crumb      = lipgloss.NewStyle().Foreground(ColorGrey)
	CrumbLast  = lipgloss.NewStyle().Bold(true)
	Header     = lipgloss.NewStyle().Bold(true).Foreground(ColorGrey)
	Cell       = lipgloss.NewStyle()
	Cursor     = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	Selected   = lipgloss.NewStyle().Bold(true).Background(ColorSelBg)
	Dim        = lipgloss.NewStyle().Foreground(ColorGrey)
	Error      = lipgloss.NewStyle().Foreground(ColorRed)
	Info       = lipgloss.NewStyle().Foreground(ColorGreen)
	Key        = lipgloss.NewStyle().Bold(true)
	Section    = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent).MarginTop(1)
	Label      = lipgloss.NewStyle().Foreground(ColorGrey).Width(14)
	Overlay    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorAccent).Padding(1, 2)
	MenuActive = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
)

// StatusColor maps a status to its color: green healthy, yellow progressing,
// red error, grey hibernated/unknown.
func StatusColor(s gardener.Status) lipgloss.TerminalColor {
	switch s {
	case gardener.StatusHealthy:
		return ColorGreen
	case gardener.StatusProgressing:
		return ColorYellow
	case gardener.StatusError:
		return ColorRed
	default:
		return ColorGrey
	}
}

// ConditionColor maps a condition status string to a color.
func ConditionColor(status string) lipgloss.TerminalColor {
	switch status {
	case "True":
		return ColorGreen
	case "False":
		return ColorRed
	default:
		return ColorYellow
	}
}

// StatusText renders "● Healthy" in the status color.
func StatusText(s gardener.Status) string {
	return "● " + s.String()
}
