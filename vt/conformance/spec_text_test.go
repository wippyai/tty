// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"strings"
	"testing"
)

func sp(n int) string { return strings.Repeat(" ", n) }

// Autowrap with the deferred (pending) wrap state, wide and combining
// characters, tab stops. Reference: ctlseqs DECAWM, HTS, TBC, CHT, CBT; Unicode
// East Asian Width for wide cells.

var wrapCases = []specCase{
	{
		name: "last column write defers the wrap", in: "\x1b[1;10HA",
		want: []string{"         A"}, cursor: at(9, 0),
		check: func(t *testing.T, s *sut) {
			if !s.scr().Cursor().PendingWrap {
				t.Error("PendingWrap want true")
			}
		},
	},
	{name: "next character wraps", in: "\x1b[1;10HAB", want: []string{"         A", "B"}, cursor: at(1, 1)},
	{name: "text wraps at the margin", in: "abcdefghijkl", want: []string{"abcdefghij", "kl"}, cursor: at(2, 1)},
	{
		name: "autowrap off overwrites the last column", in: "\x1b[?7l\x1b[1;10HABC",
		want: []string{"         C"}, cursor: at(9, 0),
		check: func(t *testing.T, s *sut) {
			if s.scr().Cursor().PendingWrap {
				t.Error("PendingWrap want false")
			}
		},
	},
	{name: "autowrap restored by DECSET 7", in: "\x1b[?7l\x1b[?7h\x1b[1;10HAB", want: []string{"         A", "B"}},
	{name: "CR clears the pending wrap", in: "\x1b[1;10HA\rB", want: []string{"B        A"}, cursor: at(1, 0)},
	{name: "CUB clears the pending wrap", in: "\x1b[1;10HA\x1b[DX", want: []string{"        XA"}, cursor: at(9, 0)},
	{name: "CUP clears the pending wrap", in: "\x1b[1;10HA\x1b[1;1HB", want: []string{"B        A"}, cursor: at(1, 0)},
	{name: "LF clears the pending wrap", in: "\x1b[1;10HA\nB", want: []string{"         A", "         B"}, cursor: at(9, 1)},
	{name: "exactly one line of text adds no blank line", in: "abcdefghij\r\nk", want: []string{"abcdefghij", "k"}, cursor: at(1, 1)},
	{
		name: "soft wrap is recorded", in: "abcdefghijk",
		check: func(t *testing.T, s *sut) {
			if !s.scr().LineWrapped(0) {
				t.Error("LineWrapped(0) want true")
			}
			if s.scr().LineWrapped(1) {
				t.Error("LineWrapped(1) want false")
			}
		},
	},
	{
		name: "hard newline is not a soft wrap", in: "abcdefghij\r\nk",
		check: func(t *testing.T, s *sut) {
			if s.scr().LineWrapped(0) {
				t.Error("LineWrapped(0) want false")
			}
		},
	},
	{
		name: "wrap at the bottom scrolls", cols: 4, rows: 2, in: "abcdefghi",
		want: []string{"efgh", "i"}, cursor: at(1, 1),
		check: func(t *testing.T, s *sut) {
			if n := s.scr().ScrollbackLen(); n != 1 {
				t.Fatalf("scrollback len want 1 got %d", n)
			}
			if g := scrollbackText(s, 0); g != "abcd" {
				t.Errorf("scrollback[0] want abcd got %q", g)
			}
		},
	},
	{name: "DECALN fills every cell", in: alnFill, want: []string{eRow, eRow, eRow, eRow, eRow}, cursor: at(0, 0)},
	{name: "DECALN resets the scroll region", in: "\x1b[2;4r\x1b#8\x1b[5;1H\n", want: []string{eRow, eRow, eRow, eRow, ""}},
}

