// SPDX-License-Identifier: MPL-2.0

package vt

import (
	"fmt"
	"strings"

	"github.com/wippyai/tty/text"
	"github.com/wippyai/tty/vt/parser"
	"github.com/wippyai/tty/vt/screen"
)

var _ parser.Handler = (*Terminal)(nil)

// decGraphics maps the DEC special graphics set (0x5f-0x7e) to Unicode.
var decGraphics = map[byte]string{
	'_': " ", '`': "◆", 'a': "▒", 'b': "␉", 'c': "␌", 'd': "␍",
	'e': "␊", 'f': "°", 'g': "±", 'h': "␤", 'i': "␋", 'j': "┘",
	'k': "┐", 'l': "┌", 'm': "└", 'n': "┼", 'o': "⎺", 'p': "⎻",
	'q': "─", 'r': "⎼", 's': "⎽", 't': "├", 'u': "┤", 'v': "┴",
	'w': "┬", 'x': "│", 'y': "≤", 'z': "≥", '{': "π", '|': "≠",
	'}': "£", '~': "·",
}

func (t *Terminal) resetCharsets() {
	t.charsets = [4]byte{'B', 'B', 'B', 'B'}
	t.gl = 0
	t.singleShft = -1
}

// Print renders one grapheme cluster, translating through the active charset.
func (t *Terminal) Print(cluster string) {
	set := t.charsets[t.gl]
	if t.singleShft >= 0 {
		set = t.charsets[t.singleShft]
		t.singleShft = -1
	}
	if len(cluster) == 1 {
		switch set {
		case '0':
			if r, ok := decGraphics[cluster[0]]; ok {
				cluster = r
			}
		case 'A':
			if cluster[0] == '#' {
				cluster = "£"
			}
		}
	}
	w := text.ClusterWidth(cluster)
	if w <= 0 {
		return
	}
	t.lastRune = cluster
	t.scr.Print(cluster, w)
}

func (t *Terminal) lineFeed() {
	t.scr.LineFeed()
	if t.modes.LinefeedNewline {
		t.scr.CarriageReturn()
	}
}

// Execute runs a C0 or C1 control function.
func (t *Terminal) Execute(code byte) {
	switch code {
	case 0x07:
		if t.opts.Bell != nil {
			t.opts.Bell()
		}
	case 0x08:
		t.scr.Backspace()
	case 0x09:
		t.scr.Tab(1)
	case 0x0a, 0x0b, 0x0c:
		t.lineFeed()
	case 0x0d:
		t.scr.CarriageReturn()
	case 0x0e:
		t.gl = 1
	case 0x0f:
		t.gl = 0
	case 0x84:
		t.scr.LineFeed()
	case 0x85:
		t.scr.LineFeed()
		t.scr.CarriageReturn()
	case 0x88:
		t.scr.SetTabStop()
	case 0x8d:
		t.scr.ReverseIndex()
	}
}

// ESC dispatches an escape sequence.
func (t *Terminal) ESC(inter []byte, final byte) {
	switch len(inter) {
	case 0:
		t.escPlain(final)
	case 1:
		switch inter[0] {
		case '(', ')', '*', '+':
			t.designate(int(inter[0]-'(')%4, final)
		case '#':
			if final == '8' {
				t.scr.FillScreen("E")
			}
		}
	}
}

func (t *Terminal) escPlain(final byte) {
	switch final {
	case '7':
		t.saveCursor()
	case '8':
		t.restoreCursor()
	case 'D':
		t.scr.LineFeed()
	case 'E':
		t.scr.LineFeed()
		t.scr.CarriageReturn()
	case 'M':
		t.scr.ReverseIndex()
	case 'H':
		t.scr.SetTabStop()
	case 'c':
		t.fullReset()
	case '=':
		t.modes.ApplicationKeypad = true
	case '>':
		t.modes.ApplicationKeypad = false
	case 'n':
		t.gl = 2
	case 'o':
		t.gl = 3
	case 'N':
		t.singleShft = 2
	case 'O':
		t.singleShft = 3
	}
}

