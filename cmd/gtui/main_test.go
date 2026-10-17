// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

func TestParseFlagsShoot(t *testing.T) {
	o, err := parseFlags([]string{"--shoot", "shoot--alpha--web"})
	if err != nil || o.shoot != "shoot--alpha--web" {
		t.Fatalf("parseFlags: %+v, %v", o, err)
	}
	if _, err := parseFlags([]string{"--shoot", "alpha/web"}); err == nil {
		t.Fatal("expected an error for an invalid technical ID")
	}
}
