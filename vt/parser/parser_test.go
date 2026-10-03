// SPDX-License-Identifier: MPL-2.0

package parser

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestSequences(t *testing.T) {
	big := func(n int) string { return strings.Repeat("a", n) }
	tests := []struct {
		name string
		in   string
		want []string
	}{
		// ground / C0
		{"ascii", "hi", []string{`P "h"`, `P "i"`}},
		{"c0 execute", "a\a\b\t\n\v\f\r\x0e\x0fb", []string{`P "a"`, "X 07", "X 08", "X 09", "X 0a", "X 0b", "X 0c", "X 0d", "X 0e", "X 0f", `P "b"`}},
		{"nul and del", "a\x00\x7fb", []string{`P "a"`, "X 00", `P "b"`}},
		{"c0 inside csi executes", "\x1b[1\n;2H", []string{"X 0a", `CSI '\x00' [1;2] "" H`}},
		{"c0 inside esc executes", "\x1b\n7", []string{"X 0a", `ESC "" 7`}},
		{"c0 inside intermediates", "\x1b(\n0", []string{"X 0a", `ESC "(" 0`}},

		// ESC
		{"esc simple", "\x1b7", []string{`ESC "" 7`}},
		{"esc intermediate", "\x1b(0", []string{`ESC "(" 0`}},
		{"esc two intermediates", "\x1b#!8", []string{`ESC "#!" 8`}},
		{"esc 7-bit c1", "\x1bM\x1bD\x1bE\x1bH", []string{`ESC "" M`, `ESC "" D`, `ESC "" E`, `ESC "" H`}},
		{"esc backslash alone", "\x1b\\", []string{`ESC "" \`}},
		{"esc restarts esc", "\x1b\x1b7", []string{`ESC "" 7`}},
		{"esc del ignored", "\x1b\x7f7", []string{`ESC "" 7`}},
		{"esc overflow intermediates", "\x1b !\"#$7a", []string{`P "a"`}},
		{"esc then high byte aborts", "\x1b\xc3\xa9", []string{`P "é"`}},
		{"esc inter then high byte aborts", "\x1b(\xc3\xa9", []string{`P "é"`}},

		// CSI
		{"csi no params", "\x1b[H", []string{`CSI '\x00' [] "" H`}},
		{"csi params", "\x1b[1;2H", []string{`CSI '\x00' [1;2] "" H`}},
		{"csi default first", "\x1b[;5H", []string{`CSI '\x00' [_;5] "" H`}},
		{"csi default last", "\x1b[5;H", []string{`CSI '\x00' [5;_] "" H`}},
		{"csi only separators", "\x1b[;;m", []string{`CSI '\x00' [_;_;_] "" m`}},
		{"csi zero", "\x1b[0m", []string{`CSI '\x00' [0] "" m`}},
		{"csi private ?", "\x1b[?1049h", []string{`CSI '?' [1049] "" h`}},
		{"csi private >", "\x1b[>0c", []string{`CSI '>' [0] "" c`}},
		{"csi private <", "\x1b[<1u", []string{`CSI '<' [1] "" u`}},
		{"csi private =", "\x1b[=1c", []string{`CSI '=' [1] "" c`}},
		{"csi private no params", "\x1b[?h", []string{`CSI '?' [] "" h`}},
		{"csi intermediate", "\x1b[0 q", []string{`CSI '\x00' [0] " " q`}},
		{"csi intermediate only", "\x1b[ q", []string{`CSI '\x00' [] " " q`}},
		{"csi two intermediates", "\x1b[1$ p", []string{`CSI '\x00' [1] "$ " p`}},
		{"csi private and intermediate", "\x1b[?1$p", []string{`CSI '?' [1] "$" p`}},
		{"csi param after intermediate ignored", "\x1b[1 2mX", []string{`P "X"`}},
		{"csi prefix after digit ignored", "\x1b[1?hX", []string{`P "X"`}},
		{"csi prefix twice ignored", "\x1b[??hX", []string{`P "X"`}},
		{"csi ignore then good", "\x1b[1?h\x1b[2J", []string{`CSI '\x00' [2] "" J`}},
		{"csi intermediate overflow ignored", "\x1b[ !\"#$mZ", []string{`P "Z"`}},
		{"csi value saturates", "\x1b[999999999999999m", []string{`CSI '\x00' [65535] "" m`}},
		{"csi value exactly max", "\x1b[65535m", []string{`CSI '\x00' [65535] "" m`}},
		{"csi max params ok", "\x1b[" + strings.Repeat("1;", MaxParams-1) + "1m",
			[]string{fmt.Sprintf(`CSI '\x00' [%s] "" m`, strings.TrimSuffix(strings.Repeat("1;", MaxParams), ";"))}},
		{"csi too many params ignored", "\x1b[" + strings.Repeat("1;", MaxParams) + "1mZ", []string{`P "Z"`}},
		{"csi del ignored", "\x1b[1\x7f2m", []string{`CSI '\x00' [12] "" m`}},
		{"csi high byte aborts", "\x1b[1\xc3\xa9", []string{`P "é"`}},
		{"csi esc restarts", "\x1b[1\x1b[2m", []string{`CSI '\x00' [2] "" m`}},
		{"csi esc to esc", "\x1b[1\x1b7", []string{`ESC "" 7`}},
		{"csi final range low", "\x1b[@", []string{`CSI '\x00' [] "" @`}},
		{"csi final range high", "\x1b[~", []string{`CSI '\x00' [] "" ~`}},

		// sub-parameters
		{"sgr colon rgb", "\x1b[38:2::1:2:3m", []string{`CSI '\x00' [38:2:_:1:2:3] "" m`}},
		{"sgr colon rgb colorspace", "\x1b[38:2:0:1:2:3m", []string{`CSI '\x00' [38:2:0:1:2:3] "" m`}},
		{"sgr semicolon rgb", "\x1b[38;2;1;2;3m", []string{`CSI '\x00' [38;2;1;2;3] "" m`}},
		{"sgr colon 256", "\x1b[48:5:200m", []string{`CSI '\x00' [48:5:200] "" m`}},
		{"sgr underline style", "\x1b[4:3m", []string{`CSI '\x00' [4:3] "" m`}},
		{"sub trailing colon", "\x1b[1:m", []string{`CSI '\x00' [1:_] "" m`}},
		{"sub leading colon", "\x1b[:1m", []string{`CSI '\x00' [_:1] "" m`}},
		{"sub mixed with semicolons", "\x1b[1;4:3;38:5:9m", []string{`CSI '\x00' [1;4:3;38:5:9] "" m`}},
		{"sub max ok", "\x1b[" + strings.Repeat("1:", MaxSubParams-1) + "1m",
			[]string{fmt.Sprintf(`CSI '\x00' [%s] "" m`, strings.TrimSuffix(strings.Repeat("1:", MaxSubParams), ":"))}},
		{"sub too many ignored", "\x1b[" + strings.Repeat("1:", MaxSubParams) + "1mZ", []string{`P "Z"`}},

		// CAN / SUB
		{"can aborts csi", "\x1b[1\x18m", []string{"X 18", `P "m"`}},
		{"sub aborts csi", "\x1b[1\x1am", []string{"X 1a", `P "m"`}},
		{"can aborts esc", "\x1b\x187", []string{"X 18", `P "7"`}},
		{"can aborts osc", "\x1b]0;t\x18x", []string{"X 18", `P "x"`}},
		{"sub aborts osc", "\x1b]0;t\x1ax", []string{"X 1a", `P "x"`}},
		{"can aborts dcs", "\x1bP1qdata\x18x", []string{"X 18", `P "x"`}},
		{"can aborts apc", "\x1b_Gabc\x18x", []string{"X 18", `P "x"`}},
		{"can aborts sos", "\x1bXjunk\x18x", []string{"X 18", `P "x"`}},
		{"can after text", "a\x18b", []string{`P "a"`, "X 18", `P "b"`}},
		{"can aborts utf8", "\xe2\x82\x18", []string{`P "�"`, "X 18"}},

		// OSC
		{"osc bel", "\x1b]0;title\a", []string{`OSC "0;title"`}},
		{"osc st", "\x1b]0;title\x1b\\", []string{`OSC "0;title"`}},
		{"osc empty", "\x1b]\a", []string{`OSC ""`}},
		{"osc utf8 payload", "\x1b]2;héllo\a", []string{`OSC "2;héllo"`}},
		{"osc ignores c0", "\x1b]0;a\n\tb\a", []string{`OSC "0;ab"`}},
		{"osc esc non st starts escape", "\x1b]0;t\x1b[1m", []string{`OSC "0;t"`, `CSI '\x00' [1] "" m`}},
		{"osc esc esc", "\x1b]0;t\x1b\x1b7", []string{`OSC "0;t"`, `ESC "" 7`}},
		{"osc then text", "\x1b]0;t\ahi", []string{`OSC "0;t"`, `P "h"`, `P "i"`}},
		{"osc st does not dispatch backslash", "\x1b]0;t\x1b\\\\", []string{`OSC "0;t"`, `P "\\"`}},
		{"osc bel in st mode is data-free", "\x1b]0;a\x1b\\\a", []string{`OSC "0;a"`, "X 07"}},
		{"osc max ok", "\x1b]" + big(MaxOSCLen) + "\a", []string{fmt.Sprintf("OSC %q", big(MaxOSCLen))}},
		{"osc oversize dropped", "\x1b]" + big(MaxOSCLen+1) + "\ax", []string{`P "x"`}},
		{"osc oversize st dropped", "\x1b]" + big(MaxOSCLen+1) + "\x1b\\x", []string{`P "x"`}},

		// DCS
		{"dcs basic", "\x1bP1$qdata\x1b\\", []string{`DCS '\x00' [1] "$" q "data"`}},
		{"dcs no params", "\x1bPqabc\x1b\\", []string{`DCS '\x00' [] "" q "abc"`}},
		{"dcs private", "\x1bP>|abc\x1b\\", []string{`DCS '>' [] "" | "abc"`}},
		{"dcs params sub", "\x1bP1;2:3q\x1b\\", []string{`DCS '\x00' [1;2:3] "" q ""`}},
		{"dcs c0 in data kept", "\x1bPqa\nb\x1b\\", []string{"DCS '\\x00' [] \"\" q \"a\\nb\""}},
		{"dcs bel is data", "\x1bPqa\ab\x1b\\", []string{"DCS '\\x00' [] \"\" q \"a\\ab\""}},
		{"dcs utf8 data", "\x1bPqé\x1b\\", []string{`DCS '\x00' [] "" q "é"`}},
		{"dcs bad order ignored", "\x1bP1?q data\x1b\\x", []string{`P "x"`}},
		{"dcs param after inter ignored", "\x1bP $1q data\x1b\\x", []string{`P "x"`}},
		{"dcs esc restarts", "\x1bPqab\x1b[1m", []string{`DCS '\x00' [] "" q "ab"`, `CSI '\x00' [1] "" m`}},
		{"dcs too many params ignored", "\x1bP" + strings.Repeat("1;", MaxParams) + "1qdata\x1b\\x", []string{`P "x"`}},
		{"dcs max ok", "\x1bPq" + big(MaxDCSLen) + "\x1b\\", []string{fmt.Sprintf(`DCS '\x00' [] "" q %q`, big(MaxDCSLen))}},
		{"dcs oversize dropped", "\x1bPq" + big(MaxDCSLen+1) + "\x1b\\x", []string{`P "x"`}},

		// APC
		{"apc basic", "\x1b_Gi=1;AAAA\x1b\\", []string{`APC "Gi=1;AAAA"`}},
		{"apc bel is not terminator", "\x1b_Ga\ab\x1b\\", []string{`APC "Gab"`}},
		{"apc empty", "\x1b_\x1b\\", []string{`APC ""`}},
		{"apc max ok", "\x1b_G" + big(MaxAPCLen-1) + "\x1b\\", []string{fmt.Sprintf("APC %q", "G"+big(MaxAPCLen-1))}},
		{"apc oversize dropped", "\x1b_G" + big(MaxAPCLen) + "\x1b\\x", []string{`P "x"`}},
		{"apc esc restarts", "\x1b_Gab\x1b7", []string{`APC "Gab"`, `ESC "" 7`}},

		// SOS / PM
		{"sos ignored", "\x1bXjunk\x1b\\a", []string{`P "a"`}},
		{"pm ignored", "\x1b^junk\x1b\\a", []string{`P "a"`}},
		{"pm bel not terminator", "\x1b^ju\ank\x1b\\a", []string{`P "a"`}},
		{"sos esc restarts", "\x1bXjunk\x1b[1m", []string{`CSI '\x00' [1] "" m`}},

		// UTF-8
		{"utf8 2 byte", "é", []string{`P "é"`}},
		{"utf8 3 byte", "€", []string{`P "€"`}},
		{"utf8 4 byte", "😀", []string{`P "😀"`}},
		{"c1 range not control", "\x85", []string{`P "�"`}},
		{"c1 9b not csi", "\x9b1m", []string{`P "�"`, `P "1"`, `P "m"`}},
		{"c1 90 not dcs", "\x90q", []string{`P "�"`, `P "q"`}},
		{"lone continuation", "\x80\xbf", []string{`P "�"`, `P "�"`}},
		{"overlong c0", "\xc0\x80", []string{`P "�"`, `P "�"`}},
		{"overlong c1", "\xc1\xbf", []string{`P "�"`, `P "�"`}},
		{"overlong e0", "\xe0\x80\x80", []string{`P "�"`, `P "�"`, `P "�"`}},
		{"surrogate", "\xed\xa0\x80", []string{`P "�"`, `P "�"`, `P "�"`}},
		{"above max", "\xf4\x90\x80\x80", []string{`P "�"`, `P "�"`, `P "�"`, `P "�"`}},
		{"f5 invalid", "\xf5a", []string{`P "�"`, `P "a"`}},
		{"ff invalid", "\xffa", []string{`P "�"`, `P "a"`}},
		{"truncated then ascii", "\xe2\x82a", []string{`P "�"`, `P "a"`}},
		{"truncated then lead", "\xe2\x82\xc3\xa9", []string{`P "�"`, `P "é"`}},
		{"truncated then control", "\xe2\x82\n", []string{`P "�"`, "X 0a"}},
		{"truncated then esc", "\xe2\x82\x1b7", []string{`P "�"`, `ESC "" 7`}},
		{"max rune", "\U0010FFFF", []string{`P "\U0010ffff"`}},

		// graphemes
		{"combining mark", "éx", []string{"P \"é\"", `P "x"`}},
		{"several combining", "à́̂", []string{"P \"à́̂\""}},
		{"zwj family", "\U0001F468\u200d\U0001F469\u200d\U0001F467\u200d\U0001F466!", []string{`P "👨\u200d👩\u200d👧\u200d👦"`, `P "!"`}},
		{"zwj not joining text", "a\u200db", []string{`P "a\u200d"`, `P "b"`}},
		{"skin tone", "👍🏽", []string{"P \"👍🏽\""}},
		{"flag", "🇺🇸🇫🇷", []string{"P \"🇺🇸\"", "P \"🇫🇷\""}},
		{"three regional indicators", "🇺🇸🇫", []string{"P \"🇺🇸\"", "P \"🇫\""}},
		{"hangul syllable jamo", "각x", []string{"P \"각\"", `P "x"`}},
		{"hangul precomposed lv", "각", []string{"P \"각\""}},
		{"variation selector", "❤️", []string{"P \"❤️\""}},
		{"combining after ascii run", "abć", []string{`P "a"`, `P "b"`, "P \"ć\""}},
		{"cluster before control", "é\n", []string{"P \"é\"", "X 0a"}},
		{"cluster before escape", "é\x1b[m", []string{"P \"é\"", `CSI '\x00' [] "" m`}},
		{"mark after control is own cluster", "\ń", []string{"X 0a", "P \"́\""}},
		{"invalid between marks", "e\xff́", []string{`P "e"`, "P \"�́\""}},
		{"prepend", "\u0600a", []string{`P "\u0600a"`}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := run(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				if len(got) > 4 || len(tc.want) > 4 {
					t.Fatalf("event mismatch: got %d events, want %d", len(got), len(tc.want))
				}
				t.Fatalf("got  %v\nwant %v", got, tc.want)
			}
			if bw := runBytewise(tc.in); !reflect.DeepEqual(bw, got) {
				t.Fatalf("bytewise differs: %v", bw)
			}
		})
	}
}

func TestPendingClusterHeldUntilBoundary(t *testing.T) {
	r := &recorder{}
	p := New(r)
	p.Write([]byte("ab"))
	if want := []string{`P "a"`}; !reflect.DeepEqual(r.ev, want) {
		t.Fatalf("after ab: %v", r.ev)
	}
	p.Write([]byte("́"))
	if len(r.ev) != 1 {
		t.Fatalf("combining mark must extend pending cluster: %v", r.ev)
	}
	p.Flush()
	if want := []string{`P "a"`, "P \"b́\""}; !reflect.DeepEqual(r.ev, want) {
		t.Fatalf("after flush: %v", r.ev)
	}
	p.Flush()
	if len(r.ev) != 2 {
		t.Fatalf("second flush must be a no-op: %v", r.ev)
	}
}

func TestFlagPendingAcrossWrites(t *testing.T) {
	got := run("\U0001F1FA", "\U0001F1F8", "\U0001F1EB")
	want := []string{"P \"\U0001F1FA\U0001F1F8\"", "P \"\U0001F1EB\""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestSplitUTF8AcrossWrites(t *testing.T) {
	got := run("\xf0\x9f", "\x98", "\x80")
	if want := []string{`P "😀"`}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestOversizeThenRecovers(t *testing.T) {
	big := strings.Repeat("x", MaxAPCLen+10)
	got := run("\x1b_G"+big+"\x1b\\", "\x1b_Gok\x1b\\")
	if want := []string{`APC "Gok"`}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestBufferReusedAcrossSequences(t *testing.T) {
	got := run("\x1b]0;one\a\x1b]0;two\a\x1bPqA\x1b\\\x1bPqB\x1b\\")
	want := []string{`OSC "0;one"`, `OSC "0;two"`, `DCS '\x00' [] "" q "A"`, `DCS '\x00' [] "" q "B"`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestParamsGet(t *testing.T) {
	var got Params
	p := New(paramsCapture{&got})
	p.Write([]byte("\x1b[;5;m"))
	if got.Len() != 3 || got.Get(0, 7) != 7 || got.Get(1, 7) != 5 || got.Get(2, 9) != 9 || got.Get(3, 4) != 4 {
		t.Fatalf("params %v", got.Values)
	}
}

type paramsCapture struct{ out *Params }

func (paramsCapture) Print(string)                           {}
func (paramsCapture) Execute(byte)                           {}
func (paramsCapture) ESC([]byte, byte)                       {}
func (paramsCapture) OSC([]byte)                             {}
func (paramsCapture) DCS(byte, Params, []byte, byte, []byte) {}
func (paramsCapture) APC([]byte)                             {}
func (c paramsCapture) CSI(_ byte, p Params, _ []byte, _ byte) {
	*c.out = Params{Values: append([][]int(nil), p.Values...)}
}

func TestReset(t *testing.T) {
	r := &recorder{}
	p := New(r)
	p.Write([]byte("\x1b]0;abc"))
	p.Reset()
	p.Write([]byte("x"))
	p.Flush()
	if want := []string{`P "x"`}; !reflect.DeepEqual(r.ev, want) {
		t.Fatalf("got %v", r.ev)
	}
}
