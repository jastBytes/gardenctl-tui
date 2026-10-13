package session

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseKubeconfigExport(t *testing.T) {
	cases := map[string]struct {
		in, want string
		err      bool
	}{
		"gardenctl output": {in: "export KUBECONFIG='/tmp/garden/sessions/abc/kubeconfig.yaml';\n\n# Run this command to configure kubectl for your shell:\n# eval $(gardenctl kubectl-env bash)\n",
			want: "/tmp/garden/sessions/abc/kubeconfig.yaml"},
		"escaped quote":    {in: `export KUBECONFIG='/tmp/it'"'"'s here/kc.yaml';`, want: "/tmp/it's here/kc.yaml"},
		"space in path":    {in: "export KUBECONFIG='/Users/A B/kc.yaml';", want: "/Users/A B/kc.yaml"},
		"unquoted":         {in: "export KUBECONFIG=/tmp/kc.yaml", want: "/tmp/kc.yaml"},
		"double quoted":    {in: `export KUBECONFIG="/tmp/k\"c.yaml";`, want: `/tmp/k"c.yaml`},
		"double escapes":   {in: `export KUBECONFIG="/a\$b\x";`, want: `/a$b\x`},
		"leading notice":   {in: "printf 'hello';\nexport KUBECONFIG='/x';\n", want: "/x"},
		"missing":          {in: "unset KUBECONFIG;\n", err: true},
		"unterminated":     {in: "export KUBECONFIG='/tmp/x;", err: true},
		"expansion":        {in: "export KUBECONFIG=$HOME/x;", err: true},
		"empty":            {in: "export KUBECONFIG='';", err: true},
		"trailing garbage": {in: "export KUBECONFIG='/a' rm -rf;", err: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseKubeconfigExport(tc.in)
			if tc.err {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestTargetArgs(t *testing.T) {
	cases := []struct {
		t    Target
		want string
		str  string
	}{
		{Target{Garden: "g", Project: "p", Shoot: "s"}, "target --garden g --project p --shoot s", "g/p/s"},
		{Target{Garden: "g", Project: "p", Shoot: "s", ControlPlane: true}, "target --garden g --project p --shoot s --control-plane", "g/p/s (control plane)"},
		{Target{Garden: "g", Seed: "seed", Shoot: "s"}, "target --garden g --seed seed --shoot s", "g/seed/s"},
		{Target{Garden: "g", Seed: "seed"}, "target --garden g --seed seed", "g/seed"},
		{Target{Garden: "g"}, "target --garden g", "g"},
	}
	for _, tc := range cases {
		if got := strings.Join(tc.t.TargetArgs(), " "); got != tc.want {
			t.Errorf("args: got %q want %q", got, tc.want)
		}
		if got := tc.t.String(); got != tc.str {
			t.Errorf("string: got %q want %q", got, tc.str)
		}
	}
}

type call struct {
	env  []string
	args []string
}

type mockRunner struct {
	calls   []call
	results map[string]struct {
		stdout, stderr string
		err            error
	}
}

func (m *mockRunner) Run(_ context.Context, env []string, args ...string) (string, string, error) {
	m.calls = append(m.calls, call{env, args})
	r := m.results[args[0]]
	return r.stdout, r.stderr, r.err
}

func (m *mockRunner) Command(env []string, args ...string) *exec.Cmd {
	m.calls = append(m.calls, call{env, args})
	return exec.Command("true")
}

// prepare runs what the UI runs: target (in the terminal) and kubectl-env.
func prepare(m *Manager, t Target) (*Session, error) {
	s := m.New(t)
	if err := m.TargetCommand(s).Run(); err != nil {
		return s, err
	}
	return s, m.Env(context.Background(), s)
}

func envValue(env []string, key string) []string {
	var out []string
	for _, kv := range env {
		if k, v, _ := strings.Cut(kv, "="); k == key {
			out = append(out, v)
		}
	}
	return out
}

func newMock() *mockRunner {
	return &mockRunner{results: map[string]struct {
		stdout, stderr string
		err            error
	}{
		"target":      {stdout: "Successfully targeted shoot \"s\"\n"},
		"kubectl-env": {stdout: "export KUBECONFIG='/tmp/garden/sessions/sid-1/kubeconfig.yaml';\n"},
	}}
}

func TestPrepare(t *testing.T) {
	r := newMock()
	m := &Manager{
		Runner:  r,
		BaseEnv: []string{"PATH=/bin", "GCTL_SESSION_ID=parent", "KUBECONFIG=/parent", "TERM_SESSION_ID=x"},
		NewID:   func() string { return "sid-1" },
	}
	s, err := prepare(m, Target{Garden: "g", Project: "p", Shoot: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 2 {
		t.Fatalf("calls: %+v", r.calls)
	}
	if got := strings.Join(r.calls[0].args, " "); got != "target --garden g --project p --shoot s" {
		t.Fatalf("target call: %s", got)
	}
	if got := strings.Join(r.calls[1].args, " "); got != "kubectl-env bash" {
		t.Fatalf("env call: %s", got)
	}
	for _, c := range r.calls {
		if v := envValue(c.env, EnvSessionID); !reflect.DeepEqual(v, []string{"sid-1"}) {
			t.Fatalf("gardenctl must run with the new session id only, got %v", v)
		}
		if v := envValue(c.env, EnvKubeconfig); len(v) != 1 || v[0] == "/parent" {
			t.Fatalf("gardenctl must get the session KUBECONFIG, not the parent's: %v", v)
		}
	}
	if s.Kubeconfig != "/tmp/garden/sessions/sid-1/kubeconfig.yaml" {
		t.Fatalf("kubeconfig %q", s.Kubeconfig)
	}
	if v := envValue(s.Env, EnvKubeconfig); !reflect.DeepEqual(v, []string{s.Kubeconfig}) {
		t.Fatalf("session KUBECONFIG: %v", v)
	}
	if v := envValue(s.Env, EnvTarget); !reflect.DeepEqual(v, []string{"g/p/s"}) {
		t.Fatalf("GTUI_TARGET: %v", v)
	}
	if v := envValue(s.Env, "PATH"); !reflect.DeepEqual(v, []string{"/bin"}) {
		t.Fatalf("PATH not inherited: %v", v)
	}
}

func TestPrepareErrors(t *testing.T) {
	r := newMock()
	r.results["kubectl-env"] = struct {
		stdout, stderr string
		err            error
	}{stdout: "garbage"}
	m := &Manager{Runner: r, BaseEnv: []string{}}
	if _, err := prepare(m, Target{Garden: "g"}); err == nil {
		t.Fatal("expected parse error")
	}

	r.results["kubectl-env"] = struct {
		stdout, stderr string
		err            error
	}{stderr: "I1012 some log\nError: no target\n", err: errors.New("exit status 1")}
	if _, err := prepare(m, Target{Garden: "g"}); err == nil || err.Error() != "gardenctl kubectl-env: no target" {
		t.Fatalf("err = %v", err)
	}
}

func TestGardenctlError(t *testing.T) {
	err := GardenctlError("target", "I1012 log line\nError: shoot \"s\" not found\n", errors.New("exit status 1"))
	if err.Error() != `gardenctl target: shoot "s" not found` {
		t.Fatal(err)
	}
	if err := GardenctlError("target", "", errors.New("exit status 1")); err.Error() != "gardenctl target: exit status 1" {
		t.Fatal(err)
	}
}

func TestCleanupRemovesOnlyOwnSessions(t *testing.T) {
	tmp := t.TempDir()
	mk := func(parts ...string) string {
		p := filepath.Join(append([]string{tmp}, parts...)...)
		if err := os.MkdirAll(p, 0o700); err != nil {
			t.Fatal(err)
		}
		return p
	}
	own := mk("garden", "sessions", "own")
	ownOld := mk("garden", "own")
	foreign := mk("garden", "sessions", "foreign")

	m := &Manager{Runner: newMock(), TempDir: tmp, BaseEnv: []string{}, NewID: func() string { return "own" }}
	m.New(Target{Garden: "g"})
	if err := m.Cleanup(); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{own, ownOld} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s still exists", p)
		}
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Errorf("foreign session removed: %v", err)
	}
}

func TestCleanupIgnoresUnsafeIDs(t *testing.T) {
	tmp := t.TempDir()
	victim := filepath.Join(tmp, "victim")
	if err := os.MkdirAll(victim, 0o700); err != nil {
		t.Fatal(err)
	}
	m := &Manager{Runner: newMock(), TempDir: filepath.Join(tmp, "x"), BaseEnv: []string{}, NewID: func() string { return "../../victim" }}
	m.New(Target{Garden: "g"})
	_ = m.Cleanup()
	if _, err := os.Stat(victim); err != nil {
		t.Fatal("path traversal in session id")
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v2.16.0", "v2.10.0", 1},
		{"v2.9.3", "v2.10.0", -1},
		{"v2.10.0", "v2.10.0", 0},
		{"v2.17.0-dev", "v2.10.0", 1},
		{"v2.10.0-dev", "v2.10.0", 0},
	}
	for _, tc := range cases {
		if got := CompareVersions(tc.a, tc.b); got != tc.want {
			t.Errorf("%s vs %s: %d", tc.a, tc.b, got)
		}
	}
}
