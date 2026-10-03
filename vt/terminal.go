// SPDX-License-Identifier: MPL-2.0

// Package vt is a VT/xterm terminal emulator for PTY-backed windows. It feeds
// child output through package parser into package screen, tracks modes,
// answers queries, encodes input for the child and keeps kitty graphics image
// placements. It is not safe for concurrent use; callers serialize access.
package vt

import (
	"github.com/wippyai/tty/text"
	"github.com/wippyai/tty/vt/parser"
	"github.com/wippyai/tty/vt/screen"
)

// Modes are the DEC private and ANSI modes the embedding application observes.
type Modes struct {
	ApplicationCursor  bool // DECCKM
	ApplicationKeypad  bool // DECKPAM
	BracketedPaste     bool // 2004
	FocusEvents        bool // 1004
	SynchronizedOutput bool // 2026
	AlternateScroll    bool // 1007
	CursorBlink        bool // 12 and DECSCUSR
	LinefeedNewline    bool // LNM
	MouseTracking      MouseTracking
	MouseEncoding      MouseEncoding
	KittyKeyboardFlags int // current flags of the kitty keyboard protocol stack
}

// MouseTracking is the active mouse reporting mode (9, 1000, 1002, 1003).
type MouseTracking int

const (
	MouseOff MouseTracking = iota
	MouseX10
	MouseNormal
	MouseButtonMotion
	MouseAnyMotion
)

// MouseEncoding is the active report encoding (default, 1005 UTF-8, 1006 SGR, 1016 SGR pixels).
type MouseEncoding int

const (
	MouseEncodingDefault MouseEncoding = iota
	MouseEncodingUTF8
	MouseEncodingSGR
	MouseEncodingSGRPixels
)

// Image is a kitty graphics image transmitted by the child. Data is the
// pixel data as transmitted (RGB, RGBA or PNG bytes, decompressed).
type Image struct {
	ID     uint32
	Format int // 24 RGB, 32 RGBA, 100 PNG
	Width  int
	Height int
	Data   []byte
}

// Placement shows an image at a cell position. Y counts rows from the top of
// the primary scrollback on the primary buffer (ScrollbackLen()+row at
// placement time) and from the top of the grid on the alternate buffer.
// Cols and Rows are always filled; when the child omits them they derive from
// the image size and Options.CellWidth/CellHeight.
type Placement struct {
	ImageID     uint32
	PlacementID uint32
	X, Y        int
	Cols, Rows  int
	Z           int
}

// Default cell size in pixels used to derive placement extents.
const (
	defaultCellWidth  = 8
	defaultCellHeight = 16
)

// Options configure a Terminal.
type Options struct {
	Cols, Rows      int
	ScrollbackLines int
	// CellWidth and CellHeight are the cell size in pixels, used for kitty
	// graphics extents and size reports. Zero selects 8x16.
	CellWidth, CellHeight int
	// Screen is the display state; nil creates one of Cols x Rows.
	Screen screen.Screen
	// Reply receives bytes the terminal sends back to the child (DA, DSR,
	// OSC color queries, kitty keyboard reports, encoded input).
	Reply func([]byte)
	// Title is called when the child sets the window title (OSC 0/2).
	Title func(string)
	// Clipboard is called for OSC 52 writes; nil ignores them.
	Clipboard func(selection string, data []byte)
	// Bell is called on BEL.
	Bell func()
}

const maxKittyStack = 16

type kittyStack struct{ flags []int }

func (k *kittyStack) top() int {
	if len(k.flags) == 0 {
		return 0
	}
	return k.flags[len(k.flags)-1]
}

type savedState struct {
	valid    bool
	wrap     bool
	charsets [4]byte
	gl       int
	origin   bool
}

