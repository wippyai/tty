// SPDX-License-Identifier: MPL-2.0

package vt

import (
	"github.com/wippyai/tty/text"
	"github.com/wippyai/tty/vt/parser"
)

func (t *Terminal) sgr(p parser.Params) {
	c := t.scr.Cursor()
	applySGR(&c.Style, p)
	t.scr.SetCursor(c)
}

// sub returns sub-parameter j of parameter i, or 0 when absent or defaulted.
func sub(p parser.Params, i, j int) int {
	if i >= len(p.Values) || j >= len(p.Values[i]) || p.Values[i][j] == parser.Default {
		return 0
	}
	return p.Values[i][j]
}

func clamp8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func indexedColor(n int) text.Color {
	if n < 16 {
		return text.Basic(uint8(n))
	}
	return text.Indexed(clamp8(n))
}

// extendedColor decodes the color introduced by parameter i (38, 48 or 58) in
// the colon or semicolon form. It returns the color and the index of the last
// parameter consumed.
func extendedColor(p parser.Params, i int) (text.Color, int, bool) {
	if len(p.Values[i]) > 1 {
		v := p.Values[i]
		switch sub(p, i, 1) {
		case 5:
			if len(v) > 2 {
				return indexedColor(sub(p, i, 2)), i, true
			}
		case 2:
			switch {
			case len(v) >= 6:
				return text.RGB(clamp8(sub(p, i, 3)), clamp8(sub(p, i, 4)), clamp8(sub(p, i, 5))), i, true
			case len(v) == 5:
				return text.RGB(clamp8(sub(p, i, 2)), clamp8(sub(p, i, 3)), clamp8(sub(p, i, 4))), i, true
			}
		}
		return text.Color{}, i, false
	}
	switch p.Get(i+1, -2) {
	case 5:
		if i+2 < p.Len() {
			return indexedColor(sub(p, i+2, 0)), i + 2, true
		}
	case 2:
		if i+4 < p.Len() {
			return text.RGB(clamp8(sub(p, i+2, 0)), clamp8(sub(p, i+3, 0)), clamp8(sub(p, i+4, 0))), i + 4, true
		}
	}
	return text.Color{}, p.Len(), false
}

func applySGR(s *text.Style, p parser.Params) {
	if p.Len() == 0 {
		*s = text.Style{}
		return
	}
	for i := 0; i < p.Len(); i++ {
		v := sub(p, i, 0)
		switch {
		case v == 0:
			*s = text.Style{}
		case v == 1:
			s.Attrs |= text.AttrBold
		case v == 2:
			s.Attrs |= text.AttrFaint
		case v == 3:
			s.Attrs |= text.AttrItalic
		case v == 4:
			s.Underline = text.UnderlineSingle
			if len(p.Values[i]) > 1 {
				if u := sub(p, i, 1); u >= 0 && u <= int(text.UnderlineDashed) {
					s.Underline = text.Underline(u)
				}
			}
		case v == 5:
			s.Attrs |= text.AttrBlink
		case v == 6:
			s.Attrs |= text.AttrRapidBlink
		case v == 7:
			s.Attrs |= text.AttrReverse
		case v == 8:
			s.Attrs |= text.AttrConceal
		case v == 9:
			s.Attrs |= text.AttrStrikethrough
		case v == 21:
			s.Underline = text.UnderlineDouble
		case v == 22:
			s.Attrs &^= text.AttrBold | text.AttrFaint
		case v == 23:
			s.Attrs &^= text.AttrItalic
		case v == 24:
			s.Underline = text.UnderlineNone
		case v == 25:
			s.Attrs &^= text.AttrBlink | text.AttrRapidBlink
		case v == 27:
			s.Attrs &^= text.AttrReverse
		case v == 28:
			s.Attrs &^= text.AttrConceal
		case v == 29:
			s.Attrs &^= text.AttrStrikethrough
		case v >= 30 && v <= 37:
			s.Fg = text.Basic(uint8(v - 30))
		case v == 38 || v == 48 || v == 58:
			c, last, ok := extendedColor(p, i)
			if ok {
				switch v {
				case 38:
					s.Fg = c
				case 48:
					s.Bg = c
				default:
					s.UnderlineColor = c
				}
			}
			i = last
		case v == 39:
			s.Fg = text.Color{}
		case v >= 40 && v <= 47:
			s.Bg = text.Basic(uint8(v - 40))
		case v == 49:
			s.Bg = text.Color{}
		case v == 59:
			s.UnderlineColor = text.Color{}
		case v >= 90 && v <= 97:
			s.Fg = text.Basic(uint8(v - 90 + 8))
		case v >= 100 && v <= 107:
			s.Bg = text.Basic(uint8(v - 100 + 8))
		}
	}
}
