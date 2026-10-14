package ui

import (
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/jastbytes/gardenctl-tui/internal/gardener"
	"github.com/jastbytes/gardenctl-tui/internal/gardener/fake"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.9.0", "1.10.0", -1},
		{"1.33.2", "1.33.2", 0},
		{"1.33", "1.33.0", 0},
		{"1.31.9", "1.30.12", 1},
		{"", "1.0", -1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func names(shoots []gardener.Shoot) []string {
	var out []string
	for _, s := range shoots {
		out = append(out, s.Name)
	}
	return out
}

func TestSortShoots(t *testing.T) {
	shoots := fake.Landscape().Shoots // api-gw, batch, web, db, monitoring
	shoots[4].LastOp.Updated = shoots[4].LastOp.Updated.Add(time.Hour)
	cases := map[sortMode][]string{
		sortName:    {"api-gw", "batch", "web", "db", "monitoring"},
		sortStatus:  {"db", "web", "api-gw", "monitoring", "batch"},
		sortVersion: {"db", "batch", "api-gw", "web", "monitoring"},
		sortUpdated: {"monitoring", "api-gw", "batch", "web", "db"},
	}
	for m, want := range cases {
		if got := names(sortShoots(shoots, m)); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %v, want %v", m, got, want)
		}
	}
	if got := names(shoots); got[0] != "api-gw" || got[3] != "db" {
		t.Errorf("input was modified: %v", got)
	}
}

func TestSortKey(t *testing.T) {
	tm := startApp(t, testOptions(fake.Landscape()))
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	send(tm, "A")
	waitFor(t, tm, "monitoring")
	send(tm, "o")
	waitFor(t, tm, "problems first")
	app := quitAndSave(t, tm)
	sv := app.top().(*shootsView)
	var got []string
	for _, r := range sv.tbl.all {
		got = append(got, r.data.(gardener.Shoot).Name)
	}
	if want := []string{"db", "web", "api-gw", "monitoring", "batch"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order %v, want %v", got, want)
	}
	if !strings.Contains(app.View(), "sort: status") {
		t.Fatal("header does not show the sort order")
	}
}

// clip records copied texts.
type clip struct {
	mu   sync.Mutex
	text []string
}

func (c *clip) copy(s string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.text = append(c.text, s)
	return nil
}

func (c *clip) all() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.text...)
}

func TestYank(t *testing.T) {
	var c clip
	opts := testOptions(fake.Landscape())
	opts.Copy = c.copy
	tm := startApp(t, opts)
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	send(tm, "enter")
	waitFor(t, tm, "api-gw")
	send(tm, "y")
	waitFor(t, tm, "technical ID")
	send(tm, "t")
	waitFor(t, tm, "copied target command")
	send(tm, "y")
	waitFor(t, tm, "n/f/t/i copy")
	send(tm, "i")
	waitFor(t, tm, "copied technical ID")
	send(tm, "y")
	waitFor(t, tm, "n/f/t/i copy")
	send(tm, "enter") // first row: name
	waitFor(t, tm, "copied name")
	app := quitAndSave(t, tm)
	want := []string{"gardenctl target --garden dev --project alpha --shoot api-gw", "shoot--alpha--api-gw", "api-gw"}
	if got := c.all(); !reflect.DeepEqual(got, want) {
		t.Fatalf("copied %q, want %q", got, want)
	}
	if _, ok := app.top().(*shootsView); !ok {
		t.Fatalf("copy menu not closed: top is %q", app.top().title())
	}
}

func TestYAMLView(t *testing.T) {
	var c clip
	opts := testOptions(fake.Landscape())
	opts.Copy = c.copy
	tm := teatest.NewTestModel(t, New(opts), teatest.WithInitialTermSize(100, 30))
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	send(tm, "down", "enter")
	waitFor(t, tm, "db")
	send(tm, "v")
	waitFor(t, tm, "technicalID")
	send(tm, "y")
	waitFor(t, tm, "copied YAML of db")
	snapshot(t, tm)
	got := c.all()
	if len(got) != 1 || !strings.Contains(got[0], "kind: Shoot") || !strings.Contains(got[0], "name: db") {
		t.Fatalf("copied %q", got)
	}
}

func TestProjectsWithoutShootAccess(t *testing.T) {
	c := fake.Landscape()
	forbidden := apierrors.NewForbidden(schema.GroupResource{Group: "core.gardener.cloud", Resource: "shoots"}, "", nil)
	c.MethodErr = map[string]error{"ListShoots": forbidden, "WatchShoots": forbidden}
	opts := testOptions(c)
	opts.Config.RefreshInterval.Duration = 20 * time.Millisecond
	tm := startApp(t, opts)
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	deadline := time.Now().Add(3 * time.Second)
	for c.CallCount("ListProjects") < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	app := quitAndSave(t, tm)
	if n, w := c.CallCount("ListShoots"), c.CallCount("WatchShoots"); n != 1 || w != 1 {
		t.Fatalf("ListShoots called %d times, WatchShoots %d times; want once each", n, w)
	}
	if app.status != "" {
		t.Fatalf("missing shoot access should not be an error, status %q", app.status)
	}
}

// liveOptions enable watching (auto-refresh on, but no tick within a test).
func liveOptions(c *fake.Client) Options {
	opts := testOptions(c)
	opts.Config.RefreshInterval.Duration = time.Hour
	opts.WatchOptions = gardener.WatchOptions{Throttle: time.Millisecond}
	return opts
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func setShootStatus(c *fake.Client, name string, st gardener.Status) {
	c.Update(func(c *fake.Client) {
		for i := range c.Shoots {
			if c.Shoots[i].Name == name {
				c.Shoots[i].Status = st
			}
		}
	})
}

func TestLiveShootList(t *testing.T) {
	c := fake.Landscape()
	tm := startApp(t, liveOptions(c))
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "3 ⟳1") // counts of alpha come from the all-shoots watch
	send(tm, "enter")
	waitFor(t, tm, "● live")
	waitUntil(t, func() bool { return c.Watchers() == 2 }) // projects (counts) + alpha

	setShootStatus(c, "api-gw", gardener.StatusError)
	waitFor(t, tm, "1 error")

	send(tm, "a") // pause: updates are kept back
	waitFor(t, tm, "auto-refresh off")
	setShootStatus(c, "batch", gardener.StatusError)
	time.Sleep(50 * time.Millisecond)
	send(tm, "r") // shows them
	waitFor(t, tm, "2 error")
	if n := c.CallCount("ListShoots"); n != 0 {
		t.Fatalf("watched views must not poll, ListShoots called %d times", n)
	}

	send(tm, "esc")
	waitUntil(t, func() bool { return c.Watchers() == 1 })
	send(tm, "esc")
	waitUntil(t, func() bool { return c.Watchers() == 0 })
	_ = tm.Quit()
}

func TestWatchFallsBackToPolling(t *testing.T) {
	c := fake.Landscape()
	c.MethodErr = map[string]error{"WatchShoots": apierrors.NewMethodNotSupported(schema.GroupResource{Resource: "shoots"}, "watch")}
	tm := startApp(t, liveOptions(c))
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "3 ⟳1")
	send(tm, "enter")
	waitFor(t, tm, "api-gw")
	app := quitAndSave(t, tm)
	if strings.Contains(app.View(), "● live") {
		t.Fatal("polled view shown as live")
	}
	if n := c.CallCount("ListShoots"); n != 2 {
		t.Fatalf("ListShoots called %d times, want 2 (counts and alpha)", n)
	}
}
