// SPDX-License-Identifier: MPL-2.0

package screen

import (
	"fmt"
	"strings"
	"testing"
)

// logical joins every soft-wrapped row of scrollback and screen into logical
// lines, trailing blanks trimmed.
func logical(s Screen) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		out = append(out, strings.TrimRight(cur.String(), " "))
		cur.Reset()
	}
	add := func(cells []Cell, wrapped bool) {
		for i, c := range cells {
			if c.Cluster == "" {
				if i > 0 && cells[i-1].Wide {
					continue
				}
				cur.WriteByte(' ')
				continue
			}
			cur.WriteString(c.Cluster)
		}
		if !wrapped {
			flush()
		}
	}
	for i := 0; i < s.ScrollbackLen(); i++ {
		add(s.ScrollbackLine(i), s.ScrollbackLineWrapped(i))
	}
	_, rows := s.Size()
	for y := 0; y < rows; y++ {
		add(s.Line(y), s.LineWrapped(y))
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

func TestReflowNarrower(t *testing.T) {
	s := New(10, 4, 20)
	put(s, "hello world")
	eqCursor(t, s, 1, 1)
	s.Resize(5, 4)
	eqLines(t, screenText(s), "hello", " worl", "d", "")
	if !s.LineWrapped(0) || !s.LineWrapped(1) || s.LineWrapped(2) {
		t.Fatal("wrap flags after reflow")
	}
	eqCursor(t, s, 1, 2)
}

func TestReflowWiderJoins(t *testing.T) {
	s := New(5, 4, 20)
	put(s, "hello world")
	eqCursor(t, s, 1, 2)
	s.Resize(11, 4)
	eqLines(t, screenText(s), "hello world", "", "", "")
	eqCursor(t, s, 11-1, 0)
	eqPending(t, s, true)
	if s.LineWrapped(0) {
		t.Fatal("still wrapped")
	}
	s.Resize(20, 4)
	eqLines(t, screenText(s), "hello world", "", "", "")
}

func TestReflowKeepsHardBreaks(t *testing.T) {
	s := New(6, 5, 20)
	put(s, "abcdefgh\nij")
	s.Resize(3, 5)
	eqLines(t, logical(s), "abcdefgh", "ij")
	s.Resize(12, 5)
	eqLines(t, screenText(s), "abcdefgh", "ij", "", "", "")
}

func TestReflowCursorFollowsCharacter(t *testing.T) {
	tests := []struct {
		name         string
		cols         int
		x, y         int // cursor placed before resize
		newCols      int
		wantX, wantY int
	}{
		{"on a char narrower", 10, 7, 0, 4, 3, 1},
		{"at start", 10, 0, 0, 4, 0, 0},
		{"second row of wrapped", 5, 2, 1, 10, 7, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(tc.cols, 6, 20)
			put(s, "0123456789")
			s.MoveTo(tc.x, tc.y)
			want := ""
			if c := s.Line(tc.y)[tc.x]; c.Cluster != "" {
				want = c.Cluster
			}
			s.Resize(tc.newCols, 6)
			c := s.Cursor()
			if want != "" {
				if got := s.Line(c.Y)[c.X].Cluster; got != want {
					t.Fatalf("cursor on %q, was on %q", got, want)
				}
			}
		})
	}
}

func TestReflowCursorOnEmptyRows(t *testing.T) {
	s := New(10, 4, 20)
	put(s, "$ ")
	s.Resize(4, 4)
	eqCursor(t, s, 2, 0)
	s.Resize(10, 4)
	eqCursor(t, s, 2, 0)

	s = New(10, 4, 20)
	s.MoveTo(9, 2)
	s.Resize(4, 4)
	eqCursor(t, s, 3, 2)
}

func TestReflowPendingWrapCursor(t *testing.T) {
	s := New(5, 3, 20)
	put(s, "abcde")
	eqPending(t, s, true)
	s.Resize(8, 3)
	eqCursor(t, s, 5, 0)
	eqPending(t, s, false)
	put(s, "fgh")
	eqLines(t, screenText(s), "abcdefgh", "", "")
	s.Resize(4, 3)
	eqLines(t, screenText(s), "abcd", "efgh", "")
	eqCursor(t, s, 3, 1)
	eqPending(t, s, true)
}

