// SPDX-License-Identifier: MPL-2.0

package vt

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/wippyai/tty/text"
)

var (
	defaultFg     = text.RGB(0xe5, 0xe5, 0xe5)
	defaultBg     = text.RGB(0, 0, 0)
	defaultCursor = text.RGB(0xe5, 0xe5, 0xe5)
)

// OSC dispatches an operating system command.
func (t *Terminal) OSC(data []byte, bel bool) {
	term := "\x1b\\"
	if bel {
		term = "\x07"
	}
	s := string(data)
	num, rest, _ := strings.Cut(s, ";")
	n, err := strconv.Atoi(num)
	if err != nil {
		return
	}
	switch n {
	case 0, 2:
		t.title = rest
		if t.opts.Title != nil {
			t.opts.Title(rest)
		}
	case 4:
		t.oscPalette(rest, term)
	case 8:
		t.oscHyperlink(rest)
	case 10:
		t.oscDynamicColors(10, rest, term)
	case 11:
		t.oscDynamicColors(11, rest, term)
	case 12:
		t.oscDynamicColors(12, rest, term)
	case 52:
		t.oscClipboard(rest)
	case 104:
		if rest == "" {
			t.palette = map[int]text.Color{}
			return
		}
		for _, f := range strings.Split(rest, ";") {
			if i, err := strconv.Atoi(f); err == nil {
				delete(t.palette, i)
			}
		}
	case 110:
		t.fg = text.Color{}
	case 111:
		t.bg = text.Color{}
	case 112:
		t.cursorColor = text.Color{}
	}
}

func (t *Terminal) oscHyperlink(rest string) {
	_, uri, ok := strings.Cut(rest, ";")
	if !ok {
		return
	}
	c := t.scr.Cursor()
	c.Link = uri
	t.scr.SetCursor(c)
}

func (t *Terminal) oscClipboard(rest string) {
	if t.opts.Clipboard == nil {
		return
	}
	sel, payload, ok := strings.Cut(rest, ";")
	if !ok || payload == "?" {
		return
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(payload, "="))
		if err != nil {
			return
		}
	}
	t.opts.Clipboard(sel, data)
}

func (t *Terminal) paletteColor(i int) text.Color {
	if c, ok := t.palette[i]; ok {
		return c
	}
	return indexedColor(i)
}

func (t *Terminal) oscPalette(rest, term string) {
	f := strings.Split(rest, ";")
	for i := 0; i+1 < len(f); i += 2 {
		idx, err := strconv.Atoi(f[i])
		if err != nil || idx < 0 || idx > 255 {
			continue
		}
		if f[i+1] == "?" {
			t.replyString(fmt.Sprintf("\x1b]4;%d;%s%s", idx, formatXColor(t.paletteColor(idx)), term))
			continue
		}
		if c, ok := parseXColor(f[i+1]); ok {
			t.palette[idx] = c
		}
	}
}

// oscDynamicColors handles OSC 10, 11 and 12; consecutive parameters address
// consecutive dynamic colors.
func (t *Terminal) oscDynamicColors(first int, rest, term string) {
	for i, v := range strings.Split(rest, ";") {
		slot := first + i
		if slot > 12 {
			return
		}
		var cur *text.Color
		var def text.Color
		switch slot {
		case 10:
			cur, def = &t.fg, defaultFg
		case 11:
			cur, def = &t.bg, defaultBg
		default:
			cur, def = &t.cursorColor, defaultCursor
		}
		if v == "?" {
			c := *cur
			if c.IsNone() {
				c = t.providedColor(slot)
			}
			if c.IsNone() {
				c = def
			}
			t.replyString(fmt.Sprintf("\x1b]%d;%s%s", slot, formatXColor(c), term))
			continue
		}
		if c, ok := parseXColor(v); ok {
			*cur = c
		}
	}
}

// providedColor asks Options.Colors for the dynamic color of slot 10, 11 or 12.
func (t *Terminal) providedColor(slot int) text.Color {
	if t.opts.Colors == nil {
		return text.Color{}
	}
	fg, bg, cursor := t.opts.Colors()
	switch slot {
	case 10:
		return fg
	case 11:
		return bg
	}
	return cursor
}

func formatXColor(c text.Color) string {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return "rgb:0000/0000/0000"
	}
	return fmt.Sprintf("rgb:%04x/%04x/%04x", r, g, b)
}

// parseXColor reads "rgb:r/g/b" with 1-4 hex digits per channel, or "#rrggbb".
func parseXColor(s string) (text.Color, bool) {
	if rest, ok := strings.CutPrefix(s, "rgb:"); ok {
		f := strings.Split(rest, "/")
		if len(f) != 3 {
			return text.Color{}, false
		}
		var ch [3]uint8
		for i, h := range f {
			if len(h) < 1 || len(h) > 4 {
				return text.Color{}, false
			}
			v, err := strconv.ParseUint(h, 16, 16)
			if err != nil {
				return text.Color{}, false
			}
			max := uint64(1)<<(4*uint(len(h))) - 1
			ch[i] = uint8((v*255 + max/2) / max)
		}
		return text.RGB(ch[0], ch[1], ch[2]), true
	}
	if strings.HasPrefix(s, "#") && len(s) == 7 {
		return text.ParseColor(s)
	}
	return text.Color{}, false
}
