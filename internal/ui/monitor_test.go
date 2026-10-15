// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jastbytes/gardenctl-tui/internal/gardener"
	"github.com/jastbytes/gardenctl-tui/internal/gardener/fake"
	"github.com/jastbytes/gardenctl-tui/internal/session"
	"github.com/jastbytes/gardenctl-tui/internal/state"
)

// notes records notifications.
type notes struct {
	mu    sync.Mutex
	items []string
}

func (n *notes) notify(title, body string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.items = append(n.items, title+" | "+body)
}

func (n *notes) all() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.items...)
}

var apiGW = session.Target{Garden: "dev", Project: "alpha", Shoot: "api-gw", Namespace: "garden-alpha"}

func favoriteOptions(t *testing.T, c *fake.Client, n *notes) Options {
	t.Helper()
	st, _ := state.Load("")
	if _, err := st.ToggleFavorite(apiGW); err != nil {
		t.Fatal(err)
	}
	opts := liveOptions(c)
	opts.State = st
	opts.Notify = n.notify
	return opts
}

func TestFavoriteTurnsRed(t *testing.T) {
	c := fake.Landscape()
	var n notes
	tm := startApp(t, favoriteOptions(t, c, &n))
	waitFor(t, tm, "live")
	waitUntil(t, func() bool { return c.Watchers() == 1 }) // the monitor watches garden-alpha
	time.Sleep(50 * time.Millisecond)                      // first list: the initial status is no change

	setShootStatus(c, "web", gardener.StatusError) // not a favorite
	setShootStatus(c, "api-gw", gardener.StatusError)
	waitFor(t, tm, "★ dev/alpha/api-gw is error")
	waitUntil(t, func() bool { return len(n.all()) == 1 })
	if got := n.all()[0]; !strings.Contains(got, "api-gw failed") || !strings.Contains(got, "dev/alpha/api-gw is red") {
		t.Fatalf("notification %q", got)
	}

	setShootStatus(c, "api-gw", gardener.StatusHealthy)
	waitFor(t, tm, "is healthy again")
	_ = tm.Quit()
	if got := n.all(); len(got) != 1 {
		t.Fatalf("notifications %q, want only the one for api-gw", got)
	}
}

func TestFavoritesOnStart(t *testing.T) {
	c := fake.Landscape()
	var n notes
	opts := favoriteOptions(t, c, &n)
	opts.Config.FavoritesOnStart = true
	tm := startApp(t, opts)
	waitFor(t, tm, "✓ Healthy") // live status of api-gw in the favorites view
	send(tm, "*")               // no favorites left: the monitor stops watching
	waitUntil(t, func() bool { return c.Watchers() == 0 })
	app := quitAndSave(t, tm)
	if _, ok := app.top().(*recentView); !ok {
		t.Fatalf("top = %T, want the favorites view", app.top())
	}
}

func TestChangedRowsAreMarked(t *testing.T) {
	c := fake.Landscape()
	opts := liveOptions(c)
	var now atomic.Int64
	now.Store(testNow.UnixNano())
	opts.Now = func() time.Time { return time.Unix(0, now.Load()) }
	tm := startApp(t, opts)
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	send(tm, "enter")
	waitFor(t, tm, "● live")
	waitUntil(t, func() bool { return c.Watchers() == 2 })

	setShootStatus(c, "batch", gardener.StatusError)
	waitFor(t, tm, "▮ batch")

	now.Add(int64(highlightFor + time.Second))
	app := quitAndSave(t, tm)
	app.Update(rebuildMsg{viewID: app.top().id()})
	if strings.Contains(app.View(), "▮") {
		t.Fatalf("highlight did not expire:\n%s", app.View())
	}
}

func TestRecentTargetsGetStatus(t *testing.T) {
	c := fake.Landscape()
	var n notes
	opts := favoriteOptions(t, c, &n)
	db := session.Target{Garden: "dev", Project: "beta", Shoot: "db", Namespace: "garden-beta"}
	if err := opts.State.AddRecent(db, "k9s"); err != nil {
		t.Fatal(err)
	}
	tm := startApp(t, opts)
	waitFor(t, tm, "live")
	waitUntil(t, func() bool { return c.Watchers() == 2 }) // garden-alpha (favorite) and garden-beta (recent)
	send(tm, "ctrl+r")
	waitFor(t, tm, "✗ Error") // dev/beta/db is only a recent target
	_ = tm.Quit()
	if got := n.all(); len(got) != 0 {
		t.Fatalf("recent targets must not notify: %q", got)
	}
}
