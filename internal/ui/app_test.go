// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/gardener"
	"github.com/jastbytes/gardenctl-tui/internal/gardener/fake"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

var testNow = time.Date(2026, 1, 2, 6, 4, 5, 0, time.UTC)

func testOptions(c *fake.Client) Options {
	cfg := config.DefaultGTUI()
	cfg.RefreshInterval.Duration = 0
	return Options{
		Gardens: []config.Garden{{Name: "dev", Alias: "d", Kubeconfig: "/k/dev.yaml"}, {Name: "live", Kubeconfig: "/k/live.yaml"}},
		Clients: func(config.Garden) (gardener.Client, error) { return c, nil },
		Config:  cfg,
		Now:     func() time.Time { return testNow },
	}
}

func startApp(t *testing.T, opts Options) *teatest.TestModel {
	t.Helper()
	return teatest.NewTestModel(t, New(opts), teatest.WithInitialTermSize(100, 18))
}

func waitFor(t *testing.T, tm *teatest.TestModel, s string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte(s)) },
		teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(10*time.Millisecond))
}

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "ctrl+r":
		return tea.KeyMsg{Type: tea.KeyCtrlR}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func send(tm *teatest.TestModel, keys ...string) {
	for _, k := range keys {
		tm.Send(keyMsg(k))
	}
}

// snapshot quits the program and compares the final view with the golden file.
func snapshot(t *testing.T, tm *teatest.TestModel) *App {
	t.Helper()
	if err := tm.Quit(); err != nil {
		t.Fatal(err)
	}
	app := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).(*App)
	golden.RequireEqual(t, []byte(app.View()))
	return app
}

func TestGardensView(t *testing.T) {
	tm := startApp(t, testOptions(fake.Landscape()))
	waitFor(t, tm, "live")
	snapshot(t, tm)
}

func TestProjectsView(t *testing.T) {
	tm := startApp(t, testOptions(fake.Landscape()))
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	snapshot(t, tm)
}

func TestShootsView(t *testing.T) {
	tm := startApp(t, testOptions(fake.Landscape()))
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	send(tm, "enter")
	waitFor(t, tm, "api-gw")
	snapshot(t, tm)
}

func TestDetailView(t *testing.T) {
	tm := teatest.NewTestModel(t, New(testOptions(fake.Landscape())), teatest.WithInitialTermSize(100, 40))
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	send(tm, "down", "enter")
	waitFor(t, tm, "db")
	send(tm, "enter")
	waitFor(t, tm, "Technical ID")
	snapshot(t, tm)
}

func TestDetailViewBottom(t *testing.T) {
	tm := teatest.NewTestModel(t, New(testOptions(fake.Landscape())), teatest.WithInitialTermSize(100, 30))
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	send(tm, "down", "enter")
	waitFor(t, tm, "db")
	send(tm, "enter")
	waitFor(t, tm, "Technical ID")
	send(tm, "G")
	waitFor(t, tm, "Credentials Rotation")
	snapshot(t, tm)
}

func TestSeedsView(t *testing.T) {
	tm := startApp(t, testOptions(fake.Landscape()))
	waitFor(t, tm, "live")
	send(tm, "tab")
	waitFor(t, tm, "gcp-us1")
	snapshot(t, tm)
}

func TestSeedShootsView(t *testing.T) {
	tm := startApp(t, testOptions(fake.Landscape()))
	waitFor(t, tm, "live")
	send(tm, "tab")
	waitFor(t, tm, "gcp-us1")
	send(tm, "down", "enter")
	waitFor(t, tm, "monitoring")
	snapshot(t, tm)
}

func TestFilter(t *testing.T) {
	tm := startApp(t, testOptions(fake.Landscape()))
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	send(tm, "enter")
	waitFor(t, tm, "api-gw")
	send(tm, "/", "w", "b")
	send(tm, "q") // typed into the filter, must not quit
	waitFor(t, tm, "/wbq")
	send(tm, "backspace")
	waitFor(t, tm, "/wb ")
	app := snapshot(t, tm)
	if !app.top().capturing() {
		t.Fatal("filter should still be focused")
	}
}

func TestFilterEscClearsBeforeBack(t *testing.T) {
	tm := startApp(t, testOptions(fake.Landscape()))
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	send(tm, "/", "o", "p", "s", "enter")
	waitFor(t, tm, "filter: ops")
	send(tm, "esc") // clears filter, stays in projects
	waitFor(t, tm, "alice@example.com")
	if err := tm.Quit(); err != nil {
		t.Fatal(err)
	}
	app := tm.FinalModel(t).(*App)
	if got := app.top().title(); got != "dev" {
		t.Fatalf("top view = %q, want projects of dev", got)
	}
}

