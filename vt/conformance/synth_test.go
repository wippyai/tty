// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wippyai/tty/text"
)

// Synthetic corpora mimic the output of agent CLIs: an alternate-screen TUI
// redrawn under synchronized output (mode 2026) after kitty keyboard
// negotiation, an inline live region redrawn with erase-and-move-up, a bottom
// viewport fed by scroll-region history insertion, and truecolor and
// hyperlink heavy output. Each generator returns the bytes and checkpoints
// whose plain-text rows follow from the generator's own model, so the
// expectation is independent of any emulator.

type checkpoint struct {
	offset int      // bytes of the stream fed before the check
	rows   []string // expected visible rows (plain text), padded with blanks
	alt    bool     // expected alternate-screen state
	check  func(t *testing.T, s *sut)
}

type synthStream struct {
	file        string
	data        []byte
	checkpoints []checkpoint
	replies     string // regexp for the complete reply stream, empty when none expected
}

func sgr(params string, s string) string { return "\x1b[" + params + "m" + s + "\x1b[0m" }

func link(url, label string) string { return "\x1b]8;;" + url + "\x1b\\" + label + "\x1b]8;;\x1b\\" }

func padRight(s string, n int) string {
	if w := len([]rune(s)); w < n {
		return s + strings.Repeat(" ", n-w)
	}
	return s
}

