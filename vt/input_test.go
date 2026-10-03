// SPDX-License-Identifier: MPL-2.0

package vt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wippyai/tty/input"
)

func press(code rune, mod input.Mod, text string) input.KeyPressEvent {
	return input.KeyPressEvent{Code: code, Mod: mod, Text: text}
}

func encode(t *testing.T, setup string, ev input.Event) string {
	h := newHarness(t, 20, 4)
	h.write(setup)
	h.take()
	h.term.SendKey(ev)
	return h.take()
}

func TestLegacyKeys(t *testing.T) {
	cases := []struct {
		name  string
		setup string
		ev    input.Event
		want  string
	}{
		{"letter", "", press('a', 0, "a"), "a"},
		{"shifted", "", input.KeyPressEvent{Code: 'a', ShiftedCode: 'A', Mod: input.ModShift, Text: "A"}, "A"},
		{"unicode", "", press('x', 0, "é"), "é"},
		{"ctrl-a", "", press('a', input.ModCtrl, ""), "\x01"},
		{"ctrl-z", "", press('z', input.ModCtrl, ""), "\x1a"},
		{"ctrl-space", "", press(' ', input.ModCtrl, ""), "\x00"},
		{"ctrl-[", "", press('[', input.ModCtrl, ""), "\x1b"},
		{"ctrl-\\", "", press('\\', input.ModCtrl, ""), "\x1c"},
		{"ctrl-?", "", press('?', input.ModCtrl, ""), "\x7f"},
		{"ctrl-2", "", press('2', input.ModCtrl, ""), "\x00"},
		{"ctrl-5", "", press('5', input.ModCtrl, ""), "\x1d"},
		{"alt-a", "", press('a', input.ModAlt, ""), "\x1ba"},
		{"alt-ctrl-a", "", press('a', input.ModAlt|input.ModCtrl, ""), "\x1b\x01"},
		{"enter", "", press(input.KeyEnter, 0, ""), "\r"},
		{"enter lnm", "\x1b[20h", press(input.KeyEnter, 0, ""), "\r\n"},
		{"alt-enter", "", press(input.KeyEnter, input.ModAlt, ""), "\x1b\r"},
		{"tab", "", press(input.KeyTab, 0, ""), "\t"},
		{"shift-tab", "", press(input.KeyTab, input.ModShift, ""), "\x1b[Z"},
		{"backspace", "", press(input.KeyBackspace, 0, ""), "\x7f"},
		{"ctrl-backspace", "", press(input.KeyBackspace, input.ModCtrl, ""), "\x08"},
		{"escape", "", press(input.KeyEscape, 0, ""), "\x1b"},
		{"alt-escape", "", press(input.KeyEscape, input.ModAlt, ""), "\x1b\x1b"},
		{"up", "", press(input.KeyUp, 0, ""), "\x1b[A"},
		{"up DECCKM", "\x1b[?1h", press(input.KeyUp, 0, ""), "\x1bOA"},
		{"ctrl-up DECCKM", "\x1b[?1h", press(input.KeyUp, input.ModCtrl, ""), "\x1b[1;5A"},
		{"shift-alt-left", "", press(input.KeyLeft, input.ModShift|input.ModAlt, ""), "\x1b[1;4D"},
		{"home", "", press(input.KeyHome, 0, ""), "\x1b[H"},
		{"end DECCKM", "\x1b[?1h", press(input.KeyEnd, 0, ""), "\x1bOF"},
		{"insert", "", press(input.KeyInsert, 0, ""), "\x1b[2~"},
		{"delete", "", press(input.KeyDelete, 0, ""), "\x1b[3~"},
		{"ctrl-delete", "", press(input.KeyDelete, input.ModCtrl, ""), "\x1b[3;5~"},
		{"pgup", "", press(input.KeyPgUp, 0, ""), "\x1b[5~"},
		{"pgdn", "", press(input.KeyPgDown, 0, ""), "\x1b[6~"},
		{"f1", "", press(input.KeyF1, 0, ""), "\x1bOP"},
		{"f1 DECCKM", "\x1b[?1h", press(input.KeyF1, 0, ""), "\x1bOP"},
		{"shift-f1", "", press(input.KeyF1, input.ModShift, ""), "\x1b[1;2P"},
		{"f4", "", press(input.KeyF4, 0, ""), "\x1bOS"},
		{"f5", "", press(input.KeyF5, 0, ""), "\x1b[15~"},
		{"f12", "", press(input.KeyF12, 0, ""), "\x1b[24~"},
		{"ctrl-f5", "", press(input.KeyF5, input.ModCtrl, ""), "\x1b[15;5~"},
		{"f20", "", press(input.KeyF20, 0, ""), "\x1b[34~"},
		{"f30 unsupported", "", press(input.KeyF30, 0, ""), ""},
		{"kp5 numeric", "", press(input.KeyKp5, 0, ""), "5"},
		{"kp5 application", "\x1b=", press(input.KeyKp5, 0, ""), "\x1bOu"},
		{"kpenter application", "\x1b=", press(input.KeyKpEnter, 0, ""), "\x1bOM"},
		{"kpenter numeric", "", press(input.KeyKpEnter, 0, ""), "\r"},
		{"kpplus application shift", "\x1b=", press(input.KeyKpPlus, input.ModShift, ""), "\x1bO2k"},
		{"kpplus after DECKPNM", "\x1b=\x1b>", press(input.KeyKpPlus, 0, ""), "+"},
		{"release ignored", "", input.KeyReleaseEvent{Code: 'a', Text: "a"}, ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, encode(t, c.setup, c.ev), c.name)
	}
}

