// SPDX-License-Identifier: MPL-2.0

package vt

import (
	"fmt"

	"github.com/wippyai/tty/text"
	"github.com/wippyai/tty/vt/parser"
	"github.com/wippyai/tty/vt/screen"
)

const (
	terminalName    = "wippy-tty"
	terminalVersion = "0.1"
)

// count returns parameter i as a repeat count: missing or zero selects 1.
func count(p parser.Params, i int) int {
	if n := p.Get(i, 1); n > 0 {
		return n
	}
	return 1
}

// CSI dispatches a control sequence.
func (t *Terminal) CSI(prefix byte, p parser.Params, inter []byte, final byte) {
	switch {
	case prefix == 0 && len(inter) == 0:
		t.csiPlain(p, final)
	case prefix == 0 && string(inter) == "!" && final == 'p':
		t.softReset()
	case prefix == 0 && string(inter) == "$" && final == 'p':
		t.queryMode(p.Get(0, 0), false)
	case prefix == 0 && string(inter) == " " && final == 'q':
		t.decscusr(p.Get(0, 0))
	case prefix == '?' && len(inter) == 0:
		t.csiPrivate(p, final)
	case prefix == '?' && string(inter) == "$" && final == 'p':
		t.queryMode(p.Get(0, 0), true)
	case prefix == '>' && len(inter) == 0:
		t.csiGreater(p, final)
	case prefix == '<' && len(inter) == 0 && final == 'u':
		t.kittyPop(count(p, 0))
	case prefix == '=' && len(inter) == 0 && final == 'u':
		t.kittySet(p.Get(0, 0), p.Get(1, 1))
	}
}

func (t *Terminal) csiPlain(p parser.Params, final byte) {
	s := t.scr
	switch final {
	case '@':
		s.InsertChars(count(p, 0))
	case 'A':
		s.MoveBy(0, -count(p, 0))
	case 'B', 'e':
		s.MoveBy(0, count(p, 0))
	case 'C', 'a':
		s.MoveBy(count(p, 0), 0)
	case 'D':
		s.MoveBy(-count(p, 0), 0)
	case 'E':
		s.MoveBy(0, count(p, 0))
		s.CarriageReturn()
	case 'F':
		s.MoveBy(0, -count(p, 0))
		s.CarriageReturn()
	case 'G', '`':
		s.MoveBy(count(p, 0)-1-s.Cursor().X, 0)
	case 'H', 'f':
		s.MoveTo(count(p, 1)-1, count(p, 0)-1)
	case 'I':
		s.Tab(count(p, 0))
	case 'J':
		t.eraseDisplay(p.Get(0, 0))
	case 'K':
		if m, ok := eraseMode(p.Get(0, 0)); ok && m != screen.EraseScrollback {
			s.EraseLine(m)
		}
	case 'L':
		s.InsertLines(count(p, 0))
	case 'M':
		s.DeleteLines(count(p, 0))
	case 'P':
		s.DeleteChars(count(p, 0))
	case 'S':
		s.ScrollUp(count(p, 0))
	case 'T':
		s.ScrollDown(count(p, 0))
	case 'X':
		s.EraseChars(count(p, 0))
	case 'Z':
		s.Tab(-count(p, 0))
	case 'b':
		t.repeat(count(p, 0))
	case 'c':
		if p.Get(0, 0) == 0 {
			t.replyString("\x1b[?62;22c")
		}
	case 'd':
		s.MoveTo(s.Cursor().X, count(p, 0)-1)
	case 'g':
		switch p.Get(0, 0) {
		case 0:
			s.ClearTabStop(false)
		case 3:
			s.ClearTabStop(true)
		}
	case 'h':
		t.ansiModes(p, true)
	case 'l':
		t.ansiModes(p, false)
	case 'm':
		t.sgr(p)
	case 'n':
		t.dsr(p.Get(0, 0), false)
	case 'r':
		t.setMargins(p)
	case 's':
		t.saveCursor()
	case 't':
		t.windowOps(p)
	case 'u':
		t.restoreCursor()
	}
}

func eraseMode(n int) (screen.EraseMode, bool) {
	switch n {
	case 0:
		return screen.EraseToEnd, true
	case 1:
		return screen.EraseToStart, true
	case 2:
		return screen.EraseAll, true
	case 3:
		return screen.EraseScrollback, true
	}
	return 0, false
}

func (t *Terminal) eraseDisplay(n int) {
	m, ok := eraseMode(n)
	if !ok {
		return
	}
	t.scr.EraseDisplay(m)
	if m == screen.EraseAll || m == screen.EraseScrollback {
		t.clearPlacements()
	}
}

