// SPDX-License-Identifier: MPL-2.0

package screen

// pos is a cursor-like position that follows its cell through a resize.
type pos struct {
	x, y    int
	pending bool
}

// Resize changes the grid size. The primary buffer reflows soft-wrapped
// lines and scrollback; the alternate buffer clips or extends.
func (s *screen) Resize(cols, rows int) {
	cols, rows = max(cols, 1), max(rows, 1)
	if cols == s.cols && rows == s.rows {
		return
	}

	// The primary buffer's cursor is the live cursor, or the saved cursor
	// while the alternate buffer is active.
	var marks []*pos
	var live *pos
	var saved *pos
	if !s.useAlt {
		live = &pos{s.cur.X, s.cur.Y, s.cur.PendingWrap}
		marks = append(marks, live)
	}
	if s.prim.saved.valid {
		sv := &s.prim.saved
		saved = &pos{sv.x, sv.y, sv.pending}
		marks = append(marks, saved)
	}

	if cols == s.cols {
		s.resizeRows(rows, live, marks)
	} else {
		s.reflow(cols, rows, live, marks)
	}
	s.resizeAlt(cols, rows)

	if live != nil {
		s.cur.X, s.cur.Y, s.cur.PendingWrap = live.x, live.y, live.pending
	}
	if saved != nil {
		s.prim.saved.x, s.prim.saved.y, s.prim.saved.pending = saved.x, saved.y, saved.pending
	}

	oldCols := s.cols
	s.cols, s.rows = cols, rows
	s.top, s.bottom = 0, rows-1
	tabs := make([]bool, cols)
	copy(tabs, s.tabs)
	for i := oldCols; i < cols; i++ {
		tabs[i] = i%defaultTabWidth == 0 && i > 0
	}
	s.tabs = tabs

	s.cur.Y = clamp(s.cur.Y, 0, rows-1)
	if s.cur.X >= cols {
		s.cur.X, s.cur.PendingWrap = cols-1, false
	}
	for _, sv := range []*savedCursor{&s.prim.saved, &s.alt.saved} {
		sv.y = clamp(sv.y, 0, rows-1)
		if sv.x >= cols {
			sv.x, sv.pending = cols-1, false
		}
	}
}

func rowBlank(l *line) bool {
	if l.wrapped {
		return false
	}
	for i := range l.cells {
		if l.cells[i].Cluster != "" {
			return false
		}
	}
	return true
}

// resizeRows changes the primary buffer height at constant width. Growing
// pulls lines back from scrollback; shrinking drops blank rows below the
// cursor first, then moves top rows into scrollback to keep the cursor.
func (s *screen) resizeRows(rows int, live *pos, marks []*pos) {
	b := &s.prim
	if rows > s.rows {
		pull := min(rows-s.rows, s.sb.len())
		lines := make([]line, 0, rows)
		for i := 0; i < pull; i++ {
			lines = append(lines, line{})
		}
		for i := pull - 1; i >= 0; i-- {
			lines[i] = s.sb.pop()
		}
		lines = append(lines, b.lines...)
		for len(lines) < rows {
			lines = append(lines, line{cells: make([]Cell, s.cols)})
		}
		b.lines = lines
		for _, m := range marks {
			m.y += pull
		}
		return
	}
	excess := s.rows - rows
	cy := 0
	if live != nil {
		cy = live.y
	}
	lines := b.lines
	for excess > 0 && len(lines)-1 > cy && rowBlank(&lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
		excess--
	}
	pushed := 0
	for excess > 0 && cy > 0 {
		s.sb.push(lines[pushed])
		pushed++
		cy--
		excess--
	}
	lines = lines[pushed:]
	lines = lines[:len(lines)-excess]
	b.lines = append([]line(nil), lines[:rows]...)
	for _, m := range marks {
		m.y = clamp(m.y-pushed, 0, rows-1)
	}
}

type target struct {
	mark int // index into marks; -1 is the screen top
	cell int // cell index within the logical line
}

