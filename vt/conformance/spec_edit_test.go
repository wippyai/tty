// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"testing"

	"github.com/wippyai/tty/text"
)

// Editing and erasing: IL DL ICH DCH ECH REP IRM, ED EL with background color
// erase. Reference: ctlseqs "ICH", "DCH", "ECH", "IL", "DL", "ED", "EL", "REP".

const (
	abc10 = "\x1b[1;1Habcdefghij"
	eRow  = "EEEEEEEEEE"
)

var editingCases = []specCase{
	{name: "IL inserts blank lines", in: fill5 + "\x1b[2;1H\x1b[2L", want: []string{"1", "", "", "2", "3"}, cursor: at(0, 1)},
	{name: "IL default count", in: fill5 + "\x1b[2;1H\x1b[L", want: []string{"1", "", "2", "3", "4"}, cursor: at(0, 1)},
	{name: "IL moves cursor to column 0", in: fill5 + "\x1b[2;5H\x1b[L", cursor: at(0, 1)},
	{name: "IL beyond region clears to margin", in: fill5 + "\x1b[2;4r\x1b[3;1H\x1b[9L", want: []string{"1", "2", "", "", "5"}},
	{name: "IL stays inside the region", in: fill5 + "\x1b[2;4r\x1b[2;1H\x1b[L", want: []string{"1", "", "2", "3", "5"}},
	{name: "IL outside the region is ignored", in: fill5 + "\x1b[2;4r\x1b[5;1H\x1b[L", want: []string{"1", "2", "3", "4", "5"}},
	{name: "IL above the region is ignored", in: fill5 + "\x1b[2;4r\x1b[1;1H\x1b[L", want: []string{"1", "2", "3", "4", "5"}},
	{name: "DL deletes lines", in: fill5 + "\x1b[2;1H\x1b[2M", want: []string{"1", "4", "5", "", ""}, cursor: at(0, 1)},
	{name: "DL default count", in: fill5 + "\x1b[2;1H\x1b[M", want: []string{"1", "3", "4", "5", ""}},
	{name: "DL moves cursor to column 0", in: fill5 + "\x1b[2;5H\x1b[M", cursor: at(0, 1)},
	{name: "DL stays inside the region", in: fill5 + "\x1b[2;4r\x1b[2;1H\x1b[M", want: []string{"1", "3", "4", "", "5"}},
	{name: "DL beyond region clears to margin", in: fill5 + "\x1b[2;4r\x1b[3;1H\x1b[9M", want: []string{"1", "2", "", "", "5"}},
	{name: "DL outside the region is ignored", in: fill5 + "\x1b[2;4r\x1b[5;1H\x1b[M", want: []string{"1", "2", "3", "4", "5"}},
	{
		name: "IL fills new lines with the current background", in: fill5 + "\x1b[44m\x1b[2;1H\x1b[L",
		want:  []string{"1", "", "2", "3", "4"},
		check: func(t *testing.T, s *sut) { requireBgRow(t, s, 1, 0, defCols, text.Basic(4)) },
	},
	{
		name: "DL fills freed lines with the current background", in: fill5 + "\x1b[44m\x1b[2;1H\x1b[M",
		want:  []string{"1", "3", "4", "5", ""},
		check: func(t *testing.T, s *sut) { requireBgRow(t, s, 4, 0, defCols, text.Basic(4)) },
	},

	{name: "ICH inserts blanks", in: abc10 + "\x1b[1;3H\x1b[2@", want: []string{"ab  cdefgh"}, cursor: at(2, 0)},
	{name: "ICH default count", in: abc10 + "\x1b[1;3H\x1b[@", want: []string{"ab cdefghi"}, cursor: at(2, 0)},
	{name: "ICH zero is one", in: abc10 + "\x1b[1;3H\x1b[0@", want: []string{"ab cdefghi"}},
	{name: "ICH beyond the line clears to the margin", in: abc10 + "\x1b[1;9H\x1b[99@", want: []string{"abcdefgh"}},
	{name: "DCH deletes chars", in: abc10 + "\x1b[1;3H\x1b[2P", want: []string{"abefghij"}, cursor: at(2, 0)},
	{name: "DCH default count", in: abc10 + "\x1b[1;3H\x1b[P", want: []string{"abdefghij"}},
	{name: "DCH beyond the line clears to the margin", in: abc10 + "\x1b[1;4H\x1b[99P", want: []string{"abc"}},
	{
		name: "DCH fills the right edge with the current background", in: abc10 + "\x1b[44m\x1b[1;3H\x1b[2P",
		want:  []string{"abefghij"},
		check: func(t *testing.T, s *sut) { requireBgRow(t, s, 0, 8, 10, text.Basic(4)) },
	},
	{
		name: "ICH fills inserted cells with the current background", in: abc10 + "\x1b[44m\x1b[1;3H\x1b[2@",
		want: []string{"ab  cdefgh"},
		check: func(t *testing.T, s *sut) {
			requireBgRow(t, s, 0, 2, 4, text.Basic(4))
			requireBgRow(t, s, 0, 4, 10, text.Color{})
		},
	},
	{name: "ECH erases without moving", in: abc10 + "\x1b[1;3H\x1b[3X", want: []string{"ab   fghij"}, cursor: at(2, 0)},
	{name: "ECH default count", in: abc10 + "\x1b[1;3H\x1b[X", want: []string{"ab defghij"}},
	{name: "ECH clamps at the right edge", in: abc10 + "\x1b[1;9H\x1b[99X", want: []string{"abcdefgh"}},
	{
		name: "ECH erases with the current background only", in: abc10 + "\x1b[1;31;44m\x1b[1;3H\x1b[3X",
		want: []string{"ab   fghij"},
		check: func(t *testing.T, s *sut) {
			for x := 2; x < 5; x++ {
				requireStyle(t, s, x, 0, text.Style{Bg: text.Basic(4)})
			}
		},
	},

	{name: "IRM inserts", in: "abc\x1b[1;1H\x1b[4hX", want: []string{"Xabc"}, cursor: at(1, 0)},
	{name: "IRM reset overwrites", in: "abc\x1b[1;1H\x1b[4h\x1b[4lX", want: []string{"Xbc"}, cursor: at(1, 0)},
	{name: "IRM pushes text off the edge", in: abc10 + "\x1b[1;1H\x1b[4hXY", want: []string{"XYabcdefgh"}},
	{name: "REP repeats the last character", in: "a\x1b[3b", want: []string{"aaaa"}, cursor: at(4, 0)},
	{name: "REP default count", in: "a\x1b[b", want: []string{"aa"}, cursor: at(2, 0)},
	{name: "REP wraps like printing", in: "\x1b[1;9Ha\x1b[3b", want: []string{"        aa", "aa"}, cursor: at(2, 1)},
}