func (t *Terminal) repeat(n int) {
	if t.lastRune == "" {
		return
	}
	if n > 65535 {
		n = 65535
	}
	for i := 0; i < n; i++ {
		t.scr.Print(t.lastRune, wcwidth(t.lastRune))
	}
}

func (t *Terminal) setMargins(p parser.Params) {
	top := count(p, 0) - 1
	bottom := p.Get(1, 0) - 1
	if p.Get(1, 0) <= 0 {
		bottom = t.rows - 1
	}
	if bottom >= t.rows {
		bottom = t.rows - 1
	}
	if top >= bottom {
		return
	}
	t.marginTop = top
	t.marginBottom = bottom
	if bottom == t.rows-1 {
		t.marginBottom = -1
	}
	t.scr.SetScrollRegion(top, t.marginBottom)
	t.scr.MoveTo(0, 0)
}

func (t *Terminal) decscusr(n int) {
	switch n {
	case 0, 1:
		t.setCursorShape(screen.CursorBlock, true)
	case 2:
		t.setCursorShape(screen.CursorBlock, false)
	case 3:
		t.setCursorShape(screen.CursorUnderline, true)
	case 4:
		t.setCursorShape(screen.CursorUnderline, false)
	case 5:
		t.setCursorShape(screen.CursorBar, true)
	case 6:
		t.setCursorShape(screen.CursorBar, false)
	}
}

func (t *Terminal) csiPrivate(p parser.Params, final byte) {
	switch final {
	case 'h':
		for i := 0; i < p.Len(); i++ {
			t.setDECMode(p.Get(i, 0), true)
		}
	case 'l':
		for i := 0; i < p.Len(); i++ {
			t.setDECMode(p.Get(i, 0), false)
		}
	case 'n':
		t.dsr(p.Get(0, 0), true)
	case 'u':
		t.replyString(fmt.Sprintf("\x1b[?%du", t.kb().top()))
	}
}

func (t *Terminal) csiGreater(p parser.Params, final byte) {
	switch final {
	case 'c':
		if p.Get(0, 0) == 0 {
			t.replyString("\x1b[>1;10;0c")
		}
	case 'q':
		if p.Get(0, 0) == 0 {
			t.replyString("\x1bP>|" + terminalName + "(" + terminalVersion + ")\x1b\\")
		}
	case 'u':
		t.kittyPush(p.Get(0, 0))
	}
}

func (t *Terminal) dsr(n int, private bool) {
	switch n {
	case 5:
		if !private {
			t.replyString("\x1b[0n")
		}
	case 6:
		c := t.scr.Cursor()
		y := c.Y
		if t.origin {
			y -= t.marginTop
		}
		prefix := ""
		if private {
			prefix = "?"
		}
		t.replyString(fmt.Sprintf("\x1b[%s%d;%dR", prefix, y+1, c.X+1))
	}
}

func (t *Terminal) windowOps(p parser.Params) {
	switch p.Get(0, 0) {
	case 14:
		t.replyString(fmt.Sprintf("\x1b[4;%d;%dt", t.rows*t.cellH, t.cols*t.cellW))
	case 16:
		t.replyString(fmt.Sprintf("\x1b[6;%d;%dt", t.cellH, t.cellW))
	case 18:
		t.replyString(fmt.Sprintf("\x1b[8;%d;%dt", t.rows, t.cols))
	case 19:
		t.replyString(fmt.Sprintf("\x1b[9;%d;%dt", t.rows, t.cols))
	}
}

func (t *Terminal) ansiModes(p parser.Params, on bool) {
	for i := 0; i < p.Len(); i++ {
		switch p.Get(i, 0) {
		case 4:
			t.setInsert(on)
		case 20:
			t.modes.LinefeedNewline = on
		}
	}
}

// queryMode answers DECRQM.
func (t *Terminal) queryMode(n int, private bool) {
	state := 0
	if private {
		if known, set := t.decModeState(n); known {
			state = 2
			if set {
				state = 1
			}
		}
	} else {
		switch n {
		case 4:
			state = boolState(t.insert)
		case 20:
			state = boolState(t.modes.LinefeedNewline)
		}
	}
	prefix := ""
	if private {
		prefix = "?"
	}
	t.replyString(fmt.Sprintf("\x1b[%s%d;%d$y", prefix, n, state))
}

func boolState(b bool) int {
	if b {
		return 1
	}
	return 2
}

func wcwidth(cluster string) int {
	if w := text.ClusterWidth(cluster); w > 0 {
		return w
	}
	return 1
}