// Terminal is one emulated terminal.
//
//	New(Options) *Terminal
//	Write(p []byte) (int, error)          child output
//	Resize(cols, rows int)
//	SetScrollbackLimit(lines int)
//	Screen() screen.Screen                cells, cursor, scrollback
//	Modes() Modes
//	Title() string
//	Images() []Image; Placements() []Placement
//	SendKey(input.Event) bool             key press and release
//	SendMouse(input.Event) bool           click, release, motion, wheel
//	SendPaste(string) bool                bracketed paste aware
//	SendFocus(bool) bool                  mode 1004 aware
//	SetColors(fg, bg, cursor text.Color)  answers OSC 10/11/12 queries
//	RenderRow(y int) string               ANSI row
type Terminal struct {
	opts   Options
	scr    screen.Screen
	prs    *parser.Parser
	title  string
	modes  Modes
	cols   int
	rows   int
	cellW  int
	cellH  int
	origin bool
	wrap   bool
	insert bool

	marginTop, marginBottom int // bottom < 0 means the last row

	charsets   [4]byte
	gl         int
	singleShft int // -1 or the G-set for the next character
	saved      [2]savedState
	lastRune   string

	fg, bg, cursorColor text.Color
	palette             map[int]text.Color

	kitty [2]kittyStack // primary, alternate

	gfx gfxState

	lastMouseX, lastMouseY int
	lastMouseValid         bool
}

// New creates a Terminal.
func New(opts Options) *Terminal {
	if opts.Cols <= 0 {
		opts.Cols = 80
	}
	if opts.Rows <= 0 {
		opts.Rows = 24
	}
	t := &Terminal{opts: opts, cols: opts.Cols, rows: opts.Rows, wrap: true, marginBottom: -1, singleShft: -1}
	t.cellW, t.cellH = opts.CellWidth, opts.CellHeight
	if t.cellW <= 0 {
		t.cellW = defaultCellWidth
	}
	if t.cellH <= 0 {
		t.cellH = defaultCellHeight
	}
	t.scr = opts.Screen
	if t.scr == nil {
		t.scr = screen.New(opts.Cols, opts.Rows, opts.ScrollbackLines)
	}
	if opts.ScrollbackLines > 0 {
		t.scr.SetScrollbackLimit(opts.ScrollbackLines)
	}
	t.resetCharsets()
	t.palette = map[int]text.Color{}
	t.gfx.init()
	t.prs = parser.New(t)
	c := t.scr.Cursor()
	c.Visible = true
	t.scr.SetCursor(c)
	return t
}

// Write feeds child output into the terminal. The parser holds the last
// grapheme cluster of a run until a boundary, so Write flushes after each
// chunk.
func (t *Terminal) Write(p []byte) (int, error) {
	n, err := t.prs.Write(p)
	t.prs.Flush()
	return n, err
}

// Resize changes the grid size and resets the scrolling region.
func (t *Terminal) Resize(cols, rows int) {
	if cols <= 0 || rows <= 0 {
		return
	}
	t.cols, t.rows = cols, rows
	t.marginTop, t.marginBottom = 0, -1
	t.scr.Resize(cols, rows)
}

// SetScrollbackLimit changes the scrollback line limit.
func (t *Terminal) SetScrollbackLimit(lines int) { t.scr.SetScrollbackLimit(lines) }

// Screen returns the display state.
func (t *Terminal) Screen() screen.Screen { return t.scr }

// Modes returns the current mode state.
func (t *Terminal) Modes() Modes {
	m := t.modes
	m.KittyKeyboardFlags = t.kb().top()
	return m
}

// Title returns the last title set by the child.
func (t *Terminal) Title() string { return t.title }

// SetColors sets the default foreground, background and cursor colors used to
// answer OSC 10/11/12 queries. A zero color selects the built-in default.
func (t *Terminal) SetColors(fg, bg, cursor text.Color) {
	t.fg, t.bg, t.cursorColor = fg, bg, cursor
}

func (t *Terminal) reply(b []byte) {
	if t.opts.Reply != nil && len(b) > 0 {
		t.opts.Reply(b)
	}
}

func (t *Terminal) replyString(s string) { t.reply([]byte(s)) }

func (t *Terminal) kb() *kittyStack {
	if t.scr.Alternate() {
		return &t.kitty[1]
	}
	return &t.kitty[0]
}
