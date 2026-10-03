// SPDX-License-Identifier: MPL-2.0

package vt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wippyai/tty/text"
	"github.com/wippyai/tty/vt/screen"
)

func TestCursorMovement(t *testing.T) {
	cases := []struct {
		name string
		in   string
		x, y int
	}{
		{"CUP", "\x1b[5;10H", 9, 4},
		{"CUP default", "\x1b[H", 0, 0},
		{"HVP", "\x1b[3;4f", 3, 2},
		{"CUU CUD", "\x1b[10;10H\x1b[3A\x1b[1B", 9, 7},
		{"CUF CUB", "\x1b[1;10H\x1b[5C\x1b[2D", 12, 0},
		{"CNL", "\x1b[3;5H\x1b[2E", 0, 4},
		{"CPL", "\x1b[5;5H\x1b[2F", 0, 2},
		{"CHA", "\x1b[3;5H\x1b[9G", 8, 2},
		{"HPA", "\x1b[3;5H\x1b[2`", 1, 2},
		{"VPA", "\x1b[3;5H\x1b[8d", 4, 7},
		{"zero count is one", "\x1b[5;5H\x1b[0A", 4, 3},
		{"tab", "\tX", 9, 0},
		{"CHT", "\x1b[2I", 16, 0},
		{"CBT", "\x1b[1;20H\x1b[Z", 16, 0},
		{"BS", "ab\b", 1, 0},
		{"CR LF", "ab\r\n", 0, 1},
		{"NEL", "ab\x1bE", 0, 1},
		{"IND", "\x1bD", 0, 1},
		{"RI at top scrolls", "\x1bM", 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t, 40, 10)
			h.write(c.in)
			x, y := h.pos()
			assert.Equal(t, [2]int{c.x, c.y}, [2]int{x, y})
		})
	}
}

func TestSaveRestoreCursor(t *testing.T) {
	h := newHarness(t, 20, 5)
	h.write("\x1b[3;4H\x1b[31m\x1b7\x1b[1;1H\x1b[0m\x1b8")
	x, y := h.pos()
	assert.Equal(t, 3, x)
	assert.Equal(t, 2, y)
	assert.Equal(t, text.Basic(1), h.cursor().Style.Fg)
	h.write("\x1b[5;5H\x1b[s\x1b[1;1H\x1b[u")
	x, y = h.pos()
	assert.Equal(t, [2]int{4, 4}, [2]int{x, y})
}

func TestEraseAndEdit(t *testing.T) {
	h := newHarness(t, 10, 4)
	h.write("aaaaaaaaaa\r\nbbbbbbbbbb\r\ncccccccccc")
	h.write("\x1b[2;5H\x1b[K")
	assert.Equal(t, "bbbb", h.row(1))
	h.write("\x1b[1K")
	assert.Equal(t, "", h.row(1))
	h.write("\x1b[3;5H\x1b[1J")
	assert.Equal(t, "", h.row(0))
	assert.Equal(t, "", h.row(1))
	assert.Equal(t, "     ccccc"[5:], h.row(2)[5:])
	h.write("\x1b[2J")
	assert.Equal(t, "", h.row(2))
}

func TestInsertDeleteScroll(t *testing.T) {
	h := newHarness(t, 8, 4)
	h.write("abcdefgh\x1b[1;3H\x1b[2@")
	assert.Equal(t, "ab  cdef", h.row(0))
	h.write("\x1b[2P")
	assert.Equal(t, "abcdef", h.row(0))
	h.write("\x1b[3X")
	assert.Equal(t, "ab   f", h.row(0))

	h = newHarness(t, 4, 4)
	h.write("1\r\n2\r\n3\r\n4")
	h.write("\x1b[2;1H\x1b[L")
	assert.Equal(t, []string{"1", "", "2", "3"}, []string{h.row(0), h.row(1), h.row(2), h.row(3)})
	h.write("\x1b[M")
	assert.Equal(t, []string{"1", "2", "3", ""}, []string{h.row(0), h.row(1), h.row(2), h.row(3)})
	h.write("\x1b[S")
	assert.Equal(t, []string{"2", "3", "", ""}, []string{h.row(0), h.row(1), h.row(2), h.row(3)})
	h.write("\x1b[T")
	assert.Equal(t, []string{"", "2", "3", ""}, []string{h.row(0), h.row(1), h.row(2), h.row(3)})
}

