// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"testing"

	"github.com/wippyai/tty/text"
)

// SGR and character sets. Reference: ctlseqs "CSI Pm m" (including the
// colon sub-parameter forms), "ESC ( C" character set designation, and the
// DEC Special Graphics table.

func sgrCase(name, seq string, want text.Style) specCase {
	return specCase{
		name: name, in: seq + "X", want: []string{"X"},
		check: func(t *testing.T, s *sut) { requireStyle(t, s, 0, 0, want) },
	}
}

func attr(a text.Attr) text.Style { return text.Style{Attrs: a} }

var sgrCases = []specCase{
	sgrCase("default", "", text.Style{}),
	sgrCase("bold", "\x1b[1m", attr(text.AttrBold)),
	sgrCase("faint", "\x1b[2m", attr(text.AttrFaint)),
	sgrCase("italic", "\x1b[3m", attr(text.AttrItalic)),
	sgrCase("underline", "\x1b[4m", text.Style{Underline: text.UnderlineSingle}),
	sgrCase("blink", "\x1b[5m", attr(text.AttrBlink)),
	sgrCase("rapid blink", "\x1b[6m", attr(text.AttrRapidBlink)),
	sgrCase("reverse", "\x1b[7m", attr(text.AttrReverse)),
	sgrCase("conceal", "\x1b[8m", attr(text.AttrConceal)),
	sgrCase("strikethrough", "\x1b[9m", attr(text.AttrStrikethrough)),
	sgrCase("double underline", "\x1b[21m", text.Style{Underline: text.UnderlineDouble}),
	sgrCase("22 clears bold and faint", "\x1b[1;2m\x1b[22m", text.Style{}),
	sgrCase("23 clears italic", "\x1b[3m\x1b[23m", text.Style{}),
	sgrCase("24 clears underline", "\x1b[4m\x1b[24m", text.Style{}),
	sgrCase("25 clears blink", "\x1b[5m\x1b[25m", text.Style{}),
	sgrCase("27 clears reverse", "\x1b[7m\x1b[27m", text.Style{}),
	sgrCase("28 clears conceal", "\x1b[8m\x1b[28m", text.Style{}),
	sgrCase("29 clears strikethrough", "\x1b[9m\x1b[29m", text.Style{}),
	sgrCase("bold and faint are independent", "\x1b[1;2m", attr(text.AttrBold|text.AttrFaint)),
	sgrCase("22 keeps other attributes", "\x1b[1;3m\x1b[22m", attr(text.AttrItalic)),

	sgrCase("fg 30", "\x1b[30m", text.Style{Fg: text.Basic(0)}),
	sgrCase("fg 31", "\x1b[31m", text.Style{Fg: text.Basic(1)}),
	sgrCase("fg 37", "\x1b[37m", text.Style{Fg: text.Basic(7)}),
	sgrCase("fg 90 bright", "\x1b[90m", text.Style{Fg: text.Basic(8)}),
	sgrCase("fg 97 bright", "\x1b[97m", text.Style{Fg: text.Basic(15)}),
	sgrCase("bg 40", "\x1b[40m", text.Style{Bg: text.Basic(0)}),
	sgrCase("bg 47", "\x1b[47m", text.Style{Bg: text.Basic(7)}),
	sgrCase("bg 100 bright", "\x1b[100m", text.Style{Bg: text.Basic(8)}),
	sgrCase("bg 107 bright", "\x1b[107m", text.Style{Bg: text.Basic(15)}),
	sgrCase("39 resets fg", "\x1b[31m\x1b[39m", text.Style{}),
	sgrCase("49 resets bg", "\x1b[41m\x1b[49m", text.Style{}),
	sgrCase("bold does not brighten", "\x1b[1;31m", text.Style{Fg: text.Basic(1), Attrs: text.AttrBold}),
	sgrCase("fg 256 color", "\x1b[38;5;196m", text.Style{Fg: text.Indexed(196)}),
	sgrCase("fg 256 color with a low index", "\x1b[38;5;1m", text.Style{Fg: text.Indexed(1)}),
	sgrCase("bg 256 color", "\x1b[48;5;21m", text.Style{Bg: text.Indexed(21)}),
	sgrCase("fg truecolor", "\x1b[38;2;1;2;3m", text.Style{Fg: text.RGB(1, 2, 3)}),
	sgrCase("bg truecolor", "\x1b[48;2;255;128;0m", text.Style{Bg: text.RGB(255, 128, 0)}),
	sgrCase("fg truecolor colon form", "\x1b[38:2::10:20:30m", text.Style{Fg: text.RGB(10, 20, 30)}),
	sgrCase("fg truecolor colon form without colorspace", "\x1b[38:2:10:20:30m", text.Style{Fg: text.RGB(10, 20, 30)}),
	sgrCase("bg truecolor colon form", "\x1b[48:2::10:20:30m", text.Style{Bg: text.RGB(10, 20, 30)}),
	sgrCase("fg 256 colon form", "\x1b[38:5:200m", text.Style{Fg: text.Indexed(200)}),
	sgrCase("later color wins", "\x1b[31;38;5;100m", text.Style{Fg: text.Indexed(100)}),
	sgrCase("color after extended color", "\x1b[38;5;100;42m", text.Style{Fg: text.Indexed(100), Bg: text.Basic(2)}),
	sgrCase("extended color then attribute", "\x1b[38;2;1;2;3;1m", text.Style{Fg: text.RGB(1, 2, 3), Attrs: text.AttrBold}),

	sgrCase("underline style none", "\x1b[4m\x1b[4:0m", text.Style{}),
	sgrCase("underline style single", "\x1b[4:1m", text.Style{Underline: text.UnderlineSingle}),
	sgrCase("underline style double", "\x1b[4:2m", text.Style{Underline: text.UnderlineDouble}),
	sgrCase("underline style curly", "\x1b[4:3m", text.Style{Underline: text.UnderlineCurly}),
	sgrCase("underline style dotted", "\x1b[4:4m", text.Style{Underline: text.UnderlineDotted}),
	sgrCase("underline style dashed", "\x1b[4:5m", text.Style{Underline: text.UnderlineDashed}),
	sgrCase("underline color truecolor", "\x1b[58;2;1;2;3m", text.Style{UnderlineColor: text.RGB(1, 2, 3)}),
	sgrCase("underline color colon form", "\x1b[58:2::1:2:3m", text.Style{UnderlineColor: text.RGB(1, 2, 3)}),
	sgrCase("underline color indexed", "\x1b[58;5;9m", text.Style{UnderlineColor: text.Indexed(9)}),
	sgrCase("59 resets underline color", "\x1b[58;5;9m\x1b[59m", text.Style{}),

	sgrCase("combination", "\x1b[1;3;4;7;31;44m", text.Style{
		Fg: text.Basic(1), Bg: text.Basic(4), Underline: text.UnderlineSingle,
		Attrs: text.AttrBold | text.AttrItalic | text.AttrReverse,
	}),
	sgrCase("0 resets everything", "\x1b[1;31;44m\x1b[0m", text.Style{}),
	sgrCase("empty sequence resets", "\x1b[1;31m\x1b[m", text.Style{}),
	sgrCase("0 inside a list resets earlier parameters", "\x1b[1;0;3m", attr(text.AttrItalic)),
	sgrCase("empty parameter counts as 0", "\x1b[1;;3m", attr(text.AttrItalic)),
	sgrCase("leading empty parameter resets", "\x1b[1m\x1b[;3m", attr(text.AttrItalic)),
	sgrCase("separate sequences accumulate", "\x1b[1m\x1b[3m\x1b[31m", text.Style{Fg: text.Basic(1), Attrs: text.AttrBold | text.AttrItalic}),
	sgrCase("unknown parameter is ignored", "\x1b[1;999;3m", attr(text.AttrBold|text.AttrItalic)),
	sgrCase("truncated extended color is ignored", "\x1b[1;38;5m", attr(text.AttrBold)),
	sgrCase("truncated truecolor is ignored", "\x1b[3;48;2;1;2m", attr(text.AttrItalic)),

	{
		name: "pen persists across lines and wraps", in: "\x1b[1;31mab\r\nc\x1b[2;10Hde",
		want: []string{"ab", "c        d", "e"},
		check: func(t *testing.T, s *sut) {
			want := text.Style{Fg: text.Basic(1), Attrs: text.AttrBold}
			requireStyle(t, s, 0, 0, want)
			requireStyle(t, s, 0, 1, want)
			requireStyle(t, s, 0, 2, want)
		},
	},
	{
		name: "cells keep the rendition they were written with", in: "\x1b[1ma\x1b[0mb",
		check: func(t *testing.T, s *sut) {
			requireStyle(t, s, 0, 0, attr(text.AttrBold))
			requireStyle(t, s, 1, 0, text.Style{})
		},
	},
	{
		name: "SGR with private marker is not SGR", in: "\x1b[?1mX",
		check: func(t *testing.T, s *sut) { requireStyle(t, s, 0, 0, text.Style{}) },
	},
	{
		name: "SGR with an intermediate is not SGR", in: "\x1b[1 mX",
		check: func(t *testing.T, s *sut) { requireStyle(t, s, 0, 0, text.Style{}) },
	},
}

