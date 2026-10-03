// SPDX-License-Identifier: MPL-2.0

package vt

import "github.com/wippyai/tty/vt/screen"

// decFlag returns the stored flag of a DEC private mode that needs no side
// effect beyond the flag itself.
func (t *Terminal) decFlag(n int) *bool {
	switch n {
	case 1:
		return &t.modes.ApplicationCursor
	case 12:
		return &t.modes.CursorBlink
	case 66:
		return &t.modes.ApplicationKeypad
	case 1004:
		return &t.modes.FocusEvents
	case 1007:
		return &t.modes.AlternateScroll
	case 2004:
		return &t.modes.BracketedPaste
	case 2026:
		return &t.modes.SynchronizedOutput
	}
	return nil
}

var mouseTrackingModes = map[int]MouseTracking{
	9: MouseX10, 1000: MouseNormal, 1002: MouseButtonMotion, 1003: MouseAnyMotion,
}

var mouseEncodingModes = map[int]MouseEncoding{
	1005: MouseEncodingUTF8, 1006: MouseEncodingSGR, 1016: MouseEncodingSGRPixels,
}

// decModeState reports whether DEC private mode n is recognized and set.
func (t *Terminal) decModeState(n int) (known, set bool) {
	if f := t.decFlag(n); f != nil {
		return true, *f
	}
	if tr, ok := mouseTrackingModes[n]; ok {
		return true, t.modes.MouseTracking == tr
	}
	if enc, ok := mouseEncodingModes[n]; ok {
		return true, t.modes.MouseEncoding == enc
	}
	switch n {
	case 6:
		return true, t.origin
	case 7:
		return true, t.wrap
	case 25:
		return true, t.scr.Cursor().Visible
	case 47, 1047, 1049:
		return true, t.scr.Alternate()
	case 1048:
		return true, false
	}
	return false, false
}

func (t *Terminal) setDECMode(n int, on bool) {
	if f := t.decFlag(n); f != nil {
		*f = on
		return
	}
	if tr, ok := mouseTrackingModes[n]; ok {
		switch {
		case on:
			t.modes.MouseTracking = tr
		case t.modes.MouseTracking == tr:
			t.modes.MouseTracking = MouseOff
		}
		return
	}
	if enc, ok := mouseEncodingModes[n]; ok {
		switch {
		case on:
			t.modes.MouseEncoding = enc
		case t.modes.MouseEncoding == enc:
			t.modes.MouseEncoding = MouseEncodingDefault
		}
		return
	}
	switch n {
	case 6:
		t.setOrigin(on)
		t.scr.MoveTo(0, 0)
	case 7:
		t.setWrap(on)
	case 25:
		c := t.scr.Cursor()
		c.Visible = on
		t.scr.SetCursor(c)
	case 47:
		t.useAlternate(on, false)
	case 1047:
		if !on && t.scr.Alternate() {
			t.scr.EraseDisplay(screen.EraseAll)
		}
		t.useAlternate(on, false)
	case 1048:
		if on {
			t.saveCursor()
		} else {
			t.restoreCursor()
		}
	case 1049:
		if on {
			if !t.scr.Alternate() {
				t.saveCursor()
			}
			t.useAlternate(true, true)
		} else {
			t.useAlternate(false, false)
			t.restoreCursor()
		}
	}
}

func (t *Terminal) useAlternate(on, clear bool) {
	if t.scr.Alternate() == on {
		return
	}
	t.scr.UseAlternate(on, clear)
	if on {
		t.gfx.placements[1] = nil
	}
}

func (t *Terminal) kittyPush(flags int) {
	k := t.kb()
	if len(k.flags) >= maxKittyStack {
		k.flags = k.flags[1:]
	}
	k.flags = append(k.flags, flags&0x1f)
}

func (t *Terminal) kittyPop(n int) {
	k := t.kb()
	if n > len(k.flags) {
		n = len(k.flags)
	}
	k.flags = k.flags[:len(k.flags)-n]
}

// kittySet applies CSI = flags ; mode u to the top of the stack: mode 1
// replaces, 2 sets the given bits, 3 clears them.
func (t *Terminal) kittySet(flags, mode int) {
	k := t.kb()
	if len(k.flags) == 0 {
		k.flags = []int{0}
	}
	top := &k.flags[len(k.flags)-1]
	flags &= 0x1f
	switch mode {
	case 1:
		*top = flags
	case 2:
		*top |= flags
	case 3:
		*top &^= flags
	}
}
