// SPDX-License-Identifier: MPL-2.0

package conformance

import "testing"

// Terminal replies. Reference: ctlseqs "DSR", "DA", "DECRQM", "DECRQSS",
// "Window ops 18/t", "kitty keyboard protocol" progressive enhancement.

var reportCases = []specCase{
	{name: "DSR 5 operating status", in: "\x1b[5n", reply: exact("\x1b[0n")},
	{name: "DSR 6 at home", in: "\x1b[6n", reply: exact("\x1b[1;1R")},
	{name: "DSR 6 cursor position", in: "\x1b[3;4H\x1b[6n", reply: exact("\x1b[3;4R")},
	{name: "DSR 6 is margin relative under DECOM", in: "\x1b[2;4r\x1b[?6h\x1b[2;3H\x1b[6n", reply: exact("\x1b[2;3R")},
	{name: "DSR 6 reports the parked column when a wrap is pending", in: "\x1b[1;10HA\x1b[6n", reply: exact("\x1b[1;10R")},
	{name: "DSR ?6 extended cursor position", in: "\x1b[3;4H\x1b[?6n", reply: `^\x1b\[\?3;4(;\d+)?R$`},
	{name: "DA1", in: "\x1b[c", reply: `^\x1b\[\?\d+(;\d+)*c$`},
	{name: "DA1 explicit zero", in: "\x1b[0c", reply: `^\x1b\[\?\d+(;\d+)*c$`},
	{name: "DECID", in: "\x1bZ", reply: `^\x1b\[\?\d+(;\d+)*c$`},
	{name: "DA2", in: "\x1b[>c", reply: `^\x1b\[>\d+;\d+;\d+c$`},
	{name: "XTVERSION", in: "\x1b[>q", reply: `^\x1bP>\|[^\x1b]+\x1b\\$`},
	{name: "window size in characters", in: "\x1b[18t", reply: exact("\x1b[8;5;10t")},

	{name: "DECRQM IRM reset", in: "\x1b[4$p", reply: exact("\x1b[4;2$y")},
	{name: "DECRQM IRM set", in: "\x1b[4h\x1b[4$p", reply: exact("\x1b[4;1$y")},
	{name: "DECRQM LNM", in: "\x1b[20$p\x1b[20h\x1b[20$p", reply: exact("\x1b[20;2$y\x1b[20;1$y")},
	{name: "DECRQM DECAWM default set", in: "\x1b[?7$p", reply: exact("\x1b[?7;1$y")},
	{name: "DECRQM DECAWM reset", in: "\x1b[?7l\x1b[?7$p", reply: exact("\x1b[?7;2$y")},
	{name: "DECRQM DECOM", in: "\x1b[?6$p\x1b[?6h\x1b[?6$p", reply: exact("\x1b[?6;2$y\x1b[?6;1$y")},
	{name: "DECRQM DECTCEM default set", in: "\x1b[?25$p", reply: exact("\x1b[?25;1$y")},
	{name: "DECRQM DECCKM", in: "\x1b[?1$p\x1b[?1h\x1b[?1$p", reply: exact("\x1b[?1;2$y\x1b[?1;1$y")},
	{name: "DECRQM 1049", in: "\x1b[?1049$p\x1b[?1049h\x1b[?1049$p", reply: exact("\x1b[?1049;2$y\x1b[?1049;1$y")},
	{name: "DECRQM bracketed paste", in: "\x1b[?2004$p\x1b[?2004h\x1b[?2004$p", reply: exact("\x1b[?2004;2$y\x1b[?2004;1$y")},
	{name: "DECRQM synchronized output", in: "\x1b[?2026$p\x1b[?2026h\x1b[?2026$p", reply: exact("\x1b[?2026;2$y\x1b[?2026;1$y")},
	{name: "DECRQM focus events", in: "\x1b[?1004$p\x1b[?1004h\x1b[?1004$p", reply: exact("\x1b[?1004;2$y\x1b[?1004;1$y")},
	{name: "DECRQM SGR mouse", in: "\x1b[?1006$p\x1b[?1006h\x1b[?1006$p", reply: exact("\x1b[?1006;2$y\x1b[?1006;1$y")},
	{name: "DECRQM unknown private mode", in: "\x1b[?9999$p", reply: exact("\x1b[?9999;0$y")},

	{name: "DECRQSS SGR", in: "\x1b[1;31m\x1bP$qm\x1b\\", reply: `^\x1bP1\$r0?;?1;31m\x1b\\$`},
	{name: "DECRQSS SGR default", in: "\x1bP$qm\x1b\\", reply: `^\x1bP1\$r0?m\x1b\\$`},
	{name: "DECRQSS DECSTBM", in: "\x1b[2;4r\x1bP$qr\x1b\\", reply: exact("\x1bP1$r2;4r\x1b\\")},
	{name: "DECRQSS DECSCUSR", in: "\x1b[5 q\x1bP$q q\x1b\\", reply: exact("\x1bP1$r5 q\x1b\\")},
	{name: "DECRQSS unknown", in: "\x1bP$qzz\x1b\\", reply: exact("\x1bP0$r\x1b\\")},

	kittyCase("kitty query default", "\x1b[?u", "\x1b[?0u", 0),
	kittyCase("kitty push then query", "\x1b[>1u\x1b[?u", "\x1b[?1u", 1),
	kittyCase("kitty set replaces flags", "\x1b[>1u\x1b[=3;1u\x1b[?u", "\x1b[?3u", 3),
	kittyCase("kitty set or-in", "\x1b[>1u\x1b[=6;2u\x1b[?u", "\x1b[?7u", 7),
	kittyCase("kitty set and-not", "\x1b[>7u\x1b[=2;3u\x1b[?u", "\x1b[?5u", 5),
	kittyCase("kitty pop restores the previous entry", "\x1b[>1u\x1b[>3u\x1b[<u\x1b[?u", "\x1b[?1u", 1),
	kittyCase("kitty pop count", "\x1b[>1u\x1b[>2u\x1b[>4u\x1b[<2u\x1b[?u", "\x1b[?1u", 1),
	kittyCase("kitty pop beyond the stack", "\x1b[>1u\x1b[<9u\x1b[?u", "\x1b[?0u", 0),
}

func kittyCase(name, in, reply string, flags int) specCase {
	return specCase{
		name: name, in: in, reply: exact(reply),
		check: func(t *testing.T, s *sut) {
			if got := s.term.Modes().KittyKeyboardFlags; got != flags {
				t.Errorf("KittyKeyboardFlags want %d got %d", flags, got)
			}
		},
	}
}