func synthClaudeAltTUI() synthStream {
	const frames = 40
	var b bytes.Buffer
	b.WriteString("$ claude\r\n")
	b.WriteString("\x1b[?u\x1b[?2004h\x1b[?1004h\x1b[?25l\x1b[c\x1b[?2026$p")
	b.WriteString("\x1b[?1049h\x1b[>1u\x1b[2J\x1b[H")
	var rows []string
	for f := 0; f < frames; f++ {
		rows = rows[:0]
		b.WriteString("\x1b[?2026h")
		put := func(row int, styled, plain string) {
			fmt.Fprintf(&b, "\x1b[%d;1H%s", row, styled)
			if len([]rune(plain)) < 80 {
				b.WriteString("\x1b[K")
			}
			for len(rows) < row-1 {
				rows = append(rows, "")
			}
			rows = append(rows, plain)
		}
		title := " Claude Code  v2.0.0  ~/project"
		put(1, "\x1b[48;2;30;30;46m\x1b[38;2;205;214;244m"+padRight(title, 80)+"\x1b[0m", padRight(title, 80))
		n := min(f+1, 18)
		for i := 0; i < n; i++ {
			file := fmt.Sprintf("src/mod%02d.go", i)
			lines := 100 + i*7
			plain := fmt.Sprintf(" ● step %02d: reading %s (%d lines) 你好", i, file, lines)
			styled := fmt.Sprintf(" %s step %02d: reading %s (%d lines) %s", sgr("38;2;166;227;161", "●"), i, link("file:///project/"+file, file), lines, sgr("1;38;5;213", "你好"))
			put(2+i, styled, plain)
		}
		sep := strings.Repeat("─", 80)
		put(22, sgr("2", sep), sep)
		spin := string([]rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")[f%10])
		think := fmt.Sprintf(" %s Thinking… (%ds · esc to interrupt)", spin, f)
		put(23, "\x1b[38;2;203;166;247m"+think+"\x1b[0m", think)
		status := " ? for shortcuts"
		put(24, "\x1b[7m"+padRight(status, 80)+"\x1b[0m", padRight(status, 80))
		b.WriteString("\x1b[?2026l")
	}
	altRows := append([]string(nil), rows...)
	altEnd := b.Len()
	b.WriteString("\x1b[<u\x1b[?25h\x1b[?1049l\x1b[?2004l\x1b[?1004l$ ")
	return synthStream{
		file:    "synth-claude-alt-tui.vt",
		data:    b.Bytes(),
		replies: `^\x1b\[\?0u\x1b\[\?\d+(;\d+)*c\x1b\[\?2026;2\$y$`,
		checkpoints: []checkpoint{
			{offset: altEnd, rows: altRows, alt: true, check: func(t *testing.T, s *sut) {
				requireStyle(t, s, 0, 0, text.Style{Fg: text.RGB(205, 214, 244), Bg: text.RGB(30, 30, 46)})
				// "step 00: reading " ends before the link on the first transcript row.
				x := strings.Index(altRows[1], "src/mod00.go")
				if x < 0 {
					t.Fatal("model row has no link text")
				}
				if l := s.cell(len([]rune(altRows[1][:x])), 1).Link; !strings.Contains(l, "file:///project/src/mod00.go") {
					t.Errorf("link cell has Link %q", l)
				}
				if m := s.term.Modes(); !m.BracketedPaste || !m.FocusEvents || m.KittyKeyboardFlags != 1 {
					t.Errorf("modes %+v", m)
				}
				if s.scr().Cursor().Visible {
					t.Error("cursor should be hidden")
				}
			}},
			{offset: len(b.Bytes()), rows: []string{"$ claude", "$"}, alt: false, check: func(t *testing.T, s *sut) {
				if m := s.term.Modes(); m.BracketedPaste || m.FocusEvents || m.KittyKeyboardFlags != 0 {
					t.Errorf("modes after exit %+v", m)
				}
				if !s.scr().Cursor().Visible {
					t.Error("cursor should be visible after exit")
				}
				if x, y := s.Cursor(); x != 2 || y != 1 {
					t.Errorf("cursor (%d,%d) want (2,1)", x, y)
				}
			}},
		},
	}
}

func synthClaudeInline() synthStream {
	var b bytes.Buffer
	var history []string
	b.WriteString("$ claude\r\n")
	history = append(history, "$ claude")
	prev := 0
	var region []string
	draw := func(frame int) {
		b.WriteString("\x1b[?25l")
		for i := 0; i < prev-1; i++ {
			b.WriteString("\x1b[2K\x1b[1A")
		}
		if prev > 0 {
			b.WriteString("\x1b[2K\x1b[G")
		}
		spin := string([]rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")[frame%10])
		region = []string{
			fmt.Sprintf("%s Working on the refactor… (%ds)", spin, frame),
			"",
			"╭" + strings.Repeat("─", 40) + "╮",
			"│ > " + padRight(fmt.Sprintf("draft message %d", frame), 37) + "│",
			"╰" + strings.Repeat("─", 40) + "╯",
			"  ? for shortcuts · 中文",
		}
		styled := []string{
			sgr("38;5;215", spin) + fmt.Sprintf(" Working on the refactor… (%ds)", frame),
			"",
			sgr("2", region[2]), "│ > " + sgr("1", padRight(fmt.Sprintf("draft message %d", frame), 37)) + "│", sgr("2", region[4]),
			"  " + link("https://example.com/help", "? for shortcuts") + " · 中文",
		}
		b.WriteString(strings.Join(styled, "\r\n"))
		prev = len(region)
		b.WriteString("\x1b[?25h")
	}
	for f := 0; f < 30; f++ {
		if f > 0 && f%5 == 0 {
			// Commit a finished message above the live region.
			for i := 0; i < prev-1; i++ {
				b.WriteString("\x1b[2K\x1b[1A")
			}
			b.WriteString("\x1b[2K\x1b[G")
			line := fmt.Sprintf("● Edited file %d: updated 12 lines", f/5)
			b.WriteString(sgr("32", "●") + fmt.Sprintf(" Edited file %d: updated 12 lines\r\n", f/5))
			history = append(history, line)
			prev = 0
		}
		draw(f)
	}
	want := append(append([]string{}, history...), region...)
	return synthStream{
		file: "synth-claude-inline.vt", data: b.Bytes(),
		checkpoints: []checkpoint{{offset: b.Len(), rows: want}},
	}
}

func synthCodexViewport() synthStream {
	const histRows = 20
	var b bytes.Buffer
	var hist []string
	viewport := func(n int) []string {
		return []string{
			"╭" + strings.Repeat("─", 78) + "╮",
			"│ " + padRight(fmt.Sprintf("▌ ask codex anything (%d messages)", n), 76) + " │",
			"╰" + strings.Repeat("─", 78) + "╯",
			padRight("  ⏎ send   ⌃C quit", 80),
		}
	}
	var vp []string
	b.WriteString("\x1b[?2004h\x1b[?u\x1b[>7u")
	for i := 1; i <= 50; i++ {
		var plain, styled string
		switch i % 4 {
		case 0:
			plain = fmt.Sprintf("+ added line %d of the patch", i)
			styled = sgr("32", plain)
		case 1:
			plain = fmt.Sprintf("- removed line %d of the patch", i)
			styled = sgr("31", plain)
		case 2:
			plain = fmt.Sprintf("  context line %d with some trailing words to read", i)
			styled = sgr("2", plain)
		default:
			plain = fmt.Sprintf("• ran `go test ./pkg%d` ok", i)
			styled = sgr("1", "•") + fmt.Sprintf(" ran `go test ./pkg%d` ok", i)
		}
		hist = append(hist, plain)
		b.WriteString("\x1b[?2026h")
		fmt.Fprintf(&b, "\x1b[1;%dr\x1b[%d;1H\r\n%s\x1b[r", histRows, histRows, styled)
		vp = viewport(i)
		styledVP := []string{sgr("2", vp[0]), "│ " + sgr("1", "▌") + padRight(fmt.Sprintf(" ask codex anything (%d messages)", i), 75) + " │", sgr("2", vp[2]), sgr("2", vp[3])}
		for r, line := range styledVP {
			fmt.Fprintf(&b, "\x1b[%d;1H\x1b[2K%s", histRows+1+r, line)
		}
		fmt.Fprintf(&b, "\x1b[%d;5H", histRows+2)
		b.WriteString("\x1b[?2026l")
	}
	want := append([]string{}, hist[len(hist)-histRows:]...)
	want = append(want, vp...)
	return synthStream{
		file: "synth-codex-viewport.vt", data: b.Bytes(),
		checkpoints: []checkpoint{{offset: b.Len(), rows: want}},
	}
}

func synthTruecolor() synthStream {
	var b bytes.Buffer
	for y := 0; y < corpusRows; y++ {
		for x := 0; x < corpusCols; x++ {
			fmt.Fprintf(&b, "\x1b[48;2;%d;%d;%dm ", x*255/(corpusCols-1), y*255/(corpusRows-1), 128)
		}
		b.WriteString("\x1b[0m")
		if y < corpusRows-1 {
			b.WriteString("\r\n")
		}
	}
	b.WriteString("\x1b[1;1H\x1b[4:3m\x1b[58:2::255:0:0mwavy\x1b[0m \x1b[9mgone\x1b[0m \x1b[38:2::1:2:3;1mbold\x1b[0m")
	return synthStream{
		file: "synth-truecolor.vt", data: b.Bytes(),
		checkpoints: []checkpoint{{offset: b.Len(), rows: []string{"wavy gone bold"}, check: func(t *testing.T, s *sut) {
			requireStyle(t, s, 0, 0, text.Style{Underline: text.UnderlineCurly, UnderlineColor: text.RGB(255, 0, 0)})
			requireStyle(t, s, 5, 0, text.Style{Attrs: text.AttrStrikethrough})
			requireStyle(t, s, 10, 0, text.Style{Fg: text.RGB(1, 2, 3), Attrs: text.AttrBold})
			for _, p := range [][2]int{{79, 23}, {40, 12}, {0, 5}, {79, 0}} {
				x, y := p[0], p[1]
				if x < 14 && y == 0 {
					continue
				}
				want := text.RGB(uint8(x*255/(corpusCols-1)), uint8(y*255/(corpusRows-1)), 128)
				if got := s.cell(x, y).Style.Bg; got != want {
					t.Errorf("cell (%d,%d) bg want %+v got %+v", x, y, want, got)
				}
			}
		}}},
	}
}

func synthKeyboardNegotiation() synthStream {
	in := "\x1b[?u\x1b[>31u\x1b[?u\x1b[=1;3u\x1b[?u\x1b[<u\x1b[?u\x1b[>c\x1b[?2004$p\x1b[?1004h\x1b[?1004$p\x1b[?2026$p\x1b[5n\x1b[6n"
	return synthStream{
		file: "synth-keyboard-negotiation.vt", data: []byte(in),
		replies: `^\x1b\[\?0u\x1b\[\?31u\x1b\[\?30u\x1b\[\?0u\x1b\[>\d+;\d+;\d+c\x1b\[\?2004;2\$y\x1b\[\?1004;1\$y\x1b\[\?2026;2\$y\x1b\[0n\x1b\[1;1R$`,
	}
}

func synthStressScroll() synthStream {
	var b bytes.Buffer
	var rows []string
	const lines = 2200
	for i := 0; i < lines; i++ {
		body := fmt.Sprintf("%04d %s", i, strings.Repeat(string(rune('a'+i%26)), 20+(i*7)%90))
		colour := 31 + i%7
		fmt.Fprintf(&b, "\x1b[%dm%s\x1b[0m", colour, body)
		for len(body) > corpusCols {
			rows = append(rows, body[:corpusCols])
			body = body[corpusCols:]
		}
		rows = append(rows, body)
		if i < lines-1 {
			b.WriteString("\r\n")
		}
	}
	want := rows[len(rows)-corpusRows:]
	return synthStream{
		file: "synth-stress-scroll.vt", data: b.Bytes(),
		checkpoints: []checkpoint{{offset: b.Len(), rows: want, check: func(t *testing.T, s *sut) {
			if n := s.scr().ScrollbackLen(); n != scrollbackLines {
				t.Errorf("scrollback len want %d got %d", scrollbackLines, n)
			}
			wantTop := rows[len(rows)-corpusRows-scrollbackLines]
			if got := scrollbackText(s, 0); got != wantTop {
				t.Errorf("oldest scrollback line want %q got %q", wantTop, got)
			}
		}}},
	}
}

func synthStreams() []synthStream {
	return []synthStream{
		synthClaudeAltTUI(), synthClaudeInline(), synthCodexViewport(), synthTruecolor(),
		synthKeyboardNegotiation(), synthStressScroll(),
	}
}

func writeSynthetic(t *testing.T) {
	t.Helper()
	for _, st := range synthStreams() {
		if len(st.data) > corpusMaxSize {
			t.Fatalf("%s is %d bytes, limit %d", st.file, len(st.data), corpusMaxSize)
		}
		if err := os.WriteFile(filepath.Join("testdata", st.file), st.data, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %d bytes", st.file, len(st.data))
	}
}

func TestWriteSyntheticCorpora(t *testing.T) {
	if !*updateCorpora {
		t.Skip("run with -update-corpora to regenerate")
	}
	writeSynthetic(t)
}

// TestSyntheticCorporaAreCurrent keeps the committed files identical to what
// the generators produce.
func TestSyntheticCorporaAreCurrent(t *testing.T) {
	for _, st := range synthStreams() {
		got, err := os.ReadFile(filepath.Join("testdata", st.file))
		if err != nil {
			t.Errorf("%s: %v (run with -update-corpora)", st.file, err)
			continue
		}
		if !bytes.Equal(got, st.data) {
			t.Errorf("%s differs from its generator (run with -update-corpora)", st.file)
		}
	}
}

// TestSyntheticFinalState replays each synthetic stream and checks the grid
// at every checkpoint against the generator's model.
func TestSyntheticFinalState(t *testing.T) {
	for _, st := range synthStreams() {
		t.Run(st.file, func(t *testing.T) {
			s := newSUT(corpusCols, corpusRows)
			fed := 0
			for _, cp := range st.checkpoints {
				s.Write(string(st.data[fed:cp.offset]))
				fed = cp.offset
				want := make([]string, corpusRows)
				copy(want, cp.rows)
				got := gridOf(s, corpusRows)
				for y := range want {
					want[y] = strings.TrimRight(want[y], " ")
				}
				if joinGrid(want) != joinGrid(got) {
					t.Errorf("checkpoint at %d:\n%s", cp.offset, diffGrid(want, got))
				}
				if s.scr().Alternate() != cp.alt {
					t.Errorf("checkpoint at %d: Alternate want %v", cp.offset, cp.alt)
				}
				if cp.check != nil {
					cp.check(t, s)
				}
			}
			s.Write(string(st.data[fed:]))
			if st.replies != "" {
				if r := s.Replies(); !regexpMatch(st.replies, r) {
					t.Errorf("replies %q do not match /%s/", r, st.replies)
				}
			}
		})
	}
}
