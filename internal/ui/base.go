// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jastbytes/gardenctl-tui/internal/ui/styles"
)

// listBase implements the shared parts of table-based views.
type listBase struct {
	app     *App
	vid     int
	loading bool
	loaded  bool
	err     error
	tbl     *table
}

func newListBase(a *App, cols ...column) listBase {
	return listBase{app: a, vid: a.newID(), tbl: newTable(cols...)}
}

func (b *listBase) id() int                { return b.vid }
func (b *listBase) capturing() bool        { return b.tbl.filtering() }
func (b *listBase) isLoading() bool        { return b.loading }
func (b *listBase) update(tea.Msg) tea.Cmd { return nil }

// startLoad marks the view as loading and returns cmd.
func (b *listBase) startLoad(cmd tea.Cmd) tea.Cmd {
	b.loading = true
	return cmd
}

// finishLoad records the result of a load; it returns false on error.
func (b *listBase) finishLoad(msg loadedMsg) bool {
	b.loading = false
	b.err = msg.err
	if msg.err != nil {
		return false
	}
	b.loaded = true
	return true
}

// tableKey handles filter and navigation keys.
func (b *listBase) tableKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	if !b.tbl.filtering() {
		switch msg.String() {
		case "/":
			return true, b.tbl.startFilter()
		case "esc":
			if b.tbl.filterActive() {
				b.tbl.clearFilter()
				return true, nil
			}
		}
	}
	return b.tbl.update(msg)
}

func (b *listBase) renderTable(width, height int) string {
	f := b.tbl.filterView()
	if b.err != nil && !b.loaded {
		return styles.Error.Render("Could not load: " + b.err.Error())
	}
	if !b.loaded && b.loading {
		return styles.Dim.Render("loading…")
	}
	if f == "" {
		return b.tbl.view(width, height)
	}
	body := lipgloss.NewStyle().Height(height - 1).Render(b.tbl.view(width, height-1))
	return lipgloss.JoinVertical(lipgloss.Left, body, f)
}

func baseHints() []hint {
	return []hint{{"enter", "open"}, {"/", "filter"}, {"esc", "back"}, {"r", "refresh"}}
}
