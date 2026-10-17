// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package styles

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jastbytes/gardenctl-tui/internal/config"
)

// Theme is the set of colors the UI is drawn with. The status colors keep
// their meaning in every theme: green healthy, yellow progressing, red error,
// grey hibernated/unknown.
type Theme struct {
	Green, Yellow, Red, Grey lipgloss.TerminalColor
	// Accent colors the logo, the cursor, sections and borders.
	Accent lipgloss.TerminalColor
	// SelBg is the background of selected rows.
	SelBg lipgloss.TerminalColor
}

// DefaultTheme is used when no theme is configured.
const DefaultTheme = "default"

// ac is a color with a variant for light and one for dark terminals.
func ac(light, dark string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

// Themes are the built-in themes. Each takes its colors from the official
// palette and has a light and a dark variant; lipgloss picks the one that
// matches the terminal's background.
var Themes = map[string]Theme{
	// GitHub Primer.
	DefaultTheme: {
		Green:  ac("#1a7f37", "#3fb950"),
		Yellow: ac("#9a6700", "#d29922"),
		Red:    ac("#cf222e", "#f85149"),
		Grey:   ac("#6e7781", "#8b949e"),
		Accent: ac("#0969da", "#58a6ff"),
		SelBg:  ac("#ddf4ff", "#1f3a5f"),
	},
	// https://catppuccin.com/palette – Latte / Mocha.
	"catppuccin": {
		Green:  ac("#40a02b", "#a6e3a1"),
		Yellow: ac("#df8e1d", "#f9e2af"),
		Red:    ac("#d20f39", "#f38ba8"),
		Grey:   ac("#8c8fa1", "#7f849c"),
		Accent: ac("#8839ef", "#cba6f7"),
		SelBg:  ac("#ccd0da", "#313244"),
	},
	// https://draculatheme.com/spec – Alucard / Dracula.
	"dracula": {
		Green:  ac("#14710a", "#50fa7b"),
		Yellow: ac("#846e15", "#f1fa8c"),
		Red:    ac("#cb3a2a", "#ff5555"),
		Grey:   ac("#6c664b", "#6272a4"),
		Accent: ac("#644ac9", "#bd93f9"),
		SelBg:  ac("#cfcfde", "#44475a"),
	},
	// https://github.com/morhetz/gruvbox – light / dark.
	"gruvbox": {
		Green:  ac("#79740e", "#b8bb26"),
		Yellow: ac("#b57614", "#fabd2f"),
		Red:    ac("#9d0006", "#fb4934"),
		Grey:   ac("#7c6f64", "#928374"),
		Accent: ac("#076678", "#83a598"),
		SelBg:  ac("#d5c4a1", "#504945"),
	},
	// https://rosepinetheme.com/palette – Dawn / Main. Rosé Pine has no
	// green; pine and foam take its place.
	"rose-pine": {
		Green:  ac("#286983", "#9ccfd8"),
		Yellow: ac("#ea9d34", "#f6c177"),
		Red:    ac("#b4637a", "#eb6f92"),
		Grey:   ac("#9893a5", "#6e6a86"),
		Accent: ac("#907aa9", "#c4a7e7"),
		SelBg:  ac("#dfdad9", "#403d52"),
	},
	// https://ethanschoonover.com/solarized – light / dark.
	"solarized": {
		Green:  ac("#859900", "#859900"),
		Yellow: ac("#b58900", "#b58900"),
		Red:    ac("#dc322f", "#dc322f"),
		Grey:   ac("#93a1a1", "#586e75"),
		Accent: ac("#268bd2", "#268bd2"),
		SelBg:  ac("#eee8d5", "#073642"),
	},
	// https://github.com/folke/tokyonight.nvim – Day / Night.
	"tokyonight": {
		Green:  ac("#587539", "#9ece6a"),
		Yellow: ac("#8c6c3e", "#e0af68"),
		Red:    ac("#f52a65", "#f7768e"),
		Grey:   ac("#848cb5", "#737aa2"),
		Accent: ac("#2e7de9", "#7aa2f7"),
		SelBg:  ac("#b7c1e3", "#283457"),
	},
	// No colors at all: bold text and reverse video only, like NO_COLOR.
	"mono": {
		Green:  lipgloss.NoColor{},
		Yellow: lipgloss.NoColor{},
		Red:    lipgloss.NoColor{},
		Grey:   lipgloss.NoColor{},
		Accent: lipgloss.NoColor{},
		SelBg:  lipgloss.NoColor{},
	},
}

// ThemeNames returns the names of the built-in themes, sorted.
func ThemeNames() []string {
	names := make([]string, 0, len(Themes))
	for n := range Themes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Apply switches the UI to the named theme (empty: DefaultTheme) with the
// given colors overridden. Keys of colors: green, yellow, red, grey, accent,
// selBg. Call it before the UI is built.
func Apply(name string, colors map[string]config.Color) error {
	if name == "" {
		name = DefaultTheme
	}
	t, ok := Themes[name]
	if !ok {
		return fmt.Errorf("theme: unknown theme %q (%s)", name, strings.Join(ThemeNames(), ", "))
	}
	fields := map[string]*lipgloss.TerminalColor{
		"green": &t.Green, "yellow": &t.Yellow, "red": &t.Red,
		"grey": &t.Grey, "accent": &t.Accent, "selBg": &t.SelBg,
	}
	keys := make([]string, 0, len(colors))
	for k := range colors {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		f, ok := fields[k]
		if !ok {
			return fmt.Errorf("colors: unknown color %q (green, yellow, red, grey, accent, selBg)", k)
		}
		c, err := override(colors[k])
		if err != nil {
			return fmt.Errorf("colors.%s: %w", k, err)
		}
		*f = c
	}
	setTheme(t)
	return nil
}

// override turns a configured color into a lipgloss color. A missing variant
// falls back to the other one.
func override(c config.Color) (lipgloss.TerminalColor, error) {
	light, dark := c.Light, c.Dark
	if light == "" {
		light = dark
	}
	if dark == "" {
		dark = light
	}
	for _, v := range []string{light, dark} {
		if !validColor(v) {
			return nil, fmt.Errorf("invalid color %q (#rgb, #rrggbb or 0-255)", v)
		}
	}
	return ac(light, dark), nil
}

var hexColor = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// validColor accepts what lipgloss understands: a hex color or an ANSI color
// number.
func validColor(s string) bool {
	if hexColor.MatchString(s) {
		return true
	}
	n, err := strconv.Atoi(s)
	return err == nil && n >= 0 && n <= 255
}
