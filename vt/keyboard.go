// SPDX-License-Identifier: MPL-2.0

package vt

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/wippyai/tty/input"
)

// Kitty keyboard protocol progressive enhancement flags.
const (
	kittyDisambiguate = 1 << iota
	kittyEventTypes
	kittyAlternateKeys
	kittyAllKeys
	kittyAssociatedText
)

type funcKey struct {
	final byte // letter-final form: CSI 1;m final, SS3 final unmodified
	num   int  // tilde form: CSI num;m ~
}

var funcKeys = map[rune]funcKey{
	input.KeyUp: {final: 'A'}, input.KeyDown: {final: 'B'}, input.KeyRight: {final: 'C'},
	input.KeyLeft: {final: 'D'}, input.KeyBegin: {final: 'E'}, input.KeyEnd: {final: 'F'},
	input.KeyHome: {final: 'H'},
	input.KeyKpUp: {final: 'A'}, input.KeyKpDown: {final: 'B'}, input.KeyKpRight: {final: 'C'},
	input.KeyKpLeft: {final: 'D'}, input.KeyKpBegin: {final: 'E'}, input.KeyKpEnd: {final: 'F'},
	input.KeyKpHome: {final: 'H'},
	input.KeyF1:     {final: 'P'}, input.KeyF2: {final: 'Q'}, input.KeyF3: {final: 'R', num: 13}, input.KeyF4: {final: 'S'},
	input.KeyFind: {num: 1}, input.KeyInsert: {num: 2}, input.KeyDelete: {num: 3}, input.KeySelect: {num: 4},
	input.KeyPgUp: {num: 5}, input.KeyPgDown: {num: 6},
	input.KeyKpInsert: {num: 2}, input.KeyKpDelete: {num: 3}, input.KeyKpPgUp: {num: 5}, input.KeyKpPgDown: {num: 6},
	input.KeyF5: {num: 15}, input.KeyF6: {num: 17}, input.KeyF7: {num: 18}, input.KeyF8: {num: 19},
	input.KeyF9: {num: 20}, input.KeyF10: {num: 21}, input.KeyF11: {num: 23}, input.KeyF12: {num: 24},
	input.KeyF13: {num: 25}, input.KeyF14: {num: 26}, input.KeyF15: {num: 28}, input.KeyF16: {num: 29},
	input.KeyF17: {num: 31}, input.KeyF18: {num: 32}, input.KeyF19: {num: 33}, input.KeyF20: {num: 34},
}

// kittyCodes maps keys to their kitty keyboard protocol CSI u key numbers.
var kittyCodes = map[rune]int{
	input.KeyEscape: 27, input.KeyEnter: 13, input.KeyTab: 9, input.KeyBackspace: 127,
	input.KeyCapsLock: 57358, input.KeyScrollLock: 57359, input.KeyNumLock: 57360,
	input.KeyPrintScreen: 57361, input.KeyPause: 57362, input.KeyMenu: 57363,
	input.KeyKp0: 57399, input.KeyKp1: 57400, input.KeyKp2: 57401, input.KeyKp3: 57402,
	input.KeyKp4: 57403, input.KeyKp5: 57404, input.KeyKp6: 57405, input.KeyKp7: 57406,
	input.KeyKp8: 57407, input.KeyKp9: 57408, input.KeyKpDecimal: 57409, input.KeyKpDivide: 57410,
	input.KeyKpMultiply: 57411, input.KeyKpMinus: 57412, input.KeyKpPlus: 57413,
	input.KeyKpEnter: 57414, input.KeyKpEqual: 57415, input.KeyKpSep: 57416,
	input.KeyKpLeft: 57417, input.KeyKpRight: 57418, input.KeyKpUp: 57419, input.KeyKpDown: 57420,
	input.KeyKpPgUp: 57421, input.KeyKpPgDown: 57422, input.KeyKpHome: 57423, input.KeyKpEnd: 57424,
	input.KeyKpInsert: 57425, input.KeyKpDelete: 57426, input.KeyKpBegin: 57427,
	input.KeyMediaPlay: 57428, input.KeyMediaPause: 57429, input.KeyMediaPlayPause: 57430,
	input.KeyMediaReverse: 57431, input.KeyMediaStop: 57432, input.KeyMediaFastForward: 57433,
	input.KeyMediaRewind: 57434, input.KeyMediaNext: 57435, input.KeyMediaPrev: 57436,
	input.KeyMediaRecord: 57437, input.KeyLowerVol: 57438, input.KeyRaiseVol: 57439, input.KeyMute: 57440,
	input.KeyLeftShift: 57441, input.KeyLeftCtrl: 57442, input.KeyLeftAlt: 57443,
	input.KeyLeftSuper: 57444, input.KeyLeftHyper: 57445, input.KeyLeftMeta: 57446,
	input.KeyRightShift: 57447, input.KeyRightCtrl: 57448, input.KeyRightAlt: 57449,
	input.KeyRightSuper: 57450, input.KeyRightHyper: 57451, input.KeyRightMeta: 57452,
	input.KeyIsoLevel3Shift: 57453, input.KeyIsoLevel5Shift: 57454,
}

