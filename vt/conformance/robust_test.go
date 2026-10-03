// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/wippyai/tty/vt/screen"
)

// padCells renders a row keeping trailing blanks.
func padCells(cells []screen.Cell) string {
	var b strings.Builder
	prevWide := false
	for _, c := range cells {
		switch {
		case prevWide && c.Cluster == "":
		case c.Cluster == "":
			b.WriteByte(' ')
		default:
			b.WriteString(c.Cluster)
		}
		prevWide = c.Wide
	}
	return b.String()
}

// logicalLines joins soft-wrapped visible rows into logical lines.
func logicalLines(s *sut) []string {
	_, rows := s.Size()
	var out []string
	var cur strings.Builder
	for y := 0; y < rows; y++ {
		row := padCells(s.scr().Line(y))
		if !s.scr().LineWrapped(y) {
			cur.WriteString(strings.TrimRight(row, " "))
			out = append(out, cur.String())
			cur.Reset()
		} else {
			cur.WriteString(row)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

// TestResizeReflowKeepsLogicalLines requires that resizing the primary buffer
// only changes where soft-wrapped lines break, never their text.
func TestResizeReflowKeepsLogicalLines(t *testing.T) {
	var in strings.Builder
	for i := 0; i < 5; i++ {
		in.WriteString(strings.Repeat(fmt.Sprintf("w%d ", i), 15+i*4) + "\r\n")
	}
	in.WriteString("short\r\n\r\nlast line")
	s := newSUT(40, 300)
	s.Write(in.String())
	want := logicalLines(s)
	if len(want) != 8 {
		t.Fatalf("setup produced %d logical lines: %q", len(want), want)
	}
	for _, cols := range []int{23, 11, 7, 30, 80, 40} {
		s.Resize(cols, 300)
		got := logicalLines(s)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("after resize to %d columns logical lines differ\nwant %q\ngot  %q", cols, want, got)
		}
	}
}

// TestResizeKeepsCursorOnItsText requires the cursor to stay inside the grid
// and after the text it followed when the screen is resized.
func TestResizeKeepsCursorInGrid(t *testing.T) {
	s := newSUT(20, 10)
	s.Write("hello\r\nworld")
	for _, sz := range [][2]int{{5, 3}, {1, 1}, {30, 12}, {2, 20}, {80, 24}} {
		s.Resize(sz[0], sz[1])
		x, y := s.Cursor()
		if x < 0 || x >= sz[0] || y < 0 || y >= sz[1] {
			t.Errorf("after resize to %dx%d cursor (%d,%d) is outside the grid", sz[0], sz[1], x, y)
		}
	}
}

// TestRandomBytesDoNotPanic feeds seeded random and mutated escape-heavy
// streams through the emulator at several sizes, resizing in between.
func TestRandomBytesDoNotPanic(t *testing.T) {
	frags := []string{
		"\x1b[", "\x1b]", "\x1bP", "\x1b_", "\x1b^", "\x1bX", "\x1b\\", "\x07", "\x9c", "\x9b", ";", ":", "?", "$p", "99999999999", "-1",
		"m", "H", "J", "K", "L", "M", "@", "P", "X", "r", "s", "u", "h", "l", "t", "c", "n", "q", "b", "I", "Z", "S", "T",
		"38;2;", "48;5;", "58:2::", "0;", "1;", "2;", "\x1b(0", "\x1b)B", "\x0e", "\x0f", "\x1b#8", "\x1bc", "\x1b[!p",
		"\x1b[?1049h", "\x1b[?1049l", "\x1b[?47h", "\x1b[?2026h", "\x1b[>1u", "\x1b[<u", "\x1b[?u", "中", "é", "\U0001F468‍\U0001F469", "\xff", "\xe4\xb8",
		"\x1b]8;;http://x\x07", "\x1b]52;c;AAAA\x07", "\x1b]0;t\x07", "\x1b_Ga=T,f=24,s=1,v=1;AAAA\x1b\\", "\x1bP$qm\x1b\\",
	}
	sizes := [][2]int{{1, 1}, {2, 2}, {3, 5}, {17, 4}, {80, 24}}
	for seed := int64(1); seed <= 40; seed++ {
		r := rand.New(rand.NewSource(seed))
		sz := sizes[int(seed)%len(sizes)]
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			s := newSUT(sz[0], sz[1])
			requireNoPanic(t, "random stream", func() {
				for i := 0; i < 400; i++ {
					switch r.Intn(12) {
					case 0:
						s.Resize(1+r.Intn(40), 1+r.Intn(15))
					case 1:
						b := make([]byte, 1+r.Intn(20))
						r.Read(b)
						s.Write(string(b))
					default:
						s.Write(frags[r.Intn(len(frags))])
					}
				}
			})
			cols, rows := s.Size()
			x, y := s.Cursor()
			if x < 0 || x >= cols || y < 0 || y >= rows {
				t.Errorf("cursor (%d,%d) outside %dx%d", x, y, cols, rows)
			}
			for yy := 0; yy < rows; yy++ {
				if n := len(s.scr().Line(yy)); n != cols {
					t.Fatalf("row %d has %d cells want %d", yy, n, cols)
				}
			}
		})
	}
}