func TestKittyKeyboard(t *testing.T) {
	rel := func(code rune, mod input.Mod) input.KeyReleaseEvent {
		return input.KeyReleaseEvent{Code: code, Mod: mod}
	}
	cases := []struct {
		name  string
		flags string
		ev    input.Event
		want  string
	}{
		{"1 plain text legacy", "1", press('a', 0, "a"), "a"},
		{"1 shift text legacy", "1", input.KeyPressEvent{Code: 'a', ShiftedCode: 'A', Mod: input.ModShift, Text: "A"}, "A"},
		{"1 ctrl-a", "1", press('a', input.ModCtrl, ""), "\x1b[97;5u"},
		{"1 alt-a", "1", press('a', input.ModAlt, ""), "\x1b[97;3u"},
		{"1 ctrl-shift-a", "1", press('a', input.ModCtrl|input.ModShift, ""), "\x1b[97;6u"},
		{"1 super-a", "1", press('a', input.ModSuper, ""), "\x1b[97;9u"},
		{"1 caps lock text legacy", "1", press('a', input.ModCapsLock, "A"), "A"},
		{"1 ctrl-caps", "1", press('a', input.ModCtrl|input.ModCapsLock, ""), "\x1b[97;69u"},
		{"1 escape", "1", press(input.KeyEscape, 0, ""), "\x1b[27u"},
		{"1 enter legacy", "1", press(input.KeyEnter, 0, ""), "\r"},
		{"1 shift-enter", "1", press(input.KeyEnter, input.ModShift, ""), "\x1b[13;2u"},
		{"1 ctrl-enter", "1", press(input.KeyEnter, input.ModCtrl, ""), "\x1b[13;5u"},
		{"1 tab legacy", "1", press(input.KeyTab, 0, ""), "\t"},
		{"1 shift-tab", "1", press(input.KeyTab, input.ModShift, ""), "\x1b[9;2u"},
		{"1 backspace legacy", "1", press(input.KeyBackspace, 0, ""), "\x7f"},
		{"1 ctrl-backspace", "1", press(input.KeyBackspace, input.ModCtrl, ""), "\x1b[127;5u"},
		{"1 up", "1", press(input.KeyUp, 0, ""), "\x1b[A"},
		{"1 ctrl-up", "1", press(input.KeyUp, input.ModCtrl, ""), "\x1b[1;5A"},
		{"1 f1", "1", press(input.KeyF1, 0, ""), "\x1bOP"},
		{"1 shift-f3", "1", press(input.KeyF3, input.ModShift, ""), "\x1b[13;2~"},
		{"1 f5", "1", press(input.KeyF5, 0, ""), "\x1b[15~"},
		{"1 delete", "1", press(input.KeyDelete, 0, ""), "\x1b[3~"},
		{"1 f13 plain", "1", press(input.KeyF13, 0, ""), "\x1b[25~"},
		{"1 f21", "1", press(input.KeyF21, 0, ""), "\x1b[57384u"},
		{"1 shift key not reported", "1", press(input.KeyLeftShift, input.ModShift, ""), ""},
		{"1 kp1 plain", "1", press(input.KeyKp1, 0, ""), "1"},
		{"1 ctrl-kp1", "1", press(input.KeyKp1, input.ModCtrl, ""), "\x1b[57400;5u"},
		{"1 release not reported", "1", rel('a', input.ModCtrl), ""},

		{"3 ctrl-a press", "3", press('a', input.ModCtrl, ""), "\x1b[97;5u"},
		{"3 ctrl-a repeat", "3", input.KeyPressEvent{Code: 'a', Mod: input.ModCtrl, IsRepeat: true}, "\x1b[97;5:2u"},
		{"3 ctrl-a release", "3", rel('a', input.ModCtrl), "\x1b[97;5:3u"},
		{"3 text release not reported", "3", input.KeyReleaseEvent{Code: 'a', Text: "a"}, ""},
		{"3 enter release not reported", "3", rel(input.KeyEnter, 0), ""},
		{"3 up release", "3", rel(input.KeyUp, 0), "\x1b[1;1:3A"},
		{"3 f5 release", "3", rel(input.KeyF5, input.ModShift), "\x1b[15;2:3~"},
		{"3 escape release", "3", rel(input.KeyEscape, 0), "\x1b[27;1:3u"},
		{"3 up repeat", "3", input.KeyPressEvent{Code: input.KeyUp, IsRepeat: true}, "\x1b[1;1:2A"},

		{"5 alternates", "5", input.KeyPressEvent{Code: 'a', ShiftedCode: 'A', BaseCode: 'q', Mod: input.ModCtrl | input.ModShift}, "\x1b[97:65:113;6u"},
		{"5 base only", "5", input.KeyPressEvent{Code: 'a', BaseCode: 'q', Mod: input.ModCtrl}, "\x1b[97::113;5u"},
		{"5 base equal", "5", input.KeyPressEvent{Code: 'a', BaseCode: 'a', Mod: input.ModCtrl}, "\x1b[97;5u"},

		{"8 plain a", "8", press('a', 0, "a"), "\x1b[97u"},
		{"8 shift a", "8", input.KeyPressEvent{Code: 'a', ShiftedCode: 'A', Mod: input.ModShift, Text: "A"}, "\x1b[97;2u"},
		{"8 enter", "8", press(input.KeyEnter, 0, ""), "\x1b[13u"},
		{"8 tab", "8", press(input.KeyTab, 0, ""), "\x1b[9u"},
		{"8 backspace", "8", press(input.KeyBackspace, 0, ""), "\x1b[127u"},
		{"8 escape", "8", press(input.KeyEscape, 0, ""), "\x1b[27u"},
		{"8 left shift", "8", press(input.KeyLeftShift, input.ModShift, ""), "\x1b[57441;2u"},
		{"8 caps lock", "8", press(input.KeyCapsLock, input.ModCapsLock, ""), "\x1b[57358;65u"},
		{"8 kp1", "8", press(input.KeyKp1, 0, ""), "\x1b[57400u"},
		{"8 kp enter", "8", press(input.KeyKpEnter, 0, ""), "\x1b[57414u"},
		{"8 media play", "8", press(input.KeyMediaPlay, 0, ""), "\x1b[57428u"},
		{"8 space", "8", press(' ', 0, " "), "\x1b[32u"},
		{"8 up", "8", press(input.KeyUp, 0, ""), "\x1b[A"},
		{"8 text a no assoc text", "8", press('a', 0, "a"), "\x1b[97u"},

		{"10 enter release", "10", rel(input.KeyEnter, 0), "\x1b[13;1:3u"},
		{"10 a release", "10", rel('a', 0), "\x1b[97;1:3u"},
		{"10 left shift release", "10", rel(input.KeyLeftShift, 0), "\x1b[57441;1:3u"},

		{"24 text", "24", press('a', 0, "a"), "\x1b[97;;97u"},
		{"24 shifted text", "24", input.KeyPressEvent{Code: 'a', ShiftedCode: 'A', Mod: input.ModShift, Text: "A"}, "\x1b[97;2;65u"},
		{"24 multi text", "24", press('x', 0, "ab"), "\x1b[120;;97:98u"},
		{"24 release no text", "24", input.KeyReleaseEvent{Code: 'a', Text: "a"}, ""},
		{"26 release no text", "26", input.KeyReleaseEvent{Code: 'a', Text: "a"}, "\x1b[97;1:3u"},
		{"16 alone ignores text", "16", press('a', 0, "a"), "a"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, encode(t, "\x1b[="+c.flags+"u", c.ev), c.name)
	}
}