func init() {
	for i := 0; i < 23; i++ {
		kittyCodes[input.KeyF13+rune(i)] = 57376 + i
	}
}

// keypadKeys maps keypad keys to their numeric-mode text and
// application-mode SS3 final.
var keypadKeys = map[rune]struct {
	text  string
	final byte
}{
	input.KeyKp0: {"0", 'p'}, input.KeyKp1: {"1", 'q'}, input.KeyKp2: {"2", 'r'}, input.KeyKp3: {"3", 's'},
	input.KeyKp4: {"4", 't'}, input.KeyKp5: {"5", 'u'}, input.KeyKp6: {"6", 'v'}, input.KeyKp7: {"7", 'w'},
	input.KeyKp8: {"8", 'x'}, input.KeyKp9: {"9", 'y'}, input.KeyKpMultiply: {"*", 'j'},
	input.KeyKpPlus: {"+", 'k'}, input.KeyKpComma: {",", 'l'}, input.KeyKpMinus: {"-", 'm'},
	input.KeyKpDecimal: {".", 'n'}, input.KeyKpDivide: {"/", 'o'}, input.KeyKpEqual: {"=", 'X'},
	input.KeyKpEnter: {"\r", 'M'}, input.KeyKpSep: {",", 'l'},
}

func isKeypad(code rune) bool { return code >= input.KeyKpEnter && code <= input.KeyKpBegin }

const (
	lockMods = input.ModCapsLock | input.ModNumLock | input.ModScrollLock
)

// kittyModBits converts modifiers to the kitty bit layout (shift 1, alt 2,
// ctrl 4, super 8, hyper 16, meta 32, caps 64, num 128).
func kittyModBits(m input.Mod) int {
	n := 0
	for _, e := range []struct {
		m   input.Mod
		bit int
	}{
		{input.ModShift, 1}, {input.ModAlt, 2}, {input.ModCtrl, 4}, {input.ModSuper, 8},
		{input.ModHyper, 16}, {input.ModMeta, 32}, {input.ModCapsLock, 64}, {input.ModNumLock, 128},
	} {
		if m&e.m != 0 {
			n |= e.bit
		}
	}
	return n
}

// xtermModBits converts modifiers to the xterm layout (shift 1, alt 2, ctrl 4, meta 8).
func xtermModBits(m input.Mod) int {
	n := 0
	if m&input.ModShift != 0 {
		n |= 1
	}
	if m&input.ModAlt != 0 {
		n |= 2
	}
	if m&input.ModCtrl != 0 {
		n |= 4
	}
	if m&(input.ModMeta|input.ModSuper|input.ModHyper) != 0 {
		n |= 8
	}
	return n
}

// SendKey encodes a key press or release for the child and writes it through
// Options.Reply. It reports whether any bytes were sent.
func (t *Terminal) SendKey(ev input.Event) bool {
	var k input.Key
	release := false
	switch e := ev.(type) {
	case input.KeyPressEvent:
		k = input.Key(e)
	case input.KeyReleaseEvent:
		k = input.Key(e)
		release = true
	default:
		return false
	}
	out := t.encodeKey(k, release)
	if out == "" {
		return false
	}
	t.replyString(out)
	return true
}

func (t *Terminal) encodeKey(k input.Key, release bool) string {
	flags := t.kb().top()
	if flags != 0 {
		if s, handled := t.kittyKey(k, release, flags); handled {
			return s
		}
	}
	if release {
		return ""
	}
	if t.modes.ModifyOtherKeys == 2 {
		if s := t.modifiedKey(k); s != "" {
			return s
		}
	}
	return t.legacyKey(k)
}

