// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"testing"

	"github.com/wippyai/tty/text"
)

// Scrolling: DECSTBM, IND, RI, NEL, LF at the margins, SU/SD and scrollback.
// Reference: ctlseqs "DECSTBM", "IND", "RI", "NEL", "SU", "SD".

func scrollbackText(s *sut, i int) string { return cellsText(s.scr().ScrollbackLine(i)) }

var scrollingCases = []specCase{
	{name: "DECSTBM homes cursor", in: "\x1b[3;3H\x1b[2;4r", cursor: at(0, 0)},
	{name: "IND at bottom margin scrolls region", in: fill5 + "\x1b[2;4r\x1b[4;1H\x1bD",
		want: []string{"1", "3", "4", "", "5"}, cursor: at(0, 3)},
	{name: "IND inside margins moves down", in: "\x1b[2;4r\x1b[2;1H\x1bD", cursor: at(0, 2)},
	{name: "IND below bottom margin does not scroll", in: fill5 + "\x1b[2;4r\x1b[5;1H\x1bD",
		want: []string{"1", "2", "3", "4", "5"}, cursor: at(0, 4)},
	{name: "RI at top margin scrolls region down", in: fill5 + "\x1b[2;4r\x1b[2;1H\x1bM",
		want: []string{"1", "", "2", "3", "5"}, cursor: at(0, 1)},
	{name: "RI above top margin does not scroll", in: fill5 + "\x1b[2;4r\x1b[1;1H\x1bM",
		want: []string{"1", "2", "3", "4", "5"}, cursor: at(0, 0)},
	{name: "RI inside margins moves up", in: "\x1b[2;4r\x1b[3;1H\x1bM", cursor: at(0, 1)},
	{name: "RI at top of full screen scrolls down", in: fill5 + "\x1b[H\x1bM",
		want: []string{"", "1", "2", "3", "4"}, cursor: at(0, 0)},
	{name: "NEL scrolls and returns to column 0", in: fill5 + "\x1b[2;4r\x1b[4;5H\x1bE",
		want: []string{"1", "3", "4", "", "5"}, cursor: at(0, 3)},
	{name: "NEL inside margins", in: "\x1b[2;4r\x1b[2;5H\x1bE", cursor: at(0, 2)},
	{name: "LF at bottom margin scrolls region", in: fill5 + "\x1b[2;4r\x1b[4;1H\n",
		want: []string{"1", "3", "4", "", "5"}, cursor: at(0, 3)},
	{name: "LF at bottom of screen scrolls", in: fill5 + "\r\n6",
		want: []string{"2", "3", "4", "5", "6"}, cursor: at(1, 4)},
	{name: "invalid DECSTBM top below bottom is ignored", in: fill5 + "\x1b[4;2r\x1b[5;1H\n",
		want: []string{"2", "3", "4", "5", ""}},
	{name: "DECSTBM default parameters reset the region", in: fill5 + "\x1b[2;4r\x1b[r\x1b[5;1H\n",
		want: []string{"2", "3", "4", "5", ""}},
	{name: "DECSTBM bottom beyond screen is clamped", in: fill5 + "\x1b[2;99r\x1b[5;1H\n",
		want: []string{"1", "3", "4", "5", ""}},
	{name: "SU", in: fill5 + "\x1b[2S", want: []string{"3", "4", "5", "", ""}, cursor: at(1, 4)},
	{name: "SU default", in: fill5 + "\x1b[S", want: []string{"2", "3", "4", "5", ""}},
	{name: "SU inside region", in: fill5 + "\x1b[2;4r\x1b[S", want: []string{"1", "3", "4", "", "5"}},
	{name: "SD", in: fill5 + "\x1b[2T", want: []string{"", "", "1", "2", "3"}, cursor: at(1, 4)},
	{name: "SD inside region", in: fill5 + "\x1b[2;4r\x1b[T", want: []string{"1", "", "2", "3", "5"}},
	{name: "SU count beyond region clears it", in: fill5 + "\x1b[2;4r\x1b[99S", want: []string{"1", "", "", "", "5"}},

	{
		name: "full-screen scroll fills scrollback in order", rows: 3,
		in:   "1\r\n2\r\n3\r\n4\r\n5\r\n6\r\n7\r\n8",
		want: []string{"6", "7", "8"}, cursor: at(1, 2),
		check: func(t *testing.T, s *sut) {
			if n := s.scr().ScrollbackLen(); n != 5 {
				t.Fatalf("scrollback len want 5 got %d", n)
			}
			for i, w := range []string{"1", "2", "3", "4", "5"} {
				if g := scrollbackText(s, i); g != w {
					t.Errorf("scrollback[%d] want %q got %q", i, w, g)
				}
			}
		},
	},
	{
		name: "partial region scroll with top margin below row 0 keeps no scrollback",
		in:   fill5 + "\x1b[2;4r\x1b[4;1H\n\n\n",
		check: func(t *testing.T, s *sut) {
			if n := s.scr().ScrollbackLen(); n != 0 {
				t.Errorf("scrollback len want 0 got %d", n)
			}
		},
	},
	{
		name: "scrolling inside the alternate screen keeps no scrollback", rows: 3,
		in: "\x1b[?1049h1\r\n2\r\n3\r\n4\r\n5",
		check: func(t *testing.T, s *sut) {
			if n := s.scr().ScrollbackLen(); n != 0 {
				t.Errorf("alt scrollback len want 0 got %d", n)
			}
		},
	},
	{
		name: "scroll exposes lines in the current background", in: fill5 + "\x1b[44m\n",
		want: []string{"2", "3", "4", "5", ""},
		check: func(t *testing.T, s *sut) {
			requireBgRow(t, s, 4, 0, defCols, text.Basic(4))
			requireBgRow(t, s, 0, 0, defCols, text.Color{})
		},
	},
	{
		name: "RI exposes lines in the current background", in: fill5 + "\x1b[44m\x1b[H\x1bM",
		want:  []string{"", "1", "2", "3", "4"},
		check: func(t *testing.T, s *sut) { requireBgRow(t, s, 0, 0, defCols, text.Basic(4)) },
	},
	{
		name: "ED 2 keeps scrollback and ED 3 clears it", rows: 3,
		in: "1\r\n2\r\n3\r\n4\r\n5\x1b[2J",
		check: func(t *testing.T, s *sut) {
			if n := s.scr().ScrollbackLen(); n != 2 {
				t.Fatalf("scrollback after ED 2 want 2 got %d", n)
			}
			s.Write("\x1b[3J")
			if n := s.scr().ScrollbackLen(); n != 0 {
				t.Errorf("scrollback after ED 3 want 0 got %d", n)
			}
		},
	},
}
