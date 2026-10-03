// SPDX-License-Identifier: MPL-2.0

package screen

import (
	"fmt"
	"strings"
	"testing"
)

func TestNewAndSize(t *testing.T) {
	s := New(10, 4, 100)
	if c, r := s.Size(); c != 10 || r != 4 {
		t.Fatalf("size %dx%d", c, r)
	}
	if !s.Cursor().Visible || !s.Autowrap() || s.Origin() || s.InsertMode() {
		t.Fatalf("defaults: %+v", s.Cursor())
	}
	if top, bot := s.ScrollRegion(); top != 0 || bot != 3 {
		t.Fatalf("region %d,%d", top, bot)
	}
	if s.Line(-1) != nil || s.Line(4) != nil || s.LineWrapped(9) {
		t.Fatal("out of range access")
	}
	z := New(0, -3, -1)
	if c, r := z.Size(); c != 1 || r != 1 {
		t.Fatalf("clamped size %dx%d", c, r)
	}
}

func TestPrintAndPendingWrap(t *testing.T) {
	s := New(5, 3, 10)
	put(s, "abcde")
	eqCursor(t, s, 4, 0)
	eqPending(t, s, true)
	put(s, "f")
	eqCursor(t, s, 1, 1)
	eqPending(t, s, false)
	eqLines(t, screenText(s), "abcde", "f", "")
	if !s.LineWrapped(0) || s.LineWrapped(1) {
		t.Fatal("wrap flags")
	}
}

func TestPendingWrapCancelled(t *testing.T) {
	tests := []struct {
		name  string
		act   func(Screen)
		x, y  int
		after string
	}{
		{"CR", func(s Screen) { s.CarriageReturn() }, 0, 0, "Xbcde"},
		{"LF", func(s Screen) { s.LineFeed() }, 4, 1, "abcdX"},
		{"BS", func(s Screen) { s.Backspace() }, 3, 0, "abcXe"},
		{"MoveTo", func(s Screen) { s.MoveTo(2, 0) }, 2, 0, "abXde"},
		{"MoveBy", func(s Screen) { s.MoveBy(-1, 0) }, 3, 0, "abcXe"},
		{"Tab", func(s Screen) { s.Tab(1) }, 4, 0, "abcdX"},
		{"RI", func(s Screen) { s.ReverseIndex() }, 4, 0, "    X"},
		{"EraseLine", func(s Screen) { s.EraseLine(EraseToStart) }, 4, 0, "    X"},
		{"ECH", func(s Screen) { s.EraseChars(1) }, 4, 0, "abcdX"},
		{"ICH", func(s Screen) { s.InsertChars(1) }, 4, 0, "abcdX"},
		{"DCH", func(s Screen) { s.DeleteChars(1) }, 4, 0, "abcdX"},
		{"ED", func(s Screen) { s.EraseDisplay(EraseToEnd) }, 4, 0, "abcdX"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(5, 3, 0)
			put(s, "abcde")
			eqPending(t, s, true)
			tc.act(s)
			eqPending(t, s, false)
			eqCursor(t, s, tc.x, tc.y)
			put(s, "X")
			y := tc.y
			if tc.name == "LF" {
				y = 1
			}
			if tc.name == "LF" {
				if got := rowText(s, 1); !strings.HasSuffix(got, "X") {
					t.Fatalf("row 1 %q", got)
				}
				return
			}
			if got := rowText(s, y); got != tc.after {
				t.Fatalf("row %d %q, want %q", y, got, tc.after)
			}
		})
	}
}

func TestNoAutowrap(t *testing.T) {
	s := New(4, 2, 0)
	s.SetAutowrap(false)
	put(s, "abcdef")
	eqCursor(t, s, 3, 0)
	eqPending(t, s, false)
	eqLines(t, screenText(s), "abcf", "")
	if s.LineWrapped(0) {
		t.Fatal("wrapped without autowrap")
	}
	s.SetAutowrap(true)
	put(s, "g")
	eqPending(t, s, true)
	s.SetAutowrap(false)
	eqPending(t, s, false)
}

func TestAutowrapScrolls(t *testing.T) {
	s := New(3, 2, 10)
	put(s, "abcdefgh")
	eqLines(t, screenText(s), "def", "gh")
	eqLines(t, sbText(s), "abc")
	if !s.ScrollbackLineWrapped(0) {
		t.Fatal("scrollback line lost wrap flag")
	}
}

func TestLineFeedAndScrollRegion(t *testing.T) {
	tests := []struct {
		name     string
		top, bot int
		y        int
		wantY    int
		want     []string
		sb       int
	}{
		{"full screen bottom scrolls into scrollback", 0, -1, 3, 3, []string{"bbb", "ccc", "ddd", ""}, 1},
		{"region bottom scrolls inside region", 1, 2, 2, 2, []string{"aaa", "ccc", "", "ddd"}, 0},
		{"above region moves down", 1, 2, 0, 1, []string{"aaa", "bbb", "ccc", "ddd"}, 0},
		{"below region moves down until last row", 0, 1, 2, 3, []string{"aaa", "bbb", "ccc", "ddd"}, 0},
		{"last row below region stays", 0, 1, 3, 3, []string{"aaa", "bbb", "ccc", "ddd"}, 0},
		{"top-anchored partial region keeps no scrollback", 0, 2, 2, 2, []string{"bbb", "ccc", "", "ddd"}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(3, 4, 10)
			numbered(s)
			s.SetScrollRegion(tc.top, tc.bot)
			s.SetOrigin(false)
			s.MoveTo(0, tc.y)
			s.LineFeed()
			eqCursor(t, s, 0, tc.wantY)
			eqLines(t, screenText(s), tc.want...)
			if s.ScrollbackLen() != tc.sb {
				t.Fatalf("scrollback %d, want %d", s.ScrollbackLen(), tc.sb)
			}
		})
	}
}

