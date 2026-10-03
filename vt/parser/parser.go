// SPDX-License-Identifier: MPL-2.0

package parser

import (
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// Limits. Sequences that exceed a limit are dropped whole: the handler never
// sees a truncated or partially collected command.
const (
	// MaxParams is the maximum number of parameters in one CSI or DCS.
	MaxParams = 32
	// MaxSubParams is the maximum number of sub-parameters (colon separated
	// values, including the first) in one parameter.
	MaxSubParams = 16
	// MaxParamValue is the saturation value of a numeric parameter.
	MaxParamValue = 65535
	// MaxIntermediates is the maximum number of intermediate bytes.
	MaxIntermediates = 4
	// MaxOSCLen is the maximum OSC payload in bytes.
	MaxOSCLen = 1 << 20
	// MaxDCSLen is the maximum DCS data payload in bytes.
	MaxDCSLen = 1 << 20
	// MaxAPCLen is the maximum APC payload in bytes (kitty graphics chunks
	// are base64 and arrive as many APC sequences, each well below this).
	MaxAPCLen = 1 << 20

	// maxClusterBytes bounds a pending grapheme cluster; a longer run of
	// extending code points is split.
	maxClusterBytes = 256
	// retainBuf is the largest string buffer kept between sequences.
	retainBuf = 64 << 10
)

type state uint8

const (
	stGround state = iota
	stEscape
	stEscInter
	stCSIEntry
	stCSIParam
	stCSIInter
	stCSIIgnore
	stDCSEntry
	stDCSParam
	stDCSInter
	stDCSPass
	stDCSIgnore
	stOSC
	stAPC
	stSOS
	stStringEsc
)

var asciiStr [128]string

func init() {
	for i := range asciiStr {
		asciiStr[i] = string(rune(i))
	}
}

var replacementBytes = []byte("\xef\xbf\xbd")

// Parser is a pure byte-stream to Handler translator. It keeps no timers and
// no screen state; the same bytes always produce the same calls regardless of
// how the stream is split across Write calls.
//
// Text is delivered one grapheme cluster per Print call. The last cluster of a
// text run stays pending because later bytes may extend it; it is delivered
// when the next code point starts a new cluster, when a control or escape
// byte arrives, or on Flush. Hosts call Flush after draining a read so the
// final character is not held back.
//
// Slices and Params passed to Handler methods are only valid during the call.
//
// Behavior notes:
//   - Input is UTF-8. Bytes 0x80-0x9F are never C1 controls; invalid UTF-8
//     yields U+FFFD per maximal invalid subpart. 7-bit C1 (ESC @ .. ESC _)
//     is delivered through ESC, CSI, OSC, DCS and APC.
//   - CAN and SUB abort the current sequence and are delivered via Execute.
//   - C0 controls inside CSI and ESC sequences execute without aborting them.
//   - ESC inside a string (OSC, DCS, APC) terminates it: ESC \ is the string
//     terminator, any other byte after ESC ends the string and starts a new
//     escape sequence. OSC is also terminated by BEL.
//   - A byte >= 0x80 inside ESC or CSI aborts the sequence and is decoded as
//     text. Inside strings it is payload.
//   - Too many parameters, sub-parameters or intermediates make the whole
//     sequence ignored. Numeric values saturate at MaxParamValue.
//   - OSC, DCS and APC payloads over MaxOSCLen, MaxDCSLen and MaxAPCLen are
//     dropped without a handler call. SOS and PM strings are consumed and
//     ignored.
type Parser struct {
	h Handler

	state state
	str   state // string state interrupted by ESC

	// text
	pend []byte
	u8   [4]byte
	u8n  int
	u8nd int
	lo   byte
	hi   byte
	one  [1]byte

	// sequence
	prefix byte
	inter  [MaxIntermediates]byte
	ninter int
	bad    bool

	sub    [MaxParams][MaxSubParams]int
	ns     [MaxParams]int
	n      int
	have   bool
	values [][]int

	dcsFinal byte
	buf      []byte
	over     bool
}

// New returns a parser that reports to h.
func New(h Handler) *Parser {
	return &Parser{h: h, values: make([][]int, 0, MaxParams)}
}

// Reset returns the parser to the ground state and discards all pending input.
func (p *Parser) Reset() {
	p.state = stGround
	p.pend = p.pend[:0]
	p.u8n = 0
	p.endString()
	p.clear()
}

// Flush delivers the pending grapheme cluster, if any. Incomplete UTF-8 and
// open escape sequences stay pending.
func (p *Parser) Flush() { p.emitPend() }

// Write parses data and reports the recognized actions. It implements io.Writer
// and never fails.
func (p *Parser) Write(data []byte) (int, error) {
	for i := 0; i < len(data); {
		if p.state == stGround && p.u8n == 0 {
			j := i
			for j < len(data) && data[j] >= 0x20 && data[j] < 0x7F {
				j++
			}
			if j > i {
				for k := i; k < j; k++ {
					p.feed(rune(data[k]), data[k:k+1])
				}
				i = j
				continue
			}
		}
		p.step(data[i])
		i++
	}
	return len(data), nil
}

func (p *Parser) emitPend() {
	if len(p.pend) == 0 {
		return
	}
	if len(p.pend) == 1 && p.pend[0] < 0x80 {
		p.h.Print(asciiStr[p.pend[0]])
	} else {
		p.h.Print(string(p.pend))
	}
	p.pend = p.pend[:0]
}

// feed appends one code point to the pending cluster or starts a new one.
func (p *Parser) feed(r rune, enc []byte) {
	if len(p.pend) == 0 {
		p.pend = append(p.pend, enc...)
		return
	}
	if r < 0x80 && len(p.pend) == 1 && p.pend[0] < 0x80 {
		p.emitPend()
		p.pend = append(p.pend, enc...)
		return
	}
	n := len(p.pend)
	p.pend = append(p.pend, enc...)
	if len(p.pend) <= maxClusterBytes {
		c, _, _, _ := uniseg.FirstGraphemeCluster(p.pend, -1)
		if len(c) == len(p.pend) {
			return
		}
	}
	p.pend = p.pend[:n]
	p.emitPend()
	p.pend = append(p.pend, enc...)
}

func (p *Parser) clear() {
	p.prefix = 0
	p.ninter = 0
	p.bad = false
	p.n = 0
	p.ns[0] = 1
	p.sub[0][0] = Default
	p.have = false
}

func (p *Parser) collect(b byte) {
	if p.ninter == MaxIntermediates {
		p.bad = true
		return
	}
	p.inter[p.ninter] = b
	p.ninter++
}

// param consumes a digit, ':' or ';'. It reports false on overflow.
func (p *Parser) param(b byte) bool {
	p.have = true
	switch {
	case b <= '9':
		s := &p.sub[p.n][p.ns[p.n]-1]
		if *s == Default {
			*s = 0
		}
		*s = *s*10 + int(b-'0')
		if *s > MaxParamValue {
			*s = MaxParamValue
		}
	case b == ':':
		if p.ns[p.n] == MaxSubParams {
			return false
		}
		p.sub[p.n][p.ns[p.n]] = Default
		p.ns[p.n]++
	default:
		if p.n+1 == MaxParams {
			return false
		}
		p.n++
		p.ns[p.n] = 1
		p.sub[p.n][0] = Default
	}
	return true
}

func (p *Parser) params() Params {
	p.values = p.values[:0]
	if p.have {
		for i := 0; i <= p.n; i++ {
			p.values = append(p.values, p.sub[i][:p.ns[i]])
		}
	}
	return Params{Values: p.values}
}

func (p *Parser) enterEscape() {
	p.state = stEscape
	p.clear()
}

func (p *Parser) startString(s state) {
	p.state = s
	p.buf = p.buf[:0]
	p.over = false
}

func (p *Parser) appendString(b byte, limit int) {
	if p.over {
		return
	}
	if len(p.buf) >= limit {
		p.over = true
		p.buf = nil
		return
	}
	p.buf = append(p.buf, b)
}

// endString dispatches the open string, if complete, and releases its buffer.
func (p *Parser) endString() { p.finishString(p.str) }

func (p *Parser) finishString(kind state) {
	if !p.over {
		switch kind {
		case stOSC:
			p.h.OSC(p.buf)
		case stAPC:
			p.h.APC(p.buf)
		case stDCSPass:
			p.h.DCS(p.prefix, p.params(), p.inter[:p.ninter], p.dcsFinal, p.buf)
		}
	}
	p.over = false
	if cap(p.buf) > retainBuf {
		p.buf = nil
	} else {
		p.buf = p.buf[:0]
	}
}

func isStringState(s state) bool {
	return s == stDCSPass || s == stDCSIgnore || s == stOSC || s == stAPC || s == stSOS
}

func (p *Parser) step(b byte) {
	if p.u8n > 0 {
		if b >= p.lo && b <= p.hi {
			p.u8[p.u8n] = b
			p.u8n++
			p.lo, p.hi = 0x80, 0xBF
			if p.u8n == p.u8nd {
				r, _ := utf8.DecodeRune(p.u8[:p.u8n])
				n := p.u8n
				p.u8n = 0
				p.feed(r, p.u8[:n])
			}
			return
		}
		p.u8n = 0
		p.feed(utf8.RuneError, replacementBytes)
	}

	switch b {
	case 0x18, 0x1A:
		if p.state == stGround {
			p.emitPend()
		}
		p.state = stGround
		p.over = false
		p.buf = p.buf[:0]
		p.h.Execute(b)
		return
	case 0x1B:
		switch {
		case isStringState(p.state):
			p.str = p.state
			p.state = stStringEsc
		case p.state == stStringEsc:
			p.endString()
			p.enterEscape()
		default:
			p.emitPend()
			p.enterEscape()
		}
		return
	}

	switch p.state {
	case stGround:
		p.ground(b)
	case stEscape:
		p.escape(b)
	case stEscInter:
		switch {
		case b < 0x20:
			p.h.Execute(b)
		case b <= 0x2F:
			p.collect(b)
		case b == 0x7F:
		case b >= 0x80:
			p.state = stGround
			p.step(b)
		default:
			p.dispatchESC(b)
		}
	case stCSIEntry, stCSIParam, stCSIInter, stCSIIgnore:
		p.csi(b)
	case stDCSEntry, stDCSParam, stDCSInter, stDCSIgnore:
		p.dcs(b)
	case stDCSPass:
		if b != 0x7F {
			p.appendString(b, MaxDCSLen)
		}
	case stOSC:
		switch {
		case b == 0x07:
			p.finishString(stOSC)
			p.state = stGround
		case b >= 0x20:
			p.appendString(b, MaxOSCLen)
		}
	case stAPC:
		if b >= 0x20 {
			p.appendString(b, MaxAPCLen)
		}
	case stSOS:
	case stStringEsc:
		p.endString()
		if b == '\\' {
			p.state = stGround
			return
		}
		p.enterEscape()
		p.step(b)
	}
}

func (p *Parser) ground(b byte) {
	switch {
	case b >= 0x80:
		var need int
		p.lo, p.hi = 0x80, 0xBF
		switch {
		case b >= 0xC2 && b <= 0xDF:
			need = 2
		case b == 0xE0:
			need, p.lo = 3, 0xA0
		case b == 0xED:
			need, p.hi = 3, 0x9F
		case b >= 0xE1 && b <= 0xEF:
			need = 3
		case b == 0xF0:
			need, p.lo = 4, 0x90
		case b >= 0xF1 && b <= 0xF3:
			need = 4
		case b == 0xF4:
			need, p.hi = 4, 0x8F
		default:
			p.feed(utf8.RuneError, replacementBytes)
			return
		}
		p.u8[0] = b
		p.u8n = 1
		p.u8nd = need
	case b >= 0x20 && b < 0x7F:
		p.one[0] = b
		p.feed(rune(b), p.one[:])
	case b == 0x7F:
	default:
		p.emitPend()
		p.h.Execute(b)
	}
}

func (p *Parser) escape(b byte) {
	switch {
	case b < 0x20:
		p.h.Execute(b)
	case b <= 0x2F:
		p.collect(b)
		p.state = stEscInter
	case b == 0x7F:
	case b >= 0x80:
		p.state = stGround
		p.step(b)
	default:
		switch b {
		case '[':
			p.state = stCSIEntry
		case ']':
			p.startString(stOSC)
		case 'P':
			p.state = stDCSEntry
		case '_':
			p.startString(stAPC)
		case 'X', '^':
			p.state = stSOS
		default:
			p.dispatchESC(b)
		}
	}
}

func (p *Parser) dispatchESC(b byte) {
	if !p.bad {
		p.h.ESC(p.inter[:p.ninter], b)
	}
	p.state = stGround
}

// csi handles CSI states.
func (p *Parser) csi(b byte) {
	switch {
	case b < 0x20:
		p.h.Execute(b)
	case b == 0x7F:
	case b >= 0x80:
		p.state = stGround
		p.step(b)
	case b <= 0x2F:
		if p.state == stCSIIgnore {
			return
		}
		p.collect(b)
		p.state = stCSIInter
	case b <= 0x3F:
		p.csiParamByte(b)
	default:
		if p.state != stCSIIgnore && !p.bad {
			p.h.CSI(p.prefix, p.params(), p.inter[:p.ninter], b)
		}
		p.state = stGround
	}
}

func (p *Parser) csiParamByte(b byte) {
	switch p.state {
	case stCSIEntry:
		if b >= 0x3C {
			p.prefix = b
			p.state = stCSIParam
			return
		}
		p.state = stCSIParam
		fallthrough
	case stCSIParam:
		if b >= 0x3C || !p.param(b) {
			p.state = stCSIIgnore
		}
	default:
		p.state = stCSIIgnore
	}
}

// dcs handles DCS states up to and including the hook.
func (p *Parser) dcs(b byte) {
	switch {
	case b < 0x20:
	case b == 0x7F:
	case b >= 0x80:
		p.state = stGround
		p.step(b)
	case b <= 0x2F:
		switch p.state {
		case stDCSIgnore:
		default:
			p.collect(b)
			p.state = stDCSInter
		}
	case b <= 0x3F:
		switch p.state {
		case stDCSEntry:
			if b >= 0x3C {
				p.prefix = b
				p.state = stDCSParam
				return
			}
			p.state = stDCSParam
			fallthrough
		case stDCSParam:
			if b >= 0x3C || !p.param(b) {
				p.state = stDCSIgnore
			}
		default:
			p.state = stDCSIgnore
		}
	default:
		if p.state == stDCSIgnore || p.bad {
			p.state = stDCSIgnore
			return
		}
		p.dcsFinal = b
		p.startString(stDCSPass)
	}
}
