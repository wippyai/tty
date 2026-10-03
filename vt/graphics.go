// SPDX-License-Identifier: MPL-2.0

package vt

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"image/png"
	"io"
	"sort"
	"strconv"
	"strings"
)

const (
	maxImageBytes   = 64 << 20
	maxStoreBytes   = 320 << 20
	autoImageIDBase = 1 << 31
)

type gfxPending struct {
	kv      map[byte]string
	payload []byte
}

type gfxState struct {
	images     map[uint32]*Image
	numbers    map[uint32]uint32 // image number (I) to id
	order      []uint32
	placements [2][]Placement
	pending    *gfxPending
	nextAuto   uint32
	stored     int
}

func (g *gfxState) init() {
	g.images = map[uint32]*Image{}
	g.numbers = map[uint32]uint32{}
	g.nextAuto = autoImageIDBase
}

func (g *gfxState) reset() {
	*g = gfxState{}
	g.init()
}

// Images returns the stored kitty graphics images in transmission order.
func (t *Terminal) Images() []Image {
	out := make([]Image, 0, len(t.gfx.order))
	for _, id := range t.gfx.order {
		out = append(out, *t.gfx.images[id])
	}
	return out
}

// Placements returns the image placements of the active buffer.
func (t *Terminal) Placements() []Placement {
	return append([]Placement(nil), t.gfx.placements[t.bufferIndex()]...)
}

func (t *Terminal) bufferIndex() int {
	if t.scr.Alternate() {
		return 1
	}
	return 0
}

func (t *Terminal) clearPlacements() { t.gfx.placements[t.bufferIndex()] = nil }

func (g *gfxState) removeImage(id uint32) {
	img, ok := g.images[id]
	if !ok {
		return
	}
	g.stored -= len(img.Data)
	delete(g.images, id)
	for n, v := range g.numbers {
		if v == id {
			delete(g.numbers, n)
		}
	}
	for i, v := range g.order {
		if v == id {
			g.order = append(g.order[:i], g.order[i+1:]...)
			break
		}
	}
	for b := range g.placements {
		g.placements[b] = filterPlacements(g.placements[b], func(p Placement) bool { return p.ImageID != id })
	}
}

