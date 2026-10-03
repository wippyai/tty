// SPDX-License-Identifier: MPL-2.0

package screen

import "github.com/wippyai/tty/text"

const defaultTabWidth = 8

// line is one grid row. wrapped marks a soft wrap into the next row; padded
// marks that the last cell is a blank left because a wide character did not
// fit and moved to the next row.
type line struct {
	cells   []Cell
	wrapped bool
	padded  bool
}

type savedCursor struct {
	valid    bool
	x, y     int
	style    text.Style
	link     string
	pending  bool
	origin   bool
	autowrap bool
}

type buffer struct {
	lines []line
	saved savedCursor
}

type screen struct {
	cols, rows int
	prim, alt  buffer
	useAlt     bool
	cur        Cursor
	origin     bool
	autowrap   bool
	insert     bool
	top        int
	bottom     int
	tabs       []bool
	sb         ring
	scratch    []line
}

// New returns a Screen of the given size whose primary buffer retains at most
// scrollbackLimit lines of scrollback.
func New(cols, rows, scrollbackLimit int) Screen {
	cols, rows = max(cols, 1), max(rows, 1)
	s := &screen{
		cols:     cols,
		rows:     rows,
		autowrap: true,
		bottom:   rows - 1,
		cur:      Cursor{Visible: true},
		sb:       ring{limit: max(scrollbackLimit, 0)},
	}
	s.prim.lines = newLines(cols, rows)
	s.alt.lines = newLines(cols, rows)
	s.tabs = make([]bool, cols)
	for i := defaultTabWidth; i < cols; i += defaultTabWidth {
		s.tabs[i] = true
	}
	return s
}

func newLines(cols, rows int) []line {
	ls := make([]line, rows)
	for i := range ls {
		ls[i].cells = make([]Cell, cols)
	}
	return ls
}

func (s *screen) buf() *buffer {
	if s.useAlt {
		return &s.alt
	}
	return &s.prim
}

func (s *screen) Size() (int, int) { return s.cols, s.rows }

func (s *screen) blank() Cell {
	return Cell{Style: text.Style{Bg: s.cur.Style.Bg}}
}

func fill(cells []Cell, c Cell) {
	for i := range cells {
		cells[i] = c
	}
}

// fixEdges blanks the orphaned halves of wide characters cut by an operation
// over [a, b) of cells.
func fixEdges(cells []Cell, a, b int, blank Cell) {
	if a > 0 && cells[a-1].Wide {
		cells[a-1] = blank
	}
	if b > a && b < len(cells) && cells[b-1].Wide {
		cells[b] = blank
	}
}

// clearRange erases [a, b) of l with the pen background.
func (s *screen) clearRange(l *line, a, b int) {
	a, b = max(a, 0), min(b, s.cols)
	if a >= b {
		return
	}
	blank := s.blank()
	fixEdges(l.cells, a, b, blank)
	fill(l.cells[a:b], blank)
}

// resetLine blanks a whole row and clears its wrap state.
func (s *screen) resetLine(l *line) {
	if len(l.cells) != s.cols {
		l.cells = make([]Cell, s.cols)
	}
	fill(l.cells, s.blank())
	l.wrapped, l.padded = false, false
}

func (s *screen) Cursor() Cursor { return s.cur }

