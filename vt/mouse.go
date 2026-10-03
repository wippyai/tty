// SPDX-License-Identifier: MPL-2.0

package vt

import (
	"fmt"
	"strings"

	"github.com/wippyai/tty/input"
)

var mouseButtonCodes = map[input.MouseButton]int{
	input.MouseLeft: 0, input.MouseMiddle: 1, input.MouseRight: 2,
	input.MouseWheelUp: 64, input.MouseWheelDown: 65, input.MouseWheelLeft: 66, input.MouseWheelRight: 67,
	input.MouseBackward: 128, input.MouseForward: 129, input.MouseButton10: 130, input.MouseButton11: 131,
}

func isWheelButton(b input.MouseButton) bool {
	return b >= input.MouseWheelUp && b <= input.MouseWheelRight
}

// SendMouse encodes a pointer event per the active tracking mode and encoding
// and writes it through Options.Reply. Without tracking, wheel events on the
// alternate screen become cursor keys when alternate scroll (1007) is set.
// SGR-pixel reports use the center of the cell as the pixel position. It
// reports whether any bytes were sent.
func (t *Terminal) SendMouse(ev input.Event) bool {
	var m input.Mouse
	var press, motion, release bool
	switch e := ev.(type) {
	case input.MouseClickEvent:
		m, press = input.Mouse(e), true
	case input.MouseWheelEvent:
		m, press = input.Mouse(e), true
	case input.MouseReleaseEvent:
		m, release = input.Mouse(e), true
	case input.MouseMotionEvent:
		m, motion = input.Mouse(e), true
	default:
		return false
	}
	tracking := t.modes.MouseTracking
	if tracking == MouseOff {
		return t.alternateScroll(m, press)
	}
	switch {
	case tracking == MouseX10 && !press:
		return false
	case motion && tracking < MouseButtonMotion:
		return false
	case motion && tracking == MouseButtonMotion && m.Button == input.MouseNone:
		return false
	}
	base, ok := mouseButtonCodes[m.Button]
	if m.Button == input.MouseNone && motion {
		base, ok = 3, true
	}
	if !ok {
		return false
	}
	enc := t.modes.MouseEncoding
	sgr := enc == MouseEncodingSGR || enc == MouseEncodingSGRPixels
	if release && !sgr {
		base = 3
	}
	if tracking != MouseX10 {
		if m.Mod&input.ModShift != 0 {
			base |= 4
		}
		if m.Mod&input.ModAlt != 0 {
			base |= 8
		}
		if m.Mod&input.ModCtrl != 0 {
			base |= 16
		}
	}
	if motion {
		if t.lastMouseValid && t.lastMouseX == m.X && t.lastMouseY == m.Y {
			return false
		}
		base |= 32
	}
	t.lastMouseX, t.lastMouseY, t.lastMouseValid = m.X, m.Y, true

	x, y := m.X+1, m.Y+1
	var out string
	switch enc {
	case MouseEncodingSGRPixels:
		out = sgrMouse(base, m.X*t.cellW+t.cellW/2+1, m.Y*t.cellH+t.cellH/2+1, release)
	case MouseEncodingSGR:
		out = sgrMouse(base, x, y, release)
	case MouseEncodingUTF8:
		if x > 2015 || y > 2015 {
			return false
		}
		var b strings.Builder
		b.WriteString("\x1b[M")
		b.WriteByte(byte(base + 32))
		b.WriteString(string(rune(x + 32)))
		b.WriteString(string(rune(y + 32)))
		out = b.String()
	default:
		if x > 223 || y > 223 {
			return false
		}
		out = "\x1b[M" + string([]byte{byte(base + 32), byte(x + 32), byte(y + 32)})
	}
	t.replyString(out)
	return true
}

func sgrMouse(code, x, y int, release bool) string {
	final := 'M'
	if release {
		final = 'm'
	}
	return fmt.Sprintf("\x1b[<%d;%d;%d%c", code, x, y, final)
}

func (t *Terminal) alternateScroll(m input.Mouse, press bool) bool {
	if !press || !t.modes.AlternateScroll || !t.scr.Alternate() {
		return false
	}
	switch m.Button {
	case input.MouseWheelUp:
		t.replyString(t.cursorIntro() + "A")
	case input.MouseWheelDown:
		t.replyString(t.cursorIntro() + "B")
	default:
		return false
	}
	return true
}

// SendPaste writes pasted text, wrapped in bracketed paste markers when mode
// 2004 is set. Embedded ESC [ 201 ~ sequences are removed so the text cannot
// end the paste early; without bracketed paste, newlines become carriage
// returns. It reports whether any bytes were sent.
func (t *Terminal) SendPaste(s string) bool {
	if t.modes.BracketedPaste {
		const end = "\x1b[201~"
		for strings.Contains(s, end) {
			s = strings.ReplaceAll(s, end, "")
		}
		t.replyString("\x1b[200~" + s + end)
		return true
	}
	if s == "" {
		return false
	}
	s = strings.ReplaceAll(s, "\r\n", "\r")
	t.replyString(strings.ReplaceAll(s, "\n", "\r"))
	return true
}

// SendFocus reports a focus change when mode 1004 is set.
func (t *Terminal) SendFocus(focused bool) bool {
	if !t.modes.FocusEvents {
		return false
	}
	if focused {
		t.replyString("\x1b[I")
	} else {
		t.replyString("\x1b[O")
	}
	return true
}