func filterPlacements(ps []Placement, keep func(Placement) bool) []Placement {
	out := ps[:0]
	for _, p := range ps {
		if keep(p) {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// APC handles kitty graphics commands (APC G ...).
func (t *Terminal) APC(data []byte) {
	if len(data) == 0 || data[0] != 'G' {
		return
	}
	ctrl, payload, _ := bytes.Cut(data[1:], []byte{';'})
	kv := parseGraphicsControl(string(ctrl))

	if p := t.gfx.pending; p != nil {
		if len(p.payload)+len(payload) > maxImageBytes*4/3+4 {
			t.gfx.pending = nil
			t.gfxRespond(p.kv, "ENOSPC:image data too large")
			return
		}
		p.payload = append(p.payload, payload...)
		if kv['m'] == "1" {
			return
		}
		t.gfx.pending = nil
		if q, ok := kv['q']; ok {
			p.kv['q'] = q
		}
		t.gfxCommand(p.kv, p.payload)
		return
	}
	if kv['m'] == "1" {
		t.gfx.pending = &gfxPending{kv: kv, payload: append([]byte(nil), payload...)}
		return
	}
	t.gfxCommand(kv, payload)
}

func parseGraphicsControl(s string) map[byte]string {
	kv := map[byte]string{}
	for _, f := range strings.Split(s, ",") {
		if len(f) >= 3 && f[1] == '=' {
			kv[f[0]] = f[2:]
		}
	}
	return kv
}

func kvInt(kv map[byte]string, k byte, def int) int {
	if v, ok := kv[k]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func kvAction(kv map[byte]string, k byte, def byte) byte {
	if v := kv[k]; v != "" {
		return v[0]
	}
	return def
}

// gfxRespond sends a reply addressed by image id or number. msg "" is OK.
func (t *Terminal) gfxRespond(kv map[byte]string, msg string) {
	id, num := kvInt(kv, 'i', 0), kvInt(kv, 'I', 0)
	if id == 0 && num == 0 {
		return
	}
	quiet := kvInt(kv, 'q', 0)
	if msg == "" {
		if quiet >= 1 {
			return
		}
		msg = "OK"
	} else if quiet >= 2 {
		return
	}
	var b strings.Builder
	b.WriteString("\x1b_G")
	var parts []string
	if id != 0 {
		parts = append(parts, "i="+strconv.Itoa(id))
	}
	if num != 0 {
		parts = append(parts, "I="+strconv.Itoa(num))
	}
	if p := kvInt(kv, 'p', 0); p != 0 {
		parts = append(parts, "p="+strconv.Itoa(p))
	}
	b.WriteString(strings.Join(parts, ","))
	b.WriteString(";" + msg + "\x1b\\")
	t.replyString(b.String())
}

func (t *Terminal) gfxCommand(kv map[byte]string, payload []byte) {
	switch a := kvAction(kv, 'a', 't'); a {
	case 't', 'T', 'q':
		t.gfxTransmit(kv, payload, a)
	case 'p':
		t.gfxPlaceCommand(kv)
	case 'd':
		t.gfxDelete(kv)
	default:
		t.gfxRespond(kv, "EINVAL:unsupported action")
	}
}

type gfxError string

func (e gfxError) Error() string { return string(e) }

func decodeImage(kv map[byte]string, payload []byte) (*Image, error) {
	if m := kvAction(kv, 't', 'd'); m != 'd' {
		return nil, gfxError("EPERM:file and shared memory transmission is not permitted")
	}
	raw, err := base64.StdEncoding.DecodeString(string(payload))
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(string(payload), "="))
		if err != nil {
			return nil, gfxError("EINVAL:invalid base64 payload")
		}
	}
	if kv['o'] == "z" {
		zr, err := zlib.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, gfxError("EINVAL:invalid zlib data")
		}
		raw, err = io.ReadAll(io.LimitReader(zr, maxImageBytes+1))
		if err != nil {
			return nil, gfxError("EINVAL:invalid zlib data")
		}
	} else if v, ok := kv['o']; ok && v != "" {
		return nil, gfxError("EINVAL:unsupported compression")
	}
	if len(raw) > maxImageBytes {
		return nil, gfxError("ENOSPC:image data too large")
	}
	format := kvInt(kv, 'f', 32)
	img := &Image{Format: format, Data: raw}
	switch format {
	case 24, 32:
		w, h := kvInt(kv, 's', 0), kvInt(kv, 'v', 0)
		if w <= 0 || h <= 0 {
			return nil, gfxError("EINVAL:image width and height are required")
		}
		bpp := format / 8
		if w > maxImageBytes/bpp/h || len(raw) != w*h*bpp {
			return nil, gfxError(fmt.Sprintf("EINVAL:data length %d does not match %dx%d", len(raw), w, h))
		}
		img.Width, img.Height = w, h
	case 100:
		cfg, err := png.DecodeConfig(bytes.NewReader(raw))
		if err != nil {
			return nil, gfxError("EBADPNG:invalid PNG data")
		}
		img.Width, img.Height = cfg.Width, cfg.Height
	default:
		return nil, gfxError("EINVAL:unsupported format")
	}
	return img, nil
}

func (t *Terminal) gfxTransmit(kv map[byte]string, payload []byte, action byte) {
	img, err := decodeImage(kv, payload)
	if err != nil {
		t.gfxRespond(kv, err.Error())
		return
	}
	if action == 'q' {
		t.gfxRespond(kv, "")
		return
	}
	g := &t.gfx
	id := uint32(kvInt(kv, 'i', 0))
	num := uint32(kvInt(kv, 'I', 0))
	if id != 0 {
		g.removeImage(id)
	}
	if g.stored+len(img.Data) > maxStoreBytes {
		t.gfxRespond(kv, "ENOSPC:image storage quota exceeded")
		return
	}
	if id == 0 {
		g.nextAuto++
		id = g.nextAuto
		if num != 0 {
			g.removeImage(g.numbers[num])
			g.numbers[num] = id
		}
	}
	img.ID = id
	g.images[id] = img
	g.order = append(g.order, id)
	g.stored += len(img.Data)
	if action == 'T' {
		if err := t.gfxPlace(kv, id); err != nil {
			t.gfxRespond(kv, err.Error())
			return
		}
	}
	t.gfxRespond(kv, "")
}

func (t *Terminal) gfxPlaceCommand(kv map[byte]string) {
	id := uint32(kvInt(kv, 'i', 0))
	if id == 0 {
		id = t.gfx.numbers[uint32(kvInt(kv, 'I', 0))]
	}
	if err := t.gfxPlace(kv, id); err != nil {
		t.gfxRespond(kv, err.Error())
		return
	}
	t.gfxRespond(kv, "")
}

func ceilDiv(a, b int) int { return (a + b - 1) / b }

// gfxPlace records a placement of image id at the cursor.
func (t *Terminal) gfxPlace(kv map[byte]string, id uint32) error {
	img, ok := t.gfx.images[id]
	if !ok {
		return gfxError("ENOENT:image not found")
	}
	cols, rows := kvInt(kv, 'c', 0), kvInt(kv, 'r', 0)
	w, h := kvInt(kv, 'w', 0), kvInt(kv, 'h', 0)
	if w <= 0 {
		w = img.Width - kvInt(kv, 'x', 0)
	}
	if h <= 0 {
		h = img.Height - kvInt(kv, 'y', 0)
	}
	if cols <= 0 {
		cols = max(1, ceilDiv(w+kvInt(kv, 'X', 0), t.cellW))
	}
	if rows <= 0 {
		rows = max(1, ceilDiv(h+kvInt(kv, 'Y', 0), t.cellH))
	}
	cur := t.scr.Cursor()
	y := cur.Y
	if !t.scr.Alternate() {
		y += t.scr.ScrollbackLen()
	}
	pl := Placement{
		ImageID: id, PlacementID: uint32(kvInt(kv, 'p', 0)),
		X: cur.X, Y: y, Cols: cols, Rows: rows, Z: kvInt(kv, 'z', 0),
	}
	b := t.bufferIndex()
	if pl.PlacementID != 0 {
		t.gfx.placements[b] = filterPlacements(t.gfx.placements[b], func(p Placement) bool {
			return p.ImageID != id || p.PlacementID != pl.PlacementID
		})
	}
	t.gfx.placements[b] = append(t.gfx.placements[b], pl)
	if kvInt(kv, 'C', 0) != 1 {
		for i := 1; i < rows; i++ {
			t.scr.LineFeed()
		}
		t.scr.MoveBy(cur.X+cols-t.scr.Cursor().X, 0)
	}
	return nil
}

func (t *Terminal) gfxDelete(kv map[byte]string) {
	spec := kvAction(kv, 'd', 'a')
	free := spec >= 'A' && spec <= 'Z'
	lower := spec | 0x20
	b := t.bufferIndex()
	g := &t.gfx
	cur := t.scr.Cursor()
	curY := cur.Y
	if !t.scr.Alternate() {
		curY += t.scr.ScrollbackLen()
	}
	covers := func(p Placement, x, y int) bool {
		return x >= p.X && x < p.X+p.Cols && y >= p.Y && y < p.Y+p.Rows
	}
	id := uint32(kvInt(kv, 'i', 0))
	var match func(Placement) bool
	switch lower {
	case 'a':
		match = func(Placement) bool { return true }
	case 'i':
		pid := uint32(kvInt(kv, 'p', 0))
		match = func(p Placement) bool { return p.ImageID == id && (pid == 0 || p.PlacementID == pid) }
	case 'n':
		nid := g.numbers[uint32(kvInt(kv, 'I', 0))]
		pid := uint32(kvInt(kv, 'p', 0))
		match = func(p Placement) bool { return p.ImageID == nid && (pid == 0 || p.PlacementID == pid) }
	case 'c':
		match = func(p Placement) bool { return covers(p, cur.X, curY) }
	case 'p':
		x, y := kvInt(kv, 'x', 1)-1, kvInt(kv, 'y', 1)-1
		match = func(p Placement) bool { return covers(p, x, y) }
	case 'x':
		x := kvInt(kv, 'x', 1) - 1
		match = func(p Placement) bool { return x >= p.X && x < p.X+p.Cols }
	case 'y':
		y := kvInt(kv, 'y', 1) - 1
		match = func(p Placement) bool { return y >= p.Y && y < p.Y+p.Rows }
	case 'z':
		z := kvInt(kv, 'z', 0)
		match = func(p Placement) bool { return p.Z == z }
	default:
		return
	}
	var removed []uint32
	g.placements[b] = filterPlacements(g.placements[b], func(p Placement) bool {
		if match(p) {
			removed = append(removed, p.ImageID)
			return false
		}
		return true
	})
	if !free {
		return
	}
	if lower == 'a' {
		for _, img := range append([]uint32(nil), g.order...) {
			if !g.referenced(img) {
				g.removeImage(img)
			}
		}
		return
	}
	sort.Slice(removed, func(i, j int) bool { return removed[i] < removed[j] })
	for i, rid := range removed {
		if (i == 0 || removed[i-1] != rid) && !g.referenced(rid) {
			g.removeImage(rid)
		}
	}
	if (lower == 'i' || lower == 'n') && len(removed) == 0 {
		target := id
		if lower == 'n' {
			target = g.numbers[uint32(kvInt(kv, 'I', 0))]
		}
		if target != 0 && !g.referenced(target) {
			g.removeImage(target)
		}
	}
}

func (g *gfxState) referenced(id uint32) bool {
	for _, ps := range g.placements {
		for _, p := range ps {
			if p.ImageID == id {
				return true
			}
		}
	}
	return false
}
