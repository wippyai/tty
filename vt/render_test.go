// SPDX-License-Identifier: MPL-2.0

package vt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderRow(t *testing.T) {
	h := newHarness(t, 6, 3)
	h.write("ab\x1b[1;31mcd\x1b[0mef")
	assert.Equal(t, "ab\x1b[31;1mcd\x1b[mef", h.term.RenderRow(0))
	assert.Equal(t, "      ", h.term.RenderRow(1))
	assert.Equal(t, "", h.term.RenderRow(-1))
	assert.Equal(t, "", h.term.RenderRow(3))
}

func TestRenderRowHyperlinkAndWide(t *testing.T) {
	h := newHarness(t, 8, 2)
	h.write("\x1b]8;;https://x.test\x1b\\界b\x1b]8;;\x1b\\c")
	assert.Equal(t, "\x1b]8;;https://x.test\x1b\\界b\x1b]8;;\x1b\\c    ", h.term.RenderRow(0))
}

func TestRenderRowTrailingStyleReset(t *testing.T) {
	h := newHarness(t, 3, 1)
	h.write("\x1b[42mab\x1b[m")
	assert.Equal(t, "\x1b[42mab\x1b[m ", h.term.RenderRow(0))
	h.write("\x1b[1;1H\x1b[42mabc")
	assert.Equal(t, "\x1b[42mabc\x1b[m", h.term.RenderRow(0))
}

func TestRenderRoundTrip(t *testing.T) {
	h := newHarness(t, 12, 1)
	h.write("\x1b[1mbold\x1b[22;38;2;1;2;3m rgb\x1b[4:3m cur")
	h2 := newHarness(t, 12, 1)
	h2.write(h.term.RenderRow(0))
	assert.Equal(t, h.term.Screen().Line(0), h2.term.Screen().Line(0))
}

func TestRenderLineOfTrimmedScrollbackRow(t *testing.T) {
	h := newHarness(t, 10, 2)
	h.write("plain\r\n\x1b[31mred\x1b[m  x\r\n\x1b[44m  \x1b[m\r\n\r\n")
	sb := h.term.Screen()
	assert.Equal(t, "plain", RenderLine(TrimLine(sb.ScrollbackLine(0))))
	assert.Equal(t, "\x1b[31mred\x1b[m  x", RenderLine(TrimLine(sb.ScrollbackLine(1))))
	assert.Equal(t, "\x1b[44m  \x1b[m", RenderLine(TrimLine(sb.ScrollbackLine(2))))
	assert.Empty(t, RenderLine(TrimLine(nil)))
}

func TestRenderLineKeepsWideRunesAtTheEdge(t *testing.T) {
	h := newHarness(t, 6, 1)
	h.write("a世b")
	assert.Equal(t, "a世b", RenderLine(TrimLine(h.term.Screen().Line(0))))
}

func TestNarrowingFullScrollbackReflowsWithinLimit(t *testing.T) {
	h := newHarness(t, 80, 2)
	h.term.SetScrollbackLimit(5)
	for range 20 {
		h.write(strings.Repeat("x", 80) + "\r\n")
	}
	assert.Equal(t, 5, h.term.Screen().ScrollbackLen())
	assert.NotPanics(t, func() { h.term.Resize(10, 2) })
	assert.LessOrEqual(t, h.term.Screen().ScrollbackLen(), 5)
}
