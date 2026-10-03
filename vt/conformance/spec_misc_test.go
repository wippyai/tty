// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"strings"
	"testing"

	"github.com/wippyai/tty/text"
	"github.com/wippyai/tty/vt"
)

// C0 controls, string sequences, and mode flags. Reference: ctlseqs "Control
// Characters", "Operating System Commands", "DECSET", "DECSCUSR".

var controlCases = []specCase{
	{name: "BS moves left", in: "ab\bX", want: []string{"aX"}, cursor: at(2, 0)},
	{name: "BS at column 0 stays", in: "\bX", want: []string{"X"}, cursor: at(1, 0)},
	{name: "BS after a pending wrap moves off the last column", in: "\x1b[1;10HA\bX", want: []string{"        XA"}},
	{name: "LF keeps the column", in: "ab\nc", want: []string{"ab", "  c"}, cursor: at(3, 1)},
	{name: "LNM makes LF return the carriage", in: "\x1b[20hab\nc", want: []string{"ab", "c"}, cursor: at(1, 1)},
	{name: "VT and FF act as LF", in: "a\x0bb\x0cc", want: []string{"a", " b", "  c"}, cursor: at(3, 2)},
	{name: "NUL is ignored", in: "a\x00b", want: []string{"ab"}},
	{name: "DEL is ignored", in: "a\x7fb", want: []string{"ab"}},
	{name: "CAN aborts a sequence", in: "\x1b[31\x18X", want: []string{"X"},
		check: func(t *testing.T, s *sut) { requireStyle(t, s, 0, 0, text.Style{}) }},
	{name: "ESC restarts a sequence", in: "\x1b[3\x1b[31mX", want: []string{"X"},
		check: func(t *testing.T, s *sut) { requireStyle(t, s, 0, 0, text.Style{Fg: text.Basic(1)}) }},
	{name: "C0 inside CSI executes and the sequence continues", in: "abc\x1b[1\rmX", want: []string{"Xbc"},
		check: func(t *testing.T, s *sut) { requireStyle(t, s, 0, 0, text.Style{Attrs: text.AttrBold}) }},
	{name: "unknown CSI final is ignored", in: "a\x1b[99zb", want: []string{"ab"}},
	{name: "unknown ESC final is ignored", in: "a\x1b%Zb", want: []string{"ab"}},
	{name: "invalid UTF-8 byte becomes U+FFFD", in: "a\xffb", want: []string{"a�b"}, cursor: at(3, 0)},
	{name: "truncated UTF-8 becomes U+FFFD", in: "\xe4\xb8a", want: []string{"�a"}},
	{name: "valid multibyte text", in: "héllo", want: []string{"héllo"}, cursor: at(5, 0)},
	{name: "DECKPAM", in: "\x1b=", check: func(t *testing.T, s *sut) {
		if !s.term.Modes().ApplicationKeypad {
			t.Error("ApplicationKeypad want true")
		}
	}},
	{name: "DECKPNM", in: "\x1b=\x1b>", check: func(t *testing.T, s *sut) {
		if s.term.Modes().ApplicationKeypad {
			t.Error("ApplicationKeypad want false")
		}
	}},
	{name: "BEL rings", in: "\x07\x07", check: func(t *testing.T, s *sut) {
		if s.bells != 2 {
			t.Errorf("bells want 2 got %d", s.bells)
		}
	}},
}

