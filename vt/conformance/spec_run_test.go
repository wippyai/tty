// SPDX-License-Identifier: MPL-2.0

package conformance

import "testing"

func TestSpecCursor(t *testing.T)    { runSpec(t, cursorCases) }
func TestSpecScrolling(t *testing.T) { runSpec(t, scrollingCases) }
func TestSpecEditing(t *testing.T)   { runSpec(t, editingCases) }
func TestSpecErase(t *testing.T)     { runSpec(t, eraseCases) }
func TestSpecWrap(t *testing.T)      { runSpec(t, wrapCases) }
func TestSpecWide(t *testing.T)      { runSpec(t, wideCases) }
func TestSpecTabs(t *testing.T)      { runSpec(t, tabCases) }
func TestSpecSaveRestore(t *testing.T) {
	runSpec(t, saveRestoreCases)
}
func TestSpecAltScreen(t *testing.T) { runSpec(t, altScreenCases) }
func TestSpecReports(t *testing.T)   { runSpec(t, reportCases) }
func TestSpecSGR(t *testing.T)       { runSpec(t, sgrCases) }
func TestSpecCharsets(t *testing.T)  { runSpec(t, charsetCases) }
func TestSpecReset(t *testing.T)     { runSpec(t, resetCases) }
func TestSpecControls(t *testing.T)  { runSpec(t, controlCases) }
func TestSpecStrings(t *testing.T)   { runSpec(t, stringCases) }
func TestSpecModes(t *testing.T)     { runSpec(t, modeCases) }

func allSpecTables() map[string][]specCase {
	return map[string][]specCase{
		"cursor": cursorCases, "scrolling": scrollingCases, "editing": editingCases,
		"erase": eraseCases, "wrap": wrapCases, "wide": wideCases, "tabs": tabCases,
		"saverestore": saveRestoreCases, "altscreen": altScreenCases, "reports": reportCases,
		"sgr": sgrCases, "charsets": charsetCases, "reset": resetCases, "controls": controlCases,
		"strings": stringCases, "modes": modeCases,
	}
}

// TestSpecByteAtATime feeds every spec input one byte per Write and requires the
// same grid, cursor and replies as one bulk Write.
func TestSpecByteAtATime(t *testing.T) {
	for name, cases := range allSpecTables() {
		for _, c := range cases {
			t.Run(name+"/"+c.name, func(t *testing.T) {
				cols, rows := c.size()
				bulk, split := newSUT(cols, rows), newSUT(cols, rows)
				bulk.Write(c.in)
				for i := 0; i < len(c.in); i++ {
					split.Write(c.in[i : i+1])
				}
				if a, b := joinGrid(gridOf(bulk, rows)), joinGrid(gridOf(split, rows)); a != b {
					t.Errorf("grid differs\nbulk:\n%s\nbyte-wise:\n%s", a, b)
				}
				bx, by := bulk.Cursor()
				sx, sy := split.Cursor()
				if bx != sx || by != sy {
					t.Errorf("cursor bulk (%d,%d) byte-wise (%d,%d)", bx, by, sx, sy)
				}
				if a, b := bulk.Replies(), split.Replies(); a != b {
					t.Errorf("replies bulk %q byte-wise %q", a, b)
				}
				if bulk.scr().ScrollbackLen() != split.scr().ScrollbackLen() {
					t.Errorf("scrollback len bulk %d byte-wise %d", bulk.scr().ScrollbackLen(), split.scr().ScrollbackLen())
				}
			})
		}
	}
}

// TestSpecOracle runs every table against the reference emulator and logs the
// disagreements. It never fails: the tables are the specification, the
// reference is only a second opinion.
func TestSpecOracle(t *testing.T) {
	for name, cases := range allSpecTables() {
		for _, c := range cases {
			cols, rows := c.size()
			o := newOracle(cols, rows)
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Logf("oracle %s/%s: panic %v", name, c.name, r)
					}
				}()
				o.Write(c.in)
				for _, p := range compareSpec(c, o, rows) {
					t.Logf("oracle disagrees %s/%s: %s", name, c.name, p)
				}
			}()
		}
	}
}
