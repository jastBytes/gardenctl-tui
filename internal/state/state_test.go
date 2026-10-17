// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"fmt"
	"path/filepath"
	"sync"
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

func TestLocation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")
	s, _ := Load(path)
	loc := []Crumb{{View: "gardens", Selected: "dev"}, {View: "details"}}
	if err := s.SetLocation(loc); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Location(); len(got) != 2 || got[0] != loc[0] || got[1] != loc[1] {
		t.Fatalf("location: %+v", got)
	}
}

// Two gtui instances share the state file; neither may lose the other's changes.
func TestSharedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")
	s1, _ := Load(path)
	s2, _ := Load(path)
	a := session.Target{Garden: "g", Project: "p", Shoot: "a", Namespace: "garden-p"}
	b := session.Target{Garden: "g", Project: "p", Shoot: "b", Namespace: "garden-p"}

	if _, err := s1.ToggleFavorite(a); err != nil {
		t.Fatal(err)
	}
	// s2 has not seen that a is a favorite: adding it again keeps it.
	if on, err := s2.ToggleFavorite(a); err != nil || !on {
		t.Fatalf("toggle a in s2: %v, %v", on, err)
	}
	if _, err := s2.ToggleFavorite(b); err != nil {
		t.Fatal(err)
	}
	if err := s1.AddRecent(a, "k9s"); err != nil {
		t.Fatal(err)
	}
	if err := s2.SetLocation([]Crumb{{View: "gardens"}}); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if f := got.Favorites(); len(f) != 2 || f[0].Shoot != "a" || f[1].Shoot != "b" {
		t.Fatalf("favorites: %+v", f)
	}
	if r := got.Recent(); len(r) != 1 || r[0].Shoot != "a" {
		t.Fatalf("recent: %+v", r)
	}
	if l := got.Location(); len(l) != 1 {
		t.Fatalf("location: %+v", l)
	}
}

func TestSharedFileConcurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")
	const n = 10
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		s, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.AddRecent(session.Target{Garden: "g", Project: "p", Shoot: fmt.Sprint(i)}, "x"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if r := got.Recent(); len(r) != n {
		t.Fatalf("got %d recent entries, want %d: %+v", len(r), n, r)
	}
}
