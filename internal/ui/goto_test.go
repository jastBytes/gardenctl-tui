// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"reflect"
	"testing"

	"github.com/jastbytes/gardenctl-tui/internal/gardener/fake"
)

func TestParseTechnicalID(t *testing.T) {
	for _, tc := range []struct {
		id, project, shoot string
		ok                 bool
	}{
		{"shoot--alpha--api-gw", "alpha", "api-gw", true},
		{"shoot--garden--core", "garden", "core", true},
		{"shoot--alpha", "", "", false},
		{"shoot--alpha--", "", "", false},
		{"shoot--a--b--c", "", "", false},
		{"alpha/api-gw", "", "", false},
	} {
		p, s, ok := ParseTechnicalID(tc.id)
		if p != tc.project || s != tc.shoot || ok != tc.ok {
			t.Errorf("ParseTechnicalID(%q) = %q, %q, %v", tc.id, p, s, ok)
		}
	}
}

func TestGotoTechnicalID(t *testing.T) {
	tm := startApp(t, testOptions(fake.Landscape()))
	waitFor(t, tm, "live")
	send(tm, "enter")
	waitFor(t, tm, "garden-ops")
	send(tm, "ctrl+g", " shoot--beta--db ", "enter")
	waitFor(t, tm, "Technical ID")
	app := quitAndSave(t, tm)
	if got := titles(app); !reflect.DeepEqual(got, []string{"gardens", "dev", "beta", "db"}) {
		t.Fatalf("stack: %v", got)
	}
}

func TestGotoTechnicalIDNotFound(t *testing.T) {
	tm := startApp(t, testOptions(fake.Landscape()))
	waitFor(t, tm, "live")
	send(tm, "ctrl+g", "shoot--beta--nope", "enter")
	waitFor(t, tm, "no shoot with technical ID shoot--beta--nope")
	send(tm, "ctrl+g", "foo", "enter")
	waitFor(t, tm, "is not a technical ID")
}

func TestStartShoot(t *testing.T) {
	opts := testOptions(fake.Landscape())
	opts.StartShoot = "shoot--alpha--web"
	tm := startApp(t, opts)
	waitFor(t, tm, "Technical ID")
	app := quitAndSave(t, tm)
	if got := titles(app); !reflect.DeepEqual(got, []string{"gardens", "dev", "alpha", "web"}) {
		t.Fatalf("stack: %v", got)
	}
}
