package ui

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/gardener/fake"
	"github.com/jastbytes/gardenctl-tui/internal/session"
	"github.com/jastbytes/gardenctl-tui/internal/testutil"
)

// execRecorder runs commands synchronously instead of handing over a terminal.
type execRecorder struct {
	mu     sync.Mutex
	stdin  []string // fed to consecutive commands
	output bytes.Buffer
	runs   int
}

func (r *execRecorder) exec(c tea.ExecCommand, cb tea.ExecCallback) tea.Cmd {
	return func() tea.Msg {
		r.mu.Lock()
		in := ""
		if len(r.stdin) > 0 {
			in, r.stdin = r.stdin[0], r.stdin[1:]
		}
		r.runs++
		r.mu.Unlock()
		out := &lockedWriter{w: &r.output, mu: &r.mu}
		c.SetStdin(strings.NewReader(in))
		c.SetStdout(out)
		c.SetStderr(out)
		return cb(c.Run())
	}
}

// lockedWriter makes concurrent stdout/stderr copies safe in tests.
type lockedWriter struct {
	w  io.Writer
	mu *sync.Mutex
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

func (r *execRecorder) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.output.String()
}

type launchEnv struct {
	opts Options
	rec  *execRecorder
	tmp  string
	out  string // file the test tool writes its environment to
	mgr  *session.Manager
}

func newLaunchEnv(t *testing.T) *launchEnv {
	t.Helper()
	bin, tmp := testutil.FakeGardenctl(t)
	out := filepath.Join(t.TempDir(), "env.txt")
	t.Setenv("GCTL_SESSION_ID", "parent-session")
	t.Setenv("KUBECONFIG", "/parent/kubeconfig")

	opts := testOptions(fake.Landscape())
	opts.Config.Gardenctl = bin
	opts.Config.Tools = []config.Tool{
		{Key: "x", ControlPlaneKey: "X", Name: "envdump", Command: []string{"sh", "-c",
			`echo "sid=$GCTL_SESSION_ID kc=$KUBECONFIG target=$GTUI_TARGET args={garden}/{project}/{shoot}/{namespace}" >> ` + out}},
		{Key: "f", Name: "sofka", Command: []string{"sofka"}},
	}
	rec := &execRecorder{}
	mgr := session.NewManager(bin)
	mgr.TempDir = tmp
	opts.Sessions = mgr
	opts.Exec = rec.exec
	opts.Available = func(tl config.Tool) bool { return tl.Name != "sofka" }
	return &launchEnv{opts: opts, rec: rec, tmp: tmp, out: out, mgr: mgr}
}

func (e *launchEnv) dumped(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(e.out)
	if err != nil {
		t.Fatalf("tool did not run: %v (exec output: %q)", err, e.rec.String())
	}
	return string(b)
}

func openShoots(t *testing.T, tm *teatest.TestModel) {
	t.Helper()
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	send(tm, "enter")
	waitFor(t, tm, "api-gw")
}

func TestLaunchToolFromShootTable(t *testing.T) {
	e := newLaunchEnv(t)
	tm := startApp(t, e.opts)
	openShoots(t, tm)
	send(tm, "x")
	waitFor(t, tm, "back from envdump")
	_ = tm.Quit()

	line := strings.TrimSpace(e.dumped(t))
	if strings.Contains(line, "parent-session") || strings.Contains(line, "/parent/kubeconfig") {
		t.Fatalf("parent session leaked into tool: %s", line)
	}
	if !strings.Contains(line, "target=dev/alpha/api-gw args=dev/alpha/api-gw/garden-alpha") {
		t.Fatalf("unexpected env: %s", line)
	}
	sid := strings.TrimPrefix(strings.Fields(line)[0], "sid=")
	wantKC := filepath.Join(e.tmp, "garden", "sessions", sid, "kubeconfig.yaml")
	if !strings.Contains(line, "kc="+wantKC) {
		t.Fatalf("KUBECONFIG should be %s: %s", wantKC, line)
	}
	target, _ := os.ReadFile(filepath.Join(e.tmp, "garden", "sessions", sid, "target"))
	if strings.TrimSpace(string(target)) != "--garden dev --project alpha --shoot api-gw" {
		t.Fatalf("gardenctl target args: %q", target)
	}
	if strings.Contains(e.rec.String(), "WARN") {
		t.Fatalf("gardenctl warned about KUBECONFIG: %q", e.rec.String())
	}
	if !strings.Contains(e.rec.String(), "gtui: targeting dev/alpha/api-gw") ||
		!strings.Contains(e.rec.String(), "gtui: envdump on dev/alpha/api-gw") {
		t.Fatalf("banner missing: %q", e.rec.String())
	}
	// The parent environment is untouched.
	if os.Getenv("KUBECONFIG") != "/parent/kubeconfig" || os.Getenv("GCTL_SESSION_ID") != "parent-session" {
		t.Fatal("parent environment was modified")
	}

	if err := e.mgr.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(e.tmp, "garden", "sessions", sid)); !os.IsNotExist(err) {
		t.Fatal("session directory not cleaned up")
	}
}