func TestReverseIndex(t *testing.T) {
	tests := []struct {
		name     string
		top, bot int
		y, wantY int
		want     []string
	}{
		{"top of screen scrolls down", 0, -1, 0, 0, []string{"", "aaa", "bbb", "ccc"}},
		{"region top scrolls region", 1, 2, 1, 1, []string{"aaa", "", "bbb", "ddd"}},
		{"inside moves up", 1, 2, 2, 1, []string{"aaa", "bbb", "ccc", "ddd"}},
		{"above region moves up", 2, 3, 1, 0, []string{"aaa", "bbb", "ccc", "ddd"}},
		{"first row above region stays", 1, 3, 0, 0, []string{"aaa", "bbb", "ccc", "ddd"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(3, 4, 10)
			numbered(s)
			s.SetScrollRegion(tc.top, tc.bot)
			s.MoveTo(0, tc.y)
			s.ReverseIndex()
			eqCursor(t, s, 0, tc.wantY)
			eqLines(t, screenText(s), tc.want...)
			if s.ScrollbackLen() != 0 {
				t.Fatal("reverse index touched scrollback")
			}
		})
	}
}

func TestSetScrollRegion(t *testing.T) {
	tests := []struct {
		name             string
		top, bot         int
		wantTop, wantBot int
	}{
		{"valid", 1, 3, 1, 3},
		{"bottom default", 2, -1, 2, 5},
		{"bottom beyond clamps", 1, 99, 1, 5},
		{"negative top clamps", -4, 2, 0, 2},
		{"top equals bottom ignored", 3, 3, 0, 5},
		{"top beyond bottom ignored", 4, 2, 0, 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(4, 6, 0)
			s.MoveTo(2, 4)
			s.SetScrollRegion(tc.top, tc.bot)
			top, bot := s.ScrollRegion()
			if top != tc.wantTop || bot != tc.wantBot {
				t.Fatalf("region %d,%d want %d,%d", top, bot, tc.wantTop, tc.wantBot)
			}
		})
	}
	t.Run("homes cursor", func(t *testing.T) {
		s := New(4, 6, 0)
		s.MoveTo(2, 4)
		s.SetScrollRegion(1, 3)
		eqCursor(t, s, 0, 0)
	})
	t.Run("homes to region top in origin mode", func(t *testing.T) {
		s := New(4, 6, 0)
		s.SetOrigin(true)
		s.SetScrollRegion(2, 4)
		eqCursor(t, s, 0, 2)
	})
}

func TestOriginMode(t *testing.T) {
	s := New(10, 8, 0)
	s.SetScrollRegion(2, 5)
	s.MoveTo(5, 5)
	s.SetOrigin(true)
	if !s.Origin() {
		t.Fatal("origin not set")
	}
	eqCursor(t, s, 0, 2)
	tests := []struct {
		x, y   int
		wx, wy int
	}{
		{0, 0, 0, 2},
		{3, 1, 3, 3},
		{3, 3, 3, 5},
		{3, 50, 3, 5},
		{50, 0, 9, 2},
		{-5, -5, 0, 2},
	}
	for _, tc := range tests {
		s.MoveTo(tc.x, tc.y)
		eqCursor(t, s, tc.wx, tc.wy)
	}
	s.SetOrigin(false)
	eqCursor(t, s, 0, 0)
	s.MoveTo(2, 7)
	eqCursor(t, s, 2, 7)
}

func TestMoveByClampsToRegion(t *testing.T) {
	tests := []struct {
		name   string
		y      int
		dx, dy int
		wx, wy int
	}{
		{"up inside stops at top margin", 3, 0, -9, 0, 2},
		{"down inside stops at bottom margin", 3, 0, 9, 0, 5},
		{"up above region goes to screen top", 1, 0, -9, 0, 0},
		{"down below region goes to screen bottom", 6, 0, 9, 0, 7},
		{"x clamps right", 3, 99, 0, 9, 3},
		{"x clamps left", 3, -99, 0, 0, 3},
		{"diagonal", 3, 2, -1, 2, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(10, 8, 0)
			s.SetScrollRegion(2, 5)
			s.MoveTo(0, tc.y)
			s.MoveBy(tc.dx, tc.dy)
			eqCursor(t, s, tc.wx, tc.wy)
		})
	}
}

func TestMoveToClamp(t *testing.T) {
	s := New(5, 4, 0)
	s.MoveTo(99, 99)
	eqCursor(t, s, 4, 3)
	s.MoveTo(-1, -1)
	eqCursor(t, s, 0, 0)
}

