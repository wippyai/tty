// SPDX-License-Identifier: MPL-2.0

package vt

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func gfx(ctrl string, payload string) string {
	return "\x1b_G" + ctrl + ";" + payload + "\x1b\\"
}

func rgb(w, h int) []byte { return bytes.Repeat([]byte{1, 2, 3}, w*h) }

func pngBytes(w, h int) []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h)))
	return b.Bytes()
}

func TestGraphicsTransmitAndPlace(t *testing.T) {
	h := newHarness(t, 20, 10)
	h.write("\x1b[3;4H")
	h.write(gfx("a=T,f=24,s=2,v=2,i=7,c=3,r=2", b64(rgb(2, 2))))
	assert.Equal(t, "\x1b_Gi=7;OK\x1b\\", h.take())
	imgs := h.term.Images()
	assert.Len(t, imgs, 1)
	assert.Equal(t, Image{ID: 7, Format: 24, Width: 2, Height: 2, Data: rgb(2, 2)}, imgs[0])
	assert.Equal(t, []Placement{{ImageID: 7, X: 3, Y: 2, Cols: 3, Rows: 2}}, h.term.Placements())
	x, y := h.pos()
	assert.Equal(t, [2]int{6, 3}, [2]int{x, y})
}

func TestGraphicsDerivedExtentAndCursorStay(t *testing.T) {
	h := newHarness(t, 20, 10)
	h.write(gfx("a=T,f=32,s=20,v=33,i=1,C=1,p=5,z=-1", b64(bytes.Repeat([]byte{0}, 20*33*4))))
	assert.Equal(t, "\x1b_Gi=1,p=5;OK\x1b\\", h.take())
	assert.Equal(t, []Placement{{ImageID: 1, PlacementID: 5, X: 0, Y: 0, Cols: 3, Rows: 3, Z: -1}}, h.term.Placements())
	x, y := h.pos()
	assert.Equal(t, [2]int{0, 0}, [2]int{x, y})
}

func TestGraphicsTransmitThenPlace(t *testing.T) {
	h := newHarness(t, 20, 10)
	h.write(gfx("a=t,f=32,s=1,v=1,i=3", b64([]byte{1, 2, 3, 4})))
	assert.Equal(t, "\x1b_Gi=3;OK\x1b\\", h.take())
	assert.Empty(t, h.term.Placements())
	h.write("\x1b[2;2H")
	h.write("\x1b_Ga=p,i=3,p=1;\x1b\\")
	assert.Equal(t, "\x1b_Gi=3,p=1;OK\x1b\\", h.take())
	h.write("\x1b[5;5H\x1b_Ga=p,i=3,p=1\x1b\\")
	pl := h.term.Placements()
	assert.Len(t, pl, 1)
	assert.Equal(t, [2]int{4, 4}, [2]int{pl[0].X, pl[0].Y})
	h.take()
	h.write("\x1b_Ga=p,i=99\x1b\\")
	assert.Equal(t, "\x1b_Gi=99;ENOENT:image not found\x1b\\", h.take())
}

func TestGraphicsChunked(t *testing.T) {
	h := newHarness(t, 20, 10)
	data := b64(rgb(2, 2))
	a, b := data[:8], data[8:]
	h.write(gfx("a=T,f=24,s=2,v=2,i=9,m=1", a))
	assert.Empty(t, h.take())
	h.write(gfx("m=1", b[:4]))
	assert.Empty(t, h.take())
	h.write(gfx("m=0", b[4:]))
	assert.Equal(t, "\x1b_Gi=9;OK\x1b\\", h.take())
	assert.Equal(t, rgb(2, 2), h.term.Images()[0].Data)
	assert.Len(t, h.term.Placements(), 1)
}

func TestGraphicsChunkedPayloadCopied(t *testing.T) {
	h := newHarness(t, 20, 10)
	data := b64(rgb(2, 2))
	buf := []byte("\x1b_Ga=t,f=24,s=2,v=2,i=9,m=1;" + data[:8] + "\x1b\\")
	h.term.Write(buf)
	for i := range buf {
		buf[i] = 'X'
	}
	h.write(gfx("m=0", data[8:]))
	assert.Equal(t, "\x1b_Gi=9;OK\x1b\\", h.take())
}

