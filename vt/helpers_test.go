// SPDX-License-Identifier: MPL-2.0

package vt

import (
	"strings"
	"testing"

	"github.com/wippyai/tty/vt/screen"
)

type harness struct {
	t       *testing.T
	term    *Terminal
	replies []string
	titles  []string
	clips   []string
	bells   int
}

func newHarness(t *testing.T, cols, rows int) *harness {
	t.Helper()
	h := &harness{t: t}
	h.term = New(Options{
		Cols: cols, Rows: rows, ScrollbackLines: 100,
		Reply:     func(b []byte) { h.replies = append(h.replies, string(b)) },
		Title:     func(s string) { h.titles = append(h.titles, s) },
		Clipboard: func(sel string, d []byte) { h.clips = append(h.clips, sel+":"+string(d)) },
		Bell:      func() { h.bells++ },
	})
	return h
}

func (h *harness) write(s string) {
	h.t.Helper()
	if _, err := h.term.Write([]byte(s)); err != nil {
		h.t.Fatal(err)
	}
}

// take returns and clears the accumulated replies.
func (h *harness) take() string {
	s := strings.Join(h.replies, "")
	h.replies = nil
	return s
}

func (h *harness) row(y int) string {
	var b strings.Builder
	for _, c := range h.term.Screen().Line(y) {
		if c.Cluster == "" {
			b.WriteByte(' ')
		} else {
			b.WriteString(c.Cluster)
		}
	}
	return strings.TrimRight(b.String(), " ")
}

func (h *harness) cursor() screen.Cursor { return h.term.Screen().Cursor() }

func (h *harness) pos() (int, int) {
	c := h.cursor()
	return c.X, c.Y
}
