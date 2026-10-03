// SPDX-License-Identifier: MPL-2.0

package screen

func count(n int) int { return max(n, 1) }

// keepsScrollback reports whether lines leaving the top of the region enter
// scrollback.
func (s *screen) keepsScrollback() bool {
	return !s.useAlt && s.top == 0 && s.bottom == s.rows-1
}

// scroll moves rows [top, bottom] up by n, blanking the rows entering at the
// bottom. With keep, the departing rows join the scrollback.
func (s *screen) scroll(top, bottom, n int, keep bool) {
	n = min(n, bottom-top+1)
	if n <= 0 {
		return
	}
	lines := s.buf().lines
	s.scratch = append(s.scratch[:0], lines[top:top+n]...)
	copy(lines[top:bottom+1-n], lines[top+n:bottom+1])
	for k := 0; k < n; k++ {
		l := s.scratch[k]
		if keep {
			l = s.sb.push(l)
		}
		s.resetLine(&l)
		lines[bottom+1-n+k] = l
	}
	clear(s.scratch)
}

// scrollDown moves rows [top, bottom] down by n, blanking the rows entering
// at the top.
func (s *screen) scrollDown(top, bottom, n int) {
	n = min(n, bottom-top+1)
	if n <= 0 {
		return
	}
	lines := s.buf().lines
	s.scratch = append(s.scratch[:0], lines[bottom+1-n:bottom+1]...)
	copy(lines[top+n:bottom+1], lines[top:bottom+1-n])
	for k := 0; k < n; k++ {
		l := s.scratch[k]
		s.resetLine(&l)
		lines[top+k] = l
	}
	clear(s.scratch)
}

func (s *screen) ScrollUp(n int) {
	s.scroll(s.top, s.bottom, count(n), s.keepsScrollback())
}

func (s *screen) ScrollDown(n int) {
	s.scrollDown(s.top, s.bottom, count(n))
}

func (s *screen) InsertLines(n int) {
	if s.cur.Y < s.top || s.cur.Y > s.bottom {
		return
	}
	s.scrollDown(s.cur.Y, s.bottom, count(n))
	s.cur.X, s.cur.PendingWrap = 0, false
}

func (s *screen) DeleteLines(n int) {
	if s.cur.Y < s.top || s.cur.Y > s.bottom {
		return
	}
	s.scroll(s.cur.Y, s.bottom, count(n), false)
	s.cur.X, s.cur.PendingWrap = 0, false
}

func (s *screen) EraseDisplay(mode EraseMode) {
	s.cur.PendingWrap = false
	lines := s.buf().lines
	y, x := s.cur.Y, s.cur.X
	switch mode {
	case EraseToEnd:
		s.clearRange(&lines[y], x, s.cols)
		lines[y].wrapped, lines[y].padded = false, false
		for i := y + 1; i < s.rows; i++ {
			s.resetLine(&lines[i])
		}
	case EraseToStart:
		for i := 0; i < y; i++ {
			s.resetLine(&lines[i])
		}
		s.clearRange(&lines[y], 0, x+1)
	case EraseAll:
		for i := range lines {
			s.resetLine(&lines[i])
		}
	case EraseScrollback:
		s.sb.clear()
	}
}

func (s *screen) EraseLine(mode EraseMode) {
	s.cur.PendingWrap = false
	l := &s.buf().lines[s.cur.Y]
	switch mode {
	case EraseToEnd:
		s.clearRange(l, s.cur.X, s.cols)
		l.wrapped, l.padded = false, false
	case EraseToStart:
		s.clearRange(l, 0, s.cur.X+1)
	case EraseAll:
		s.resetLine(l)
	}
}

func (s *screen) EraseChars(n int) {
	s.cur.PendingWrap = false
	s.clearRange(&s.buf().lines[s.cur.Y], s.cur.X, s.cur.X+count(n))
}

func (s *screen) InsertChars(n int) {
	s.cur.PendingWrap = false
	s.shiftRight(&s.buf().lines[s.cur.Y], s.cur.X, min(count(n), s.cols-s.cur.X))
}

// shiftRight opens n blank cells at x, pushing the rest of the row right.
func (s *screen) shiftRight(l *line, x, n int) {
	cells := l.cells
	blank := s.blank()
	if x > 0 && cells[x-1].Wide {
		cells[x-1] = blank
	}
	copy(cells[x+n:], cells[x:s.cols-n])
	fill(cells[x:x+n], blank)
	if cells[s.cols-1].Wide {
		cells[s.cols-1] = blank
	}
}

func (s *screen) DeleteChars(n int) {
	s.cur.PendingWrap = false
	cells := s.buf().lines[s.cur.Y].cells
	x := s.cur.X
	n = min(count(n), s.cols-x)
	blank := s.blank()
	if x > 0 && cells[x-1].Wide {
		cells[x-1] = blank
	}
	if x+n < s.cols && cells[x+n-1].Wide {
		cells[x+n] = blank
	}
	copy(cells[x:], cells[x+n:])
	fill(cells[s.cols-n:], blank)
}

func (s *screen) FillScreen(cluster string) {
	for i := range s.buf().lines {
		l := &s.buf().lines[i]
		for j := range l.cells {
			l.cells[j] = Cell{Cluster: cluster}
		}
		l.wrapped, l.padded = false, false
	}
	s.top, s.bottom = 0, s.rows-1
	s.MoveTo(0, 0)
}