func TestSetCursorClamps(t *testing.T) {
	s := New(5, 4, 0)
	s.SetCursor(Cursor{X: 20, Y: -3, Visible: true, Shape: CursorBar, PendingWrap: true})
	c := s.Cursor()
	if c.X != 4 || c.Y != 0 || c.Shape != CursorBar || !c.PendingWrap {
		t.Fatalf("%+v", c)
	}
}

func TestTabs(t *testing.T) {
	tests := []struct {
		name string
		x, n int
		want int
	}{
		{"default stop", 0, 1, 8},
		{"from mid", 3, 1, 8},
		{"from stop", 8, 1, 16},
		{"two stops", 0, 2, 16},
		{"zero counts as one", 0, 0, 8},
		{"clamps to last column", 17, 5, 19},
		{"backward", 12, -1, 8},
		{"backward from stop", 8, -1, 0},
		{"backward two", 20, -2, 8},
		{"backward clamps to zero", 3, -4, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(20, 2, 0)
			s.MoveTo(tc.x, 0)
			s.Tab(tc.n)
			eqCursor(t, s, tc.want, 0)
		})
	}
	t.Run("set and clear", func(t *testing.T) {
		s := New(20, 2, 0)
		s.MoveTo(3, 0)
		s.SetTabStop()
		s.MoveTo(0, 0)
		s.Tab(1)
		eqCursor(t, s, 3, 0)
		s.ClearTabStop(false)
		s.MoveTo(0, 0)
		s.Tab(1)
		eqCursor(t, s, 8, 0)
		s.ClearTabStop(true)
		s.MoveTo(0, 0)
		s.Tab(1)
		eqCursor(t, s, 19, 0)
		s.Tab(-1)
		eqCursor(t, s, 0, 0)
	})
}

func TestSaveRestoreCursor(t *testing.T) {
	s := New(6, 4, 0)
	st := redBg()
	setPen(s, st)
	s.SetOrigin(true)
	s.SetScrollRegion(1, 3)
	s.MoveTo(2, 1)
	put(s, "abcd")
	s.MoveTo(5, 0)
	put(s, "x")
	eqPending(t, s, true)
	s.SaveCursor()
	s.SetOrigin(false)
	s.SetAutowrap(false)
	setPen(s, Cursor{}.Style)
	s.MoveTo(0, 3)
	s.RestoreCursor()
	c := s.Cursor()
	if c.X != 5 || c.Y != 1 || !c.PendingWrap || c.Style != st {
		t.Fatalf("restored %+v", c)
	}
	if !s.Origin() || !s.Autowrap() {
		t.Fatal("modes not restored")
	}
}

func TestRestoreWithoutSaveResets(t *testing.T) {
	s := New(6, 4, 0)
	setPen(s, redBg())
	s.SetOrigin(true)
	s.MoveTo(3, 2)
	s.RestoreCursor()
	c := s.Cursor()
	if c.X != 0 || c.Y != 0 || !c.Style.IsZero() || s.Origin() {
		t.Fatalf("%+v origin=%v", c, s.Origin())
	}
}

func TestRestoreClampsAfterResize(t *testing.T) {
	s := New(10, 6, 0)
	s.MoveTo(9, 5)
	s.SaveCursor()
	s.Resize(4, 3)
	s.RestoreCursor()
	eqCursor(t, s, 3, 2)
}

func TestSavedCursorPerBuffer(t *testing.T) {
	s := New(10, 6, 0)
	s.MoveTo(2, 2)
	s.SaveCursor()
	s.UseAlternate(true, true)
	s.MoveTo(5, 5)
	s.SaveCursor()
	s.MoveTo(0, 0)
	s.RestoreCursor()
	eqCursor(t, s, 5, 5)
	s.UseAlternate(false, false)
	s.RestoreCursor()
	eqCursor(t, s, 2, 2)
}

func TestEraseDisplay(t *testing.T) {
	tests := []struct {
		name string
		mode EraseMode
		want []string
	}{
		{"to end", EraseToEnd, []string{"aaaa", "bb", "", ""}},
		{"to start", EraseToStart, []string{"", "   b", "cccc", "dddd"}},
		{"all", EraseAll, []string{"", "", "", ""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(4, 4, 10)
			for y := 0; y < 4; y++ {
				s.MoveTo(0, y)
				put(s, strings.Repeat(string(rune('a'+y)), 4))
			}
			s.MoveTo(2, 1)
			s.EraseDisplay(tc.mode)
			eqLines(t, screenText(s), tc.want...)
			eqCursor(t, s, 2, 1)
		})
	}
	t.Run("scrollback only", func(t *testing.T) {
		s := New(3, 2, 10)
		put(s, "a\nb\nc\nd")
		if s.ScrollbackLen() == 0 {
			t.Fatal("expected scrollback")
		}
		before := screenText(s)
		s.EraseDisplay(EraseScrollback)
		if s.ScrollbackLen() != 0 {
			t.Fatal("scrollback not cleared")
		}
		eqLines(t, screenText(s), before...)
	})
}

func TestEraseLine(t *testing.T) {
	tests := []struct {
		name string
		mode EraseMode
		want string
	}{
		{"to end", EraseToEnd, "ab"},
		{"to start", EraseToStart, "   defgh"},
		{"all", EraseAll, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(8, 2, 0)
			put(s, "abcdefgh")
			s.MoveTo(2, 0)
			s.EraseLine(tc.mode)
			if got := rowText(s, 0); got != tc.want {
				t.Fatalf("%q want %q", got, tc.want)
			}
		})
	}
}