func TestGraphicsQuery(t *testing.T) {
	h := newHarness(t, 20, 10)
	h.write(gfx("a=q,f=24,s=1,v=1,i=31", b64([]byte{1, 2, 3})))
	assert.Equal(t, "\x1b_Gi=31;OK\x1b\\", h.take())
	assert.Empty(t, h.term.Images())
	h.write(gfx("a=q,f=24,s=1,v=1,i=31", b64([]byte{1, 2})))
	assert.True(t, strings.HasPrefix(h.take(), "\x1b_Gi=31;EINVAL:"))
	h.write(gfx("a=q,f=24,s=1,v=1", b64([]byte{1, 2})))
	assert.Empty(t, h.take())
}

func TestGraphicsQuiet(t *testing.T) {
	h := newHarness(t, 20, 10)
	h.write(gfx("a=t,f=24,s=1,v=1,i=1,q=1", b64([]byte{1, 2, 3})))
	assert.Empty(t, h.take())
	h.write(gfx("a=t,f=24,s=1,v=1,i=2,q=1", b64([]byte{1})))
	assert.True(t, strings.HasPrefix(h.take(), "\x1b_Gi=2;EINVAL"))
	h.write(gfx("a=t,f=24,s=1,v=1,i=3,q=2", b64([]byte{1})))
	assert.Empty(t, h.take())
}

func TestGraphicsFormats(t *testing.T) {
	h := newHarness(t, 20, 10)
	h.write(gfx("a=t,f=100,i=1", b64(pngBytes(5, 7))))
	assert.Equal(t, "\x1b_Gi=1;OK\x1b\\", h.take())
	img := h.term.Images()[0]
	assert.Equal(t, [3]int{100, 5, 7}, [3]int{img.Format, img.Width, img.Height})
	h.write(gfx("a=t,f=100,i=2", b64([]byte("not a png"))))
	assert.True(t, strings.HasPrefix(h.take(), "\x1b_Gi=2;EBADPNG"))
	h.write(gfx("a=t,f=7,i=3", b64([]byte("x"))))
	assert.True(t, strings.HasPrefix(h.take(), "\x1b_Gi=3;EINVAL"))
	h.write(gfx("a=t,f=24,i=4", b64([]byte("xxx"))))
	assert.True(t, strings.HasPrefix(h.take(), "\x1b_Gi=4;EINVAL"))
	h.write(gfx("a=t,f=24,s=1,v=1,i=5", "!!!!"))
	assert.True(t, strings.HasPrefix(h.take(), "\x1b_Gi=5;EINVAL"))
}

func TestGraphicsCompressed(t *testing.T) {
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	w.Write(rgb(4, 4))
	w.Close()
	h := newHarness(t, 20, 10)
	h.write(gfx("a=t,f=24,s=4,v=4,o=z,i=1", b64(z.Bytes())))
	assert.Equal(t, "\x1b_Gi=1;OK\x1b\\", h.take())
	assert.Equal(t, rgb(4, 4), h.term.Images()[0].Data)
}

func TestGraphicsRefusesFileMedia(t *testing.T) {
	h := newHarness(t, 20, 10)
	for _, m := range []string{"f", "t", "s"} {
		h.write(gfx("a=t,f=24,s=1,v=1,i=1,t="+m, b64([]byte("/etc/passwd"))))
		assert.True(t, strings.HasPrefix(h.take(), "\x1b_Gi=1;EPERM:"), m)
	}
	assert.Empty(t, h.term.Images())
}

func TestGraphicsImageNumber(t *testing.T) {
	h := newHarness(t, 20, 10)
	h.write(gfx("a=t,f=24,s=1,v=1,I=13", b64([]byte{1, 2, 3})))
	assert.Equal(t, "\x1b_GI=13;OK\x1b\\", h.take())
	imgs := h.term.Images()
	assert.Len(t, imgs, 1)
	assert.NotZero(t, imgs[0].ID)
	h.write("\x1b_Ga=p,I=13\x1b\\")
	assert.Equal(t, "\x1b_GI=13;OK\x1b\\", h.take())
	assert.Len(t, h.term.Placements(), 1)
}

