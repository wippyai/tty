// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// Differential replay: identical bytes go to the emulator under test and to the
// reference (github.com/gitpod-io/xterm-go, test-only). After every chunk the
// visible grid text and the cursor are compared. Differences the reference is
// known to get wrong, or that are deliberate, are listed in allowlist with a
// reason; any other difference fails the test.

type divergence struct {
	chunk  int
	offset int // bytes fed when the difference appeared
	kind   string
	detail string
	prev   string // the bytes of the offending chunk
}

func (d divergence) String() string {
	return fmt.Sprintf("chunk %d (offset %d) %s\n%s\nchunk bytes: %q", d.chunk, d.offset, d.kind, d.detail, d.prev)
}

// compareEmus returns the first difference between ours and the reference.
func compareEmus(ours, ref emu) (kind, detail string) {
	cols, rows := ours.Size()
	rc, rr := ref.Size()
	if cols != rc || rows != rr {
		return "size", fmt.Sprintf("ours %dx%d ref %dx%d", cols, rows, rc, rr)
	}
	og, rg := gridOf(ours, rows), gridOf(ref, rows)
	if joinGrid(og) != joinGrid(rg) {
		return "grid", diffGrid(rg, og) + "(want = reference, got = ours)"
	}
	ox, oy := ours.Cursor()
	rx, ry := ref.Cursor()
	if ox != rx || oy != ry {
		return "cursor", fmt.Sprintf("ours (%d,%d) reference (%d,%d)", ox, oy, rx, ry)
	}
	return "", ""
}

func chunks(data []byte, size int) [][]byte {
	var out [][]byte
	for len(data) > 0 {
		n := min(size, len(data))
		out = append(out, data[:n])
		data = data[n:]
	}
	return out
}

// replayDiff feeds data to both emulators in chunks of size and returns the
// first divergence, or nil.
func replayDiff(data []byte, cols, rows, size int) *divergence {
	ours, ref := newSUT(cols, rows), newOracle(cols, rows)
	off := 0
	for i, c := range chunks(data, size) {
		off += len(c)
		var perr any
		func() {
			defer func() { perr = recover() }()
			ours.Write(string(c))
		}()
		if perr != nil {
			return &divergence{i, off, "panic", fmt.Sprint(perr), string(c)}
		}
		func() {
			defer func() { recover() }()
			ref.Write(string(c))
		}()
		if kind, detail := compareEmus(ours, ref); kind != "" {
			return &divergence{i, off, kind, detail, string(c)}
		}
	}
	return nil
}

type allow struct {
	stream string // stream name, or a prefix ending in *
	kind   string // grid, cursor or size; empty matches any
	reason string
}

func (a allow) matches(stream string, d *divergence) bool {
	if a.kind != "" && a.kind != d.kind {
		return false
	}
	if p, ok := strings.CutSuffix(a.stream, "*"); ok {
		return strings.HasPrefix(stream, p)
	}
	return a.stream == stream
}

// reportDivergence fails the test unless an allowlist entry covers d.
func reportDivergence(t *testing.T, stream string, d *divergence) {
	t.Helper()
	for _, a := range allowlist {
		if a.matches(stream, d) {
			t.Logf("allowed difference (%s): %s", a.reason, d)
			return
		}
	}
	t.Errorf("%s: differs from the reference: %s", stream, d)
}

// diffStream is one named byte stream replayed through both emulators.
type diffStream struct {
	name       string
	data       []byte
	cols, rows int
}

func directedStreams() []diffStream {
	s := func(name, data string) diffStream { return diffStream{name, []byte(data), 20, 6} }
	long := "0123456789abcdefghijklmnopqrstuvwxyz0123456789"
	return []diffStream{
		s("directed/c1-csi-8bit", "a\x9b31mX\r\nb"),
		s("directed/c1-st-8bit", "\x1b]0;title\x9cX"),
		s("directed/utf8-invalid", "a\xffb\xe4\xb8c"),
		s("directed/lines-and-wrap", long+"\r\n"+long+"\r\nend"),
		s("directed/region-scroll", "\x1b[2;5r\x1b[5;1H1\n2\n3\n4\n\x1b[2;1H\x1bM\x1bM x"),
		s("directed/il-dl", "1\r\n2\r\n3\r\n4\r\n5\x1b[2;1H\x1b[2L\x1b[4;1H\x1b[M"),
		s("directed/alt-screen", "main\x1b[?1049h\x1b[2J\x1b[HALT\x1b[?1049lX"),
		s("directed/tabs", "a\tb\tc\x1b[1;3H\x1bH\r\t\tz\x1b[g\r\tq"),
		s("directed/wide", "中文abc\r\n\x1b[1;20H中x\r\n\x1b[2;1H中\x1b[2;2Hx"),
		s("directed/charset", "\x1b(0lqqk\r\nx  x\r\nmqqj\x1b(B ok"),
		s("directed/decaln-erase", "\x1b#8\x1b[3;5H\x1b[1J\x1b[2;3H\x1b[K"),
	}
}