func TestEraseUsesPenBackground(t *testing.T) {
	st := redBg()
	for _, tc := range []struct {
		name string
		do   func(Screen)
		x, y int
	}{
		{"EL end", func(s Screen) { s.EraseLine(EraseToEnd) }, 3, 1},
		{"EL start", func(s Screen) { s.EraseLine(EraseToStart) }, 0, 1},
		{"EL all", func(s Screen) { s.EraseLine(EraseAll) }, 3, 1},
		{"ED end", func(s Screen) { s.EraseDisplay(EraseToEnd) }, 3, 2},
		{"ED start", func(s Screen) { s.EraseDisplay(EraseToStart) }, 0, 0},
		{"ED all", func(s Screen) { s.EraseDisplay(EraseAll) }, 4, 2},
		{"ECH", func(s Screen) { s.EraseChars(3) }, 3, 1},
		{"ICH", func(s Screen) { s.InsertChars(2) }, 2, 1},
		{"DCH", func(s Screen) { s.DeleteChars(2) }, 7, 1},
		{"IL", func(s Screen) { s.InsertLines(1) }, 5, 1},
		{"DL", func(s Screen) { s.DeleteLines(1) }, 5, 3},
		{"SU", func(s Screen) { s.ScrollUp(1) }, 5, 3},
		{"SD", func(s Screen) { s.ScrollDown(1) }, 5, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New(8, 4, 0)
			for y := 0; y < 4; y++ {
				s.MoveTo(0, y)
				put(s, "abcdefgh")
			}
			s.MoveTo(2, 1)
			// pen carries fg too; only bg may reach blank cells
			pen := st
			pen.Fg = st.Bg
			pen.Attrs = 1
			setPen(s, pen)
			tc.do(s)
			cell := s.Line(tc.y)[tc.x]
			if cell.Cluster != "" || cell.Style.Bg != st.Bg || cell.Style.Fg != (st.Fg) || cell.Style.Attrs != 0 {
				t.Fatalf("cell (%d,%d) = %+v", tc.x, tc.y, cell)
			}
		})
	}
}

func TestEraseChars(t *testing.T) {
	tests := []struct {
		name string
		x, n int
		want string
	}{
		{"middle", 2, 3, "ab   fgh"},
		{"zero is one", 2, 0, "ab defgh"},
		{"clamps at right", 6, 9, "abcdef"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(8, 1, 0)
			put(s, "abcdefgh")
			s.MoveTo(tc.x, 0)
			s.EraseChars(tc.n)
			if got := rowText(s, 0); got != tc.want {
				t.Fatalf("%q want %q", got, tc.want)
			}
			eqCursor(t, s, tc.x, 0)
		})
	}
}

func TestInsertDeleteChars(t *testing.T) {
	tests := []struct {
		name string
		op   func(Screen)
		x    int
		want string
	}{
		{"ICH 2", func(s Screen) { s.InsertChars(2) }, 2, "ab  cdef"},
		{"ICH 0 is one", func(s Screen) { s.InsertChars(0) }, 2, "ab cdefg"},
		{"ICH overflow", func(s Screen) { s.InsertChars(99) }, 5, "abcde"},
		{"ICH at last column", func(s Screen) { s.InsertChars(3) }, 7, "abcdefg"},
		{"DCH 2", func(s Screen) { s.DeleteChars(2) }, 2, "abefgh"},
		{"DCH overflow", func(s Screen) { s.DeleteChars(99) }, 5, "abcde"},
		{"DCH at last column", func(s Screen) { s.DeleteChars(1) }, 7, "abcdefg"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(8, 1, 0)
			put(s, "abcdefgh")
			s.MoveTo(tc.x, 0)
			tc.op(s)
			if got := rowText(s, 0); got != tc.want {
				t.Fatalf("%q want %q", got, tc.want)
			}
			eqCursor(t, s, tc.x, 0)
		})
	}
}

func TestInsertDeleteLines(t *testing.T) {
	tests := []struct {
		name     string
		top, bot int
		y        int
		op       func(Screen)
		want     []string
		wantY    int
	}{
		{"IL", 0, -1, 1, func(s Screen) { s.InsertLines(1) }, []string{"aaa", "", "bbb", "ccc", "ddd"}, 1},
		{"IL n", 0, -1, 1, func(s Screen) { s.InsertLines(2) }, []string{"aaa", "", "", "bbb", "ccc"}, 1},
		{"IL beyond region", 0, -1, 1, func(s Screen) { s.InsertLines(99) }, []string{"aaa", "", "", "", ""}, 1},
		{"IL in region", 1, 3, 2, func(s Screen) { s.InsertLines(1) }, []string{"aaa", "bbb", "", "ccc", "eee"}, 2},
		{"IL above region ignored", 2, 3, 1, func(s Screen) { s.InsertLines(1) }, []string{"aaa", "bbb", "ccc", "ddd", "eee"}, 1},
		{"IL below region ignored", 1, 2, 4, func(s Screen) { s.InsertLines(1) }, []string{"aaa", "bbb", "ccc", "ddd", "eee"}, 4},
		{"DL", 0, -1, 1, func(s Screen) { s.DeleteLines(1) }, []string{"aaa", "ccc", "ddd", "eee", ""}, 1},
		{"DL in region", 1, 3, 2, func(s Screen) { s.DeleteLines(1) }, []string{"aaa", "bbb", "ddd", "", "eee"}, 2},
		{"DL beyond region", 1, 3, 2, func(s Screen) { s.DeleteLines(99) }, []string{"aaa", "bbb", "", "", "eee"}, 2},
		{"DL above region ignored", 2, 3, 0, func(s Screen) { s.DeleteLines(1) }, []string{"aaa", "bbb", "ccc", "ddd", "eee"}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(3, 5, 10)
			numbered(s)
			s.SetScrollRegion(tc.top, tc.bot)
			s.SetOrigin(false)
			s.MoveTo(2, tc.y)
			tc.op(s)
			eqLines(t, screenText(s), tc.want...)
			x := 0
			if strings.Contains(tc.name, "ignored") {
				x = 2
			}
			eqCursor(t, s, x, tc.wantY)
			if s.ScrollbackLen() != 0 {
				t.Fatal("IL/DL fed scrollback")
			}
		})
	}
}

