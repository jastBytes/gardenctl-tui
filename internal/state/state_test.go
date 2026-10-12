package state

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/jastbytes/gardenctl-tui/internal/session"
)

func TestRecent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "state.yaml")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { clock = clock.Add(time.Minute); return clock }

	a := session.Target{Garden: "g", Project: "p", Shoot: "a", Namespace: "garden-p"}
	b := session.Target{Garden: "g", Seed: "seed", Shoot: "b"}
	for _, step := range []struct {
		t    session.Target
		tool string
	}{{a, "k9s"}, {b, "Shell"}, {a, "kubectl"}} {
		if err := s.AddRecent(step.t, step.tool); err != nil {
			t.Fatal(err)
		}
	}
	r := s.Recent()
	if len(r) != 2 || r[0].Shoot != "a" || r[0].Tool != "kubectl" || r[1].Shoot != "b" {
		t.Fatalf("recent: %+v", r)
	}
	ac := a
	ac.ControlPlane = true
	if err := s.AddRecent(ac, "k9s"); err != nil {
		t.Fatal(err)
	}
	if len(s.Recent()) != 2 {
		t.Fatal("control plane flag must not create a separate entry")
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Recent(); len(got) != 2 || got[0].Target() != a {
		t.Fatalf("reloaded: %+v", got)
	}
}

func TestRecentIsBounded(t *testing.T) {
	s, _ := Load("")
	for i := 0; i < MaxRecent+5; i++ {
		_ = s.AddRecent(session.Target{Garden: "g", Project: "p", Shoot: fmt.Sprint(i)}, "x")
	}
	if n := len(s.Recent()); n != MaxRecent {
		t.Fatalf("len %d", n)
	}
	if s.Recent()[0].Shoot != fmt.Sprint(MaxRecent+4) {
		t.Fatal("newest entry must come first")
	}
}

func TestFavorites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")
	s, _ := Load(path)
	a := session.Target{Garden: "g", Project: "p", Shoot: "a"}
	on, err := s.ToggleFavorite(a)
	if err != nil || !on || !s.IsFavorite(a) {
		t.Fatalf("toggle on: %v %v", on, err)
	}
	reloaded, _ := Load(path)
	if !reloaded.IsFavorite(a) {
		t.Fatal("favorite not persisted")
	}
	on, _ = s.ToggleFavorite(a)
	if on || s.IsFavorite(a) || len(s.Favorites()) != 0 {
		t.Fatal("toggle off")
	}
}

func TestLoadInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")
	s, _ := Load(path)
	_ = s.AddRecent(session.Target{Garden: "g"}, "x")
	if err := writeFile(path, "recent: [::"); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected parse error")
	}
}