// modifiedKey encodes a modified character key as CSI 27 ; modifiers ; code ~
// (modifyOtherKeys level 2). Named keys and the keys with a legacy control
// encoding are not included.
func (t *Terminal) modifiedKey(k input.Key) string {
	mods := xtermModBits(k.Mod)
	if mods == 0 || k.Code <= 0 || k.Code >= input.KeyExtended {
		return ""
	}
	switch k.Code {
	case input.KeyEnter, input.KeyTab, input.KeyBackspace, input.KeyEscape, input.KeySpace:
		return ""
	}
	code := k.Code
	if k.Mod&input.ModShift != 0 && k.ShiftedCode != 0 {
		code = k.ShiftedCode
	}
	return "\x1b[27;" + strconv.Itoa(mods+1) + ";" + strconv.Itoa(int(code)) + "~"
}

func (t *Terminal) cursorIntro() string {
	if t.modes.ApplicationCursor {
		return "\x1bO"
	}
	return "\x1b["
}

func ctrlByte(r rune) (byte, bool) {
	switch {
	case r >= 'a' && r <= 'z':
		return byte(r-'a') + 1, true
	case r >= '@' && r <= '_':
		return byte(r - '@'), true
	case r == ' ':
		return 0, true
	case r == '?':
		return 0x7f, true
	case r == '2':
		return 0, true
	case r >= '3' && r <= '7':
		return byte(r-'3') + 0x1b, true
	case r == '8':
		return 0x7f, true
	}
	return 0, false
}

func (t *Terminal) legacyKey(k input.Key) string {
	mods := xtermModBits(k.Mod)
	if fk, ok := funcKeys[k.Code]; ok {
		return t.legacyFuncKey(k, fk, mods)
	}
	if isKeypad(k.Code) {
		if ent, ok := keypadKeys[k.Code]; ok {
			return t.legacyKeypad(k, ent.text, ent.final, mods)
		}
		return ""
	}
	prefix := ""
	if k.Mod&(input.ModAlt|input.ModMeta) != 0 {
		prefix = "\x1b"
	}
	switch k.Code {
	case input.KeyEnter:
		if t.modes.LinefeedNewline {
			return prefix + "\r\n"
		}
		return prefix + "\r"
	case input.KeyTab:
		if k.Mod&input.ModShift != 0 {
			return prefix + "\x1b[Z"
		}
		return prefix + "\t"
	case input.KeyBackspace:
		if k.Mod&input.ModCtrl != 0 {
			return prefix + "\x08"
		}
		return prefix + "\x7f"
	case input.KeyEscape:
		return prefix + "\x1b"
	}
	if k.Code >= input.KeyExtended && k.Text == "" {
		return ""
	}
	r := k.Code
	if k.Mod&input.ModShift != 0 && k.ShiftedCode != 0 {
		r = k.ShiftedCode
	}
	if k.Mod&input.ModCtrl != 0 && r < input.KeyExtended {
		base := k.Code
		if base == 0 {
			base = r
		}
		if b, ok := ctrlByte(unicode.ToLower(base)); ok {
			return prefix + string([]byte{b})
		}
		if b, ok := ctrlByte(r); ok {
			return prefix + string([]byte{b})
		}
	}
	s := k.Text
	if s == "" && r > 0 && r < input.KeyExtended {
		s = string(r)
	}
	if s == "" {
		return ""
	}
	return prefix + s
}

func (t *Terminal) legacyFuncKey(k input.Key, fk funcKey, mods int) string {
	if fk.final == 0 {
		if mods == 0 {
			return "\x1b[" + strconv.Itoa(fk.num) + "~"
		}
		return "\x1b[" + strconv.Itoa(fk.num) + ";" + strconv.Itoa(mods+1) + "~"
	}
	if mods == 0 {
		intro := t.cursorIntro()
		if k.Code >= input.KeyF1 && k.Code <= input.KeyF4 {
			intro = "\x1bO"
		}
		return intro + string(fk.final)
	}
	return "\x1b[1;" + strconv.Itoa(mods+1) + string(fk.final)
}