func TestScrollUpDown(t *testing.T) {
	tests := []struct {
		name     string
		top, bot int
		op       func(Screen)
		want     []string
		sb       []string
	}{
		{"SU full", 0, -1, func(s Screen) { s.ScrollUp(2) }, []string{"ccc", "ddd", "eee", "", ""}, []string{"aaa", "bbb"}},
		{"SU zero is one", 0, -1, func(s Screen) { s.ScrollUp(0) }, []string{"bbb", "ccc", "ddd", "eee", ""}, []string{"aaa"}},
		{"SU region", 1, 3, func(s Screen) { s.ScrollUp(1) }, []string{"aaa", "ccc", "ddd", "", "eee"}, nil},
		{"SU top-anchored region", 0, 3, func(s Screen) { s.ScrollUp(1) }, []string{"bbb", "ccc", "ddd", "", "eee"}, nil},
		{"SU beyond", 1, 3, func(s Screen) { s.ScrollUp(50) }, []string{"aaa", "", "", "", "eee"}, nil},
		{"SD full", 0, -1, func(s Screen) { s.ScrollDown(2) }, []string{"", "", "aaa", "bbb", "ccc"}, nil},
		{"SD region", 1, 3, func(s Screen) { s.ScrollDown(1) }, []string{"aaa", "", "bbb", "ccc", "eee"}, nil},
		{"SD beyond", 1, 3, func(s Screen) { s.ScrollDown(50) }, []string{"aaa", "", "", "", "eee"}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(3, 5, 10)
			numbered(s)
			s.MoveTo(1, 2)
			s.SetScrollRegion(tc.top, tc.bot)
			s.MoveTo(1, 2)
			tc.op(s)
			eqLines(t, screenText(s), tc.want...)
			eqLines(t, sbText(s), tc.sb...)
		})
	}
}

func TestDECALN(t *testing.T) {
	s := New(4, 3, 0)
	s.SetScrollRegion(1, 2)
	setPen(s, redBg())
	s.MoveTo(2, 1)
	s.FillScreen("E")
	eqLines(t, screenText(s), "EEEE", "EEEE", "EEEE")
	eqCursor(t, s, 0, 0)
	if top, bot := s.ScrollRegion(); top != 0 || bot != 2 {
		t.Fatalf("region %d,%d", top, bot)
	}
}

func TestInsertMode(t *testing.T) {
	s := New(6, 2, 0)
	put(s, "abcd")
	s.SetInsert(true)
	if !s.InsertMode() {
		t.Fatal("insert not set")
	}
	s.MoveTo(1, 0)
	put(s, "XY")
	eqLines(t, screenText(s), "aXYbcd", "")
	put(s, "Z")
	eqLines(t, screenText(s), "aXYZbc", "")
	eqCursor(t, s, 4, 0)
}

func TestInsertModeWide(t *testing.T) {
	t.Run("wide insert pushes cells", func(t *testing.T) {
		s := New(6, 1, 0)
		put(s, "abcd")
		s.SetInsert(true)
		s.MoveTo(1, 0)
		put(s, "世")
		eqLines(t, screenText(s), "a世bcd")
		if !s.Line(0)[1].Wide || s.Line(0)[2].Cluster != "" {
			t.Fatal("wide cell layout")
		}
	})
	t.Run("pushing wide char off the end blanks it", func(t *testing.T) {
		s := New(5, 1, 0)
		put(s, "ab世c")
		s.SetInsert(true)
		s.MoveTo(0, 0)
		put(s, "X")
		// 世 would straddle the margin after the shift: abX... "ab世c" -> "Xab" then 世 at 3..4 fits
		eqLines(t, screenText(s), "Xab世c"[:0]+"Xab世")
		s.MoveTo(0, 0)
		put(s, "Y")
		cells := s.Line(0)
		if cells[4].Wide {
			t.Fatalf("orphaned leading half at the margin: %+v", cells)
		}
	})
	t.Run("insert inside a wide pair splits it", func(t *testing.T) {
		s := New(6, 1, 0)
		put(s, "a世b")
		s.SetInsert(true)
		s.MoveTo(2, 0)
		put(s, "X")
		cells := s.Line(0)
		for i, c := range cells {
			if c.Wide {
				t.Fatalf("stray wide flag at %d: %+v", i, cells)
			}
		}
		if got := rowText(s, 0); got != "a X b" {
			t.Fatalf("%q", got)
		}
	})
}

