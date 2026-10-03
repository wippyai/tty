// SPDX-License-Identifier: MPL-2.0

package screen

import (
	"strings"
	"testing"

	"github.com/wippyai/tty/text"
)

// put drives s with printable text; \n is CarriageReturn plus LineFeed, \r is CarriageReturn,
// \b is Backspace, \t is Tab(1).
func put(s Screen, str string) {
	for _, r := range str {
		switch r {
		case '\n':
			s.CarriageReturn()
			s.LineFeed()
		case '\r':
			s.CarriageReturn()
		case '\b':
			s.Backspace()
		case '\t':
			s.Tab(1)
		default:
			c := string(r)
			s.Print(c, text.ClusterWidth(c))
		}
	}
}

func cellsText(cells []Cell) string {
	var b strings.Builder
	for i, c := range cells {
		if c.Cluster == "" {
			if i > 0 && cells[i-1].Wide {
				continue
			}
			b.WriteByte(' ')
			continue
		}
		b.WriteString(c.Cluster)
	}
	return strings.TrimRight(b.String(), " ")
}

func rowText(s Screen, y int) string { return cellsText(s.Line(y)) }

func screenText(s Screen) []string {
	_, rows := s.Size()
	out := make([]string, rows)
	for y := range out {
		out[y] = rowText(s, y)
	}
	return out
}

func sbText(s Screen) []string {
	out := make([]string, s.ScrollbackLen())
	for i := range out {
		out[i] = cellsText(s.ScrollbackLine(i))
	}
	return out
}

func eqLines(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d: got %q, want %q (all %q)", i, got[i], want[i], got)
		}
	}
}

func eqCursor(t *testing.T, s Screen, x, y int) {
	t.Helper()
	c := s.Cursor()
	if c.X != x || c.Y != y {
		t.Fatalf("cursor (%d,%d), want (%d,%d)", c.X, c.Y, x, y)
	}
}

func eqPending(t *testing.T, s Screen, want bool) {
	t.Helper()
	if got := s.Cursor().PendingWrap; got != want {
		t.Fatalf("pending wrap %v, want %v", got, want)
	}
}

func redBg() text.Style {
	var st text.Style
	st.ApplySGR("41")
	return st
}

func setPen(s Screen, st text.Style) {
	c := s.Cursor()
	c.Style = st
	s.SetCursor(c)
}

// numbered fills each of the rows with a distinct letter.
func numbered(s Screen) {
	_, rows := s.Size()
	for y := 0; y < rows; y++ {
		s.MoveTo(0, y)
		put(s, strings.Repeat(string(rune('a'+y)), 3))
	}
	s.MoveTo(0, 0)
}