func TestLoadError(t *testing.T) {
	c := fake.Landscape()
	c.Err = errors.New("forbidden: projects is forbidden")
	tm := startApp(t, testOptions(c))
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "Could not load")
	snapshot(t, tm)
}

func TestClientFactoryError(t *testing.T) {
	opts := testOptions(nil)
	opts.Clients = func(config.Garden) (gardener.Client, error) { return nil, errors.New("no kubeconfig") }
	tm := startApp(t, opts)
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "no kubeconfig")
	_ = tm.Quit()
}

func TestBackAndQuit(t *testing.T) {
	tm := startApp(t, testOptions(fake.Landscape()))
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	send(tm, "h", "q")
	app := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).(*App)
	if len(app.stack) != 1 {
		t.Fatalf("stack depth %d, want 1", len(app.stack))
	}
}

func TestRefreshAndAutoRefresh(t *testing.T) {
	c := fake.Landscape()
	opts := testOptions(c)
	opts.Config.RefreshInterval.Duration = 50 * time.Millisecond
	tm := startApp(t, opts)
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	deadline := time.Now().Add(3 * time.Second)
	for c.CallCount("ListProjects") < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := c.CallCount("ListProjects"); n < 3 {
		t.Fatalf("auto-refresh: ListProjects called %d times", n)
	}
	send(tm, "a") // disable auto-refresh
	waitFor(t, tm, "auto-refresh off")
	time.Sleep(100 * time.Millisecond)
	n := c.CallCount("ListProjects")
	time.Sleep(200 * time.Millisecond)
	if m := c.CallCount("ListProjects"); m > n+1 {
		t.Fatalf("auto-refresh still running after toggle: %d -> %d", n, m)
	}
	send(tm, "r")
	deadline = time.Now().Add(3 * time.Second)
	for c.CallCount("ListProjects") <= n && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if c.CallCount("ListProjects") <= n {
		t.Fatal("manual refresh did not reload")
	}
	_ = tm.Quit()
}

func TestStaleLoadIsDropped(t *testing.T) {
	a := New(testOptions(fake.Landscape()))
	a.Update(pushMsg{newProjectsView(a, a.opts.Gardens[0])})
	popped := a.top().id()
	a.Update(keyMsg("esc"))
	_, cmd := a.Update(loadedMsg{viewID: popped, err: errors.New("late")})
	if cmd != nil || a.status != "" {
		t.Fatalf("stale message should be ignored, status=%q", a.status)
	}
}

func TestBreadcrumbAndStatusFitWidth(t *testing.T) {
	a := New(testOptions(fake.Landscape()))
	a.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	for _, line := range strings.Split(a.View(), "\n") {
		if w := lipgloss.Width(line); w > 40 {
			t.Fatalf("line wider than terminal (%d): %q", w, line)
		}
	}
}

func TestLoadSuccessKeepsOtherErrors(t *testing.T) {
	a := New(testOptions(fake.Landscape()))
	a.Update(pushMsg{newProjectsView(a, a.opts.Gardens[0])})
	id := a.top().id()

	a.Update(loadedMsg{viewID: id, err: errors.New("forbidden")})
	a.Update(loadedMsg{viewID: id, data: projectsData{}})
	if a.status != "" {
		t.Fatalf("a successful load should clear a load error, got %q", a.status)
	}

	// e.g. "k9s exited: exit status 1", followed by the refresh after the tool exits
	a.Update(statusMsg{text: "k9s exited: exit status 1", err: true})
	a.Update(loadedMsg{viewID: id, data: projectsData{}})
	if a.status != "k9s exited: exit status 1" {
		t.Fatalf("tool error was cleared by the refresh, status %q", a.status)
	}
}

func TestAllShootsView(t *testing.T) {
	tm := startApp(t, testOptions(fake.Landscape()))
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	send(tm, "A")
	waitFor(t, tm, "db")
	snapshot(t, tm)
}

func TestSingleGardenStartsInProjects(t *testing.T) {
	opts := testOptions(fake.Landscape())
	opts.Gardens = opts.Gardens[:1]
	tm := startApp(t, opts)
	waitFor(t, tm, "garden-ops")
	send(tm, "esc")
	app := quitAndSave(t, tm)
	if got := titles(app); !reflect.DeepEqual(got, []string{"gardens"}) {
		t.Fatalf("stack after esc: %v", got)
	}
}

func TestProblemsOnly(t *testing.T) {
	tm := startApp(t, testOptions(fake.Landscape()))
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	send(tm, "A")
	waitFor(t, tm, "monitoring")
	send(tm, "!")
	waitFor(t, tm, "problems only")
	snapshot(t, tm)
}
