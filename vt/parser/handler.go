// SPDX-License-Identifier: MPL-2.0

// Package parser implements the DEC/xterm escape sequence state machine
// (Paul Williams' DEC-compatible parser, extended for UTF-8, OSC, DCS and APC).
// It turns a byte stream into Handler calls and holds no screen state.
package parser

// Params holds CSI/DCS parameters. Each parameter may carry colon-separated
// sub-parameters (e.g. SGR 38:2::r:g:b). A missing parameter is Default.
type Params struct {
	Values [][]int
}

// Default marks an omitted parameter or sub-parameter.
const Default = -1

// Get returns parameter i, or fallback when it is missing or Default.
func (p Params) Get(i, fallback int) int {
	if i < 0 || i >= len(p.Values) || len(p.Values[i]) == 0 || p.Values[i][0] == Default {
		return fallback
	}
	return p.Values[i][0]
}

// Len reports the number of parameters.
func (p Params) Len() int { return len(p.Values) }

// Handler receives the actions the parser recognizes, in input order.
type Handler interface {
	// Print renders one grapheme cluster (UTF-8 text, already segmented).
	Print(cluster string)
	// Execute runs a C0 or C1 control function.
	Execute(code byte)
	// CSI dispatches a control sequence: prefix is the private marker
	// ('?', '>', '<', '=' or 0), intermediates the bytes 0x20-0x2F, final the final byte.
	CSI(prefix byte, params Params, intermediates []byte, final byte)
	// ESC dispatches an escape sequence with its intermediates and final byte.
	ESC(intermediates []byte, final byte)
	// OSC delivers a complete operating system command (data excludes the
	// terminator). bel reports a BEL terminator; otherwise the string ended
	// with ST (ESC followed by backslash).
	OSC(data []byte, bel bool)
	// DCS delivers a complete device control string.
	DCS(prefix byte, params Params, intermediates []byte, final byte, data []byte)
	// APC delivers a complete application program command (kitty graphics uses APC G).
	APC(data []byte)
}
