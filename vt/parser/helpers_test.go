// SPDX-License-Identifier: MPL-2.0

package parser

import (
	"fmt"
	"strings"
)

// recorder renders every handler call as one line.
type recorder struct{ ev []string }

func fmtParams(p Params) string {
	parts := make([]string, len(p.Values))
	for i, v := range p.Values {
		s := make([]string, len(v))
		for j, x := range v {
			if x == Default {
				s[j] = "_"
			} else {
				s[j] = fmt.Sprint(x)
			}
		}
		parts[i] = strings.Join(s, ":")
	}
	return strings.Join(parts, ";")
}

func (r *recorder) add(f string, a ...any) { r.ev = append(r.ev, fmt.Sprintf(f, a...)) }

func (r *recorder) Print(c string)       { r.add("P %q", c) }
func (r *recorder) Execute(b byte)       { r.add("X %02x", b) }
func (r *recorder) ESC(i []byte, f byte) { r.add("ESC %q %c", i, f) }
func (r *recorder) OSC(d []byte)         { r.add("OSC %q", d) }
func (r *recorder) APC(d []byte)         { r.add("APC %q", d) }
func (r *recorder) CSI(pre byte, p Params, i []byte, f byte) {
	r.add("CSI %q [%s] %q %c", pre, fmtParams(p), i, f)
}
func (r *recorder) DCS(pre byte, p Params, i []byte, f byte, d []byte) {
	r.add("DCS %q [%s] %q %c %q", pre, fmtParams(p), i, f, d)
}

// run feeds the chunks to a fresh parser and flushes at the end.
func run(chunks ...string) []string {
	r := &recorder{}
	p := New(r)
	for _, c := range chunks {
		p.Write([]byte(c))
	}
	p.Flush()
	return r.ev
}

func runBytewise(s string) []string {
	r := &recorder{}
	p := New(r)
	for i := 0; i < len(s); i++ {
		p.Write([]byte{s[i]})
	}
	p.Flush()
	return r.ev
}
