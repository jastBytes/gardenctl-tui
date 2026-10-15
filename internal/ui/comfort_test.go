// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest"

	"github.com/jastbytes/gardenctl-tui/internal/gardener/fake"
	"github.com/jastbytes/gardenctl-tui/internal/session"
	"github.com/jastbytes/gardenctl-tui/internal/state"
)

func TestHelpOverlay(t *testing.T) {
	tm := teatest.NewTestModel(t, New(testOptions(fake.Landscape())), teatest.WithInitialTermSize(100, 30))
	waitFor(t, tm, "live")
	send(tm, "?")
	waitFor(t, tm, "Navigation")
	snapshot(t, tm)
}

func TestHelpClosesOnAnyKey(t *testing.T) {
	tm := startApp(t, testOptions(fake.Landscape()))
	waitFor(t, tm, "live")
	send(tm, "?")
	waitFor(t, tm, "Navigation")
	send(tm, "enter") // closes help only, does not open the garden
	time.Sleep(50 * time.Millisecond)
	if err := tm.Quit(); err != nil {
		t.Fatal(err)
	}
	app := tm.FinalModel(t).(*App)
	if app.showHelp || len(app.stack) != 1 {
		t.Fatalf("help=%v depth=%d", app.showHelp, len(app.stack))
	}
}

func TestFavoritesAndRecent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")
	st, err := state.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = st.AddRecent(session.Target{Garden: "dev", Project: "beta", Shoot: "db", Namespace: "garden-beta"}, "k9s")
	opts := testOptions(fake.Landscape())
	opts.State = st

	tm := startApp(t, opts)
	openShoots(t, tm)
	send(tm, "down", "*")
	waitFor(t, tm, "★ batch")
	send(tm, "ctrl+r")
	waitFor(t, tm, "✗ Error") // status of dev/beta/db
	app := snapshot(t, tm)
	if _, ok := app.top().(*recentView); !ok {
		t.Fatalf("top = %T", app.top())
	}

	reloaded, _ := state.Load(path)
	if !reloaded.IsFavorite(session.Target{Garden: "dev", Project: "alpha", Shoot: "batch", Namespace: "garden-alpha"}) {
		t.Fatal("favorite not saved")
	}
}

func TestRecentViewEmpty(t *testing.T) {
	tm := startApp(t, testOptions(fake.Landscape()))
	waitFor(t, tm, "live")
	send(tm, "ctrl+r")
	waitFor(t, tm, "No favorites or recent targets yet")
	_ = tm.Quit()
}

func TestLaunchRecordsRecentAndRelaunch(t *testing.T) {
	e := newLaunchEnv(t)
	st, _ := state.Load(filepath.Join(t.TempDir(), "state.yaml"))
	e.opts.State = st
	tm := startApp(t, e.opts)
	openShoots(t, tm)
	send(tm, "down", "down", "x")
	waitFor(t, tm, "back from envdump")
	if r := st.Recent(); len(r) != 1 || r[0].Key() != "dev/alpha/web" || r[0].Tool != "envdump" {
		t.Fatalf("recent: %+v", r)
	}

	// Launch again from the recent view.
	send(tm, "ctrl+r")
	waitFor(t, tm, "dev/alpha/web")
	send(tm, "X")
	waitFor(t, tm, "back from envdump (dev/alpha/web (control plane))")
	_ = tm.Quit()
	if lines := strings.Split(strings.TrimSpace(e.dumped(t)), "\n"); len(lines) != 2 {
		t.Fatalf("expected two launches, got %q", lines)
	}
}
