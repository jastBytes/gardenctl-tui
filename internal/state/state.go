// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

// Package state persists recently used targets and favorites.
package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/jastbytes/gardenctl-tui/internal/session"
)

// MaxRecent is the number of recent targets that are kept.
const MaxRecent = 20

// Entry is a stored target.
type Entry struct {
	Garden    string    `json:"garden"`
	Project   string    `json:"project,omitempty"`
	Seed      string    `json:"seed,omitempty"`
	Shoot     string    `json:"shoot,omitempty"`
	Namespace string    `json:"namespace,omitempty"`
	Tool      string    `json:"tool,omitempty"`
	Used      time.Time `json:"used,omitempty"`
}

// Target converts the entry into a launch target.
func (e Entry) Target() session.Target {
	return session.Target{Garden: e.Garden, Project: e.Project, Seed: e.Seed, Shoot: e.Shoot, Namespace: e.Namespace}
}

// Key identifies the target (independent of tool and time).
func (e Entry) Key() string { return e.Target().String() }

// FromTarget creates an entry (control-plane flag is not stored).
func FromTarget(t session.Target) Entry {
	return Entry{Garden: t.Garden, Project: t.Project, Seed: t.Seed, Shoot: t.Shoot, Namespace: t.Namespace}
}

// Crumb is one level of the navigation path: a view and its selected row.
type Crumb struct {
	View     string `json:"view"`
	Selected string `json:"selected,omitempty"`
}

type file struct {
	Recent    []Entry `json:"recent,omitempty"`
	Favorites []Entry `json:"favorites,omitempty"`
	Location  []Crumb `json:"location,omitempty"`
}

// Store holds recent targets and favorites and saves them to a YAML file.
type Store struct {
	mu   sync.Mutex
	path string
	data file
	now  func() time.Time
}

// Load reads the store from path; a missing file yields an empty store.
func Load(path string) (*Store, error) {
	s := &Store{path: path, now: time.Now}
	if path == "" {
		return s, nil
	}
	buf, err := os.ReadFile(path) // #nosec G304 -- gtui state file
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(buf, &s.data); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return s, nil
}

// Recent returns the recent targets, newest first.
func (s *Store) Recent() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Entry(nil), s.data.Recent...)
}

// Favorites returns the favorites in the order they were added.
func (s *Store) Favorites() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Entry(nil), s.data.Favorites...)
}

// IsFavorite reports whether t is a favorite.
func (s *Store) IsFavorite(t session.Target) bool {
	key := FromTarget(t).Key()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.data.Favorites {
		if f.Key() == key {
			return true
		}
	}
	return false
}

// Location returns the last saved navigation path.
func (s *Store) Location() []Crumb {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Crumb(nil), s.data.Location...)
}

// SetLocation stores the navigation path and saves the store.
func (s *Store) SetLocation(loc []Crumb) error {
	s.mu.Lock()
	s.data.Location = append([]Crumb(nil), loc...)
	s.mu.Unlock()
	return s.save()
}

// AddRecent records a launch of tool on t and saves the store.
func (s *Store) AddRecent(t session.Target, tool string) error {
	e := FromTarget(t)
	e.Tool, e.Used = tool, s.now().UTC().Truncate(time.Second)
	s.mu.Lock()
	out := []Entry{e}
	for _, r := range s.data.Recent {
		if r.Key() != e.Key() && len(out) < MaxRecent {
			out = append(out, r)
		}
	}
	s.data.Recent = out
	s.mu.Unlock()
	return s.save()
}

// ToggleFavorite adds or removes t from the favorites, saves, and returns the new state.
func (s *Store) ToggleFavorite(t session.Target) (bool, error) {
	e := FromTarget(t)
	s.mu.Lock()
	var out []Entry
	removed := false
	for _, f := range s.data.Favorites {
		if f.Key() == e.Key() {
			removed = true
			continue
		}
		out = append(out, f)
	}
	if !removed {
		out = append(out, e)
	}
	s.data.Favorites = out
	s.mu.Unlock()
	return !removed, s.save()
}

func (s *Store) save() error {
	if s.path == "" {
		return nil
	}
	s.mu.Lock()
	buf, err := yaml.Marshal(s.data)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