func TestReflowWideCharacters(t *testing.T) {
	t.Run("narrower pads at the margin", func(t *testing.T) {
		s := New(8, 4, 20)
		put(s, "ab世cd世")
		s.Resize(3, 4)
		eqLines(t, screenText(s), "ab", "世c", "d世", "")
		for y := 0; y < 3; y++ {
			assertWellFormed(t, s.Line(y))
		}
		if !s.LineWrapped(0) || !s.LineWrapped(1) {
			t.Fatal("wrap flags")
		}
	})
	t.Run("round trip keeps wide chars joined", func(t *testing.T) {
		s := New(9, 4, 20)
		put(s, "ab世cd世e")
		s.Resize(3, 4)
		s.Resize(9, 4)
		eqLines(t, screenText(s), "ab世cd世e", "", "", "")
		assertWellFormed(t, s.Line(0))
	})
	t.Run("cursor after wide char", func(t *testing.T) {
		s := New(8, 4, 20)
		put(s, "ab世")
		s.Resize(3, 4)
		eqLines(t, screenText(s), "ab", "世", "", "")
		eqCursor(t, s, 2, 1)
		eqPending(t, s, false)
	})
	t.Run("one column drops wide characters", func(t *testing.T) {
		s := New(4, 4, 20)
		put(s, "a世b")
		s.Resize(1, 4)
		for y := 0; y < 4; y++ {
			assertWellFormed(t, s.Line(y))
		}
		eqLines(t, logical(s), "a b")
	})
}

func TestReflowScrollback(t *testing.T) {
	s := New(6, 3, 50)
	for i := 0; i < 6; i++ {
		put(s, fmt.Sprintf("line-%d\n", i))
	}
	before := logical(s)
	s.Resize(3, 3)
	eqLines(t, logical(s), before...)
	if s.ScrollbackLen() == 0 {
		t.Fatal("expected scrollback")
	}
	for i := 0; i < s.ScrollbackLen(); i++ {
		if n := len(s.ScrollbackLine(i)); n != 3 {
			t.Fatalf("scrollback line %d has %d cells", i, n)
		}
	}
	s.Resize(12, 3)
	eqLines(t, logical(s), before...)
}

func TestReflowRespectsScrollbackLimit(t *testing.T) {
	s := New(10, 2, 3)
	for i := 0; i < 6; i++ {
		put(s, fmt.Sprintf("%d\n", i))
	}
	s.Resize(5, 2)
	if s.ScrollbackLen() > 3 {
		t.Fatalf("scrollback %d exceeds limit", s.ScrollbackLen())
	}
	s.Resize(1, 2)
	if s.ScrollbackLen() > 3 {
		t.Fatalf("scrollback %d exceeds limit", s.ScrollbackLen())
	}
}

func TestResizeRowsOnly(t *testing.T) {
	t.Run("shrink drops blank rows below cursor", func(t *testing.T) {
		s := New(3, 6, 10)
		put(s, "a\nb")
		s.Resize(3, 3)
		eqLines(t, screenText(s), "a", "b", "")
		if s.ScrollbackLen() != 0 {
			t.Fatal("scrollback fed")
		}
		eqCursor(t, s, 1, 1)
	})
	t.Run("shrink pushes top rows to keep cursor", func(t *testing.T) {
		s := New(3, 4, 10)
		put(s, "a\nb\nc\nd")
		s.Resize(3, 2)
		eqLines(t, screenText(s), "c", "d")
		eqLines(t, sbText(s), "a", "b")
		eqCursor(t, s, 1, 1)
	})
	t.Run("grow pulls from scrollback", func(t *testing.T) {
		s := New(3, 2, 10)
		put(s, "a\nb\nc\nd")
		eqLines(t, sbText(s), "a", "b")
		s.Resize(3, 4)
		eqLines(t, screenText(s), "a", "b", "c", "d")
		if s.ScrollbackLen() != 0 {
			t.Fatal("scrollback not consumed")
		}
		eqCursor(t, s, 1, 3)
	})
	t.Run("grow without scrollback adds blank rows", func(t *testing.T) {
		s := New(3, 2, 0)
		put(s, "a\nb")
		s.Resize(3, 4)
		eqLines(t, screenText(s), "a", "b", "", "")
		eqCursor(t, s, 1, 1)
	})
	t.Run("grow pulling partially", func(t *testing.T) {
		s := New(3, 2, 10)
		put(s, "a\nb\nc")
		s.Resize(3, 5)
		eqLines(t, screenText(s), "a", "b", "c", "", "")
	})
	t.Run("keeps styled blank rows", func(t *testing.T) {
		s := New(3, 2, 10)
		setPen(s, redBg())
		s.EraseLine(EraseAll)
		s.Resize(3, 3)
		if s.Line(0)[0].Style.Bg != redBg().Bg {
			t.Fatal("styled blank lost")
		}
	})
	t.Run("size and region reset", func(t *testing.T) {
		s := New(5, 6, 0)
		s.SetScrollRegion(1, 3)
		s.Resize(5, 4)
		if top, bot := s.ScrollRegion(); top != 0 || bot != 3 {
			t.Fatalf("region %d,%d", top, bot)
		}
		if c, r := s.Size(); c != 5 || r != 4 {
			t.Fatal("size")
		}
	})
	t.Run("same size is a no-op", func(t *testing.T) {
		s := New(5, 4, 0)
		put(s, "ab")
		s.Resize(5, 4)
		eqLines(t, screenText(s), "ab", "", "", "")
	})
}

