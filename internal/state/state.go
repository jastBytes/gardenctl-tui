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

	"golang.org/x/sys/unix"
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
// Several gtui instances can share the file: every change is applied to the
// file's current content under a file lock (see update).
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
	data, err := read(path)
	if err != nil {
		return nil, err
	}
	s.data = data
	return s, nil
}

// read parses the file at path; a missing file yields empty data.
func read(path string) (file, error) {
	var data file
	buf, err := os.ReadFile(path) // #nosec G304 -- gtui state file
	if errors.Is(err, os.ErrNotExist) {
		return data, nil
	}
	if err != nil {
		return data, err
	}
	if err := yaml.Unmarshal(buf, &data); err != nil {
		return data, fmt.Errorf("parse %s: %w", path, err)
	}
	return data, nil
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
	loc = append([]Crumb(nil), loc...)
	return s.update(func(d *file) { d.Location = loc })
}

// AddRecent records a launch of tool on t and saves the store.
func (s *Store) AddRecent(t session.Target, tool string) error {
	e := FromTarget(t)
	e.Tool, e.Used = tool, s.now().UTC().Truncate(time.Second)
	return s.update(func(d *file) {
		out := []Entry{e}
		for _, r := range d.Recent {
			if r.Key() != e.Key() && len(out) < MaxRecent {
				out = append(out, r)
			}
		}
		d.Recent = out
	})
}

// ToggleFavorite adds or removes t from the favorites, saves, and returns the new state.
// Whether t is added depends on what this store showed: if another instance
// added t meanwhile, it stays a favorite.
func (s *Store) ToggleFavorite(t session.Target) (bool, error) {
	e := FromTarget(t)
	add := !s.IsFavorite(t)
	return add, s.update(func(d *file) {
		var out []Entry
		for _, f := range d.Favorites {
			if f.Key() != e.Key() {
				out = append(out, f)
			}
		}
		if add {
			out = append(out, e)
		}
		d.Favorites = out
	})
}

// update applies change to the store and saves it. With a file, change is
// applied to the file's current content (changes of other gtui instances
// included) while holding a lock on path.lock, so that no instance loses
// the changes of another.
func (s *Store) update(change func(*file)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		change(&s.data)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	lock, err := os.OpenFile(s.path+".lock", os.O_CREATE|os.O_RDWR, 0o600) // #nosec G304 -- gtui state file
	if err != nil {
		return err
	}
	defer func() { _ = lock.Close() }() // closing releases the lock
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return fmt.Errorf("lock %s: %w", lock.Name(), err)
	}
	data, err := read(s.path)
	if err != nil {
		return err
	}
	change(&data)
	buf, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}
	s.data = data
	return nil
}