func TestScrollRegionAndOrigin(t *testing.T) {
	h := newHarness(t, 6, 6)
	h.write("\x1b[2;4r")
	x, y := h.pos()
	assert.Equal(t, [2]int{0, 0}, [2]int{x, y})
	h.write("\x1b[?6h\x1b[1;1H")
	_, y = h.pos()
	assert.Equal(t, 1, y)
	h.write("\x1b[6n")
	assert.Equal(t, "\x1b[1;1R", h.take())
	h.write("\x1b[?6l")
	h.write("\x1b[?6$p")
	assert.Equal(t, "\x1b[?6;2$y", h.take())
	h.write("\x1b[1;1Hr0\x1b[2;1Ha\r\nb\r\nc\r\nd")
	assert.Equal(t, "r0", h.row(0))
	assert.Equal(t, "b", h.row(1))
	assert.Equal(t, "c", h.row(2))
	assert.Equal(t, "d", h.row(3))
}

func TestTabStops(t *testing.T) {
	h := newHarness(t, 30, 2)
	h.write("\x1b[1;4H\x1bH\x1b[1;1H\t")
	x, _ := h.pos()
	assert.Equal(t, 3, x)
	h.write("\x1b[3g\x1b[1;1H\t")
	x, _ = h.pos()
	assert.Equal(t, 29, x)
}

func TestRepeat(t *testing.T) {
	h := newHarness(t, 10, 2)
	h.write("x\x1b[3b")
	assert.Equal(t, "xxxx", h.row(0))
}

func TestWideAndCombining(t *testing.T) {
	h := newHarness(t, 10, 2)
	h.write("a界́b")
	x, _ := h.pos()
	assert.Equal(t, 4, x)
	assert.Equal(t, "a", h.term.Screen().Line(0)[0].Cluster)
	assert.True(t, h.term.Screen().Line(0)[1].Wide)
}

func TestSGR(t *testing.T) {
	cases := []struct {
		in   string
		want text.Style
	}{
		{"\x1b[1;3;4;5;7;9m", text.Style{Attrs: text.AttrBold | text.AttrItalic | text.AttrBlink | text.AttrReverse | text.AttrStrikethrough, Underline: text.UnderlineSingle}},
		{"\x1b[31;42m", text.Style{Fg: text.Basic(1), Bg: text.Basic(2)}},
		{"\x1b[91;102m", text.Style{Fg: text.Basic(9), Bg: text.Basic(10)}},
		{"\x1b[38;5;200m", text.Style{Fg: text.Indexed(200)}},
		{"\x1b[38;5;5m", text.Style{Fg: text.Basic(5)}},
		{"\x1b[48;2;1;2;3m", text.Style{Bg: text.RGB(1, 2, 3)}},
		{"\x1b[38:2::10:20:30m", text.Style{Fg: text.RGB(10, 20, 30)}},
		{"\x1b[38:2:10:20:30m", text.Style{Fg: text.RGB(10, 20, 30)}},
		{"\x1b[38:5:99m", text.Style{Fg: text.Indexed(99)}},
		{"\x1b[4:3m", text.Style{Underline: text.UnderlineCurly}},
		{"\x1b[4:2;58:2::255:0:0m", text.Style{Underline: text.UnderlineDouble, UnderlineColor: text.RGB(255, 0, 0)}},
		{"\x1b[4;58;5;120m", text.Style{Underline: text.UnderlineSingle, UnderlineColor: text.Indexed(120)}},
		{"\x1b[21m", text.Style{Underline: text.UnderlineDouble}},
		{"\x1b[1;31;4m\x1b[m", text.Style{}},
		{"\x1b[1;31;4m\x1b[0m", text.Style{}},
		{"\x1b[1;2;3;4m\x1b[22;23;24m", text.Style{}},
		{"\x1b[31;41;4;58;5;1m\x1b[39;49;59;24m", text.Style{}},
		{"\x1b[;31m", text.Style{Fg: text.Basic(1)}},
		{"\x1b[38;2;1;2m", text.Style{}},
	}
	for _, c := range cases {
		h := newHarness(t, 10, 2)
		h.write(c.in)
		assert.Equal(t, c.want, h.cursor().Style, "%q", c.in)
	}
}