var seqPieces = []func(r *rand.Rand) string{
	func(r *rand.Rand) string { return randText(r) },
	func(r *rand.Rand) string { return randText(r) },
	func(r *rand.Rand) string { return randText(r) },
	func(r *rand.Rand) string { return []string{"\r", "\n", "\r\n", "\b", "\t"}[r.Intn(5)] },
	func(r *rand.Rand) string { return fmt.Sprintf("\x1b[%d%c", r.Intn(5), "ABCD"[r.Intn(4)]) },
	func(r *rand.Rand) string { return fmt.Sprintf("\x1b[%d;%dH", 1+r.Intn(8), 1+r.Intn(24)) },
	func(r *rand.Rand) string { return fmt.Sprintf("\x1b[%d%s", r.Intn(3), []string{"J", "K"}[r.Intn(2)]) },
	func(r *rand.Rand) string { return fmt.Sprintf("\x1b[%d%c", 1+r.Intn(4), "@PXLM"[r.Intn(5)]) },
	func(r *rand.Rand) string { return fmt.Sprintf("\x1b[%d;%dr", 1+r.Intn(3), 4+r.Intn(5)) },
	func(r *rand.Rand) string { return "\x1b[r" },
	func(r *rand.Rand) string { return []string{"\x1bD", "\x1bE", "\x1bM"}[r.Intn(3)] },
	func(r *rand.Rand) string { return fmt.Sprintf("\x1b[%d%c", 1+r.Intn(3), "ST"[r.Intn(2)]) },
	func(r *rand.Rand) string { return []string{"\x1b7", "\x1b8"}[r.Intn(2)] },
	func(r *rand.Rand) string { return fmt.Sprintf("\x1b[%d;%dm", r.Intn(8), 30+r.Intn(8)) },
	func(r *rand.Rand) string {
		return []string{"\x1bH", "\x1b[g", "\x1b[3g", "\x1b[I", "\x1b[Z"}[r.Intn(5)]
	},
}

func randText(r *rand.Rand) string {
	n := 1 + r.Intn(12)
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + r.Intn(26))
	}
	return string(b)
}

func randomStream(seed int64, pieces int) []byte {
	r := rand.New(rand.NewSource(seed))
	var b strings.Builder
	for i := 0; i < pieces; i++ {
		b.WriteString(seqPieces[r.Intn(len(seqPieces))](r))
	}
	return []byte(b.String())
}

func TestDifferentialDirected(t *testing.T) {
	for _, st := range directedStreams() {
		for _, size := range []int{1, 5, 4096} {
			t.Run(fmt.Sprintf("%s/chunk%d", st.name, size), func(t *testing.T) {
				if d := replayDiff(st.data, st.cols, st.rows, size); d != nil {
					reportDivergence(t, st.name, d)
				}
			})
		}
	}
}

// tokens splits a stream before every ESC so a failing stream can be reduced
// by whole sequences.
func tokens(data []byte) []string {
	var out []string
	start := 0
	for i := 1; i < len(data); i++ {
		if data[i] == 0x1b {
			out = append(out, string(data[start:i]))
			start = i
		}
	}
	return append(out, string(data[start:]))
}

// minimize removes tokens while fails keeps returning true.
func minimize(toks []string, fails func(string) bool) string {
	for n := len(toks) / 2; n >= 1; n /= 2 {
		for i := 0; i+n <= len(toks); {
			cand := append(append([]string{}, toks[:i]...), toks[i+n:]...)
			if fails(strings.Join(cand, "")) {
				toks = cand
			} else {
				i += n
			}
		}
	}
	return strings.Join(toks, "")
}

func TestDifferentialRandom(t *testing.T) {
	for seed := int64(1); seed <= 60; seed++ {
		name := fmt.Sprintf("random/seed%d", seed)
		data := randomStream(seed, 120)
		for _, size := range []int{1, 13, 4096} {
			t.Run(fmt.Sprintf("%s/chunk%d", name, size), func(t *testing.T) {
				if d := replayDiff(data, 24, 8, size); d != nil {
					min := minimize(tokens(data), func(s string) bool { return replayDiff([]byte(s), 24, 8, size) != nil })
					d.prev = fmt.Sprintf("minimal stream %q", min)
					reportDivergence(t, name, d)
				}
			})
		}
	}
}

// TestDifferentialResize replays text through a sequence of resizes.
func TestDifferentialResize(t *testing.T) {
	text := strings.Repeat("abcdefghijklmnopqrstuvwxyz0123456789 ", 6) + "\r\nshort\r\n" + strings.Repeat("x", 45)
	steps := [][2]int{{30, 8}, {12, 8}, {12, 4}, {40, 4}, {40, 10}, {20, 10}}
	ours, ref := newSUT(20, 10), newOracle(20, 10)
	ours.Write(text)
	ref.Write(text)
	if kind, detail := compareEmus(ours, ref); kind != "" {
		reportDivergence(t, "resize/initial", &divergence{kind: kind, detail: detail})
	}
	for _, st := range steps {
		ours.Resize(st[0], st[1])
		ref.Resize(st[0], st[1])
		if kind, detail := compareEmus(ours, ref); kind != "" {
			reportDivergence(t, fmt.Sprintf("resize/to%dx%d", st[0], st[1]), &divergence{kind: kind, detail: detail})
		}
	}
}