var eraseCases = []specCase{
	{name: "ED 0", in: alnFill + "\x1b[3;5H\x1b[J", want: []string{eRow, eRow, "EEEE"}, cursor: at(4, 2)},
	{name: "ED 0 explicit", in: alnFill + "\x1b[3;5H\x1b[0J", want: []string{eRow, eRow, "EEEE"}, cursor: at(4, 2)},
	{name: "ED 1 includes the cursor cell", in: alnFill + "\x1b[3;5H\x1b[1J", want: []string{"", "", "     EEEEE", eRow, eRow}, cursor: at(4, 2)},
	{name: "ED 2", in: alnFill + "\x1b[3;5H\x1b[2J", want: []string{}, cursor: at(4, 2)},
	{name: "EL 0", in: alnFill + "\x1b[3;5H\x1b[K", want: []string{eRow, eRow, "EEEE", eRow, eRow}, cursor: at(4, 2)},
	{name: "EL 0 explicit", in: alnFill + "\x1b[3;5H\x1b[0K", want: []string{eRow, eRow, "EEEE", eRow, eRow}},
	{name: "EL 1 includes the cursor cell", in: alnFill + "\x1b[3;5H\x1b[1K", want: []string{eRow, eRow, "     EEEEE", eRow, eRow}, cursor: at(4, 2)},
	{name: "EL 2", in: alnFill + "\x1b[3;5H\x1b[2K", want: []string{eRow, eRow, "", eRow, eRow}, cursor: at(4, 2)},
	{name: "ED 0 at home clears everything", in: alnFill + "\x1b[H\x1b[J", want: []string{}},
	{name: "ED 1 at the last cell clears everything", in: alnFill + "\x1b[5;10H\x1b[1J", want: []string{}},
	{
		name: "ED 2 uses the current background", in: alnFill + "\x1b[44m\x1b[2J",
		want: []string{},
		check: func(t *testing.T, s *sut) {
			for y := 0; y < defRows; y++ {
				requireBgRow(t, s, y, 0, defCols, text.Basic(4))
			}
		},
	},
	{
		name: "ED 0 uses the current background", in: alnFill + "\x1b[44m\x1b[3;5H\x1b[J",
		want: []string{eRow, eRow, "EEEE"},
		check: func(t *testing.T, s *sut) {
			requireBgRow(t, s, 2, 4, defCols, text.Basic(4))
			requireBgRow(t, s, 3, 0, defCols, text.Basic(4))
			requireBgRow(t, s, 2, 0, 4, text.Color{})
		},
	},
	{
		name: "ED 1 uses the current background", in: alnFill + "\x1b[44m\x1b[3;5H\x1b[1J",
		want: []string{"", "", "     EEEEE", eRow, eRow},
		check: func(t *testing.T, s *sut) {
			requireBgRow(t, s, 0, 0, defCols, text.Basic(4))
			requireBgRow(t, s, 2, 0, 5, text.Basic(4))
			requireBgRow(t, s, 2, 5, defCols, text.Color{})
		},
	},
	{
		name: "EL 0 uses the current background", in: alnFill + "\x1b[44m\x1b[3;5H\x1b[K",
		check: func(t *testing.T, s *sut) {
			requireBgRow(t, s, 2, 4, defCols, text.Basic(4))
			requireBgRow(t, s, 2, 0, 4, text.Color{})
		},
	},
	{
		name: "EL 1 uses the current background", in: alnFill + "\x1b[44m\x1b[3;5H\x1b[1K",
		check: func(t *testing.T, s *sut) {
			requireBgRow(t, s, 2, 0, 5, text.Basic(4))
			requireBgRow(t, s, 2, 5, defCols, text.Color{})
		},
	},
	{
		name: "EL 2 uses the current background", in: alnFill + "\x1b[44m\x1b[3;5H\x1b[2K",
		check: func(t *testing.T, s *sut) { requireBgRow(t, s, 2, 0, defCols, text.Basic(4)) },
	},
	{
		name: "erase keeps only the background of a rich pen", in: alnFill + "\x1b[1;3;4;31;44m\x1b[2K",
		check: func(t *testing.T, s *sut) {
			for x := 0; x < defCols; x++ {
				requireStyle(t, s, x, 0, text.Style{Bg: text.Basic(4)})
			}
		},
	},
	{
		name: "erase with truecolor background", in: alnFill + "\x1b[48;2;10;20;30m\x1b[2J",
		check: func(t *testing.T, s *sut) { requireBgRow(t, s, 4, 0, defCols, text.RGB(10, 20, 30)) },
	},
	{
		name: "erase with the default background leaves default cells", in: alnFill + "\x1b[44m\x1b[49m\x1b[2J",
		check: func(t *testing.T, s *sut) { requireBgRow(t, s, 2, 0, defCols, text.Color{}) },
	},
}
