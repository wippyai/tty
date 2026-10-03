// SPDX-License-Identifier: MPL-2.0

package conformance

// Cursor movement: CUU CUD CUF CUB CNL CPL CHA CUP HVP VPA HPA VPR HPR with
// default, zero and oversized parameters, scroll-margin clamping and DECOM.
// Reference: ctlseqs "Functions using CSI", DECOM and DECSTBM entries.

var cursorCases = []specCase{
	{name: "CUU default", in: "\x1b[3;5H\x1b[AX", want: []string{"", "    X"}, cursor: at(5, 1)},
	{name: "CUU zero is one", in: "\x1b[3;5H\x1b[0AX", want: []string{"", "    X"}, cursor: at(5, 1)},
	{name: "CUU count", in: "\x1b[4;5H\x1b[2AX", want: []string{"", "    X"}, cursor: at(5, 1)},
	{name: "CUU clamps at top", in: "\x1b[2;5H\x1b[99AX", want: []string{"    X"}, cursor: at(5, 0)},
	{name: "CUU stops at top margin", in: "\x1b[2;4r\x1b[4;5H\x1b[99AX", want: []string{"", "    X"}, cursor: at(5, 1)},
	{name: "CUU above margin ignores margin", in: "\x1b[2;4r\x1b[1;5H\x1b[3AX", want: []string{"    X"}, cursor: at(5, 0)},
	{name: "CUU below margin stops at top margin", in: "\x1b[2;4r\x1b[5;5H\x1b[99AX", want: []string{"", "    X"}, cursor: at(5, 1)},

	{name: "CUD default", in: "\x1b[2;5H\x1b[BX", want: []string{"", "", "    X"}, cursor: at(5, 2)},
	{name: "CUD zero is one", in: "\x1b[2;5H\x1b[0BX", want: []string{"", "", "    X"}, cursor: at(5, 2)},
	{name: "CUD count", in: "\x1b[1;5H\x1b[3BX", want: []string{"", "", "", "    X"}, cursor: at(5, 3)},
	{name: "CUD clamps at bottom", in: "\x1b[2;5H\x1b[99BX", want: []string{"", "", "", "", "    X"}, cursor: at(5, 4)},
	{name: "CUD stops at bottom margin", in: "\x1b[2;4r\x1b[2;5H\x1b[99BX", want: []string{"", "", "", "    X"}, cursor: at(5, 3)},
	{name: "CUD above margin stops at bottom margin", in: "\x1b[2;4r\x1b[1;5H\x1b[99BX", want: []string{"", "", "", "    X"}, cursor: at(5, 3)},
	{name: "CUD below margin clamps at last row", in: "\x1b[2;4r\x1b[5;5H\x1b[99BX", want: []string{"", "", "", "", "    X"}, cursor: at(5, 4)},
	{name: "VPR moves down", in: "\x1b[2;5H\x1b[2eX", want: []string{"", "", "", "    X"}, cursor: at(5, 3)},

	{name: "CUF default", in: "\x1b[1;3H\x1b[CX", want: []string{"   X"}, cursor: at(4, 0)},
	{name: "CUF zero is one", in: "\x1b[1;3H\x1b[0CX", want: []string{"   X"}, cursor: at(4, 0)},
	{name: "CUF count", in: "\x1b[1;3H\x1b[4CX", want: []string{"      X"}, cursor: at(7, 0)},
	{name: "CUF clamps at right edge", in: "\x1b[1;3H\x1b[99CX", want: []string{"         X"}, cursor: at(9, 0)},
	{name: "HPR moves right", in: "\x1b[1;3H\x1b[2aX", want: []string{"    X"}, cursor: at(5, 0)},

	{name: "CUB default", in: "\x1b[1;5H\x1b[DX", want: []string{"   X"}, cursor: at(4, 0)},
	{name: "CUB zero is one", in: "\x1b[1;5H\x1b[0DX", want: []string{"   X"}, cursor: at(4, 0)},
	{name: "CUB count", in: "\x1b[1;5H\x1b[3DX", want: []string{" X"}, cursor: at(2, 0)},
	{name: "CUB clamps at left edge", in: "\x1b[1;5H\x1b[99DX", want: []string{"X"}, cursor: at(1, 0)},

	{name: "CNL default", in: "\x1b[2;5H\x1b[EX", want: []string{"", "", "X"}, cursor: at(1, 2)},
	{name: "CNL count", in: "\x1b[2;5H\x1b[2EX", want: []string{"", "", "", "X"}, cursor: at(1, 3)},
	{name: "CNL stops at bottom margin", in: "\x1b[2;4r\x1b[2;5H\x1b[99EX", want: []string{"", "", "", "X"}, cursor: at(1, 3)},
	{name: "CPL default", in: "\x1b[3;5H\x1b[FX", want: []string{"", "X"}, cursor: at(1, 1)},
	{name: "CPL count", in: "\x1b[4;5H\x1b[2FX", want: []string{"", "X"}, cursor: at(1, 1)},
	{name: "CPL stops at top margin", in: "\x1b[2;4r\x1b[4;5H\x1b[99FX", want: []string{"", "X"}, cursor: at(1, 1)},

	{name: "CHA", in: "\x1b[3;5H\x1b[8GX", want: []string{"", "", "       X"}, cursor: at(8, 2)},
	{name: "CHA default", in: "\x1b[3;5H\x1b[GX", want: []string{"", "", "X"}, cursor: at(1, 2)},
	{name: "CHA zero is one", in: "\x1b[3;5H\x1b[0GX", want: []string{"", "", "X"}, cursor: at(1, 2)},
	{name: "CHA clamps", in: "\x1b[3;5H\x1b[99GX", want: []string{"", "", "         X"}, cursor: at(9, 2)},
	{name: "HPA", in: "\x1b[3;1H\x1b[5`X", want: []string{"", "", "    X"}, cursor: at(5, 2)},
	{name: "HPA clamps", in: "\x1b[3;1H\x1b[99`X", want: []string{"", "", "         X"}, cursor: at(9, 2)},

	{name: "VPA", in: "\x1b[2;4H\x1b[4dX", want: []string{"", "", "", "   X"}, cursor: at(4, 3)},
	{name: "VPA default", in: "\x1b[3;4H\x1b[dX", want: []string{"   X"}, cursor: at(4, 0)},
	{name: "VPA clamps", in: "\x1b[2;4H\x1b[99dX", want: []string{"", "", "", "", "   X"}, cursor: at(4, 4)},

	{name: "CUP", in: "\x1b[3;5HX", want: []string{"", "", "    X"}, cursor: at(5, 2)},
	{name: "CUP no params homes", in: "\x1b[3;5H\x1b[HX", want: []string{"X"}, cursor: at(1, 0)},
	{name: "CUP zero params home", in: "\x1b[3;5H\x1b[0;0HX", want: []string{"X"}, cursor: at(1, 0)},
	{name: "CUP row only", in: "\x1b[3;5H\x1b[2HX", want: []string{"", "X"}, cursor: at(1, 1)},
	{name: "CUP column only", in: "\x1b[3;5H\x1b[;7HX", want: []string{"      X"}, cursor: at(7, 0)},
	{name: "CUP clamps both axes", in: "\x1b[99;99HX", want: []string{"", "", "", "", "         X"}, cursor: at(9, 4)},
	{name: "HVP", in: "\x1b[3;5fX", want: []string{"", "", "    X"}, cursor: at(5, 2)},
	{name: "HVP no params homes", in: "\x1b[3;5H\x1b[fX", want: []string{"X"}, cursor: at(1, 0)},

	{name: "DECOM set homes to margin top", in: "\x1b[2;4r\x1b[?6h", cursor: at(0, 1)},
	{name: "DECOM reset homes to screen origin", in: "\x1b[2;4r\x1b[?6h\x1b[?6l", cursor: at(0, 0)},
	{name: "DECOM CUP is margin relative", in: "\x1b[2;4r\x1b[?6h\x1b[1;1HX", want: []string{"", "X"}, cursor: at(1, 1)},
	{name: "DECOM CUP clamps to bottom margin", in: "\x1b[2;4r\x1b[?6h\x1b[99;1HX", want: []string{"", "", "", "X"}, cursor: at(1, 3)},
	{name: "DECOM VPA is margin relative", in: "\x1b[2;4r\x1b[?6h\x1b[3dX", want: []string{"", "", "", "X"}, cursor: at(1, 3)},
	{name: "DECOM CUU stops at top margin", in: "\x1b[2;4r\x1b[?6h\x1b[3;1H\x1b[99AX", want: []string{"", "X"}, cursor: at(1, 1)},
	{name: "DECOM then DECSTBM homes to margin top", in: "\x1b[?6h\x1b[2;4r", cursor: at(0, 1)},
	{name: "DECOM set without margins is screen relative", in: "\x1b[?6h\x1b[2;2HX", want: []string{"", " X"}, cursor: at(2, 1)},
	{name: "no DECOM CUP ignores margins", in: "\x1b[2;4r\x1b[1;1HX", want: []string{"X"}, cursor: at(1, 0)},
}
