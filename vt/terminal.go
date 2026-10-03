// SPDX-License-Identifier: MPL-2.0

// Package vt is a VT/xterm terminal emulator for PTY-backed windows. It feeds
// child output through package parser into package screen, tracks modes,
// answers queries, encodes input for the child and keeps kitty graphics image
// placements. It is not safe for concurrent use; callers serialize access.
package vt

import (
	"github.com/wippyai/tty/input"
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

// Image is a kitty graphics image transmitted by the child.
type Image struct {
	ID     uint32
	Format int // 24 RGB, 32 RGBA, 100 PNG
	Width  int
	Height int
	Data   []byte
}

// Placement shows an image at a cell position.
type Placement struct {
	ImageID     uint32
	PlacementID uint32
	X, Y        int // cell position in the buffer at placement time
	Cols, Rows  int // cell extent (0 = derived from image size)
	Z           int
}

// Options configure a Terminal.
type Options struct {
	Cols, Rows      int
	ScrollbackLines int
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

// Terminal is one emulated terminal. Its surface:
//
//	New(Options) *Terminal
//	Write(p []byte) (int, error)        // child output
//	Resize(cols, rows int)
//	SetScrollbackLimit(lines int)
//	Screen() screen.Screen              // read cells, cursor, scrollback
//	Modes() Modes
//	Title() string
//	Images() []Image; Placements() []Placement
//	SendKey(ev input.KeyPressEvent / input.KeyReleaseEvent) // encodes per modes, writes via Reply
//	SendMouse(ev input.MouseClickEvent / ... )               // encodes per tracking and encoding
//	SendPaste(text string)                                   // honors bracketed paste
//	SendFocus(focused bool)                                  // honors focus events
//	SetColors(fg, bg, cursor text.Color)                     // answers OSC 10/11/12 queries
//	RenderRow(y int) string                                  // ANSI row via package canvas
var _ = input.KeyPressEvent{}
var _ screen.Screen