func TestWide(t *testing.T) {
	t.Run("occupies two cells", func(t *testing.T) {
		s := New(6, 1, 0)
		put(s, "世a")
		c := s.Line(0)
		if !c[0].Wide || c[0].Cluster != "世" || c[1].Cluster != "" || c[1].Wide || c[2].Cluster != "a" {
			t.Fatalf("%+v", c[:3])
		}
		eqCursor(t, s, 3, 0)
	})
	tests := []struct {
		name string
		init string
		x    int
		str  string
		want string
	}{
		{"narrow over leading", "a世b", 1, "x", "ax b"},
		{"narrow over trailing", "a世b", 2, "x", "a xb"},
		{"wide over leading", "a世b", 1, "界", "a界b"},
		{"wide over trailing", "a世b", 2, "界", "a 界"},
		{"wide over two wides straddling", "世世", 1, "界", " 界"},
		{"narrow pair over wide", "a世b", 1, "xy", "axyb"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(6, 1, 0)
			put(s, tc.init)
			s.MoveTo(tc.x, 0)
			put(s, tc.str)
			if got := rowText(s, 0); got != tc.want {
				t.Fatalf("%q want %q", got, tc.want)
			}
			assertWellFormed(t, s.Line(0))
		})
	}
}

// assertWellFormed checks that every leading wide cell is followed by an
// empty trailing cell and that no trailing cell stands without its leading
// half.
func assertWellFormed(t *testing.T, cells []Cell) {
	t.Helper()
	for i, c := range cells {
		if c.Wide {
			if c.Cluster == "" || i+1 >= len(cells) || cells[i+1].Cluster != "" || cells[i+1].Wide {
				t.Fatalf("malformed wide pair at %d: %+v", i, cells)
			}
		}
	}
}

func TestWideAtLastColumn(t *testing.T) {
	t.Run("autowrap wraps and pads", func(t *testing.T) {
		s := New(4, 2, 0)
		put(s, "abc世")
		eqLines(t, screenText(s), "abc", "世")
		if !s.LineWrapped(0) {
			t.Fatal("not marked wrapped")
		}
		eqCursor(t, s, 2, 1)
		if s.Line(0)[3].Cluster != "" {
			t.Fatal("pad cell not empty")
		}
	})
	t.Run("pad uses pen background", func(t *testing.T) {
		s := New(4, 2, 0)
		put(s, "abc")
		setPen(s, redBg())
		put(s, "世")
		if s.Line(0)[3].Style.Bg != redBg().Bg {
			t.Fatal("pad cell lost bg")
		}
	})
	t.Run("no autowrap discards", func(t *testing.T) {
		s := New(4, 2, 0)
		s.SetAutowrap(false)
		put(s, "abc世")
		eqLines(t, screenText(s), "abc", "")
		eqCursor(t, s, 3, 0)
	})
	t.Run("pending wrap then wide", func(t *testing.T) {
		s := New(4, 2, 0)
		put(s, "abcd世")
		eqLines(t, screenText(s), "abcd", "世")
		eqCursor(t, s, 2, 1)
	})
	t.Run("wide filling the row sets pending", func(t *testing.T) {
		s := New(4, 2, 0)
		put(s, "ab世")
		eqCursor(t, s, 3, 0)
		eqPending(t, s, true)
	})
	t.Run("one column grid drops wide", func(t *testing.T) {
		s := New(1, 2, 0)
		put(s, "世")
		eqLines(t, screenText(s), "", "")
	})
	t.Run("wide wrap at region bottom scrolls", func(t *testing.T) {
		s := New(3, 2, 5)
		put(s, "abc\r\nab世")
		eqLines(t, screenText(s), "ab", "世")
		eqLines(t, sbText(s), "abc")
	})
}

func TestWideRepairByErase(t *testing.T) {
	tests := []struct {
		name string
		op   func(Screen)
		x    int
		want string
	}{
		{"ECH over trailing half", func(s Screen) { s.EraseChars(1) }, 2, "a  b"},
		{"ECH over leading half", func(s Screen) { s.EraseChars(1) }, 1, "a  b"},
		{"EL end from trailing", func(s Screen) { s.EraseLine(EraseToEnd) }, 2, "a"},
		{"EL start through leading", func(s Screen) { s.EraseLine(EraseToStart) }, 1, "   b"},
		{"DCH at trailing", func(s Screen) { s.DeleteChars(1) }, 2, "a b"},
		{"DCH at leading", func(s Screen) { s.DeleteChars(1) }, 1, "a b"},
		{"ICH at trailing", func(s Screen) { s.InsertChars(1) }, 2, "a   b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(8, 1, 0)
			put(s, "a世b")
			s.MoveTo(tc.x, 0)
			tc.op(s)
			if got := rowText(s, 0); got != tc.want {
				t.Fatalf("%q want %q", got, tc.want)
			}
			assertWellFormed(t, s.Line(0))
		})
	}
	t.Run("ICH pushing wide char off the end", func(t *testing.T) {
		s := New(4, 1, 0)
		put(s, "ab世")
		s.MoveTo(0, 0)
		s.InsertChars(1)
		assertWellFormed(t, s.Line(0))
		if got := rowText(s, 0); got != " ab" {
			t.Fatalf("%q", got)
		}
	})
}

