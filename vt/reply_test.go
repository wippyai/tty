// SPDX-License-Identifier: MPL-2.0

package vt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wippyai/tty/text"
)

func TestReplies(t *testing.T) {
	cases := []struct{ in, want string }{
		{"\x1b[c", "\x1b[?62;22c"},
		{"\x1b[0c", "\x1b[?62;22c"},
		{"\x1b[>c", "\x1b[>1;10;0c"},
		{"\x1b[5n", "\x1b[0n"},
		{"\x1b[3;7H\x1b[6n", "\x1b[3;7R"},
		{"\x1b[3;7H\x1b[?6n", "\x1b[?3;7R"},
		{"\x1b[>q", "\x1bP>|wippy-tty(0.1)\x1b\\"},
		{"\x1b[18t", "\x1b[8;10;40t"},
		{"\x1b[19t", "\x1b[9;10;40t"},
		{"\x1b[14t", "\x1b[4;160;320t"},
		{"\x1b[16t", "\x1b[6;16;8t"},
		{"\x1b[?25$p", "\x1b[?25;1$y"},
		{"\x1b[?25l\x1b[?25$p", "\x1b[?25;2$y"},
		{"\x1b[?2004$p", "\x1b[?2004;2$y"},
		{"\x1b[?2004h\x1b[?2004$p", "\x1b[?2004;1$y"},
		{"\x1b[?7$p", "\x1b[?7;1$y"},
		{"\x1b[?9999$p", "\x1b[?9999;0$y"},
		{"\x1b[?1049h\x1b[?1049$p", "\x1b[?1049;1$y"},
		{"\x1b[?1006h\x1b[?1006$p", "\x1b[?1006;1$y"},
		{"\x1b[?1000h\x1b[?1000$p", "\x1b[?1000;1$y"},
		{"\x1b[4$p", "\x1b[4;2$y"},
		{"\x1b[4h\x1b[4$p", "\x1b[4;1$y"},
		{"\x1b[20$p", "\x1b[20;2$y"},
		{"\x1b[99$p", "\x1b[99;0$y"},
		{"\x1b[?u", "\x1b[?0u"},
		{"\x1b[>5u\x1b[?u", "\x1b[?5u"},
		{"\x1b[>5u\x1b[>9u\x1b[?u", "\x1b[?9u"},
		{"\x1b[>5u\x1b[>9u\x1b[<u\x1b[?u", "\x1b[?5u"},
		{"\x1b[>5u\x1b[>9u\x1b[<5u\x1b[?u", "\x1b[?0u"},
		{"\x1b[=3u\x1b[?u", "\x1b[?3u"},
		{"\x1b[>1u\x1b[=6;2u\x1b[?u", "\x1b[?7u"},
		{"\x1b[>7u\x1b[=2;3u\x1b[?u", "\x1b[?5u"},
		{"\x1b[>1u\x1b[=4;1u\x1b[?u", "\x1b[?4u"},
		{"\x1b[2;5r\x1bP$qr\x1b\\", "\x1bP1$r2;5r\x1b\\"},
		{"\x1b[4 q\x1bP$q q\x1b\\", "\x1bP1$r4 q\x1b\\"},
		{"\x1b[1;31m\x1bP$qm\x1b\\", "\x1bP1$r1;31m\x1b\\"},
		{"\x1bP$qz\x1b\\", "\x1bP0$r\x1b\\"},
	}
	for _, c := range cases {
		h := newHarness(t, 40, 10)
		h.write(c.in)
		assert.Equal(t, c.want, h.take(), "%q", c.in)
	}
}

func TestKittyStackDepth(t *testing.T) {
	h := newHarness(t, 10, 2)
	for i := 0; i < 20; i++ {
		h.write("\x1b[>1u")
	}
	h.write("\x1b[>2u")
	for i := 0; i < 16; i++ {
		h.write("\x1b[<u")
	}
	assert.Equal(t, 0, h.term.Modes().KittyKeyboardFlags)
}