func TestSGRAppliesToPrintedCells(t *testing.T) {
	h := newHarness(t, 10, 2)
	h.write("\x1b[31mA\x1b[0mB")
	cells := h.term.Screen().Line(0)
	assert.Equal(t, text.Basic(1), cells[0].Style.Fg)
	assert.True(t, cells[1].Style.IsZero())
}

func TestModes(t *testing.T) {
	h := newHarness(t, 10, 4)
	set := func(s string) { h.write("\x1b[?" + s + "h") }
	reset := func(s string) { h.write("\x1b[?" + s + "l") }

	set("1")
	assert.True(t, h.term.Modes().ApplicationCursor)
	reset("1")
	assert.False(t, h.term.Modes().ApplicationCursor)

	reset("25")
	assert.False(t, h.cursor().Visible)
	set("25")
	assert.True(t, h.cursor().Visible)

	set("2004;1004;1007;2026")
	m := h.term.Modes()
	assert.True(t, m.BracketedPaste && m.FocusEvents && m.AlternateScroll && m.SynchronizedOutput)
	reset("2004;1004;1007;2026")
	m = h.term.Modes()
	assert.False(t, m.BracketedPaste || m.FocusEvents || m.AlternateScroll || m.SynchronizedOutput)

	set("12")
	assert.True(t, h.term.Modes().CursorBlink)

	h.write("\x1b[4h")
	h.write("abc\x1b[1;1HX")
	assert.Equal(t, "Xabc", h.row(0))
	h.write("\x1b[4l\x1b[1;1HY")
	assert.Equal(t, "Yabc", h.row(0))

	h.write("\x1b[20h\nZ")
	assert.Equal(t, "Z", h.row(1))
	h.write("\x1b[20l")
	assert.False(t, h.term.Modes().LinefeedNewline)

	reset("7")
	h.write("\x1b[2;1H" + "0123456789ab")
	assert.Equal(t, "012345678b", h.row(1))
	set("7")
}

func TestMouseModes(t *testing.T) {
	h := newHarness(t, 10, 4)
	for mode, want := range map[string]MouseTracking{"9": MouseX10, "1000": MouseNormal, "1002": MouseButtonMotion, "1003": MouseAnyMotion} {
		h.write("\x1b[?" + mode + "h")
		assert.Equal(t, want, h.term.Modes().MouseTracking, mode)
		h.write("\x1b[?" + mode + "l")
		assert.Equal(t, MouseOff, h.term.Modes().MouseTracking, mode)
	}
	for mode, want := range map[string]MouseEncoding{"1005": MouseEncodingUTF8, "1006": MouseEncodingSGR, "1016": MouseEncodingSGRPixels} {
		h.write("\x1b[?" + mode + "h")
		assert.Equal(t, want, h.term.Modes().MouseEncoding, mode)
		h.write("\x1b[?" + mode + "l")
		assert.Equal(t, MouseEncodingDefault, h.term.Modes().MouseEncoding, mode)
	}
}

func TestAlternateScreen(t *testing.T) {
	h := newHarness(t, 10, 4)
	h.write("primary\x1b[2;3H")
	h.write("\x1b[?1049h")
	assert.True(t, h.term.Screen().Alternate())
	assert.Equal(t, "", h.row(0))
	h.write("alt")
	h.write("\x1b[?1049l")
	assert.False(t, h.term.Screen().Alternate())
	assert.Equal(t, "primary", h.row(0))
	x, y := h.pos()
	assert.Equal(t, [2]int{2, 1}, [2]int{x, y})

	h.write("\x1b[?47h")
	assert.True(t, h.term.Screen().Alternate())
	h.write("zz\x1b[?47l")
	h.write("\x1b[?1047h")
	assert.Equal(t, "  zzt", h.row(1))
	h.write("\x1b[?1047l\x1b[?1047h")
	assert.Equal(t, "", h.row(1))
	h.write("\x1b[?1047l")
}

