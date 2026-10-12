package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jastbytes/gardenctl-tui/internal/testutil"
)

func TestExecRunnerWithFakeGardenctl(t *testing.T) {
	bin, tmp := testutil.FakeGardenctl(t)
	m := &Manager{Runner: ExecRunner{Binary: bin}, TempDir: tmp}
	ctx := context.Background()

	v, err := m.Version(ctx)
	if err != nil || v != "v2.16.0" {
		t.Fatalf("version %q, %v", v, err)
	}

	s1, err := m.Prepare(ctx, Target{Garden: "g", Project: "p", Shoot: "a"})
	if err != nil {
		t.Fatal(err)
	}
	s2, err := m.Prepare(ctx, Target{Garden: "g", Project: "p", Shoot: "b", ControlPlane: true})
	if err != nil {
		t.Fatal(err)
	}
	if s1.ID == s2.ID || s1.Kubeconfig == s2.Kubeconfig {
		t.Fatal("sessions are not isolated")
	}
	want := filepath.Join(tmp, "garden", "sessions", s1.ID, "kubeconfig.yaml")
	if s1.Kubeconfig != want {
		t.Fatalf("kubeconfig %q, want %q", s1.Kubeconfig, want)
	}
	got, _ := os.ReadFile(filepath.Join(tmp, "garden", "sessions", s2.ID, "target"))
	if strings.TrimSpace(string(got)) != "--garden g --project p --shoot b --control-plane" {
		t.Fatalf("target args: %q", got)
	}

	if err := m.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(filepath.Join(tmp, "garden", "sessions")); len(entries) != 0 {
		t.Fatalf("sessions left after cleanup: %v", entries)
	}
}

func TestVersionMissingBinary(t *testing.T) {
	m := &Manager{Runner: ExecRunner{Binary: filepath.Join(t.TempDir(), "nope")}}
	if _, err := m.Version(context.Background()); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v", err)
	}
}