func TestKittyDECCKMUnmodifiedArrows(t *testing.T) {
	assert.Equal(t, "\x1bOA", encode(t, "\x1b[=1u\x1b[?1h", press(input.KeyUp, 0, "")))
}

func mouseEvent(kind string, b input.MouseButton, x, y int, mod input.Mod) input.Event {
	m := input.Mouse{X: x, Y: y, Button: b, Mod: mod}
	switch kind {
	case "click":
		return input.MouseClickEvent(m)
	case "release":
		return input.MouseReleaseEvent(m)
	case "motion":
		return input.MouseMotionEvent(m)
	}
	return input.MouseWheelEvent(m)
}

func TestMouseEncoding(t *testing.T) {
	cases := []struct {
		name  string
		setup string
		ev    input.Event
		want  string
	}{
		{"off", "", mouseEvent("click", input.MouseLeft, 0, 0, 0), ""},
		{"normal click", "\x1b[?1000h", mouseEvent("click", input.MouseLeft, 4, 2, 0), "\x1b[M %#"},
		{"normal right", "\x1b[?1000h", mouseEvent("click", input.MouseRight, 0, 0, 0), "\x1b[M\"!!"},
		{"normal release", "\x1b[?1000h", mouseEvent("release", input.MouseLeft, 0, 0, 0), "\x1b[M#!!"},
		{"normal mods", "\x1b[?1000h", mouseEvent("click", input.MouseLeft, 0, 0, input.ModShift|input.ModAlt|input.ModCtrl), "\x1b[M<!!"},
		{"normal wheel up", "\x1b[?1000h", mouseEvent("wheel", input.MouseWheelUp, 0, 0, 0), "\x1b[M`!!"},
		{"normal wheel down", "\x1b[?1000h", mouseEvent("wheel", input.MouseWheelDown, 0, 0, 0), "\x1b[Ma!!"},
		{"normal no motion", "\x1b[?1000h", mouseEvent("motion", input.MouseLeft, 1, 1, 0), ""},
		{"x10 click", "\x1b[?9h", mouseEvent("click", input.MouseLeft, 0, 0, input.ModCtrl), "\x1b[M !!"},
		{"x10 no release", "\x1b[?9h", mouseEvent("release", input.MouseLeft, 0, 0, 0), ""},
		{"button motion drag", "\x1b[?1002h", mouseEvent("motion", input.MouseLeft, 1, 0, 0), "\x1b[M@\"!"},
		{"button motion hover", "\x1b[?1002h", mouseEvent("motion", input.MouseNone, 1, 0, 0), ""},
		{"any motion hover", "\x1b[?1003h", mouseEvent("motion", input.MouseNone, 1, 0, 0), "\x1b[MC\"!"},
		{"default coord limit", "\x1b[?1000h", mouseEvent("click", input.MouseLeft, 223, 0, 0), ""},
		{"default coord max", "\x1b[?1000h", mouseEvent("click", input.MouseLeft, 222, 0, 0), "\x1b[M \xff!"},
		{"utf8 large", "\x1b[?1000h\x1b[?1005h", mouseEvent("click", input.MouseLeft, 299, 0, 0), "\x1b[M \u014c!"},
		{"sgr press", "\x1b[?1000h\x1b[?1006h", mouseEvent("click", input.MouseLeft, 9, 4, 0), "\x1b[<0;10;5M"},
		{"sgr release", "\x1b[?1000h\x1b[?1006h", mouseEvent("release", input.MouseRight, 9, 4, 0), "\x1b[<2;10;5m"},
		{"sgr large", "\x1b[?1000h\x1b[?1006h", mouseEvent("click", input.MouseMiddle, 299, 399, input.ModCtrl), "\x1b[<17;300;400M"},
		{"sgr drag", "\x1b[?1002h\x1b[?1006h", mouseEvent("motion", input.MouseLeft, 1, 1, 0), "\x1b[<32;2;2M"},
		{"sgr hover", "\x1b[?1003h\x1b[?1006h", mouseEvent("motion", input.MouseNone, 1, 1, 0), "\x1b[<35;2;2M"},
		{"sgr wheel left", "\x1b[?1000h\x1b[?1006h", mouseEvent("wheel", input.MouseWheelLeft, 0, 0, 0), "\x1b[<66;1;1M"},
		{"sgr backward", "\x1b[?1000h\x1b[?1006h", mouseEvent("click", input.MouseBackward, 0, 0, 0), "\x1b[<128;1;1M"},
		{"sgr pixels", "\x1b[?1000h\x1b[?1016h", mouseEvent("click", input.MouseLeft, 2, 1, 0), "\x1b[<0;21;25M"},
	}
	for _, c := range cases {
		h := newHarness(t, 20, 4)
		h.write(c.setup)
		sent := h.term.SendMouse(c.ev)
		assert.Equal(t, c.want, h.take(), c.name)
		assert.Equal(t, c.want != "", sent, c.name)
	}
}