func TestCombine(t *testing.T) {
	s := New(5, 1, 0)
	put(s, "e")
	s.Print("́", 0)
	if s.Line(0)[0].Cluster != "é" {
		t.Fatalf("%q", s.Line(0)[0].Cluster)
	}
	eqCursor(t, s, 1, 0)
	s.MoveTo(0, 0)
	s.CarriageReturn()
	s.Print("́", 0)
	if s.Line(0)[0].Cluster != "é" {
		t.Fatal("combined at column 0")
	}
	t.Run("pending wrap targets the last cell", func(t *testing.T) {
		s := New(2, 1, 0)
		put(s, "ab")
		s.Print("́", 0)
		if s.Line(0)[1].Cluster != "b́" {
			t.Fatalf("%q", s.Line(0)[1].Cluster)
		}
	})
	t.Run("onto wide", func(t *testing.T) {
		s := New(5, 1, 0)
		put(s, "世")
		s.Print("️", 0)
		if s.Line(0)[0].Cluster != "世️" {
			t.Fatalf("%q", s.Line(0)[0].Cluster)
		}
	})
}

func TestStyleAndLinkOnCells(t *testing.T) {
	s := New(5, 1, 0)
	c := s.Cursor()
	c.Style = redBg()
	c.Link = "http://x"
	s.SetCursor(c)
	put(s, "a世")
	cells := s.Line(0)
	for i := 0; i < 3; i++ {
		if cells[i].Style != redBg() || cells[i].Link != "http://x" {
			t.Fatalf("cell %d: %+v", i, cells[i])
		}
	}
}

func TestWrappedFlagClearedByErase(t *testing.T) {
	for _, tc := range []struct {
		name string
		do   func(Screen)
		want bool
	}{
		{"EL end", func(s Screen) { s.EraseLine(EraseToEnd) }, false},
		{"EL all", func(s Screen) { s.EraseLine(EraseAll) }, false},
		{"EL start", func(s Screen) { s.EraseLine(EraseToStart) }, true},
		{"ED all", func(s Screen) { s.EraseDisplay(EraseAll) }, false},
		{"ED end", func(s Screen) { s.EraseDisplay(EraseToEnd) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New(4, 3, 0)
			put(s, "abcdef")
			s.MoveTo(1, 0)
			tc.do(s)
			if got := s.LineWrapped(0); got != tc.want {
				t.Fatalf("wrapped %v want %v", got, tc.want)
			}
		})
	}
	t.Run("scrolled lines carry their flag", func(t *testing.T) {
		s := New(4, 3, 0)
		put(s, "abcdef")
		s.MoveTo(0, 2)
		s.LineFeed()
		s.MoveTo(0, 0)
		s.ScrollDown(1)
		if s.LineWrapped(0) {
			t.Fatal("blank row inherited wrapped flag")
		}
	})
}

func TestAlternateBuffer(t *testing.T) {
	s := New(5, 3, 10)
	put(s, "main")
	s.UseAlternate(true, true)
	if !s.Alternate() {
		t.Fatal("not alternate")
	}
	eqLines(t, screenText(s), "", "", "")
	s.MoveTo(0, 0)
	put(s, "alt")
	s.UseAlternate(false, false)
	if s.Alternate() {
		t.Fatal("still alternate")
	}
	eqLines(t, screenText(s), "main", "", "")

	t.Run("contents kept without clear", func(t *testing.T) {
		s.UseAlternate(true, false)
		eqLines(t, screenText(s), "alt", "", "")
	})
	t.Run("clear on re-entry", func(t *testing.T) {
		s.UseAlternate(true, true)
		eqLines(t, screenText(s), "", "", "")
	})
	t.Run("clear on exit", func(t *testing.T) {
		put(s, "zzz")
		s.UseAlternate(false, true)
		s.UseAlternate(true, false)
		eqLines(t, screenText(s), "", "", "")
		s.UseAlternate(false, false)
	})
	t.Run("redundant switches", func(t *testing.T) {
		s.UseAlternate(false, false)
		eqLines(t, screenText(s), "main", "", "")
	})
}

func TestAlternateHasNoScrollback(t *testing.T) {
	s := New(3, 2, 10)
	s.UseAlternate(true, true)
	put(s, "a\nb\nc\nd\ne")
	if s.ScrollbackLen() != 0 {
		t.Fatalf("scrollback %d", s.ScrollbackLen())
	}
	s.UseAlternate(false, false)
	if s.ScrollbackLen() != 0 {
		t.Fatal("alt output reached primary scrollback")
	}
}

func TestAlternateIsolation(t *testing.T) {
	s := New(4, 3, 10)
	put(s, "abcd\nxy")
	s.UseAlternate(true, true)
	s.MoveTo(0, 0)
	s.EraseDisplay(EraseAll)
	s.SetScrollRegion(1, 2)
	put(s, "q")
	s.UseAlternate(false, false)
	eqLines(t, screenText(s), "abcd", "xy", "")
	s.UseAlternate(true, false)
	if s.LineWrapped(0) {
		t.Fatal("alt wrap flag leaked")
	}
}