func TestOSCColorQueries(t *testing.T) {
	h := newHarness(t, 10, 2)
	h.write("\x1b]10;?\x07")
	assert.Equal(t, "\x1b]10;rgb:e5e5/e5e5/e5e5\x07", h.take())
	h.write("\x1b]10;?\x1b\\")
	assert.Equal(t, "\x1b]10;rgb:e5e5/e5e5/e5e5\x1b\\", h.take())
	h.term.SetColors(text.RGB(0x12, 0x34, 0x56), text.RGB(0xff, 0, 0), text.RGB(0, 0xff, 0))
	h.write("\x1b]10;?\x1b\\\x1b]11;?\x1b\\\x1b]12;?\x1b\\")
	assert.Equal(t, "\x1b]10;rgb:1212/3434/5656\x1b\\\x1b]11;rgb:ffff/0000/0000\x1b\\\x1b]12;rgb:0000/ffff/0000\x1b\\", h.take())
	h.write("\x1b]10;?;?\x1b\\")
	assert.Equal(t, "\x1b]10;rgb:1212/3434/5656\x1b\\\x1b]11;rgb:ffff/0000/0000\x1b\\", h.take())
	h.write("\x1b]11;rgb:00/80/ff\x1b\\\x1b]11;?\x1b\\")
	assert.Equal(t, "\x1b]11;rgb:0000/8080/ffff\x1b\\", h.take())
	h.write("\x1b]11;#102030\x1b\\\x1b]11;?\x1b\\")
	assert.Equal(t, "\x1b]11;rgb:1010/2020/3030\x1b\\", h.take())
}

func TestOSCColorProvider(t *testing.T) {
	fg, bg := text.RGB(1, 2, 3), text.RGB(4, 5, 6)
	var replies []string
	term := New(Options{
		Cols: 10, Rows: 2,
		Reply:  func(b []byte) { replies = append(replies, string(b)) },
		Colors: func() (text.Color, text.Color, text.Color) { return fg, bg, text.Color{} },
	})
	query := func(s string) string {
		replies = nil
		_, _ = term.Write([]byte(s))
		return strings.Join(replies, "")
	}
	assert.Equal(t, "\x1b]10;rgb:0101/0202/0303\x07\x1b]11;rgb:0404/0505/0606\x07", query("\x1b]10;?\x07\x1b]11;?\x07"))
	assert.Equal(t, "\x1b]12;rgb:e5e5/e5e5/e5e5\x1b\\", query("\x1b]12;?\x1b\\"))
	bg = text.RGB(7, 8, 9)
	assert.Equal(t, "\x1b]11;rgb:0707/0808/0909\x1b\\", query("\x1b]11;?\x1b\\"))
	assert.Equal(t, "\x1b]11;rgb:0000/8080/ffff\x1b\\", query("\x1b]11;rgb:00/80/ff\x1b\\\x1b]11;?\x1b\\"))
}

func TestOSCPalette(t *testing.T) {
	h := newHarness(t, 10, 2)
	h.write("\x1b]4;1;?\x1b\\")
	assert.Equal(t, "\x1b]4;1;rgb:8080/0000/0000\x1b\\", h.take())
	h.write("\x1b]4;1;rgb:ff/00/00\x1b\\\x1b]4;1;?\x1b\\")
	assert.Equal(t, "\x1b]4;1;rgb:ffff/0000/0000\x1b\\", h.take())
	h.write("\x1b]104;1\x1b\\\x1b]4;1;?\x1b\\")
	assert.Equal(t, "\x1b]4;1;rgb:8080/0000/0000\x1b\\", h.take())
}

func TestOSCTitleHyperlinkClipboard(t *testing.T) {
	h := newHarness(t, 20, 2)
	h.write("\x1b]0;one\x07\x1b]2;two\x1b\\")
	assert.Equal(t, []string{"one", "two"}, h.titles)
	assert.Equal(t, "two", h.term.Title())

	h.write("\x1b]8;id=1;https://x.test\x1b\\link\x1b]8;;\x1b\\ plain")
	cells := h.term.Screen().Line(0)
	assert.Equal(t, "https://x.test", cells[0].Link)
	assert.Equal(t, "https://x.test", cells[3].Link)
	assert.Equal(t, "", cells[4].Link)

	h.write("\x1b]52;c;aGVsbG8=\x07")
	assert.Equal(t, []string{"c:hello"}, h.clips)
	h.write("\x1b]52;c;?\x07\x1b]52;c;!!!\x07")
	assert.Len(t, h.clips, 1)
	assert.Empty(t, h.take())
}

func TestWriteChunkedInput(t *testing.T) {
	h := newHarness(t, 10, 2)
	for _, b := range []byte("a\x1b[3") {
		h.write(string([]byte{b}))
	}
	h.write("1mb\x1b[")
	h.write("0mc")
	assert.Equal(t, "abc", h.row(0))
	assert.Equal(t, text.Basic(1), h.term.Screen().Line(0)[1].Style.Fg)
}