const decGraphicsIn = "`afgjklmnqtuvwxyz{|}~"
const decGraphicsOut = "◆▒°±┘┐┌└┼─├┤┴┬│≤≥π≠£·"

var charsetCases = []specCase{
	{name: "DEC special graphics table", cols: 40, in: "\x1b(0" + decGraphicsIn, want: []string{decGraphicsOut}, cursor: at(21, 0)},
	{name: "DEC scan line characters", in: "\x1b(0opqrs", want: []string{"⎺⎻─⎼⎽"}},
	{name: "ESC ( B returns to ASCII", in: "\x1b(0q\x1b(Bq", want: []string{"─q"}},
	{name: "box drawing frame", in: "\x1b(0lqqk\r\nx  x\r\nmqqj", want: []string{"┌──┐", "│  │", "└──┘"}},
	{name: "non-graphic characters pass through G0 graphics", in: "\x1b(0A1 ", want: []string{"A1"}},
	{name: "SO selects G1", in: "\x1b)0\x0eq\x0fq", want: []string{"─q"}},
	{name: "G1 stays invoked until SI", in: "\x1b)0\x0eqq\x0fq", want: []string{"──q"}},
	{name: "designating G0 does not disturb the invoked G1", in: "\x1b)0\x0e\x1b(Bq", want: []string{"─"}},
	{name: "G1 not invoked leaves G0 alone", in: "\x1b)0q", want: []string{"q"}},
	{name: "LS2 invokes G2", in: "\x1b*0\x1bnq", want: []string{"─"}},
	{name: "LS3 invokes G3", in: "\x1b+0\x1boq", want: []string{"─"}},
	{name: "UK character set", in: "\x1b(A#", want: []string{"£"}},
	{name: "US character set", in: "\x1b(A\x1b(B#", want: []string{"#"}},
	{name: "UTF-8 text bypasses graphics set", in: "\x1b(0é", want: []string{"é"}},
	{name: "graphics characters are one cell wide", in: "\x1b(0qq", cursor: at(2, 0)},
}
