package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/session"
	"github.com/jastbytes/gardenctl-tui/internal/ui/styles"
)

type menuItem struct {
	tool         config.Tool
	controlPlane bool
	key          string
}

// menuView lets the user pick a tool to launch against a target.
type menuView struct {
	listBase
	target session.Target
}

func newMenuView(a *App, t session.Target) *menuView {
	v := &menuView{
		listBase: newListBase(a, column{title: "Key"}, column{title: "Tool"}, column{title: "Target"}, column{title: "Command", max: 50}),
		target:   t,
	}
	var rows []row
	for _, tool := range a.opts.Config.Tools {
		avail := a.available(tool)
		variants := []menuItem{{tool: tool, key: tool.Key}}
		if t.Shoot != "" && tool.ControlPlaneKey != "" {
			variants = append(variants, menuItem{tool: tool, key: tool.ControlPlaneKey, controlPlane: true})
		}
		for _, it := range variants {
			where := "shoot"
			if t.Shoot == "" {
				where = "seed"
			}
			if it.controlPlane {
				where = "control plane"
			}
			cmd := strings.Join(tool.Command, " ")
			if !avail {
				cmd = "not found in PATH"
			}
			var colors []lipgloss.TerminalColor
			if !avail {
				colors = []lipgloss.TerminalColor{styles.ColorGrey, styles.ColorGrey, styles.ColorGrey, styles.ColorGrey}
			}
			rows = append(rows, row{key: it.key, cells: []string{it.key, tool.Name, where, cmd}, colors: colors, data: it})
		}
	}
	v.tbl.setRows(rows)
	v.loaded = true
	return v
}

func (v *menuView) title() string    { return "launch" }
func (v *menuView) init() tea.Cmd    { return nil }
func (v *menuView) refresh() tea.Cmd { return nil }
func (v *menuView) hints() []hint {
	return []hint{{"enter", "launch"}, {"p", "provider-env"}, {"esc", "back"}}
}

func (v *menuView) key(msg tea.KeyMsg) (bool, tea.Cmd) {
	k := msg.String()
	if !v.tbl.filtering() {
		if tool, cp, ok := v.app.toolForKey(k, v.target.Shoot != ""); ok {
			return true, v.launch(menuItem{tool: tool, controlPlane: cp})
		}
	}
	if handled, cmd := v.tableKey(msg); handled {
		return true, cmd
	}
	if k == "enter" {
		if r, ok := v.tbl.selected(); ok {
			return true, v.launch(r.data.(menuItem))
		}
	}
	return false, nil
}

func (v *menuView) launch(it menuItem) tea.Cmd {
	t := v.target
	t.ControlPlane = it.controlPlane
	return tea.Sequence(pop, v.app.launch(it.tool, t))
}

func (v *menuView) render(width, height int) string {
	head := styles.Dim.Render("Launch a tool on ") + styles.CrumbLast.Render(v.target.String()) + "\n\n"
	return head + v.renderTable(width, height-2)
}
