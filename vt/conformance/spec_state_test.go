// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"testing"

	"github.com/wippyai/tty/text"
)

// DECSC/DECRC, alternate screens, RIS and DECSTR.
// Reference: ctlseqs "DECSC", "DECRC", "DECSET 47/1047/1048/1049", "RIS", "DECSTR".

var saveRestoreCases = []specCase{
	{
		name: "DECSC DECRC restores position and rendition",
		in:   "\x1b[3;4H\x1b[1;31m\x1b7\x1b[H\x1b[0m\x1b8X",
		want: []string{"", "", "   X"}, cursor: at(4, 2),
		check: func(t *testing.T, s *sut) {
			requireStyle(t, s, 3, 2, text.Style{Fg: text.Basic(1), Attrs: text.AttrBold})
		},
	},
	{
		name: "DECRC without a save homes and resets the pen", in: "\x1b[3;3H\x1b[1m\x1b8X",
		want: []string{"X"}, cursor: at(1, 0),
		check: func(t *testing.T, s *sut) { requireStyle(t, s, 0, 0, text.Style{}) },
	},
	{name: "SCOSC SCORC pair", in: "\x1b[2;3H\x1b[s\x1b[H\x1b[uX", want: []string{"", "  X"}, cursor: at(3, 1)},
	{name: "DECRC can be repeated", in: "\x1b[2;2H\x1b7\x1b[H\x1b8\x1b[H\x1b8X", want: []string{"", " X"}},
	{name: "DECSC saves origin mode", in: "\x1b[2;4r\x1b[?6h\x1b7\x1b[?6l\x1b8\x1b[1;1HX", want: []string{"", "X"}},
	{name: "DECSC saves the active charset", in: "\x1b(0\x1b7\x1b(B\x1b8q", want: []string{"─"}},
	{name: "DECSC saves the pen color", in: "\x1b[44m\x1b7\x1b[0m\x1b8X",
		check: func(t *testing.T, s *sut) { requireStyle(t, s, 0, 0, text.Style{Bg: text.Basic(4)}) }},
	{name: "DECRC restores the saved column on another row", in: "\x1b[1;5H\x1b7\x1b[4;1H\x1b8X", want: []string{"    X"}},
	{name: "DECSET 1048 saves and DECRST 1048 restores", in: "\x1b[2;2H\x1b[?1048h\x1b[H\x1b[?1048lX", want: []string{"", " X"}},
}

var altScreenCases = []specCase{
	{
		name: "1049 round trip restores the primary screen and cursor", in: "main\x1b[?1049hALT\x1b[?1049lX",
		want: []string{"mainX"}, cursor: at(5, 0),
	},
	{
		name: "1049 enters a cleared alternate screen", in: "main\x1b[?1049h",
		want: []string{}, cursor: at(4, 0),
		check: func(t *testing.T, s *sut) {
			if !s.scr().Alternate() {
				t.Error("Alternate want true")
			}
		},
	},
	{
		name: "1049 leaves the alternate screen", in: "\x1b[?1049h\x1b[?1049l",
		check: func(t *testing.T, s *sut) {
			if s.scr().Alternate() {
				t.Error("Alternate want false")
			}
		},
	},
	{name: "1049 clears the alternate screen on every entry", in: "\x1b[?1049hALT\x1b[?1049l\x1b[?1049h", want: []string{}},
	{name: "1049 alternate content is independent", in: "main\x1b[?1049h\x1b[HALT", want: []string{"ALT"}},
	{
		name: "47 switches without clearing", in: "main\x1b[?47hALT\x1b[?47l\x1b[?47h",
		want: []string{"    ALT"},
	},
	{
		name: "47 leaves the primary screen intact", in: "main\x1b[?47hALT\x1b[?47l",
		want: []string{"main"},
	},
	{
		name: "47 shares the cursor between screens", in: "main\r\n\x1b[?47hALT\x1b[?47lX",
		want: []string{"main", "   X"}, cursor: at(4, 1),
	},
	{name: "1047 clears the alternate screen on exit", in: "\x1b[?1047hALT\x1b[?1047l\x1b[?1047h", want: []string{}},
	{name: "1047 leaves the primary screen intact", in: "main\x1b[?1047hALT\x1b[?1047l", want: []string{"main"}},
	{
		name: "alternate screen has no scrollback and does not touch the primary one", rows: 3,
		in:   "a\r\nb\r\nc\r\nd\x1b[?1049h1\r\n2\r\n3\r\n4\r\n5\x1b[?1049l",
		want: []string{"b", "c", "d"},
		check: func(t *testing.T, s *sut) {
			if n := s.scr().ScrollbackLen(); n != 1 {
				t.Errorf("primary scrollback len want 1 got %d", n)
			}
		},
	},
	{name: "1049 saves the pen too", in: "\x1b[1;31m\x1b[?1049h\x1b[0m\x1b[?1049lX",
		check: func(t *testing.T, s *sut) {
			requireStyle(t, s, 0, 0, text.Style{Fg: text.Basic(1), Attrs: text.AttrBold})
		}},
}

