// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"syscall"
	"testing"
	"time"
)

func TestParseFlagsShoot(t *testing.T) {
	o, err := parseFlags([]string{"--shoot", "shoot--alpha--web"})
	if err != nil || o.shoot != "shoot--alpha--web" {
		t.Fatalf("parseFlags: %+v, %v", o, err)
	}
	if _, err := parseFlags([]string{"--shoot", "alpha/web"}); err == nil {
		t.Fatal("expected an error for an invalid technical ID")
	}
}

type quitter chan struct{}

func (q quitter) Quit() { q <- struct{}{} }

func TestCatchHangup(t *testing.T) {
	quitOn := catchHangup()
	// A SIGHUP during start-up, before the program exists, must not be lost.
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	q := make(quitter, 1)
	quitOn(q)
	select {
	case <-q:
	case <-time.After(3 * time.Second):
		t.Fatal("SIGHUP did not quit the program")
	}
}
