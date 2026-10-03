// SPDX-License-Identifier: MPL-2.0

package vt

import (
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
