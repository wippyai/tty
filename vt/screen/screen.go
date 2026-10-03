// SPDX-License-Identifier: MPL-2.0

// Package screen holds terminal display state: a grid of cells, the cursor,
// scrolling regions, tab stops, character sets, the primary and alternate
// buffers, and a bounded scrollback ring that allocates only the lines it
// stores and can change its limit while live. It knows nothing about escape
// sequences; package vt drives it.
package screen

import "github.com/wippyai/tty/text"

// Cell is one grid position. Cluster is empty for an untouched cell and for
// the trailing half of a wide character (Wide marks the leading half).
type Cell struct {
	Cluster string
	Style   text.Style
	Link    string
	Wide    bool
}

// Cursor is the active position and pen state.
type Cursor struct {
	X, Y        int
	Style       text.Style
	Link        string
	PendingWrap bool // DECAWM deferred wrap at the right margin
	Visible     bool
	Shape       CursorShape
}

// CursorShape is set by DECSCUSR.
type CursorShape int

const (
	CursorBlock CursorShape = iota
	CursorUnderline
	CursorBar
)

// EraseMode selects ED/EL extent.
type EraseMode int

const (
	EraseToEnd EraseMode = iota
	EraseToStart
	EraseAll
	EraseScrollback // ED 3
)

// Screen is the display state of one terminal. Methods operate on the active
// buffer; Alt switches buffers. All coordinates are zero-based.
type Screen interface {
	Size() (cols, rows int)
	// Resize changes the grid; the primary buffer reflows soft-wrapped lines.
	Resize(cols, rows int)

	Cursor() Cursor
	SetCursor(c Cursor)
	MoveTo(x, y int)    // absolute, clamped (honors origin mode via SetOrigin)
	MoveBy(dx, dy int)  // relative, clamped to the scrolling region
	SaveCursor()        // DECSC
	RestoreCursor()     // DECRC

	// Print writes one grapheme cluster of the given cell width at the cursor,
	// handling autowrap, insert mode and wide-character repair.
	Print(cluster string, width int)
	LineFeed()          // index, scrolling the region when at its bottom
	ReverseIndex()      // RI
	CarriageReturn()
	Backspace()
	Tab(n int)          // forward tab stops; negative moves backward
	SetTabStop()
	ClearTabStop(all bool)

	EraseDisplay(mode EraseMode)
	EraseLine(mode EraseMode)
	EraseChars(n int)   // ECH
	InsertChars(n int)  // ICH
	DeleteChars(n int)  // DCH
	InsertLines(n int)  // IL
	DeleteLines(n int)  // DL
	ScrollUp(n int)     // SU
	ScrollDown(n int)   // SD
	SetScrollRegion(top, bottom int) // DECSTBM; bottom < 0 means last row
	SetOrigin(enabled bool)          // DECOM
	SetAutowrap(enabled bool)        // DECAWM
	SetInsert(enabled bool)          // IRM
	FillScreen(cluster string)       // DECALN

	// UseAlternate switches to (true) or from (false) the alternate buffer.
	UseAlternate(on bool, clear bool)
	Alternate() bool

	// Scrollback holds lines scrolled off the primary buffer's top.
	ScrollbackLen() int
	ScrollbackLine(i int) []Cell // 0 is the oldest retained line
	SetScrollbackLimit(lines int) // live; trims oldest lines immediately
	ClearScrollback()

	// Line returns the visible row y of the active buffer.
	Line(y int) []Cell
	// LineWrapped reports whether row y continues onto the next row (soft wrap).
	LineWrapped(y int) bool
}
