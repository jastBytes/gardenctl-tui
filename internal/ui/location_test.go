// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest"

	"github.com/jastbytes/gardenctl-tui/internal/gardener/fake"
	"github.com/jastbytes/gardenctl-tui/internal/state"
)

func rememberOptions(t *testing.T) (Options, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.yaml")
	st, err := state.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	opts := testOptions(fake.Landscape())
	opts.Config.RememberLocation = true
	opts.State = st
	return opts, path
}

func titles(a *App) []string {
	var out []string
	for _, v := range a.stack {
		out = append(out, v.title())
	}
	return out
}

func quitAndSave(t *testing.T, tm *teatest.TestModel) *App {
	t.Helper()
	if err := tm.Quit(); err != nil {
		t.Fatal(err)
	}
	app := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).(*App)
	if err := app.SaveLocation(); err != nil {
		t.Fatal(err)
	}
	return app
}

func TestRememberLocationProjectPath(t *testing.T) {
	opts, path := rememberOptions(t)
	tm := startApp(t, opts)
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	send(tm, "down", "enter") // beta
	waitFor(t, tm, "db")
	send(tm, "enter")
	waitFor(t, tm, "Technical ID")
	quitAndSave(t, tm)

	want := []state.Crumb{
		{View: kindGardens, Selected: "dev"},
		{View: kindProjects, Selected: "beta"},
		{View: kindShoots, Selected: "garden-beta/db"},
		{View: kindDetails},
	}
	st, _ := state.Load(path)
	if got := st.Location(); !reflect.DeepEqual(got, want) {
		t.Fatalf("saved location: %+v", got)
	}

	opts.State = st
	tm = startApp(t, opts)
	waitFor(t, tm, "Technical ID")
	app := quitAndSave(t, tm)
	if got := titles(app); !reflect.DeepEqual(got, []string{"gardens", "dev", "beta", "db"}) {
		t.Fatalf("restored stack: %v", got)
	}
}

func TestRememberLocationSeedPathAndSelection(t *testing.T) {
	opts, _ := rememberOptions(t)
	_ = opts.State.SetLocation([]state.Crumb{
		{View: kindGardens, Selected: "live"},
		{View: kindSeeds, Selected: "gcp-us1"},
		{View: kindShoots, Selected: "garden-ops/monitoring"},
	})
	tm := startApp(t, opts)
	waitFor(t, tm, "monitoring")
	app := quitAndSave(t, tm)
	if got := titles(app); !reflect.DeepEqual(got, []string{"gardens", "live seeds", "seed gcp-us1"}) {
		t.Fatalf("restored stack: %v", got)
	}
	if r, _ := app.top().(*shootsView).tbl.selected(); r.key != "garden-ops/monitoring" {
		t.Fatalf("selected %q", r.key)
	}
}

func TestRememberLocationStopsAtMissingItem(t *testing.T) {
	opts, _ := rememberOptions(t)
	_ = opts.State.SetLocation([]state.Crumb{
		{View: kindGardens, Selected: "dev"},
		{View: kindProjects, Selected: "deleted"},
		{View: kindShoots, Selected: "garden-deleted/x"},
	})
	tm := startApp(t, opts)
	waitFor(t, tm, "garden-ops")
	time.Sleep(50 * time.Millisecond)
	app := quitAndSave(t, tm)
	if got := titles(app); !reflect.DeepEqual(got, []string{"gardens", "dev"}) {
		t.Fatalf("restored stack: %v", got)
	}
	if len(app.restore) != 0 {
		t.Fatalf("restore not finished: %+v", app.restore)
	}
}

func TestRememberLocationDisabled(t *testing.T) {
	opts, path := rememberOptions(t)
	opts.Config.RememberLocation = false
	_ = opts.State.SetLocation([]state.Crumb{{View: kindGardens, Selected: "live"}, {View: kindSeeds, Selected: "aws-eu1"}})
	tm := startApp(t, opts)
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	app := quitAndSave(t, tm)
	if len(app.stack) != 2 || app.top().title() != "dev" {
		t.Fatalf("stack: %v", titles(app))
	}
	st, _ := state.Load(path)
	if got := st.Location(); len(got) != 2 || got[1].View != kindSeeds {
		t.Fatalf("location must stay untouched when disabled: %+v", got)
	}
}

func TestLocationSkipsMenuAndRecent(t *testing.T) {
	opts, _ := rememberOptions(t)
	tm := startApp(t, opts)
	openShoots(t, tm)
	send(tm, "l")
	waitFor(t, tm, "k9s")
	app := quitAndSave(t, tm)
	got := app.Location()
	if len(got) != 3 || got[2].View != kindShoots {
		t.Fatalf("location: %+v", got)
	}
}

func TestRememberLocationAllShoots(t *testing.T) {
	opts, path := rememberOptions(t)
	tm := startApp(t, opts)
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	send(tm, "A")
	waitFor(t, tm, "db")
	quitAndSave(t, tm)

	opts.State, _ = state.Load(path)
	tm = startApp(t, opts)
	waitFor(t, tm, "db")
	app := quitAndSave(t, tm)
	if got := titles(app); !reflect.DeepEqual(got, []string{"gardens", "dev", "all shoots"}) {
		t.Fatalf("restored stack: %v", got)
	}
}