func (s *screen) SetCursor(c Cursor) {
	c.X = clamp(c.X, 0, s.cols-1)
	c.Y = clamp(c.Y, 0, s.rows-1)
	s.cur = c
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (s *screen) MoveTo(x, y int) {
	if s.origin {
		s.cur.Y = clamp(y+s.top, s.top, s.bottom)
	} else {
		s.cur.Y = clamp(y, 0, s.rows-1)
	}
	s.cur.X = clamp(x, 0, s.cols-1)
	s.cur.PendingWrap = false
}

func (s *screen) MoveBy(dx, dy int) {
	y := s.cur.Y
	switch {
	case dy < 0:
		lo := 0
		if y >= s.top {
			lo = s.top
		}
		y = max(y+dy, lo)
	case dy > 0:
		hi := s.rows - 1
		if y <= s.bottom {
			hi = s.bottom
		}
		y = min(y+dy, hi)
	}
	s.cur.Y = y
	s.cur.X = clamp(s.cur.X+dx, 0, s.cols-1)
	s.cur.PendingWrap = false
}

func (s *screen) SaveCursor() {
	s.buf().saved = savedCursor{
		valid:    true,
		x:        s.cur.X,
		y:        s.cur.Y,
		style:    s.cur.Style,
		link:     s.cur.Link,
		pending:  s.cur.PendingWrap,
		origin:   s.origin,
		autowrap: s.autowrap,
	}
}

func (s *screen) RestoreCursor() {
	sv := s.buf().saved
	if !sv.valid {
		s.cur.Style, s.cur.Link = text.Style{}, ""
		s.origin = false
		s.cur.X, s.cur.Y, s.cur.PendingWrap = 0, 0, false
		return
	}
	s.cur.X = clamp(sv.x, 0, s.cols-1)
	s.cur.Y = clamp(sv.y, 0, s.rows-1)
	s.cur.Style, s.cur.Link = sv.style, sv.link
	s.cur.PendingWrap = sv.pending
	s.origin, s.autowrap = sv.origin, sv.autowrap
}

// Print writes one cluster at the cursor.
func (s *screen) Print(cluster string, width int) {
	if width <= 0 {
		s.combine(cluster)
		return
	}
	width = min(width, 2)
	if width == 2 && s.cols < 2 {
		return
	}
	c := &s.cur
	if c.PendingWrap {
		c.PendingWrap = false
		if s.autowrap {
			s.wrap(false)
		}
	}
	l := &s.buf().lines[c.Y]
	if width == 2 && c.X == s.cols-1 {
		if !s.autowrap {
			return
		}
		s.clearRange(l, c.X, c.X+1)
		s.wrap(true)
		l = &s.buf().lines[c.Y]
	}
	if s.insert {
		s.shiftRight(l, c.X, width)
	} else {
		s.clearRange(l, c.X, c.X+width)
	}
	l.cells[c.X] = Cell{Cluster: cluster, Style: c.Style, Link: c.Link, Wide: width == 2}
	if width == 2 {
		l.cells[c.X+1] = Cell{Style: c.Style, Link: c.Link}
	}
	c.X += width
	if c.X >= s.cols {
		c.X = s.cols - 1
		c.PendingWrap = s.autowrap
	}
}

// wrap marks the current row as soft wrapped and moves to the next row start.
func (s *screen) wrap(padded bool) {
	l := &s.buf().lines[s.cur.Y]
	l.wrapped, l.padded = true, padded
	s.cur.X = 0
	s.index()
}

func (s *screen) combine(cluster string) {
	x := s.cur.X
	if !s.cur.PendingWrap {
		x--
	}
	if x < 0 {
		return
	}
	cells := s.buf().lines[s.cur.Y].cells
	if cells[x].Cluster == "" && x > 0 && cells[x-1].Wide {
		x--
	}
	if cells[x].Cluster == "" {
		return
	}
	cells[x].Cluster += cluster
}

// index moves down one row, scrolling the region when at its bottom.
func (s *screen) index() {
	switch {
	case s.cur.Y == s.bottom:
		s.scroll(s.top, s.bottom, 1, s.keepsScrollback())
	case s.cur.Y < s.rows-1:
		s.cur.Y++
	}
}

func (s *screen) LineFeed() {
	s.cur.PendingWrap = false
	s.index()
}

func (s *screen) ReverseIndex() {
	s.cur.PendingWrap = false
	switch {
	case s.cur.Y == s.top:
		s.scrollDown(s.top, s.bottom, 1)
	case s.cur.Y > 0:
		s.cur.Y--
	}
}

func (s *screen) CarriageReturn() {
	s.cur.X = 0
	s.cur.PendingWrap = false
}

func (s *screen) Backspace() {
	if s.cur.X > 0 {
		s.cur.X--
	}
	s.cur.PendingWrap = false
}

func (s *screen) Tab(n int) {
	s.cur.PendingWrap = false
	x := s.cur.X
	if n >= 0 {
		for n = max(n, 1); n > 0 && x < s.cols-1; n-- {
			for x++; x < s.cols-1 && !s.tabs[x]; x++ {
			}
		}
	} else {
		for ; n < 0 && x > 0; n++ {
			for x--; x > 0 && !s.tabs[x]; x-- {
			}
		}
	}
	s.cur.X = x
}

func (s *screen) SetTabStop() { s.tabs[s.cur.X] = true }

func (s *screen) ClearTabStop(all bool) {
	if all {
		clear(s.tabs)
		return
	}
	s.tabs[s.cur.X] = false
}

func (s *screen) SetOrigin(enabled bool) {
	s.origin = enabled
	s.MoveTo(0, 0)
}

func (s *screen) SetAutowrap(enabled bool) {
	s.autowrap = enabled
	if !enabled {
		s.cur.PendingWrap = false
	}
}

func (s *screen) SetInsert(enabled bool) { s.insert = enabled }

func (s *screen) SetScrollRegion(top, bottom int) {
	if bottom < 0 || bottom >= s.rows {
		bottom = s.rows - 1
	}
	top = max(top, 0)
	if top >= bottom {
		return
	}
	s.top, s.bottom = top, bottom
	s.MoveTo(0, 0)
}

func (s *screen) Origin() bool             { return s.origin }
func (s *screen) Autowrap() bool           { return s.autowrap }
func (s *screen) InsertMode() bool         { return s.insert }
func (s *screen) ScrollRegion() (int, int) { return s.top, s.bottom }

func (s *screen) UseAlternate(on, clear bool) {
	if on != s.useAlt {
		if !on && clear {
			s.clearAlt()
		}
		s.useAlt = on
	}
	if on && clear {
		s.clearAlt()
	}
}

func (s *screen) clearAlt() {
	for i := range s.alt.lines {
		l := &s.alt.lines[i]
		fill(l.cells, Cell{})
		l.wrapped, l.padded = false, false
	}
}

func (s *screen) Alternate() bool { return s.useAlt }

func (s *screen) ScrollbackLen() int { return s.sb.len() }

func (s *screen) ScrollbackLine(i int) []Cell {
	if i < 0 || i >= s.sb.len() {
		return nil
	}
	return s.sb.at(i).cells
}

func (s *screen) ScrollbackLineWrapped(i int) bool {
	if i < 0 || i >= s.sb.len() {
		return false
	}
	return s.sb.at(i).wrapped
}

func (s *screen) ScrollbackLimit() int         { return s.sb.limit }
func (s *screen) SetScrollbackLimit(lines int) { s.sb.setLimit(lines) }
func (s *screen) ClearScrollback()             { s.sb.clear() }

func (s *screen) Line(y int) []Cell {
	if y < 0 || y >= s.rows {
		return nil
	}
	return s.buf().lines[y].cells
}

func (s *screen) LineWrapped(y int) bool {
	if y < 0 || y >= s.rows {
		return false
	}
	return s.buf().lines[y].wrapped
}