func TestResizeSavedCursorFollows(t *testing.T) {
	s := New(10, 4, 20)
	put(s, "0123456789ab")
	s.MoveTo(1, 1)
	s.SaveCursor()
	s.Resize(5, 4)
	s.RestoreCursor()
	c := s.Cursor()
	if got := s.Line(c.Y)[c.X].Cluster; got != "b" {
		t.Fatalf("saved cursor on %q", got)
	}
}

func TestResizeWhileAlternate(t *testing.T) {
	s := New(8, 4, 20)
	put(s, "hello world")
	s.MoveTo(2, 1)
	s.SaveCursor()
	s.UseAlternate(true, true)
	s.MoveTo(0, 0)
	put(s, "ALTERNATE")
	s.Resize(5, 3)
	// alternate clips and extends
	if c, r := s.Size(); c != 5 || r != 3 {
		t.Fatal("size")
	}
	eqLines(t, screenText(s), "ALTER", "E", "")
	if s.LineWrapped(0) {
		t.Fatal("alternate buffer must not track soft wrap across resize")
	}
	s.Resize(9, 4)
	eqLines(t, screenText(s), "ALTER", "E", "", "")
	s.UseAlternate(false, false)
	s.RestoreCursor()
	c := s.Cursor()
	if got := s.Line(c.Y)[c.X].Cluster; got != "d" {
		t.Fatalf("restored cursor on %q", got)
	}
	eqLines(t, logical(s), "hello world")
}

func TestResizeAlternateWide(t *testing.T) {
	s := New(6, 2, 0)
	s.UseAlternate(true, true)
	put(s, "ab世")
	s.Resize(3, 2)
	assertWellFormed(t, s.Line(0))
	eqLines(t, screenText(s)[:1], "ab")
}

func TestResizeClampsCursor(t *testing.T) {
	s := New(10, 6, 0)
	s.UseAlternate(true, true)
	s.MoveTo(9, 5)
	s.Resize(4, 3)
	eqCursor(t, s, 3, 2)
	eqPending(t, s, false)
}

func TestResizeTabStops(t *testing.T) {
	s := New(10, 2, 0)
	s.MoveTo(3, 0)
	s.SetTabStop()
	s.Resize(30, 2)
	s.MoveTo(0, 0)
	s.Tab(1)
	eqCursor(t, s, 3, 0)
	s.Tab(1)
	eqCursor(t, s, 8, 0)
	s.Tab(1)
	eqCursor(t, s, 16, 0)
	s.Tab(1)
	eqCursor(t, s, 24, 0)
	s.Resize(5, 2)
	s.MoveTo(0, 0)
	s.Tab(1)
	eqCursor(t, s, 3, 0)
}

func TestScrollbackLimitChangeDuringOutput(t *testing.T) {
	s := New(4, 2, 10)
	f := &feeder{}
	f.feed(s, 6)
	s.SetScrollbackLimit(2)
	f.feed(s, 3)
	s.SetScrollbackLimit(5)
	f.feed(s, 6)
	s.SetScrollbackLimit(1)
	f.feed(s, 1)
	if s.ScrollbackLen() != 1 {
		t.Fatalf("len %d", s.ScrollbackLen())
	}
	if got := cellsText(s.ScrollbackLine(0)); got != "14" {
		t.Fatalf("newest scrollback line %q", got)
	}
	s.Resize(2, 2)
	if s.ScrollbackLen() > 1 {
		t.Fatal("limit exceeded after reflow")
	}
}