func TestSaveCursorAcrossAlternate(t *testing.T) {
	s := New(8, 4, 0)
	s.MoveTo(3, 2)
	s.SaveCursor()
	s.UseAlternate(true, true)
	s.MoveTo(0, 0)
	s.UseAlternate(false, false)
	s.RestoreCursor()
	eqCursor(t, s, 3, 2)
}

type feeder struct{ n int }

func (f *feeder) feed(s Screen, n int) {
	for i := 0; i < n; i++ {
		put(s, fmt.Sprintf("%d\n", f.n))
		f.n++
	}
}

func TestScrollbackLimit(t *testing.T) {
	t.Run("bounded", func(t *testing.T) {
		s := New(4, 2, 3)
		f := &feeder{}
		f.feed(s, 10)
		eqLines(t, sbText(s), "6", "7", "8")
		if s.ScrollbackLimit() != 3 {
			t.Fatal("limit")
		}
	})
	t.Run("zero keeps nothing", func(t *testing.T) {
		s := New(4, 2, 0)
		f := &feeder{}
		f.feed(s, 10)
		if s.ScrollbackLen() != 0 {
			t.Fatal("stored lines")
		}
	})
	t.Run("shrink trims oldest immediately", func(t *testing.T) {
		s := New(4, 2, 10)
		f := &feeder{}
		f.feed(s, 8)
		eqLines(t, sbText(s), "0", "1", "2", "3", "4", "5", "6")
		s.SetScrollbackLimit(2)
		eqLines(t, sbText(s), "5", "6")
		f.feed(s, 2)
		eqLines(t, sbText(s), "7", "8")
	})
	t.Run("shrink a wrapped full ring", func(t *testing.T) {
		s := New(4, 2, 4)
		f := &feeder{}
		f.feed(s, 12)
		s.SetScrollbackLimit(2)
		eqLines(t, sbText(s), "9", "10")
		put(s, "x\n")
		eqLines(t, sbText(s), "10", "11")
	})
	t.Run("grow keeps lines and retains more", func(t *testing.T) {
		s := New(4, 2, 3)
		f := &feeder{}
		f.feed(s, 8)
		s.SetScrollbackLimit(6)
		eqLines(t, sbText(s), "4", "5", "6")
		f.feed(s, 4)
		eqLines(t, sbText(s), "5", "6", "7", "8", "9", "10")
	})
	t.Run("grow a rotated full ring", func(t *testing.T) {
		s := New(4, 2, 3)
		f := &feeder{}
		f.feed(s, 9)
		s.SetScrollbackLimit(5)
		eqLines(t, sbText(s), "5", "6", "7")
		f.feed(s, 1)
		eqLines(t, sbText(s), "5", "6", "7", "8")
	})
	t.Run("to zero then back", func(t *testing.T) {
		s := New(4, 2, 5)
		f := &feeder{}
		f.feed(s, 6)
		s.SetScrollbackLimit(0)
		if s.ScrollbackLen() != 0 {
			t.Fatal("not trimmed")
		}
		f.feed(s, 3)
		s.SetScrollbackLimit(2)
		f.feed(s, 3)
		if s.ScrollbackLen() != 2 {
			t.Fatalf("len %d", s.ScrollbackLen())
		}
	})
	t.Run("negative is zero", func(t *testing.T) {
		s := New(4, 2, 5)
		f := &feeder{}
		f.feed(s, 6)
		s.SetScrollbackLimit(-5)
		if s.ScrollbackLen() != 0 || s.ScrollbackLimit() != 0 {
			t.Fatal("negative limit")
		}
	})
	t.Run("clear", func(t *testing.T) {
		s := New(4, 2, 5)
		f := &feeder{}
		f.feed(s, 6)
		s.ClearScrollback()
		if s.ScrollbackLen() != 0 || s.ScrollbackLine(0) != nil {
			t.Fatal("not cleared")
		}
		f.feed(s, 2)
		eqLines(t, sbText(s), "5", "6")
	})
	t.Run("out of range", func(t *testing.T) {
		s := New(4, 2, 5)
		f := &feeder{}
		f.feed(s, 4)
		if s.ScrollbackLine(-1) != nil || s.ScrollbackLine(99) != nil || s.ScrollbackLineWrapped(99) {
			t.Fatal("range")
		}
	})
}

func TestScrollbackAllocatesOnlyStoredLines(t *testing.T) {
	s := New(10, 2, 1_000_000).(*screen)
	put(s, "a\nb\nc")
	if cap(s.sb.lines) > 64 {
		t.Fatalf("scrollback preallocated: cap %d", cap(s.sb.lines))
	}
	if s.ScrollbackLen() != 1 {
		t.Fatalf("len %d", s.ScrollbackLen())
	}
}

func TestPartialRegionDoesNotFeedScrollback(t *testing.T) {
	s := New(3, 4, 10)
	s.SetScrollRegion(1, 3)
	s.MoveTo(0, 3)
	for i := 0; i < 5; i++ {
		put(s, "x\n")
	}
	if s.ScrollbackLen() != 0 {
		t.Fatal("region scroll fed scrollback")
	}
	s.SetScrollRegion(0, -1)
	s.MoveTo(0, 3)
	put(s, "\n")
	if s.ScrollbackLen() != 1 {
		t.Fatal("full-screen scroll did not feed scrollback")
	}
}