func (t *Terminal) designate(slot int, final byte) {
	switch final {
	case '0', 'A', 'B':
		t.charsets[slot] = final
	default:
		t.charsets[slot] = 'B'
	}
}

func (t *Terminal) saveCursor() {
	t.scr.SaveCursor()
	t.saved[t.bufferIndex()] = savedState{valid: true, charsets: t.charsets, gl: t.gl, origin: t.origin, wrap: t.wrap}
}

// restoreCursor implements DECRC. The screen restores its own origin and
// autowrap state; the terminal mirrors the flags it tracks.
func (t *Terminal) restoreCursor() {
	t.scr.RestoreCursor()
	sv := t.saved[t.bufferIndex()]
	if !sv.valid {
		t.origin = false
		t.resetCharsets()
		return
	}
	t.charsets, t.gl, t.origin, t.wrap = sv.charsets, sv.gl, sv.origin, sv.wrap
}

func (t *Terminal) setOrigin(on bool) {
	t.origin = on
	t.scr.SetOrigin(on)
}

func (t *Terminal) setWrap(on bool) {
	t.wrap = on
	t.scr.SetAutowrap(on)
}

func (t *Terminal) setInsert(on bool) {
	t.insert = on
	t.scr.SetInsert(on)
}

func (t *Terminal) resetMargins() {
	t.marginTop, t.marginBottom = 0, -1
	t.scr.SetScrollRegion(0, -1)
}

func (t *Terminal) setCursorShape(shape screen.CursorShape, blink bool) {
	c := t.scr.Cursor()
	c.Shape = shape
	t.scr.SetCursor(c)
	t.modes.CursorBlink = blink
}

// softReset implements DECSTR.
func (t *Terminal) softReset() {
	c := t.scr.Cursor()
	c.Visible = true
	c.Style = text.Style{}
	c.Link = ""
	c.Shape = screen.CursorBlock
	t.scr.SetCursor(c)
	t.modes.CursorBlink = false
	t.setInsert(false)
	t.setOrigin(false)
	t.setWrap(true)
	t.modes.ApplicationCursor = false
	t.modes.ApplicationKeypad = false
	t.modes.LinefeedNewline = false
	t.resetCharsets()
	t.resetMargins()
	t.saved = [2]savedState{}
}

// fullReset implements RIS.
func (t *Terminal) fullReset() {
	if t.scr.Alternate() {
		t.scr.UseAlternate(false, false)
	}
	t.softReset()
	t.modes = Modes{}
	t.kitty = [2]kittyStack{}
	t.gfx.reset()
	t.palette = map[int]text.Color{}
	t.lastRune = ""
	t.lastMouseValid = false
	t.scr.EraseDisplay(screen.EraseAll)
	t.scr.ClearTabStop(true)
	for x := 8; x < t.cols; x += 8 {
		t.scr.MoveTo(x, 0)
		t.scr.SetTabStop()
	}
	t.scr.MoveTo(0, 0)
}

// DCS handles DECRQSS requests; other device control strings are ignored.
func (t *Terminal) DCS(prefix byte, params parser.Params, inter []byte, final byte, data []byte) {
	if prefix != 0 || string(inter) != "$" || final != 'q' {
		return
	}
	var body string
	switch string(data) {
	case "m":
		sgr := strings.TrimSuffix(strings.TrimPrefix(t.scr.Cursor().Style.String(), "\x1b["), "m")
		if sgr == "" {
			sgr = "0"
		}
		body = sgr + "m"
	case "r":
		bottom := t.marginBottom
		if bottom < 0 {
			bottom = t.rows - 1
		}
		body = fmt.Sprintf("%d;%dr", t.marginTop+1, bottom+1)
	case " q":
		body = fmt.Sprintf("%d q", t.cursorStyleParam())
	default:
		t.replyString("\x1bP0$r\x1b\\")
		return
	}
	t.replyString("\x1bP1$r" + body + "\x1b\\")
}

func (t *Terminal) cursorStyleParam() int {
	n := 2
	switch t.scr.Cursor().Shape {
	case screen.CursorUnderline:
		n = 4
	case screen.CursorBar:
		n = 6
	}
	if t.modes.CursorBlink {
		n--
	}
	return n
}