func TestLaunchControlPlane(t *testing.T) {
	e := newLaunchEnv(t)
	tm := startApp(t, e.opts)
	openShoots(t, tm)
	send(tm, "X")
	waitFor(t, tm, "back from envdump")
	_ = tm.Quit()
	line := e.dumped(t)
	if !strings.Contains(line, "target=dev/alpha/api-gw (control plane)") {
		t.Fatalf("env: %s", line)
	}
	sid := strings.TrimPrefix(strings.Fields(line)[0], "sid=")
	target, _ := os.ReadFile(filepath.Join(e.tmp, "garden", "sessions", sid, "target"))
	if !strings.Contains(string(target), "--control-plane") {
		t.Fatalf("target args: %q", target)
	}
}

func TestLaunchFromSeedShoots(t *testing.T) {
	e := newLaunchEnv(t)
	tm := startApp(t, e.opts)
	waitFor(t, tm, "live")
	send(tm, "tab")
	waitFor(t, tm, "gcp-us1")
	send(tm, "enter")
	waitFor(t, tm, "api-gw")
	send(tm, "x")
	waitFor(t, tm, "back from envdump")
	_ = tm.Quit()
	if line := e.dumped(t); !strings.Contains(line, "target=dev/aws-eu1/api-gw") {
		t.Fatalf("env: %s", line)
	}
}

func TestLauncherMenu(t *testing.T) {
	e := newLaunchEnv(t)
	tm := startApp(t, e.opts)
	openShoots(t, tm)
	send(tm, "l")
	waitFor(t, tm, "Launch a tool on")
	snapshot(t, tm)
}

func TestLauncherMenuLaunch(t *testing.T) {
	e := newLaunchEnv(t)
	tm := startApp(t, e.opts)
	openShoots(t, tm)
	send(tm, "down", "enter") // detail of batch
	waitFor(t, tm, "Technical ID")
	send(tm, "enter") // menu
	waitFor(t, tm, "Launch a tool on")
	send(tm, "down", "enter") // envdump on control plane
	waitFor(t, tm, "back from envdump")
	if err := tm.Quit(); err != nil {
		t.Fatal(err)
	}
	app := tm.FinalModel(t).(*App)
	if app.top().title() != "batch" {
		t.Fatalf("menu should close after launch, top = %q", app.top().title())
	}
	if line := e.dumped(t); !strings.Contains(line, "target=dev/alpha/batch (control plane)") {
		t.Fatalf("env: %s", line)
	}
}

func TestLaunchUnavailableTool(t *testing.T) {
	e := newLaunchEnv(t)
	tm := startApp(t, e.opts)
	openShoots(t, tm)
	send(tm, "f")
	waitFor(t, tm, "sofka not found in PATH")
	_ = tm.Quit()
	if e.rec.runs != 0 {
		t.Fatal("nothing should have been executed")
	}
}

func TestLaunchTargetError(t *testing.T) {
	e := newLaunchEnv(t)
	t.Setenv("FAKE_TARGET_ERROR", "shoot is hibernated")
	tm := startApp(t, e.opts)
	openShoots(t, tm)
	send(tm, "x")
	waitFor(t, tm, "gardenctl target: shoot is hibernated")
	_ = tm.Quit()
	if _, err := os.Stat(e.out); err == nil {
		t.Fatal("tool must not start after a failed target")
	}
}

func TestLaunchWithConfirmation(t *testing.T) {
	e := newLaunchEnv(t)
	t.Setenv("FAKE_RESTRICTED", "1")
	e.rec.stdin = []string{"y\n"}
	tm := startApp(t, e.opts)
	openShoots(t, tm)
	send(tm, "x")
	waitFor(t, tm, "back from envdump")
	_ = tm.Quit()
	if e.rec.runs != 2 {
		t.Fatalf("expected target + tool, got %d runs", e.rec.runs)
	}
	if !strings.Contains(e.rec.String(), "Do you want to continue?") {
		t.Fatalf("confirmation prompt not shown: %q", e.rec.String())
	}
	e.dumped(t)
}

func TestLaunchConfirmationDeclined(t *testing.T) {
	e := newLaunchEnv(t)
	t.Setenv("FAKE_RESTRICTED", "1")
	e.rec.stdin = []string{"n\n"}
	tm := startApp(t, e.opts)
	openShoots(t, tm)
	send(tm, "x")
	waitFor(t, tm, "gardenctl target: operation aborted")
	_ = tm.Quit()
	if _, err := os.Stat(e.out); err == nil {
		t.Fatal("tool ran although targeting was declined")
	}
}

func TestProviderEnvToggle(t *testing.T) {
	e := newLaunchEnv(t)
	tm := startApp(t, e.opts)
	waitFor(t, tm, "live")
	send(tm, "p")
	waitFor(t, tm, "provider-env on")
	if err := tm.Quit(); err != nil {
		t.Fatal(err)
	}
	if !tm.FinalModel(t).(*App).providerEnv {
		t.Fatal("provider env should be on")
	}
}

func TestGardenctlVersionWarning(t *testing.T) {
	opts := testOptions(fake.Landscape())
	mgr := session.NewManager(filepath.Join(t.TempDir(), "missing-gardenctl"))
	opts.Sessions = mgr
	tm := startApp(t, opts)
	waitFor(t, tm, "gardenctl not found")
	_ = tm.Quit()
}
