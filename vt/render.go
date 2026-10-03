// SPDX-License-Identifier: MPL-2.0

package vt

import (
	"strings"

	"github.com/wippyai/tty/text"
	"github.com/wippyai/tty/vt/screen"
)

// RenderRow renders visible row y as an ANSI string with minimal SGR
// transitions and OSC 8 hyperlinks. The result spans the full row width and
// ends with the style reset and any open link closed. Rows outside the grid
// render as the empty string.
func (t *Terminal) RenderRow(y int) string {
	if y < 0 || y >= t.rows {
		return ""
	}
	return RenderLine(t.scr.Line(y))
}

// RenderLine renders cells as an ANSI string with minimal SGR transitions and
// OSC 8 hyperlinks. The result ends with the style reset and any open link
// closed.
func RenderLine(cells []screen.Cell) string {
	var b strings.Builder
	var cur text.Style
	link := ""
	skip := false
	for _, c := range cells {
		if skip {
			skip = false
			continue
		}
		b.WriteString(c.Style.Diff(cur))
		cur = c.Style
		if c.Link != link {
			b.WriteString("\x1b]8;;" + c.Link + "\x1b\\")
			link = c.Link
		}
		switch {
		case c.Cluster == "":
			b.WriteByte(' ')
		default:
			b.WriteString(c.Cluster)
		}
		skip = c.Wide
	}
	if link != "" {
		b.WriteString("\x1b]8;;\x1b\\")
	}
	b.WriteString(text.Style{}.Diff(cur))
	return b.String()
}

// TrimLine returns cells without the trailing cells that are blank and carry
// the default style and no link, so a line renders to its visible extent.
func TrimLine(cells []screen.Cell) []screen.Cell {
	end := len(cells)
	for end > 0 {
		c := cells[end-1]
		if (c.Cluster != "" && c.Cluster != " ") || !c.Style.IsZero() || c.Link != "" {
			break
		}
		end--
	}
	return cells[:end]
}