// reflow rewraps scrollback and the primary screen to a new width and height.
func (s *screen) reflow(cols, rows int, live *pos, marks []*pos) {
	sbLen := s.sb.len()
	src := make([]*line, 0, sbLen+s.rows)
	for i := 0; i < sbLen; i++ {
		src = append(src, s.sb.at(i))
	}
	for i := range s.prim.lines {
		src = append(src, &s.prim.lines[i])
	}

	type result struct{ row, col int }
	res := make([]result, len(marks))
	var topRes result
	out := make([]line, 0, len(src))
	var cells []Cell
	var tgts []target

	for r := 0; r < len(src); {
		cells, tgts = cells[:0], tgts[:0]
		for {
			l := src[r]
			keep := l.cells
			if l.wrapped && l.padded && len(keep) > 0 {
				keep = keep[:len(keep)-1]
			}
			for i, m := range marks {
				if sbLen+m.y == r {
					tgts = append(tgts, target{i, len(cells) + min(m.x, len(keep))})
				}
			}
			if r == sbLen {
				tgts = append(tgts, target{-1, len(cells)})
			}
			cells = append(cells, keep...)
			r++
			if !l.wrapped || r >= len(src) {
				break
			}
		}

		n := len(cells)
		for n > 0 && cells[n-1].Cluster == "" && !(n >= 2 && cells[n-2].Wide) {
			n--
		}
		cells = cells[:n]

		ri := len(out)
		out = append(out, line{cells: make([]Cell, cols)})
		col := 0
		resolved := func(t target, row, c int) {
			if t.mark < 0 {
				topRes = result{row, c}
			} else {
				res[t.mark] = result{row, c}
			}
		}
		done := make([]bool, len(tgts))
		for i := 0; i < n; {
			c := cells[i]
			w, srcw := 1, 1
			if c.Wide && i+1 < n {
				w, srcw = 2, 2
			} else if c.Wide {
				c.Wide = false
			}
			if w == 2 && cols < 2 {
				c, w = Cell{Style: c.Style}, 1
			}
			if col+w > cols {
				out[ri].wrapped = true
				out[ri].padded = w == 2 && col == cols-1
				out = append(out, line{cells: make([]Cell, cols)})
				ri, col = len(out)-1, 0
			}
			out[ri].cells[col] = c
			if w == 2 {
				out[ri].cells[col+1] = cells[i+1]
			}
			for k, t := range tgts {
				if done[k] {
					continue
				}
				switch {
				case t.cell == i:
					resolved(t, ri, col)
					done[k] = true
				case t.cell == i+1 && srcw == 2:
					resolved(t, ri, min(col+1, cols-1))
					done[k] = true
				}
			}
			col += w
			i += srcw
		}
		// Targets past the content keep their distance from the content end
		// on its last row.
		for k, t := range tgts {
			if done[k] {
				continue
			}
			switch {
			case t.mark < 0:
				resolved(t, ri, 0)
			case t.cell == n:
				resolved(t, ri, col)
			default:
				resolved(t, ri, min(col+t.cell-n, cols-1))
			}
		}
	}

	// A mark at or past the right margin sits in the deferred-wrap state.
	for i, m := range marks {
		col := res[i].col
		if m.pending {
			col++
		}
		if col >= cols {
			col = cols - 1
			m.pending = true
		} else {
			m.pending = false
		}
		m.x, m.y = col, res[i].row
	}

	keepRows := topRes.row
	for _, m := range marks {
		keepRows = max(keepRows, m.y)
	}
	last := len(out) - 1
	for last > keepRows && rowBlank(&out[last]) {
		last--
	}
	out = out[:last+1]

	start := topRes.row
	if rows > s.rows {
		start = max(start-(rows-s.rows), 0)
	}
	start = max(start, len(out)-rows)
	if live != nil {
		start = min(start, live.y)
	}
	for len(out) < start+rows {
		out = append(out, line{cells: make([]Cell, cols)})
	}
	for _, m := range marks {
		m.y = clamp(m.y-start, 0, rows-1)
	}

	kept := out[max(0, start-s.sb.limit):start]
	s.sb.lines, s.sb.start = kept[:len(kept):len(kept)], 0
	s.prim.lines = out[start : start+rows : start+rows]
}

// resizeAlt clips or extends the alternate buffer.
func (s *screen) resizeAlt(cols, rows int) {
	lines := make([]line, rows)
	for i := range lines {
		cells := make([]Cell, cols)
		if i < len(s.alt.lines) {
			copy(cells, s.alt.lines[i].cells)
			if cols < s.cols && cells[cols-1].Wide {
				cells[cols-1] = Cell{}
			}
			lines[i].wrapped = s.alt.lines[i].wrapped && cols >= s.cols
		}
		lines[i].cells = cells
	}
	s.alt.lines = lines
}
