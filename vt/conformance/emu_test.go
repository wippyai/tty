// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"strings"
	"testing"

	xterm "github.com/gitpod-io/xterm-go"

	"github.com/wippyai/tty/vt"
	"github.com/wippyai/tty/vt/screen"
)

// emu is the observable surface both the emulator under test and the
// reference oracle expose to the harness.
type emu interface {
	Write(s string)
	// Row returns visible row y as text with trailing blanks trimmed. Untouched
	// cells read as spaces; the trailing half of a wide character is omitted.
	Row(y int) string
	Cursor() (x, y int)
	Resize(cols, rows int)
	Size() (cols, rows int)
	// Replies drains the bytes the terminal sent back to the child.
	Replies() string
}

// sut wraps the emulator under test.
type sut struct {
	term       *vt.Terminal
	cols, rows int
	replies    strings.Builder
	titles     []string
	bells      int
	clips      []clip
}

type clip struct {
	selection string
	data      string
}

const scrollbackLines = 1000

func newSUT(cols, rows int) *sut {
	s := &sut{cols: cols, rows: rows}
	s.term = vt.New(vt.Options{
		Cols:            cols,
		Rows:            rows,
		ScrollbackLines: scrollbackLines,
		Reply:           func(p []byte) { s.replies.Write(p) },
		Title:           func(t string) { s.titles = append(s.titles, t) },
		Clipboard:       func(sel string, d []byte) { s.clips = append(s.clips, clip{sel, string(d)}) },
		Bell:            func() { s.bells++ },
	})
	return s
}

func (s *sut) Write(str string) {
	if _, err := s.term.Write([]byte(str)); err != nil {
		panic(err)
	}
}

func (s *sut) Resize(cols, rows int) {
	s.cols, s.rows = cols, rows
	s.term.Resize(cols, rows)
}

func (s *sut) Size() (int, int) { return s.cols, s.rows }

func (s *sut) Row(y int) string { return cellsText(s.term.Screen().Line(y)) }

func (s *sut) Cursor() (int, int) {
	c := s.term.Screen().Cursor()
	return c.X, c.Y
}

func (s *sut) Replies() string {
	r := s.replies.String()
	s.replies.Reset()
	return r
}

func (s *sut) scr() screen.Screen { return s.term.Screen() }

func (s *sut) cell(x, y int) screen.Cell { return s.term.Screen().Line(y)[x] }

func cellsText(cells []screen.Cell) string {
	var b strings.Builder
	prevWide := false
	for _, c := range cells {
		switch {
		case prevWide && c.Cluster == "":
			prevWide = false
			continue
		case c.Cluster == "":
			b.WriteByte(' ')
		default:
			b.WriteString(c.Cluster)
		}
		prevWide = c.Wide
	}
	return strings.TrimRight(b.String(), " ")
}

// oracle wraps the reference emulator github.com/gitpod-io/xterm-go.
type oracle struct {
	term    *xterm.Terminal
	cols    int
	rows    int
	replies strings.Builder
}

func newOracle(cols, rows int) *oracle {
	o := &oracle{cols: cols, rows: rows}
	o.term = xterm.New(xterm.WithCols(cols), xterm.WithRows(rows), xterm.WithScrollback(scrollbackLines))
	o.term.OnData(func(d string) { o.replies.WriteString(d) })
	return o
}

func (o *oracle) Resize(cols, rows int) {
	o.cols, o.rows = cols, rows
	o.term.Resize(cols, rows)
}

func (o *oracle) Size() (int, int) { return o.cols, o.rows }

func (o *oracle) Write(s string) { o.term.WriteString(s) }

func (o *oracle) Row(y int) string { return strings.TrimRight(o.term.GetLine(y), " ") }

// Cursor reports the oracle cursor with its deferred-wrap representation
// normalized: xterm.js style emulators park x at cols while a wrap is pending,
// this harness (like xterm) reports cols-1.
func (o *oracle) Cursor() (int, int) {
	x, y := o.term.CursorX(), o.term.CursorY()
	if x >= o.cols {
		x = o.cols - 1
	}
	return x, y
}

func (o *oracle) Replies() string {
	r := o.replies.String()
	o.replies.Reset()
	return r
}

func gridOf(e emu, rows int) []string {
	g := make([]string, rows)
	for y := range g {
		g[y] = e.Row(y)
	}
	return g
}

func joinGrid(g []string) string {
	var b strings.Builder
	for y, r := range g {
		b.WriteString(strings.TrimRight(r, " "))
		if y < len(g)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func requireNoPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s: panic: %v", name, r)
		}
	}()
	f()
}