func TestKittyStackPerScreen(t *testing.T) {
	h := newHarness(t, 10, 4)
	h.write("\x1b[>5u")
	assert.Equal(t, 5, h.term.Modes().KittyKeyboardFlags)
	h.write("\x1b[?1049h")
	assert.Equal(t, 0, h.term.Modes().KittyKeyboardFlags)
	h.write("\x1b[>1u")
	assert.Equal(t, 1, h.term.Modes().KittyKeyboardFlags)
	h.write("\x1b[?1049l")
	assert.Equal(t, 5, h.term.Modes().KittyKeyboardFlags)
}

func TestCursorShape(t *testing.T) {
	cases := map[string]struct {
		shape screen.CursorShape
		blink bool
	}{
		"\x1b[0 q": {screen.CursorBlock, true}, "\x1b[2 q": {screen.CursorBlock, false},
		"\x1b[3 q": {screen.CursorUnderline, true}, "\x1b[4 q": {screen.CursorUnderline, false},
		"\x1b[5 q": {screen.CursorBar, true}, "\x1b[6 q": {screen.CursorBar, false},
	}
	for in, want := range cases {
		h := newHarness(t, 10, 2)
		h.write(in)
		assert.Equal(t, want.shape, h.cursor().Shape, "%q", in)
		assert.Equal(t, want.blink, h.term.Modes().CursorBlink, "%q", in)
	}
}

func TestSoftAndHardReset(t *testing.T) {
	h := newHarness(t, 10, 4)
	h.write("\x1b[?1h\x1b[?6h\x1b[4h\x1b[31m\x1b[?25l\x1b(0\x1b[2;3r")
	h.write("\x1b[!p")
	m := h.term.Modes()
	assert.False(t, m.ApplicationCursor)
	assert.True(t, h.cursor().Visible)
	assert.True(t, h.cursor().Style.IsZero())
	h.write("\x1b[?6$p")
	assert.Equal(t, "\x1b[?6;2$y", h.take())
	h.write("q")
	assert.Equal(t, "q", h.row(0))

	h.write("\x1b[?2004h\x1b[>3u\x1b[?1049hjunk\x1bc")
	assert.False(t, h.term.Screen().Alternate())
	assert.Equal(t, Modes{}, h.term.Modes())
	assert.Equal(t, "", h.row(0))
	x, y := h.pos()
	assert.Equal(t, [2]int{0, 0}, [2]int{x, y})
}

func TestDECALN(t *testing.T) {
	h := newHarness(t, 3, 2)
	h.write("\x1b#8")
	assert.Equal(t, "EEE", h.row(0))
	assert.Equal(t, "EEE", h.row(1))
}

func TestCharsets(t *testing.T) {
	h := newHarness(t, 20, 2)
	h.write("\x1b(0lqk\x1b(Blqk")
	assert.Equal(t, "┌─┐lqk", h.row(0))
	h.write("\r\n\x1b)0\x0eqx\x0fqx")
	assert.Equal(t, "─│qx", h.row(1))
	h.write("\x1b[1;1H\x1b(0\x1b7\x1b(B\x1b8q")
	assert.Equal(t, "─", h.term.Screen().Line(0)[0].Cluster)
}

func TestBell(t *testing.T) {
	h := newHarness(t, 10, 2)
	h.write("\a\a")
	assert.Equal(t, 2, h.bells)
}

func TestScrollbackAndResize(t *testing.T) {
	h := newHarness(t, 5, 2)
	h.write("a\r\nb\r\nc")
	assert.Equal(t, 1, h.term.Screen().ScrollbackLen())
	h.term.SetScrollbackLimit(0)
	assert.Equal(t, 0, h.term.Screen().ScrollbackLen())
	h.term.Resize(8, 3)
	cols, rows := h.term.Screen().Size()
	assert.Equal(t, [2]int{8, 3}, [2]int{cols, rows})
	h.write("\x1b[18t")
	assert.Equal(t, "\x1b[8;3;8t", h.take())
}
