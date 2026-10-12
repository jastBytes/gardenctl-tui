package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jastbytes/gardenctl-tui/internal/session"
	"github.com/jastbytes/gardenctl-tui/internal/state"
	"github.com/jastbytes/gardenctl-tui/internal/ui/styles"
)

// recentView lists favorites and recently used targets.
type recentView struct{ listBase }

func newRecentView(a *App) *recentView {
	v := &recentView{newListBase(a, column{title: " "}, column{title: "Target"}, column{title: "Last Tool"}, column{title: "Used"})}
	v.rebuild()
	v.loaded = true
	return v
}

func (v *recentView) rebuild() {
	st := v.app.state
	var rows []row
	seen := map[string]bool{}
	for _, f := range st.Favorites() {
		seen[f.Key()] = true
		rows = append(rows, row{key: f.Key(), cells: []string{"★", f.Key(), "", ""},
			colors: []lipgloss.TerminalColor{styles.ColorYellow}, data: f})
	}
	for _, r := range st.Recent() {
		used := ""
		if !r.Used.IsZero() {
			used = ago(v.app.opts.Now(), r.Used)
		}
		if seen[r.Key()] {
			// favorite: fill in tool and time of last use
			for i := range rows {
				if rows[i].key == r.Key() {
					rows[i].cells[2], rows[i].cells[3] = r.Tool, used
				}
			}
			continue
		}
		rows = append(rows, row{key: r.Key(), cells: []string{"", r.Key(), r.Tool, used}, data: r})
	}
	v.tbl.setRows(rows)
}

func (v *recentView) title() string    { return "recent" }
func (v *recentView) init() tea.Cmd    { return nil }
func (v *recentView) refresh() tea.Cmd { v.rebuild(); return nil }
func (v *recentView) hints() []hint {
	return append(toolHints(v.app), hint{"enter", "launch…"}, hint{"*", "favorite"}, hint{"esc", "back"})
}

func (v *recentView) selectedTarget() (session.Target, bool) {
	r, ok := v.tbl.selected()
	if !ok {
		return session.Target{}, false
	}
	return r.data.(state.Entry).Target(), true
}

func (v *recentView) key(msg tea.KeyMsg) (bool, tea.Cmd) {
	if t, ok := v.selectedTarget(); ok && !v.tbl.filtering() {
		if handled, cmd := v.app.launchKey(msg.String(), t); handled {
			return true, cmd
		}
		switch msg.String() {
		case "*":
			cmd := v.app.toggleFavorite(t)
			v.rebuild()
			return true, cmd
		case "enter", "l":
			return true, push(newMenuView(v.app, t))
		}
	}
	return v.tableKey(msg)
}

func (v *recentView) render(width, height int) string {
	if len(v.tbl.all) == 0 {
		return styles.Dim.Render("No recent targets yet. Launch a tool on a shoot, or mark one with * as favorite.")
	}
	return v.renderTable(width, height)
}
