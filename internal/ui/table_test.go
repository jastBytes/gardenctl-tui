// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"strings"
	"testing"
)

func rows(keys ...string) []row {
	out := make([]row, 0, len(keys))
	for _, k := range keys {
		out = append(out, row{key: k, cells: []string{k}})
	}
	return out
}

func TestTableKeepsSelectionOnRefresh(t *testing.T) {
	tb := newTable(column{title: "name"})
	tb.setRows(rows("a", "b", "c"))
	tb.move(2)
	tb.setRows(rows("x", "c", "a"))
	if r, _ := tb.selected(); r.key != "c" {
		t.Fatalf("selected %q, want c", r.key)
	}
	tb.setRows(rows("a"))
	if r, _ := tb.selected(); r.key != "a" {
		t.Fatalf("selected %q, want a after shrink", r.key)
	}
	tb.setRows(nil)
	if _, ok := tb.selected(); ok {
		t.Fatal("no selection expected on empty table")
	}
}

func TestTableFilter(t *testing.T) {
	tb := newTable(column{title: "name"})
	tb.setRows(rows("api-gateway", "batch", "web"))
	tb.filter.SetValue("bt")
	tb.applyFilter()
	if len(tb.visible) != 1 || tb.visible[0].key != "batch" {
		t.Fatalf("visible: %+v", tb.visible)
	}
	tb.clearFilter()
	if len(tb.visible) != 3 {
		t.Fatal("clear filter")
	}
}

func TestTableScrollsToCursor(t *testing.T) {
	tb := newTable(column{title: "name"})
	var keys []string
	for i := 0; i < 50; i++ {
		keys = append(keys, string(rune('a'+i%26))+strings.Repeat("x", i/26))
	}
	tb.setRows(rows(keys...))
	tb.move(30)
	out := tb.view(40, 6)
	if !strings.Contains(out, keys[30]) || strings.Contains(out, "\n"+keys[0]+" ") {
		t.Fatalf("cursor row not visible:\n%s", out)
	}
	if n := strings.Count(out, "\n"); n != 5 {
		t.Fatalf("rendered %d lines, want 6", n+1)
	}
}

func TestTableShrinksColumns(t *testing.T) {
	tb := newTable(column{title: "a"}, column{title: "b"})
	tb.setRows([]row{{key: "1", cells: []string{strings.Repeat("x", 50), strings.Repeat("y", 50)}}})
	w := tb.widths(40)
	if w[0]+w[1]+2 > 40 {
		t.Fatalf("widths %v exceed 40", w)
	}
}

func TestTableDropsOptionalColumns(t *testing.T) {
	tb := newTable(column{title: "name"}, column{title: "extra", drop: 1}, column{title: "more", drop: 2}, column{title: "status"})
	tb.setRows([]row{{key: "1", cells: []string{"shoot", "xxxxxxxxxx", "yyyyyyyyyy", "healthy"}}})
	if w := tb.widths(60); w[1] < 0 || w[2] < 0 {
		t.Fatalf("enough room, yet columns hidden: %v", w)
	}
	if w := tb.widths(30); w[1] < 0 || w[2] != -1 {
		t.Fatalf("width 30 should hide only the most optional column: %v", w)
	}
	w := tb.widths(16)
	if w[1] != -1 || w[2] != -1 || w[0] < 5 || w[3] < 7 {
		t.Fatalf("width 16 should hide both optional columns: %v", w)
	}
	out := tb.view(18, 3)
	if strings.Contains(out, "EXTRA") || !strings.Contains(out, "STATUS") {
		t.Fatalf("hidden column rendered:\n%s", out)
	}
}

func TestWrap(t *testing.T) {
	got := wrap("one two three four five", 10, "> ")
	if got != "> one two\n> three four\n> five" {
		t.Fatalf("%q", got)
	}
}