func TestGraphicsDelete(t *testing.T) {
	setup := func() *harness {
		h := newHarness(t, 20, 10)
		px := b64([]byte{1, 2, 3})
		h.write(gfx("a=T,f=24,s=1,v=1,i=1,p=1,C=1", px))
		h.write("\x1b[3;5H")
		h.write(gfx("a=T,f=24,s=1,v=1,i=2,p=1,C=1,z=4", px))
		h.write("\x1b[3;5H")
		h.write(gfx("a=p,i=2,p=2,C=1", ""))
		h.take()
		return h
	}
	h := setup()
	assert.Len(t, h.term.Placements(), 3)
	h.write("\x1b_Ga=d,d=i,i=2,p=2\x1b\\")
	assert.Len(t, h.term.Placements(), 2)
	h.write("\x1b_Ga=d,d=i,i=1\x1b\\")
	assert.Len(t, h.term.Placements(), 1)
	assert.Len(t, h.term.Images(), 2)
	h.write("\x1b_Ga=d,d=I,i=2\x1b\\")
	assert.Empty(t, h.term.Placements())
	assert.Len(t, h.term.Images(), 1)

	h = setup()
	h.write("\x1b_Ga=d\x1b\\")
	assert.Empty(t, h.term.Placements())
	assert.Len(t, h.term.Images(), 2)
	h.write("\x1b_Ga=d,d=A\x1b\\")
	assert.Empty(t, h.term.Images())

	h = setup()
	h.write("\x1b[3;5H\x1b_Ga=d,d=c\x1b\\")
	pl := h.term.Placements()
	assert.Len(t, pl, 1)
	assert.Equal(t, uint32(1), pl[0].ImageID)

	h = setup()
	h.write("\x1b_Ga=d,d=z,z=4\x1b\\")
	assert.Len(t, h.term.Placements(), 2)

	h = setup()
	h.write("\x1b_Ga=d,d=x,x=1\x1b\\")
	assert.Len(t, h.term.Placements(), 2)
	h = setup()
	h.write("\x1b_Ga=d,d=y,y=1\x1b\\")
	assert.Len(t, h.term.Placements(), 2)
}

func TestGraphicsClearedByEraseAndReset(t *testing.T) {
	h := newHarness(t, 20, 10)
	h.write(gfx("a=T,f=24,s=1,v=1,i=1", b64([]byte{1, 2, 3})))
	h.write("\x1b[2J")
	assert.Empty(t, h.term.Placements())
	assert.Len(t, h.term.Images(), 1)
	h.write("\x1bc")
	assert.Empty(t, h.term.Images())
}

func TestGraphicsPlacementsPerBuffer(t *testing.T) {
	h := newHarness(t, 20, 10)
	px := b64([]byte{1, 2, 3})
	h.write(gfx("a=T,f=24,s=1,v=1,i=1", px))
	h.write("\x1b[?1049h")
	assert.Empty(t, h.term.Placements())
	h.write(gfx("a=T,f=24,s=1,v=1,i=2", px))
	assert.Len(t, h.term.Placements(), 1)
	h.write("\x1b[?1049l")
	pl := h.term.Placements()
	assert.Len(t, pl, 1)
	assert.Equal(t, uint32(1), pl[0].ImageID)
}

func TestGraphicsPlacementUsesScrollbackOffset(t *testing.T) {
	h := newHarness(t, 10, 3)
	h.write("1\r\n2\r\n3\r\n4")
	sb := h.term.Screen().ScrollbackLen()
	assert.Positive(t, sb)
	h.write(gfx("a=T,f=24,s=1,v=1,i=1,C=1", b64([]byte{1, 2, 3})))
	assert.Equal(t, sb+2, h.term.Placements()[0].Y)
}

func TestGraphicsIgnoresOtherAPC(t *testing.T) {
	h := newHarness(t, 10, 3)
	h.write("\x1b_Xfoo\x1b\\")
	assert.Empty(t, h.take())
}
