// SPDX-License-Identifier: MPL-2.0

package screen

import (
	"strings"
	"testing"
)

func BenchmarkPrintLineFeed(b *testing.B) {
	s := New(80, 24, 10000)
	text := strings.Repeat("the quick brown fox ", 4)
	line := make([]string, 0, len(text))
	for _, r := range text {
		line = append(line, string(r))
	}
	b.SetBytes(int64(len(line)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, c := range line {
			s.Print(c, 1)
		}
		s.CarriageReturn()
		s.LineFeed()
	}
}

func BenchmarkPrintWide(b *testing.B) {
	s := New(80, 24, 10000)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 40; j++ {
			s.Print("世", 2)
		}
		s.CarriageReturn()
		s.LineFeed()
	}
}

func BenchmarkScrollRegion(b *testing.B) {
	s := New(80, 24, 0)
	s.SetScrollRegion(1, 22)
	s.MoveTo(0, 21)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s.LineFeed()
	}
}

func BenchmarkReflow(b *testing.B) {
	for _, bc := range []struct {
		name string
		sb   int
	}{{"screen", 0}, {"scrollback-10k", 10000}} {
		b.Run(bc.name, func(b *testing.B) {
			s := New(120, 40, bc.sb)
			line := strings.Repeat("0123456789 abcdefghij ", 8)
			for i := 0; i < bc.sb+100; i++ {
				put(s, line+"\n")
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				s.Resize(80, 40)
				s.Resize(120, 40)
			}
		})
	}
}
