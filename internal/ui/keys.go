package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/jastbytes/gardenctl-tui/internal/ui/styles"
)

// helpGroup is a titled column of the help overlay.
type helpGroup struct {
	title string
	keys  []key.Binding
}

func binding(keys, desc string) key.Binding {
	return key.NewBinding(key.WithKeys(strings.Split(keys, "/")...), key.WithHelp(keys, desc))
}

func (a *App) helpGroups() []helpGroup {
	nav := []key.Binding{
		binding("↑/↓", "move (also j/k)"),
		binding("enter", "open / details"),
		binding("esc/h", "back"),
		binding("/", "filter (fuzzy)"),
		binding("tab", "seeds of the garden"),
		binding("g/G", "first / last"),
	}
	actions := []key.Binding{
		binding("l", "launcher menu"),
		binding("*", "toggle favorite"),
		binding("ctrl+r", "recent & favorites"),
		binding("p", "toggle provider-env"),
		binding("w", "toggle new window"),
		binding("r", "refresh"),
		binding("a", "toggle auto-refresh"),
		binding("?", "help"),
		binding("q", "quit"),
	}
	var tools []key.Binding
	for _, t := range a.opts.Config.Tools {
		tools = append(tools, binding(t.Key, t.Name))
		if t.ControlPlaneKey != "" {
			tools = append(tools, binding(t.ControlPlaneKey, t.Name+" (control plane)"))
		}
	}
	return []helpGroup{{"Navigation", nav}, {"Actions", actions}, {"Tools", tools}}
}

func (a *App) renderHelp(width, height int) string {
	h := help.New()
	h.ShowAll = true
	h.FullSeparator = "    "
	h.Styles.FullKey = styles.Key
	h.Styles.FullDesc = styles.Dim
	h.Styles.FullSeparator = styles.Dim
	var cols []string
	for _, g := range a.helpGroups() {
		cols = append(cols, lipgloss.JoinVertical(lipgloss.Left,
			styles.CrumbLast.Render(g.title), h.FullHelpView([][]key.Binding{g.keys})))
	}
	groups := lipgloss.JoinHorizontal(lipgloss.Top, intersperse(cols, "    ")...)
	if lipgloss.Width(groups) > width-8 {
		groups = lipgloss.JoinVertical(lipgloss.Left, intersperse(cols, "")...)
	}
	body := lipgloss.JoinVertical(lipgloss.Left,
		groups,
		"",
		styles.Dim.Render("Tool keys work on shoots, seeds, details and recent targets;"),
		styles.Dim.Render("in those views they win over j/k. Tools: ~/.garden/gtui.yaml"),
		styles.Dim.Render("Press any key to close."),
	)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, styles.Overlay.Render(body))
}

func intersperse(items []string, sep string) []string {
	out := make([]string, 0, 2*len(items))
	for i, it := range items {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, it)
	}
	return out
}
