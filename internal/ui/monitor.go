// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/gardener"
	"github.com/jastbytes/gardenctl-tui/internal/state"
)

// monitor keeps the status of the favorite and recent shoots up to date,
// for the favorites view and for notifications (favorites only). It watches
// their project namespaces, which project members may watch without
// cluster-wide permissions.
type monitor struct {
	id int
	// watches by group ("garden/namespace").
	watches map[string]*shootWatch
	// failed groups are not watched again until a manual refresh.
	failed map[string]bool
	// shoots by shootKey.
	shoots  map[string]gardener.Shoot
	changes *changeTracker
}

func newMonitor(id int) *monitor {
	return &monitor{id: id, watches: map[string]*shootWatch{}, failed: map[string]bool{},
		shoots: map[string]gardener.Shoot{}, changes: newChangeTracker()}
}

func shootKey(garden, namespace, name string) string { return garden + "/" + namespace + "/" + name }

// favoriteKey is the shootKey of a favorite ("" for seeds and old entries without namespace).
func favoriteKey(e state.Entry) string {
	if e.Shoot == "" || e.Namespace == "" {
		return ""
	}
	return shootKey(e.Garden, e.Namespace, e.Shoot)
}

// monitorGroup is a project namespace of a garden with favorites.
type monitorGroup struct {
	garden    config.Garden
	namespace string
}

func (g monitorGroup) key() string { return g.garden.Name + "/" + g.namespace }

// monitorGroups returns the groups of the favorite shoots (and of the recent
// ones, when withRecent is set).
func (a *App) monitorGroups(withRecent bool) map[string]monitorGroup {
	gardens := map[string]config.Garden{}
	for _, g := range a.opts.Gardens {
		gardens[g.Name] = g
	}
	entries := a.state.Favorites()
	if withRecent {
		entries = append(entries, a.state.Recent()...)
	}
	out := map[string]monitorGroup{}
	for _, e := range entries {
		g, ok := gardens[e.Garden]
		if favoriteKey(e) == "" || !ok {
			continue
		}
		grp := monitorGroup{garden: g, namespace: e.Namespace}
		out[grp.key()] = grp
	}
	return out
}

// syncMonitor starts watches for new favorite and recent groups and stops unused ones.
func (a *App) syncMonitor() tea.Cmd {
	if !a.watching() {
		return nil
	}
	m := a.mon
	want := a.monitorGroups(true)
	for key, w := range m.watches {
		if _, ok := want[key]; !ok {
			w.stop()
			delete(m.watches, key)
		}
	}
	var cmds []tea.Cmd
	for key, grp := range want {
		if m.watches[key] != nil || m.failed[key] {
			continue
		}
		w, cmd := a.startWatch(m.id, grp.garden, gardener.ShootScope{Namespace: grp.namespace})
		w.group = key
		m.watches[key] = w
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

// monitorUpdate handles a message of a monitor watch.
func (a *App) monitorUpdate(msg loadedMsg) tea.Cmd {
	m, w := a.mon, msg.stream
	if w == nil || m.watches[w.group] != w {
		return nil
	}
	if msg.fallback {
		w.stop()
		delete(m.watches, w.group)
		m.failed[w.group] = true
		return nil
	}
	if msg.err != nil {
		return w.next() // keep the last known status
	}
	return tea.Batch(w.next(), a.monitorApply(w.group, msg.data.([]gardener.Shoot)))
}

// monitorApply stores the shoots of group, notifies about favorites that
// turned red and redraws the favorites view.
func (a *App) monitorApply(group string, shoots []gardener.Shoot) tea.Cmd {
	m := a.mon
	garden := strings.SplitN(group, "/", 2)[0]
	for k := range m.shoots {
		if strings.HasPrefix(k, group+"/") {
			delete(m.shoots, k)
		}
	}
	now := a.opts.Now()
	before := map[string]gardener.Status{} // previous status of changed shoots
	for _, s := range shoots {
		key := shootKey(garden, s.Namespace, s.Name)
		m.shoots[key] = s
		if old, ok := m.changes.observe(key, s.Status, now); ok {
			before[key] = old
		}
	}
	var cmds []tea.Cmd
	for _, f := range a.state.Favorites() {
		if old, ok := before[favoriteKey(f)]; ok {
			cmds = append(cmds, a.favoriteChanged(f, m.shoots[favoriteKey(f)], old))
		}
	}
	for _, v := range a.stack {
		if rv, ok := v.(*recentView); ok {
			rv.rebuild()
			if len(before) > 0 {
				cmds = append(cmds, expireHighlight(rv.vid))
			}
		}
	}
	return tea.Batch(cmds...)
}

// favoriteChanged reports a status change of a favorite: red ones in the
// status line and with a notification, recovered ones in the status line.
func (a *App) favoriteChanged(f state.Entry, s gardener.Shoot, old gardener.Status) tea.Cmd {
	text := "★ " + f.Key() + " is " + strings.ToLower(s.Status.String())
	if s.Status != gardener.StatusError {
		if old == gardener.StatusError {
			return setStatus(text+" again", false)
		}
		return nil
	}
	notify := a.opts.Notify
	body := f.Key() + " is red"
	if s.LastOp.Message != "" {
		body += ": " + s.LastOp.Message
	}
	return tea.Batch(setStatus(text, true), func() tea.Msg {
		notify("gtui: "+s.Name+" failed", body)
		return nil
	})
}
