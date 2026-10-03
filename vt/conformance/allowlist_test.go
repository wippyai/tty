// SPDX-License-Identifier: MPL-2.0

package conformance

// allowlist names every intentional or reference-side difference the
// differential replay tolerates. A stream matches an entry by exact name or by
// a trailing-* prefix; a replay stops at its first allowed difference. Entries
// marked "unverified" rest on documented xterm behavior that could not be
// checked against a real xterm in this environment.
const (
	reasonInvalidBytes = "the reference silently drops invalid UTF-8 bytes and lone 8-bit C1 bytes; xterm in UTF-8 mode prints U+FFFD for each"
	reasonEraseWrap    = "the reference parks the cursor at x=cols while a wrap is pending, so ED/EL/ECH act from beyond the last cell and keep the wrap pending; " +
		"VT/xterm keep the cursor on the last column, erase it, and clear the pending wrap (unverified against real xterm)"
	reasonTabWrap = "the reference keeps the wrap pending across HT at the last column; VT/xterm treat HT as cursor motion that clears it (unverified against real xterm)"
	reasonDECRC   = "the reference shifts the saved cursor row when the screen scrolls; xterm saves a screen-relative row (unverified against real xterm)"
	reasonReflow  = "the reference does not rejoin the final soft-wrapped logical line on widening; ours reflows every soft-wrapped line of the primary buffer"
)

var allowlist = []allow{
	{"directed/c1-csi-8bit", "grid", reasonInvalidBytes},
	{"directed/utf8-invalid", "grid", reasonInvalidBytes},
	{"random/seed27", "grid", reasonEraseWrap},
	{"random/seed56", "cursor", reasonEraseWrap},
	{"random/seed40", "grid", reasonTabWrap},
	{"random/seed49", "", reasonDECRC},
	{"resize/*", "", reasonReflow},
}
