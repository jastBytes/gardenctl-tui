// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jastbytes/gardenctl-tui/internal/gardener"
	"github.com/jastbytes/gardenctl-tui/internal/ui/styles"
)

// sortMode is the order of the shoot lists (cycled with "o").
type sortMode int

const (
	sortName sortMode = iota
	sortStatus
	sortVersion
	sortUpdated
	sortModes
)

func (m sortMode) String() string {
	switch m {
	case sortStatus:
		return "status"
	case sortVersion:
		return "version"
	case sortUpdated:
		return "last change"
	default:
		return "name"
	}
}

// describe explains the order for the status line.
func (m sortMode) describe() string {
	switch m {
	case sortStatus:
		return "sorted by status (problems first)"
	case sortVersion:
		return "sorted by Kubernetes version (oldest first)"
	case sortUpdated:
		return "sorted by last operation (newest first)"
	default:
		return "sorted by name"
	}
}

// statusRank orders statuses for sorting: problems first, hibernated last.
func statusRank(s gardener.Status) int {
	switch s {
	case gardener.StatusError:
		return 0
	case gardener.StatusProgressing:
		return 1
	case gardener.StatusUnknown:
		return 2
	case gardener.StatusHealthy:
		return 3
	default:
		return 4
	}
}

// sortShoots returns the shoots in the order of m. sortName keeps the API
// order (namespace, name); ties keep it as well.
func sortShoots(shoots []gardener.Shoot, m sortMode) []gardener.Shoot {
	out := append([]gardener.Shoot(nil), shoots...)
	var less func(a, b gardener.Shoot) bool
	switch m {
	case sortStatus:
		less = func(a, b gardener.Shoot) bool { return statusRank(a.Status) < statusRank(b.Status) }
	case sortVersion:
		less = func(a, b gardener.Shoot) bool { return compareVersions(a.Version, b.Version) < 0 }
	case sortUpdated:
		less = func(a, b gardener.Shoot) bool { return a.LastOp.Updated.After(b.LastOp.Updated) }
	default:
		return out
	}
	sort.SliceStable(out, func(i, j int) bool { return less(out[i], out[j]) })
	return out
}

// compareVersions compares dotted versions numerically ("1.9.0" < "1.10.0").
func compareVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := range max(len(as), len(bs)) {
		x, y := "0", "0" // "1.33" == "1.33.0"
		if i < len(as) && as[i] != "" {
			x = as[i]
		}
		if i < len(bs) && bs[i] != "" {
			y = bs[i]
		}
		xi, xerr := strconv.Atoi(x)
		yi, yerr := strconv.Atoi(y)
		switch {
		case xerr == nil && yerr == nil && xi != yi:
			if xi < yi {
				return -1
			}
			return 1
		case (xerr != nil || yerr != nil) && x != y:
			return strings.Compare(x, y)
		}
	}
	return 0
}

// summaryOrder lists the statuses in the summary line, problems first.
var summaryOrder = []gardener.Status{
	gardener.StatusError, gardener.StatusProgressing, gardener.StatusUnknown,
	gardener.StatusHealthy, gardener.StatusHibernated,
}

// shootSummary renders e.g. "5 shoots  ✗ 1 error  ✓ 3 healthy  ☾ 1 hibernated".
func shootSummary(shoots []gardener.Shoot, hidden int) string {
	counts := map[gardener.Status]int{}
	for _, s := range shoots {
		counts[s.Status]++
	}
	noun := "shoots"
	if len(shoots) == 1 {
		noun = "shoot"
	}
	parts := []string{fmt.Sprintf("%d %s", len(shoots), noun)}
	for _, st := range summaryOrder {
		if n := counts[st]; n > 0 {
			dot := lipgloss.NewStyle().Foreground(styles.StatusColor(st)).Render(styles.StatusSymbol(st))
			parts = append(parts, fmt.Sprintf("%s %d %s", dot, n, strings.ToLower(st.String())))
		}
	}
	out := strings.Join(parts, "  ")
	if hidden > 0 {
		out += styles.Dim.Render(fmt.Sprintf("  (%d hidden: problems only)", hidden))
	}
	return out
}

// shootCountCell summarises the shoots of a project, e.g. "5 ✗1 ⟳2", colored
// with the worst status.
func shootCountCell(shoots []gardener.Shoot) (string, lipgloss.TerminalColor) {
	if len(shoots) == 0 {
		return "0", styles.ColorGrey
	}
	counts := map[gardener.Status]int{}
	for _, s := range shoots {
		counts[s.Status]++
	}
	text := strconv.Itoa(len(shoots))
	for _, p := range []struct {
		st  gardener.Status
		sym string
	}{{gardener.StatusError, "✗"}, {gardener.StatusProgressing, "⟳"}, {gardener.StatusUnknown, "?"}} {
		if n := counts[p.st]; n > 0 {
			text += fmt.Sprintf(" %s%d", p.sym, n)
		}
	}
	for _, st := range summaryOrder {
		if counts[st] > 0 {
			return text, styles.StatusColor(st)
		}
	}
	return text, nil
}