var stringCases = []specCase{
	{name: "OSC 0 title with BEL", in: "\x1b]0;hello world\x07", want: []string{}, cursor: at(0, 0),
		check: func(t *testing.T, s *sut) { requireTitles(t, s, "hello world") }},
	{name: "OSC 2 title with ST", in: "\x1b]2;a b\x1b\\", want: []string{},
		check: func(t *testing.T, s *sut) { requireTitles(t, s, "a b") }},
	{name: "OSC title is UTF-8", in: "\x1b]2;über 中\x07", check: func(t *testing.T, s *sut) { requireTitles(t, s, "über 中") }},
	{name: "OSC terminator BEL does not ring", in: "\x1b]0;t\x07", check: func(t *testing.T, s *sut) {
		if s.bells != 0 {
			t.Errorf("bells want 0 got %d", s.bells)
		}
	}},
	{name: "terminal title reads back", in: "\x1b]0;t1\x07\x1b]2;t2\x07", check: func(t *testing.T, s *sut) {
		if got := s.term.Title(); got != "t2" {
			t.Errorf("Title want t2 got %q", got)
		}
	}},
	{
		name: "OSC 8 hyperlink marks cells", in: "\x1b]8;;http://a.b/c\x07ab\x1b]8;;\x07c", want: []string{"abc"}, cursor: at(3, 0),
		check: func(t *testing.T, s *sut) {
			for x := 0; x < 2; x++ {
				if l := s.cell(x, 0).Link; l == "" || !strings.Contains(l, "http://a.b/c") {
					t.Errorf("cell %d link %q", x, l)
				}
			}
			if l := s.cell(2, 0).Link; l != "" {
				t.Errorf("cell 2 link %q want none", l)
			}
		},
	},
	{
		name: "OSC 8 with id parameter and ST", in: "\x1b]8;id=7;http://x\x1b\\hi\x1b]8;;\x1b\\", want: []string{"hi"},
		check: func(t *testing.T, s *sut) {
			if l := s.cell(1, 0).Link; !strings.Contains(l, "http://x") {
				t.Errorf("cell 1 link %q", l)
			}
		},
	},
	{
		name: "OSC 8 link follows the cursor pen across lines", in: "\x1b]8;;http://x\x07a\r\nb\x1b]8;;\x07c", want: []string{"a", "bc"},
		check: func(t *testing.T, s *sut) {
			if !strings.Contains(s.cell(0, 1).Link, "http://x") || s.cell(1, 1).Link != "" {
				t.Errorf("links %q %q", s.cell(0, 1).Link, s.cell(1, 1).Link)
			}
		},
	},
	{
		name: "OSC 52 clipboard write", in: "\x1b]52;c;aGVsbG8=\x07", want: []string{},
		check: func(t *testing.T, s *sut) {
			if len(s.clips) != 1 || s.clips[0] != (clip{"c", "hello"}) {
				t.Errorf("clipboard %+v", s.clips)
			}
		},
	},
	{name: "OSC 133 prompt marks print nothing", in: "\x1b]133;A\x07$ \x1b]133;B\x07", want: []string{"$"}},
	{name: "OSC 7 working directory prints nothing", in: "\x1b]7;file:///tmp\x07x", want: []string{"x"}},
	{name: "OSC 9 notification prints nothing", in: "\x1b]9;done\x07x", want: []string{"x"}},
	{name: "unknown OSC prints nothing", in: "\x1b]9999;junk\x07x", want: []string{"x"}},
	{name: "DCS is swallowed", in: "a\x1bPsome data\x1b\\b", want: []string{"ab"}},
	{name: "APC is swallowed", in: "a\x1b_Xjunk\x1b\\b", want: []string{"ab"}},
	{name: "PM is swallowed", in: "a\x1b^junk\x1b\\b", want: []string{"ab"}},
	{name: "SOS is swallowed", in: "a\x1bXjunk\x1b\\b", want: []string{"ab"}},
	{name: "ESC inside a string ends it only with backslash", in: "a\x1b]0;t\x1b[31m\x07b", want: []string{"ab"}},
}

func requireTitles(t *testing.T, s *sut, want ...string) {
	t.Helper()
	if len(s.titles) != len(want) {
		t.Fatalf("titles want %q got %q", want, s.titles)
	}
	for i := range want {
		if s.titles[i] != want[i] {
			t.Errorf("title %d want %q got %q", i, want[i], s.titles[i])
		}
	}
}

func modeCheck(f func(m vt.Modes) bool) func(*testing.T, *sut) {
	return func(t *testing.T, s *sut) {
		t.Helper()
		if !f(s.term.Modes()) {
			t.Errorf("modes %+v", s.term.Modes())
		}
	}
}

