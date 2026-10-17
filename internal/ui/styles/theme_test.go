// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package styles

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/jastbytes/gardenctl-tui/internal/config"
)

func TestThemesComplete(t *testing.T) {
	for name, th := range Themes {
		for _, c := range []lipgloss.TerminalColor{th.Green, th.Yellow, th.Red, th.Grey, th.Accent, th.SelBg} {
			if c == nil {
				t.Errorf("%s: missing color", name)
				continue
			}
			if a, ok := c.(lipgloss.AdaptiveColor); ok && (!validColor(a.Light) || !validColor(a.Dark)) {
				t.Errorf("%s: invalid color %+v", name, a)
			}
		}
	}
}

func TestApply(t *testing.T) {
	t.Cleanup(func() { _ = Apply("", nil) })

	if err := Apply("dracula", map[string]config.Color{"accent": {Light: "#ff79c6"}}); err != nil {
		t.Fatal(err)
	}
	if ColorRed != Themes["dracula"].Red {
		t.Fatalf("red: %v", ColorRed)
	}
	if ColorAccent != ac("#ff79c6", "#ff79c6") {
		t.Fatalf("accent: %v", ColorAccent)
	}
	if Logo.GetForeground() != ColorAccent {
		t.Fatal("styles not rebuilt")
	}

	if err := Apply("mono", nil); err != nil {
		t.Fatal(err)
	}
	if !Selected.GetReverse() {
		t.Fatal("mono: selection not reversed")
	}

	if err := Apply("", nil); err != nil || ColorAccent != Themes[DefaultTheme].Accent || Selected.GetReverse() {
		t.Fatalf("default: %v", err)
	}
}

func TestApplyErrors(t *testing.T) {
	t.Cleanup(func() { _ = Apply("", nil) })
	for _, tc := range []struct {
		name   string
		colors map[string]config.Color
		want   string
	}{
		{"nope", nil, `unknown theme "nope"`},
		{"", map[string]config.Color{"blue": {Light: "#000"}}, `unknown color "blue"`},
		{"", map[string]config.Color{"red": {Light: "red"}}, `colors.red: invalid color "red"`},
		{"", map[string]config.Color{"red": {Dark: "256"}}, `invalid color "256"`},
	} {
		err := Apply(tc.name, tc.colors)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Apply(%q, %v) = %v, want %q", tc.name, tc.colors, err, tc.want)
		}
	}
}
