// SPDX-License-Identifier: MPL-2.0

package parser

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

var corpus = []string{
	"plain text, with spaces\r\n",
	"\x1b[1;31mred\x1b[0m \x1b[38:2::10:20:30mrgb\x1b[38;2;1;2;3mx\x1b[m",
	"\x1b[?1049h\x1b[?25l\x1b[>0c\x1b[<1u\x1b[=1c\x1b[0 q\x1b[1$p",
	"\x1b[999999999999m\x1b[;;;m\x1b[1?h\x1b[1:2:3:4m",
	"\x1b7\x1b8\x1bM\x1b(0\x1b#8\x1b\\",
	"\x1b]0;window title\a\x1b]8;;http://example.com\x1b\\link\x1b]8;;\x1b\\",
	"\x1bP1$qm\x1b\\\x1bPq#0;2;0;0;0\x1b\\\x1bP>|name\x1b\\",
	"\x1b_Ga=T,f=100;AAAABBBB\x1b\\\x1b_Gm=1;CCCC\x1b\\",
	"\x1bXsos\x1b\\\x1b^pm\x1b\\",
	"café € \U0001F600 é \U0001F468‍\U0001F469‍\U0001F467 \U0001F1FA\U0001F1F8\U0001F1EB\U0001F1F7 각",
	"\xff\xfe\xc0\x80\xe2\x82\xed\xa0\x80\xf4\x90\x80\x80ok\x80",
	"abc\x18def\x1a\x1b[12\x18g\x1b]0;t\x18h\x1bPq\x1ai",
	"\x1b]0;a\x1b[1m\x1b]0;b\x1b\x1b7\x1bPq\x1b_G\x1b\\",
	"\x00\a\b\t\n\v\f\r\x0e\x0f\x7f",
	"\x1b[1\n;2\rH\x1b(\n0",
	"\x1b\xc3\xa9\x1b[1\xc3\xa9",
}

func corpusBytes() string { return strings.Join(corpus, "") }

func splitRun(s string, cuts []int) []string {
	r := &recorder{}
	p := New(r)
	prev := 0
	for _, c := range cuts {
		p.Write([]byte(s[prev:c]))
		prev = c
	}
	p.Write([]byte(s[prev:]))
	p.Flush()
	return r.ev
}

func TestBytewiseEqualsWhole(t *testing.T) {
	for i, c := range corpus {
		whole, bw := run(c), runBytewise(c)
		if !reflect.DeepEqual(whole, bw) {
			t.Fatalf("corpus[%d] %q: bytewise differs\nwhole %v\nbytes %v", i, c, whole, bw)
		}
	}
	s := corpusBytes()
	if !reflect.DeepEqual(run(s), runBytewise(s)) {
		t.Fatal("full corpus bytewise differs")
	}
}

func TestEverySingleSplitPoint(t *testing.T) {
	s := corpusBytes()
	want := run(s)
	for i := 0; i <= len(s); i++ {
		if got := splitRun(s, []int{i}); !reflect.DeepEqual(got, want) {
			t.Fatalf("split at %d differs", i)
		}
	}
}

func TestRandomSplitPoints(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	s := corpusBytes()
	want := run(s)
	for iter := 0; iter < 2000; iter++ {
		n := rng.Intn(20)
		cuts := make([]int, n)
		for i := range cuts {
			cuts[i] = rng.Intn(len(s) + 1)
		}
		sortInts(cuts)
		if got := splitRun(s, cuts); !reflect.DeepEqual(got, want) {
			t.Fatalf("iteration %d cuts %v differ", iter, cuts)
		}
	}
}

func TestRandomBytesSplitInvariance(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	alphabet := []byte("\x1b[];:?>0123456789 !m\x18\x1a\a\\]PX^_q\xc3\xa9\xe2\x82\xac\x80\xff\n e\xcc\x81")
	for iter := 0; iter < 3000; iter++ {
		b := make([]byte, rng.Intn(64))
		for i := range b {
			b[i] = alphabet[rng.Intn(len(alphabet))]
		}
		s := string(b)
		if !reflect.DeepEqual(run(s), runBytewise(s)) {
			t.Fatalf("input %q differs", s)
		}
	}
}

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}

func FuzzParser(f *testing.F) {
	for _, c := range corpus {
		f.Add([]byte(c), uint8(3))
	}
	f.Add([]byte("\x1b[38:2::1:2:3m\x1b]0;x\a"), uint8(5))
	f.Fuzz(func(t *testing.T, data []byte, cut uint8) {
		s := string(data)
		whole := run(s)
		if got := runBytewise(s); !reflect.DeepEqual(got, whole) {
			t.Fatalf("bytewise differs for %q", s)
		}
		if len(s) > 0 {
			c := int(cut) % (len(s) + 1)
			if got := splitRun(s, []int{c}); !reflect.DeepEqual(got, whole) {
				t.Fatalf("split at %d differs for %q", c, s)
			}
		}
	})
}

type discard struct{ n int }

func (d *discard) Print(c string)                                   { d.n += len(c) }
func (d *discard) Execute(byte)                                     { d.n++ }
func (d *discard) CSI(_ byte, p Params, _ []byte, _ byte)           { d.n += p.Len() }
func (d *discard) ESC([]byte, byte)                                 { d.n++ }
func (d *discard) OSC(b []byte)                                     { d.n += len(b) }
func (d *discard) DCS(_ byte, _ Params, _ []byte, _ byte, b []byte) { d.n += len(b) }
func (d *discard) APC(b []byte)                                     { d.n += len(b) }

func benchData() []byte {
	var sb strings.Builder
	for i := 0; i < 400; i++ {
		sb.WriteString("The quick brown fox jumps over the lazy dog 0123456789 ")
		sb.WriteString("\x1b[1;38;2;200;100;50mcolor\x1b[0m\r\n")
		sb.WriteString("\x1b[10;20H\x1b[K\x1b]0;title\a")
		sb.WriteString("café 中文字 \U0001F600 é \U0001F468‍\U0001F469‍\U0001F467 \r\n")
		sb.WriteString("\x1b_Gf=100;AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\x1b\\")
	}
	return []byte(sb.String())
}

func BenchmarkMixed(b *testing.B) {
	data := benchData()
	d := &discard{}
	p := New(d)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.Write(data)
	}
}

func BenchmarkASCII(b *testing.B) {
	data := []byte(strings.Repeat("hello world, plain ascii text line\r\n", 4000))
	d := &discard{}
	p := New(d)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.Write(data)
	}
}
