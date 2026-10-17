// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

// Package styles holds the Lip Gloss styles used by the UI.
package styles

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/jastbytes/gardenctl-tui/internal/gardener"
)

// The theme's colors; Apply sets them.
var (
	ColorGreen  lipgloss.TerminalColor
	ColorYellow lipgloss.TerminalColor
	ColorRed    lipgloss.TerminalColor
	ColorGrey   lipgloss.TerminalColor
	ColorAccent lipgloss.TerminalColor
	ColorSelBg  lipgloss.TerminalColor
)

// The styles built from the colors; Apply rebuilds them.
var (
	App        lipgloss.Style
	Logo       lipgloss.Style
	Crumb      lipgloss.Style
	CrumbLast  lipgloss.Style
	Header     lipgloss.Style
	Cell       lipgloss.Style
	Cursor     lipgloss.Style
	Selected   lipgloss.Style
	Dim        lipgloss.Style
	Error      lipgloss.Style
	Info       lipgloss.Style
	Key        lipgloss.Style
	Section    lipgloss.Style
	Label      lipgloss.Style
	Overlay    lipgloss.Style
	MenuActive lipgloss.Style
	YAMLKey    lipgloss.Style
	Changed    lipgloss.Style
	Warn       lipgloss.Style
)

func init() {
	setTheme(Themes[DefaultTheme])
}

// setTheme sets the colors of t and rebuilds the styles.
func setTheme(t Theme) {
	ColorGreen, ColorYellow, ColorRed = t.Green, t.Yellow, t.Red
	ColorGrey, ColorAccent, ColorSelBg = t.Grey, t.Accent, t.SelBg

	App = lipgloss.NewStyle().Padding(0, 1)
	Logo = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	Crumb = lipgloss.NewStyle().Foreground(ColorGrey)
	CrumbLast = lipgloss.NewStyle().Bold(true)
	Header = lipgloss.NewStyle().Bold(true).Foreground(ColorGrey)
	Cell = lipgloss.NewStyle()
	Cursor = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	Selected = lipgloss.NewStyle().Bold(true).Background(ColorSelBg)
	Dim = lipgloss.NewStyle().Foreground(ColorGrey)
	Error = lipgloss.NewStyle().Foreground(ColorRed)
	Info = lipgloss.NewStyle().Foreground(ColorGreen)
	Key = lipgloss.NewStyle().Bold(true)
	Section = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent).MarginTop(1)
	Label = lipgloss.NewStyle().Foreground(ColorGrey).Width(14)
	Overlay = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorAccent).Padding(1, 2)
	MenuActive = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	YAMLKey = lipgloss.NewStyle().Foreground(ColorAccent)
	Changed = lipgloss.NewStyle().Bold(true).Foreground(ColorYellow)
	Warn = lipgloss.NewStyle().Foreground(ColorYellow)

	// Without colors (https://no-color.org, or the mono theme) the selection
	// has no background; reverse video keeps it visible.
	if termenv.EnvNoColor() || t.SelBg == (lipgloss.NoColor{}) {
		Selected = Selected.Reverse(true)
	}
}

// StatusSymbol tells the statuses apart without colors. All symbols are one
// cell wide in every terminal (no emoji, no East Asian ambiguous width).
func StatusSymbol(s gardener.Status) string {
	switch s {
	case gardener.StatusHealthy:
		return "✓"
	case gardener.StatusProgressing:
		return "⟳"
	case gardener.StatusError:
		return "✗"
	case gardener.StatusHibernated:
		return "☾"
	default:
		return "?"
	}
}

// ConditionSymbol is StatusSymbol for a condition status string.
func ConditionSymbol(status string) string {
	switch status {
	case "True":
		return "✓"
	case "False":
		return "✗"
	case "Progressing":
		return "⟳"
	default:
		return "?"
	}
}

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

// StatusText renders e.g. "✓ Healthy" (callers color it with StatusColor).
func StatusText(s gardener.Status) string {
	return StatusSymbol(s) + " " + s.String()
}