var modeCases = []specCase{
	{name: "DECCKM set", in: "\x1b[?1h", check: modeCheck(func(m vt.Modes) bool { return m.ApplicationCursor })},
	{name: "DECCKM reset", in: "\x1b[?1h\x1b[?1l", check: modeCheck(func(m vt.Modes) bool { return !m.ApplicationCursor })},
	{name: "bracketed paste", in: "\x1b[?2004h", check: modeCheck(func(m vt.Modes) bool { return m.BracketedPaste })},
	{name: "bracketed paste reset", in: "\x1b[?2004h\x1b[?2004l", check: modeCheck(func(m vt.Modes) bool { return !m.BracketedPaste })},
	{name: "focus events", in: "\x1b[?1004h", check: modeCheck(func(m vt.Modes) bool { return m.FocusEvents })},
	{name: "synchronized output set", in: "\x1b[?2026h", check: modeCheck(func(m vt.Modes) bool { return m.SynchronizedOutput })},
	{name: "synchronized output reset", in: "\x1b[?2026h\x1b[?2026l", check: modeCheck(func(m vt.Modes) bool { return !m.SynchronizedOutput })},
	{name: "alternate scroll", in: "\x1b[?1007h", check: modeCheck(func(m vt.Modes) bool { return m.AlternateScroll })},
	{name: "mouse X10", in: "\x1b[?9h", check: modeCheck(func(m vt.Modes) bool { return m.MouseTracking == vt.MouseX10 })},
	{name: "mouse normal", in: "\x1b[?1000h", check: modeCheck(func(m vt.Modes) bool { return m.MouseTracking == vt.MouseNormal })},
	{name: "mouse button motion", in: "\x1b[?1002h", check: modeCheck(func(m vt.Modes) bool { return m.MouseTracking == vt.MouseButtonMotion })},
	{name: "mouse any motion", in: "\x1b[?1003h", check: modeCheck(func(m vt.Modes) bool { return m.MouseTracking == vt.MouseAnyMotion })},
	{name: "mouse off", in: "\x1b[?1003h\x1b[?1003l", check: modeCheck(func(m vt.Modes) bool { return m.MouseTracking == vt.MouseOff })},
	{name: "mouse encoding UTF-8", in: "\x1b[?1005h", check: modeCheck(func(m vt.Modes) bool { return m.MouseEncoding == vt.MouseEncodingUTF8 })},
	{name: "mouse encoding SGR", in: "\x1b[?1006h", check: modeCheck(func(m vt.Modes) bool { return m.MouseEncoding == vt.MouseEncodingSGR })},
	{name: "mouse encoding SGR pixels", in: "\x1b[?1016h", check: modeCheck(func(m vt.Modes) bool { return m.MouseEncoding == vt.MouseEncodingSGRPixels })},
	{name: "mouse encoding reset", in: "\x1b[?1006h\x1b[?1006l", check: modeCheck(func(m vt.Modes) bool { return m.MouseEncoding == vt.MouseEncodingDefault })},
	{
		name: "tracking and encoding are independent", in: "\x1b[?1000h\x1b[?1006h\x1b[?1000l",
		check: modeCheck(func(m vt.Modes) bool {
			return m.MouseTracking == vt.MouseOff && m.MouseEncoding == vt.MouseEncodingSGR
		}),
	},
	{
		name: "several modes in one sequence", in: "\x1b[?1000;1006;2004h",
		check: modeCheck(func(m vt.Modes) bool {
			return m.MouseTracking == vt.MouseNormal && m.MouseEncoding == vt.MouseEncodingSGR && m.BracketedPaste
		}),
	},
	{name: "unknown mode is ignored", in: "\x1b[?9999hX", want: []string{"X"}},
	{
		name: "DECTCEM hides the cursor", in: "\x1b[?25l",
		check: func(t *testing.T, s *sut) {
			if s.scr().Cursor().Visible {
				t.Error("Visible want false")
			}
		},
	},
	{
		name: "DECTCEM shows the cursor", in: "\x1b[?25l\x1b[?25h",
		check: func(t *testing.T, s *sut) {
			if !s.scr().Cursor().Visible {
				t.Error("Visible want true")
			}
		},
	},
	cursorShapeCase("DECSCUSR 0", "\x1b[0 q", 0),
	cursorShapeCase("DECSCUSR 1 blinking block", "\x1b[1 q", 0),
	cursorShapeCase("DECSCUSR 2 steady block", "\x1b[2 q", 0),
	cursorShapeCase("DECSCUSR 3 blinking underline", "\x1b[3 q", 1),
	cursorShapeCase("DECSCUSR 4 steady underline", "\x1b[4 q", 1),
	cursorShapeCase("DECSCUSR 5 blinking bar", "\x1b[5 q", 2),
	cursorShapeCase("DECSCUSR 6 steady bar", "\x1b[6 q", 2),
}

func cursorShapeCase(name, in string, shape int) specCase {
	return specCase{name: name, in: in, check: func(t *testing.T, s *sut) {
		if got := int(s.scr().Cursor().Shape); got != shape {
			t.Errorf("Shape want %d got %d", shape, got)
		}
	}}
}