func TestMouseMotionDedupe(t *testing.T) {
	h := newHarness(t, 20, 4)
	h.write("\x1b[?1003h\x1b[?1006h")
	assert.True(t, h.term.SendMouse(mouseEvent("motion", input.MouseNone, 1, 1, 0)))
	assert.False(t, h.term.SendMouse(mouseEvent("motion", input.MouseNone, 1, 1, 0)))
	assert.True(t, h.term.SendMouse(mouseEvent("motion", input.MouseNone, 2, 1, 0)))
}

func TestAlternateScroll(t *testing.T) {
	h := newHarness(t, 20, 4)
	h.write("\x1b[?1007h")
	assert.False(t, h.term.SendMouse(mouseEvent("wheel", input.MouseWheelUp, 0, 0, 0)))
	h.write("\x1b[?1049h")
	assert.True(t, h.term.SendMouse(mouseEvent("wheel", input.MouseWheelUp, 0, 0, 0)))
	assert.True(t, h.term.SendMouse(mouseEvent("wheel", input.MouseWheelDown, 0, 0, 0)))
	assert.Equal(t, "\x1b[A\x1b[B", h.take())
	h.write("\x1b[?1h")
	h.term.SendMouse(mouseEvent("wheel", input.MouseWheelUp, 0, 0, 0))
	assert.Equal(t, "\x1bOA", h.take())
	h.write("\x1b[?1000h")
	h.term.SendMouse(mouseEvent("wheel", input.MouseWheelUp, 0, 0, 0))
	assert.Equal(t, "\x1b[M`!!", h.take())
}

func TestPaste(t *testing.T) {
	h := newHarness(t, 20, 4)
	assert.True(t, h.term.SendPaste("a\nb\r\nc"))
	assert.Equal(t, "a\rb\rc", h.take())
	assert.False(t, h.term.SendPaste(""))

	h.write("\x1b[?2004h")
	h.term.SendPaste("hello\nworld")
	assert.Equal(t, "\x1b[200~hello\nworld\x1b[201~", h.take())
	h.term.SendPaste("x\x1b[201~rm -rf ~\x1b[201~y")
	assert.Equal(t, "\x1b[200~xrm -rf ~y\x1b[201~", h.take())
	h.term.SendPaste("x\x1b[20\x1b[201~1~y")
	assert.Equal(t, "\x1b[200~xy\x1b[201~", h.take())
}

func TestFocus(t *testing.T) {
	h := newHarness(t, 20, 4)
	assert.False(t, h.term.SendFocus(true))
	assert.Empty(t, h.take())
	h.write("\x1b[?1004h")
	h.term.SendFocus(true)
	h.term.SendFocus(false)
	assert.Equal(t, "\x1b[I\x1b[O", h.take())
}
