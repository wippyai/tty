// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func regexpMatch(pattern, s string) bool { return regexp.MustCompile(pattern).MatchString(s) }

func corpusFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "*.vt"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no corpora in testdata: %v", err)
	}
	return files
}

// TestCorporaDifferential replays every recorded and synthetic corpus through
// both emulators and compares grid and cursor after each chunk.
func TestCorporaDifferential(t *testing.T) {
	for _, f := range corpusFiles(t) {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		name := "corpus/" + filepath.Base(f)
		sizes := []int{61, 4096}
		if len(data) <= 8192 {
			sizes = append(sizes, 1)
		}
		for _, size := range sizes {
			t.Run(fmt.Sprintf("%s/chunk%d", filepath.Base(f), size), func(t *testing.T) {
				if d := replayDiff(data, corpusCols, corpusRows, size); d != nil {
					reportDivergence(t, name, d)
				}
			})
		}
	}
}

// TestCorporaChunkingInvariant requires the emulator to reach the same final
// state however the pty output is split across reads.
func TestCorporaChunkingInvariant(t *testing.T) {
	for _, f := range corpusFiles(t) {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(filepath.Base(f), func(t *testing.T) {
			bulk := newSUT(corpusCols, corpusRows)
			bulk.Write(string(data))
			for _, size := range []int{1, 3, 61, 1024} {
				split := newSUT(corpusCols, corpusRows)
				for _, c := range chunks(data, size) {
					split.Write(string(c))
				}
				if a, b := joinGrid(gridOf(bulk, corpusRows)), joinGrid(gridOf(split, corpusRows)); a != b {
					t.Errorf("chunk size %d: grid differs\nbulk:\n%s\nsplit:\n%s", size, a, b)
				}
				bx, by := bulk.Cursor()
				sx, sy := split.Cursor()
				if bx != sx || by != sy {
					t.Errorf("chunk size %d: cursor bulk (%d,%d) split (%d,%d)", size, bx, by, sx, sy)
				}
				if bulk.scr().ScrollbackLen() != split.scr().ScrollbackLen() {
					t.Errorf("chunk size %d: scrollback bulk %d split %d", size, bulk.scr().ScrollbackLen(), split.scr().ScrollbackLen())
				}
			}
		})
	}
}

// TestRecordedProgramsFinalScreens pins what a person would see at the end of
// each recorded session.
func TestRecordedProgramsFinalScreens(t *testing.T) {
	cases := []struct {
		file  string
		check func(t *testing.T, s *sut)
	}{
		{"ls-color.vt", func(t *testing.T, s *sut) { requireOutputContains(t, s, "README.md", "main.go", "src/", "total ") }},
		{"git-log-graph.vt", func(t *testing.T, s *sut) {
			requireOutputContains(t, s, "Merge branch 'bugfix'", "initial commit", "v1.0.0")
		}},
		{"vim-edit.vt", func(t *testing.T, s *sut) {
			if s.scr().Alternate() {
				t.Error("vim should have left the alternate screen")
			}
		}},
		{"less-file.vt", func(t *testing.T, s *sut) {
			// less -X keeps the screen: the last page of the file stays visible after quit.
			requireOutputContains(t, s, "line 120 the quick brown fox")
		}},
		{"htop-1s.vt", func(t *testing.T, s *sut) {
			if s.scr().Alternate() {
				t.Error("htop should have left the alternate screen")
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", c.file))
			if err != nil {
				t.Fatal(err)
			}
			s := newSUT(corpusCols, corpusRows)
			s.Write(string(data))
			c.check(t, s)
		})
	}
}

// requireOutputContains checks scrollback plus the visible screen.
func requireOutputContains(t *testing.T, s *sut, subs ...string) {
	t.Helper()
	var all []string
	for i := 0; i < s.scr().ScrollbackLen(); i++ {
		all = append(all, scrollbackText(s, i))
	}
	all = append(all, gridOf(s, corpusRows)...)
	text := strings.Join(all, "\n")
	for _, sub := range subs {
		if !strings.Contains(text, sub) {
			t.Errorf("output does not contain %q:\n%s", sub, text)
		}
	}
}