var wideCases = []specCase{
	{
		name: "wide character occupies two cells", in: "中a",
		want: []string{"中a"}, cursor: at(3, 0),
		check: func(t *testing.T, s *sut) {
			lead, trail, next := s.cell(0, 0), s.cell(1, 0), s.cell(2, 0)
			if lead.Cluster != "中" || !lead.Wide {
				t.Errorf("lead cell %+v", lead)
			}
			if trail.Cluster != "" || trail.Wide {
				t.Errorf("trailing cell %+v", trail)
			}
			if next.Cluster != "a" || next.Wide {
				t.Errorf("next cell %+v", next)
			}
		},
	},
	{name: "emoji is wide", in: "\U0001F642x", want: []string{"\U0001F642x"}, cursor: at(3, 0)},
	{name: "wide character at the last column wraps first", in: "\x1b[1;10H中", want: []string{"", "中"}, cursor: at(2, 1)},
	{name: "wide character filling the last two columns defers the wrap", in: "\x1b[1;9H中", want: []string{"        中"}, cursor: at(9, 0)},
	{name: "narrow overwrite of the trailing half blanks the lead", in: "中\x1b[1;2Hx", want: []string{" x"}, cursor: at(2, 0)},
	{name: "narrow overwrite of the lead blanks the trailing half", in: "中\x1b[1;1Hx", want: []string{"x"}, cursor: at(1, 0)},
	{name: "wide overwrite of two narrow cells", in: "ab\x1b[1;1H中", want: []string{"中"}, cursor: at(2, 0)},
	{name: "wide overwrite straddling two wide cells", in: "中中\x1b[1;2H文", want: []string{" 文"}, cursor: at(3, 0)},
	{name: "wide characters wrap as a unit", in: "\x1b[1;7H中中", want: []string{"      中中"}, cursor: at(9, 0)},
	{name: "wide in insert mode shifts by two", in: "abc\x1b[1;1H\x1b[4h中", want: []string{"中abc"}, cursor: at(2, 0)},
	{
		name: "combining mark joins its base", in: "éx", want: []string{"éx"}, cursor: at(2, 0),
		check: func(t *testing.T, s *sut) {
			if c := s.cell(0, 0); c.Cluster != "é" || c.Wide {
				t.Errorf("cell 0 %+v", c)
			}
		},
	},
	{name: "combining mark at the last column stays in the cell", in: "\x1b[1;10Hé", want: []string{"         é"}, cursor: at(9, 0)},
	{name: "ECH over a wide character erases both halves", in: "中x\x1b[1;1H\x1b[X", want: []string{"  x"}},
	{name: "EL 1 through the lead erases the whole wide character", in: "a中b\x1b[1;3H\x1b[1K", want: []string{"   b"}},
}

var tabCases = []specCase{
	{name: "default stops every eight columns", cols: 20, rows: 3, in: "\tX", want: []string{sp(8) + "X"}, cursor: at(9, 0)},
	{name: "tab from mid-stop", cols: 20, rows: 3, in: "a\tb", want: []string{"a       b"}},
	{name: "two tabs", cols: 20, rows: 3, in: "\t\tX", want: []string{sp(16) + "X"}, cursor: at(17, 0)},
	{name: "tab clamps at the last column", cols: 20, rows: 3, in: "\t\t\tX", want: []string{sp(19) + "X"}},
	{name: "tab is non-destructive", cols: 20, rows: 3, in: "abcdefghijk\r\tX", want: []string{"abcdefghXjk"}},
	{name: "HTS sets a stop", cols: 20, rows: 3, in: "\x1b[1;4H\x1bH\r\tX", want: []string{"   X"}, cursor: at(4, 0)},
	{name: "TBC 0 clears the stop at the cursor", cols: 20, rows: 3, in: "\x1b[1;9H\x1b[g\r\tX", want: []string{sp(16) + "X"}},
	{name: "TBC 0 explicit", cols: 20, rows: 3, in: "\x1b[1;9H\x1b[0g\r\tX", want: []string{sp(16) + "X"}},
	{name: "TBC 3 clears every stop", cols: 20, rows: 3, in: "\x1b[3g\tX", want: []string{sp(19) + "X"}},
	{name: "CHT default", cols: 20, rows: 3, in: "\x1b[IX", want: []string{sp(8) + "X"}},
	{name: "CHT count", cols: 20, rows: 3, in: "\x1b[2IX", want: []string{sp(16) + "X"}},
	{name: "CHT clamps", cols: 20, rows: 3, in: "\x1b[9IX", want: []string{sp(19) + "X"}},
	{name: "CBT default", cols: 20, rows: 3, in: "\x1b[1;20H\x1b[ZX", want: []string{sp(16) + "X"}, cursor: at(17, 0)},
	{name: "CBT count", cols: 20, rows: 3, in: "\x1b[1;20H\x1b[2ZX", want: []string{sp(8) + "X"}},
	{name: "CBT stops at column 0", cols: 20, rows: 3, in: "\x1b[1;5H\x1b[9ZX", want: []string{"X"}},
	{name: "CBT uses stops set by HTS", cols: 20, rows: 3, in: "\x1b[1;4H\x1bH\x1b[1;6H\x1b[ZX", want: []string{"   X"}},
	{name: "tab stops default on a narrow screen", in: "\t\tX", want: []string{sp(9) + "X"}},
	{name: "RIS restores the default stops", cols: 20, rows: 3, in: "\x1b[3g\x1bc\tX", want: []string{sp(8) + "X"}},
}
