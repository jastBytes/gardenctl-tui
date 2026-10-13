package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/gardener"
	"github.com/jastbytes/gardenctl-tui/internal/state"
)

// View kinds stored in the navigation path (rememberLocation).
const (
	kindGardens  = "gardens"
	kindProjects = "projects"
	kindSeeds    = "seeds"
	kindShoots   = "shoots"
	kindAll      = "all-shoots"
	kindDetails  = "details"
)

// locatable views can be part of a remembered location.
type locatable interface {
	crumb() state.Crumb
}

// Location returns the navigation path of the stack. It stops at the first
// view that cannot be restored (e.g. the launcher menu or the recent list).
func (a *App) Location() []state.Crumb {
	var loc []state.Crumb
	for _, v := range a.stack {
		l, ok := v.(locatable)
		if !ok {
			break
		}
		loc = append(loc, l.crumb())
	}
	return loc
}

// SaveLocation stores the current location if rememberLocation is enabled.
func (a *App) SaveLocation() error {
	if !a.opts.Config.RememberLocation {
		return nil
	}
	return a.state.SetLocation(a.Location())
}

// startRestore schedules the saved location for restoration, starting at the gardens view.
func (a *App) startRestore() {
	if !a.opts.Config.RememberLocation {
		return
	}
	loc := a.state.Location()
	if len(loc) == 0 || loc[0].View != kindGardens {
		return
	}
	a.restore = loc
	a.restoreView = a.stack[0].id()
}

// stopRestore cancels a running restoration (e.g. when the user takes over).
func (a *App) stopRestore() {
	a.restore = nil
	a.restoreView = 0
}

// applyRestore selects the remembered row in the view with id vid once its
// rows are loaded, and opens the next remembered view via open.
func (a *App) applyRestore(vid int, t *table, open func(kind string) tea.Cmd) tea.Cmd {
	if len(a.restore) == 0 || a.restoreView != vid {
		return nil
	}
	c := a.restore[0]
	a.restore = a.restore[1:]
	a.restoreView = 0
	if !t.selectKey(c.Selected) || len(a.restore) == 0 {
		a.stopRestore()
		return nil
	}
	// The next pushMsg binds the remaining path to the opened view.
	cmd := open(a.restore[0].View)
	if cmd == nil {
		a.stopRestore()
	}
	return cmd
}

// restorePushed binds the remaining path to a freshly pushed view.
func (a *App) restorePushed(v view) {
	if len(a.restore) == 0 {
		return
	}
	if _, ok := v.(locatable); !ok || a.restore[0].Selected == "" {
		a.stopRestore() // e.g. the details view: nothing left to select
		return
	}
	a.restoreView = v.id()
}

func selectedKey(t *table) string {
	if r, ok := t.selected(); ok {
		return r.key
	}
	return ""
}

func (v *gardensView) crumb() state.Crumb {
	return state.Crumb{View: kindGardens, Selected: selectedKey(v.tbl)}
}

func (v *gardensView) open(kind string) tea.Cmd {
	r, ok := v.tbl.selected()
	if !ok {
		return nil
	}
	switch kind {
	case kindProjects:
		return push(newProjectsView(v.app, r.data.(config.Garden)))
	case kindSeeds:
		return push(newSeedsView(v.app, r.data.(config.Garden)))
	}
	return nil
}

func (v *projectsView) crumb() state.Crumb {
	return state.Crumb{View: kindProjects, Selected: selectedKey(v.tbl)}
}

func (v *projectsView) open(kind string) tea.Cmd {
	switch kind {
	case kindSeeds:
		return push(newSeedsView(v.app, v.garden))
	case kindAll:
		return push(newGardenShootsView(v.app, v.garden))
	case kindShoots:
		r, ok := v.tbl.selected()
		if !ok || r.data.(gardener.Project).Namespace == "" {
			return nil
		}
		return push(newProjectShootsView(v.app, v.garden, r.data.(gardener.Project)))
	}
	return nil
}

func (v *seedsView) crumb() state.Crumb {
	return state.Crumb{View: kindSeeds, Selected: selectedKey(v.tbl)}
}

func (v *seedsView) open(kind string) tea.Cmd {
	r, ok := v.tbl.selected()
	if !ok || kind != kindShoots {
		return nil
	}
	return push(newSeedShootsView(v.app, v.garden, r.data.(gardener.Seed).Name))
}

func (v *shootsView) crumb() state.Crumb {
	kind := kindShoots
	if v.all() {
		kind = kindAll
	}
	return state.Crumb{View: kind, Selected: selectedKey(v.tbl)}
}

func (v *shootsView) open(kind string) tea.Cmd {
	r, ok := v.tbl.selected()
	if !ok || kind != kindDetails {
		return nil
	}
	s := r.data.(gardener.Shoot)
	return push(newDetailView(v.app, v.garden, s, v.target(s)))
}

func (v *detailView) crumb() state.Crumb { return state.Crumb{View: kindDetails} }