var resetCases = []specCase{
	{
		name: "RIS clears the screen and homes the cursor", in: "abc\x1b[1;31m\x1b[2;4r\x1b[?7l\x1b[4h\x1bcX",
		want: []string{"X"}, cursor: at(1, 0),
		check: func(t *testing.T, s *sut) { requireStyle(t, s, 0, 0, text.Style{}) },
	},
	{name: "RIS restores autowrap", in: "\x1b[?7l\x1bc\x1b[1;10HAB", want: []string{"         A", "B"}},
	{name: "RIS restores insert mode off", in: "\x1b[4h\x1bcabc\x1b[1;1HX", want: []string{"Xbc"}},
	{name: "RIS resets the scroll region", in: "\x1b[2;4r\x1bc" + fill5 + "\r\n6", want: []string{"2", "3", "4", "5", "6"}},
	{name: "RIS resets origin mode", in: "\x1b[?6h\x1bc\x1b[2;2HX", want: []string{"", " X"}},
	{name: "RIS resets the charset", in: "\x1b(0\x1bcq", want: []string{"q"}},
	{
		name: "RIS leaves the alternate screen", in: "\x1b[?1049hX\x1bc",
		want: []string{},
		check: func(t *testing.T, s *sut) {
			if s.scr().Alternate() {
				t.Error("Alternate want false")
			}
		},
	},
	{
		name: "RIS clears scrollback", rows: 3, in: "1\r\n2\r\n3\r\n4\x1bc",
		check: func(t *testing.T, s *sut) {
			if n := s.scr().ScrollbackLen(); n != 0 {
				t.Errorf("scrollback len want 0 got %d", n)
			}
		},
	},
	{
		name: "RIS shows the cursor in the default shape", in: "\x1b[?25l\x1b[5 q\x1bc",
		check: func(t *testing.T, s *sut) {
			c := s.scr().Cursor()
			if !c.Visible || c.Shape != 0 {
				t.Errorf("cursor visible=%v shape=%v", c.Visible, c.Shape)
			}
		},
	},
	{
		name: "RIS resets application modes", in: "\x1b[?2004h\x1b[?1h\x1b[?1000h\x1b[?1006h\x1b[>1u\x1b=\x1bc",
		check: func(t *testing.T, s *sut) {
			if m := s.term.Modes(); m.BracketedPaste || m.ApplicationCursor || m.ApplicationKeypad ||
				m.MouseTracking != 0 || m.MouseEncoding != 0 || m.KittyKeyboardFlags != 0 {
				t.Errorf("modes not reset: %+v", m)
			}
		},
	},

	{name: "DECSTR keeps the screen", in: "abc\x1b[!p", want: []string{"abc"}},
	{name: "DECSTR resets the pen", in: "\x1b[1;31m\x1b[!p\x1b[1;1HX", want: []string{"X"},
		check: func(t *testing.T, s *sut) { requireStyle(t, s, 0, 0, text.Style{}) }},
	{name: "DECSTR resets margins and origin mode", in: "\x1b[2;4r\x1b[?6h\x1b[!p\x1b[2;2HX", want: []string{"", " X"}},
	{name: "DECSTR restores autowrap", in: "\x1b[?7l\x1b[!p\x1b[1;10HAB", want: []string{"         A", "B"}},
	{name: "DECSTR resets insert mode", in: "abc\x1b[4h\x1b[!p\x1b[1;1HX", want: []string{"Xbc"}},
	{name: "DECSTR resets charsets", in: "\x1b(0\x1b[!p\x1b[1;1Hq", want: []string{"q"}},
	{name: "DECSTR resets the saved cursor", in: "\x1b[3;3H\x1b7\x1b[!p\x1b8X", want: []string{"X"}},
	{
		name: "DECSTR shows the cursor", in: "\x1b[?25l\x1b[!p",
		check: func(t *testing.T, s *sut) {
			if !s.scr().Cursor().Visible {
				t.Error("cursor Visible want true")
			}
		},
	},
}
