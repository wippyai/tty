// SPDX-License-Identifier: MPL-2.0

package conformance

// The specification tables in this package re-express documented xterm
// behavior (XTerm Control Sequences, "ctlseqs", and the DEC VT510 reference)
// as table tests. They are written from those references. The esctest2 suite
// (GPL-2.0) is a source of ideas for which behaviors matter; none of its code
// or data is copied.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/wippyai/tty/text"
)

const (
	defCols = 10
	defRows = 5

	// fill5 leaves lines "1".."5" on a 5-row screen with the cursor at (1,4).
	fill5 = "1\r\n2\r\n3\r\n4\r\n5"
	// alnFill paints the screen with E via DECALN and homes the cursor.
	alnFill = "\x1b#8"
)

type pos struct{ x, y int }

func at(x, y int) *pos { return &pos{x, y} }

type specCase struct {
	name       string
	cols, rows int // zero selects 10x5
	in         string
	want       []string // visible rows, missing rows are blank; nil skips the grid check
	cursor     *pos     // zero-based; nil skips the cursor check
	reply      string   // regexp the whole reply stream must match; empty means no reply is expected
	replyAny   bool     // skip the reply check
	check      func(t *testing.T, s *sut)
}

// exact returns a regexp matching s literally.
func exact(s string) string { return "^" + regexp.QuoteMeta(s) + "$" }

func (c specCase) size() (int, int) {
	cols, rows := c.cols, c.rows
	if cols == 0 {
		cols = defCols
	}
	if rows == 0 {
		rows = defRows
	}
	return cols, rows
}

func (c specCase) expectedGrid(rows int) []string {
	g := make([]string, rows)
	copy(g, c.want)
	return g
}

func diffGrid(want, got []string) string {
	var b strings.Builder
	for y := range want {
		g := ""
		if y < len(got) {
			g = got[y]
		}
		mark := " "
		if want[y] != g {
			mark = "!"
		}
		fmt.Fprintf(&b, "%s row %d want %q got %q\n", mark, y, want[y], g)
	}
	return b.String()
}

// compareSpec checks the grid, cursor and replies of an emulator against c.
func compareSpec(c specCase, e emu, rows int) []string {
	var problems []string
	if c.want != nil {
		want, got := c.expectedGrid(rows), gridOf(e, rows)
		if joinGrid(want) != joinGrid(got) {
			problems = append(problems, "grid:\n"+diffGrid(want, got))
		}
	}
	if c.cursor != nil {
		x, y := e.Cursor()
		if x != c.cursor.x || y != c.cursor.y {
			problems = append(problems, fmt.Sprintf("cursor want (%d,%d) got (%d,%d)", c.cursor.x, c.cursor.y, x, y))
		}
	}
	if !c.replyAny {
		r := e.Replies()
		if c.reply == "" {
			if r != "" {
				problems = append(problems, fmt.Sprintf("unexpected reply %q", r))
			}
		} else if !regexp.MustCompile(c.reply).MatchString(r) {
			problems = append(problems, fmt.Sprintf("reply %q does not match /%s/", r, c.reply))
		}
	}
	return problems
}

func runSpec(t *testing.T, cases []specCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cols, rows := c.size()
			s := newSUT(cols, rows)
			requireNoPanic(t, c.name, func() { s.Write(c.in) })
			for _, p := range compareSpec(c, s, rows) {
				t.Error(p)
			}
			if c.check != nil {
				c.check(t, s)
			}
		})
	}
}

func styleOf(f func(*text.Style)) text.Style {
	var s text.Style
	f(&s)
	return s
}

// canon maps palette indexes 0-15 to the ANSI basic colors; both spell the
// same palette entry.
func canon(c text.Color) text.Color {
	if c.Kind() == text.ColorIndexed && c.Index() < 16 {
		return text.Basic(c.Index())
	}
	return c
}

func canonStyle(s text.Style) text.Style {
	s.Fg, s.Bg, s.UnderlineColor = canon(s.Fg), canon(s.Bg), canon(s.UnderlineColor)
	return s
}

func requireStyle(t *testing.T, s *sut, x, y int, want text.Style) {
	t.Helper()
	if got := s.cell(x, y).Style; canonStyle(got) != canonStyle(want) {
		t.Errorf("cell (%d,%d) style want %+v got %+v", x, y, want, got)
	}
}

func requireBgRow(t *testing.T, s *sut, y, from, to int, bg text.Color) {
	t.Helper()
	for x := from; x < to; x++ {
		if got := s.cell(x, y).Style.Bg; canon(got) != canon(bg) {
			t.Errorf("cell (%d,%d) bg want %+v got %+v", x, y, bg, got)
		}
	}
}