func (t *Terminal) legacyKeypad(k input.Key, txt string, final byte, mods int) string {
	if !t.modes.ApplicationKeypad {
		if k.Code == input.KeyKpEnter && t.modes.LinefeedNewline {
			return "\r\n"
		}
		return txt
	}
	if mods == 0 {
		return "\x1bO" + string(final)
	}
	return "\x1bO" + strconv.Itoa(mods+1) + string(final)
}

// kittyKey encodes k under the kitty keyboard protocol. handled is false when
// the key falls back to the legacy encoding.
func (t *Terminal) kittyKey(k input.Key, release bool, flags int) (string, bool) {
	all := flags&kittyAllKeys != 0
	disamb := flags&(kittyDisambiguate|kittyAllKeys) != 0
	report := flags&kittyEventTypes != 0
	if release && !report {
		return "", true
	}
	event := 1
	switch {
	case release:
		event = 3
	case k.IsRepeat && report:
		event = 2
	}
	nonLock := k.Mod &^ lockMods
	modField := 1 + kittyModBits(k.Mod)

	if fk, ok := funcKeys[k.Code]; ok && !isKeypad(k.Code) {
		if event == 1 && modField == 1 {
			if k.Code == input.KeyF3 {
				return "\x1b[13~", true
			}
			return "", false
		}
		return kittyFuncSeq(fk, modField, event), true
	}

	code, isU := kittyCodes[k.Code]
	if !isU && k.Code < input.KeyExtended && k.Code > 0 {
		code, isU = int(k.Code), true
	}
	if !isU {
		return "", true
	}

	switch {
	case k.Code == input.KeyEnter || k.Code == input.KeyTab || k.Code == input.KeyBackspace:
		if !all && (nonLock == 0 || !disamb) {
			return "", release
		}
		if release && !all {
			return "", true
		}
	case k.Code == input.KeyEscape:
		if !disamb {
			return "", false
		}
	case isKeypad(k.Code):
		if !all && (nonLock == 0 || !disamb) {
			return "", release
		}
	case k.Code >= input.KeyExtended:
		if !all && (k.Code < input.KeyF21 || k.Code > input.KeyF35) {
			return "", true
		}
	default:
		onlyShift := nonLock&^input.ModShift == 0
		if !all && (onlyShift || !disamb) {
			return "", release
		}
		code = int(unicode.ToLower(rune(code)))
	}

	var b strings.Builder
	b.WriteString("\x1b[")
	b.WriteString(strconv.Itoa(code))
	if flags&kittyAlternateKeys != 0 && k.Code < input.KeyExtended {
		shifted := 0
		if k.Mod&input.ModShift != 0 && k.ShiftedCode != 0 && int(k.ShiftedCode) != code {
			shifted = int(k.ShiftedCode)
		}
		base := 0
		if k.BaseCode != 0 && int(k.BaseCode) != code {
			base = int(k.BaseCode)
		}
		if shifted != 0 || base != 0 {
			b.WriteByte(':')
			if shifted != 0 {
				b.WriteString(strconv.Itoa(shifted))
			}
			if base != 0 {
				b.WriteByte(':')
				b.WriteString(strconv.Itoa(base))
			}
		}
	}
	var textField string
	if flags&kittyAssociatedText != 0 && all && k.Text != "" && event != 3 {
		parts := make([]string, 0, len(k.Text))
		for _, r := range k.Text {
			parts = append(parts, strconv.Itoa(int(r)))
		}
		textField = strings.Join(parts, ":")
	}
	if modField != 1 || event != 1 || textField != "" {
		b.WriteByte(';')
		if modField != 1 || event != 1 {
			b.WriteString(strconv.Itoa(modField))
			if event != 1 {
				b.WriteByte(':')
				b.WriteString(strconv.Itoa(event))
			}
		}
		if textField != "" {
			b.WriteByte(';')
			b.WriteString(textField)
		}
	}
	b.WriteByte('u')
	return b.String(), true
}

func kittyFuncSeq(fk funcKey, modField, event int) string {
	mod := strconv.Itoa(modField)
	if event != 1 {
		mod += ":" + strconv.Itoa(event)
	}
	if fk.num != 0 {
		return "\x1b[" + strconv.Itoa(fk.num) + ";" + mod + "~"
	}
	return "\x1b[1;" + mod + string(fk.final)
}
