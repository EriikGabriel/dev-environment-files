package icons

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/draw"
	"image/png"
	"math"
)

// Canvas is the side of the drawn icon, the largest size of an .ico.
const Canvas = 256

// body is the side of the rounded square. Apple's grid leaves ~10% around the
// shape for the shadow; on Windows the icons are small, so it is a bit wider.
const body = 236.0

// squircle returns the coverage (0..1) of a superellipse |x|^n + |y|^n <= r^n
// centred on (cx, cy), with 3x3 supersampling for smooth edges.
func squircle(size int, r, cx, cy, n float64) []float32 {
	m := make([]float32, size*size)
	const ss = 3
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			in := 0
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					px := (float64(x) + (float64(sx)+.5)/ss - cx) / r
					py := (float64(y) + (float64(sy)+.5)/ss - cy) / r
					if math.Pow(math.Abs(px), n)+math.Pow(math.Abs(py), n) <= 1 {
						in++
					}
				}
			}
			m[y*size+x] = float32(in) / (ss * ss)
		}
	}
	return m
}

// boxBlur blurs one channel in place, horizontally then vertically.
func boxBlur(a []float32, w, h, r int) {
	tmp := make([]float32, len(a))
	pass := func(src, dst []float32, step, n, lines, lineStep int) {
		for l := 0; l < lines; l++ {
			base := l * lineStep
			var sum float32
			for i := -r; i <= r; i++ {
				if j := i; j >= 0 && j < n {
					sum += src[base+j*step]
				}
			}
			for i := 0; i < n; i++ {
				dst[base+i*step] = sum / float32(2*r+1)
				if add := i + r + 1; add < n {
					sum += src[base+add*step]
				}
				if del := i - r; del >= 0 {
					sum -= src[base+del*step]
				}
			}
		}
	}
	pass(a, tmp, 1, w, h, w)
	pass(tmp, a, w, h, w, 1)
}

// resize scales with a box filter on premultiplied colour, which is what
// keeps small sizes of an icon clean.
func resize(src *image.NRGBA, w, h int) *image.NRGBA {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	fx, fy := float64(sw)/float64(w), float64(sh)/float64(h)
	for y := 0; y < h; y++ {
		y0, y1 := float64(y)*fy, float64(y+1)*fy
		for x := 0; x < w; x++ {
			x0, x1 := float64(x)*fx, float64(x+1)*fx
			var r, g, b, a, wt float64
			for yy := int(y0); yy < int(math.Ceil(y1)) && yy < sh; yy++ {
				wy := math.Min(y1, float64(yy+1)) - math.Max(y0, float64(yy))
				for xx := int(x0); xx < int(math.Ceil(x1)) && xx < sw; xx++ {
					wx := math.Min(x1, float64(xx+1)) - math.Max(x0, float64(xx))
					i := src.PixOffset(sb.Min.X+xx, sb.Min.Y+yy)
					pa := float64(src.Pix[i+3]) / 255
					k := wx * wy
					r += float64(src.Pix[i]) * pa * k
					g += float64(src.Pix[i+1]) * pa * k
					b += float64(src.Pix[i+2]) * pa * k
					a += pa * k
					wt += k
				}
			}
			if a <= 0 || wt <= 0 {
				continue
			}
			o := dst.PixOffset(x, y)
			dst.Pix[o] = uint8(math.Round(r / a))
			dst.Pix[o+1] = uint8(math.Round(g / a))
			dst.Pix[o+2] = uint8(math.Round(b / a))
			dst.Pix[o+3] = uint8(math.Round(a / wt * 255))
		}
	}
	return dst
}

func toNRGBA(img image.Image) *image.NRGBA {
	if n, ok := img.(*image.NRGBA); ok {
		return n
	}
	b := img.Bounds()
	n := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(n, n.Bounds(), img, b.Min, draw.Src)
	return n
}

// glyph crops the transparent border of src and scales what is left to fit
// in a limit x limit box, keeping the proportions.
func glyph(src image.Image, limit int) *image.NRGBA {
	n := toNRGBA(src)
	b := n.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if n.Pix[n.PixOffset(x, y)+3] > 12 {
				minX, maxX = min(minX, x), max(maxX, x+1)
				minY, maxY = min(minY, y), max(maxY, y+1)
			}
		}
	}
	if maxX <= minX || maxY <= minY {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}
	crop := n.SubImage(image.Rect(minX, minY, maxX, maxY)).(*image.NRGBA)
	w, h := maxX-minX, maxY-minY
	scale := float64(limit) / float64(maxInt(w, h))
	return resize(crop, maxInt(1, int(math.Round(float64(w)*scale))), maxInt(1, int(math.Round(float64(h)*scale))))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// over composites a straight colour with coverage a onto a premultiplied pixel.
func over(c []float32, i int, r, g, b, a float32) {
	k := 1 - a
	c[i] = r*a + c[i]*k
	c[i+1] = g*a + c[i+1]*k
	c[i+2] = b*a + c[i+2]*k
	c[i+3] = a + c[i+3]*k
}

type plate struct{ top, bottom [3]float32 }

var plates = map[string]plate{
	StyleLight: {top: [3]float32{1, 1, 1}, bottom: [3]float32{.87, .88, .91}},
	StyleDark:  {top: [3]float32{.30, .31, .35}, bottom: [3]float32{.12, .12, .15}},
}

// Render draws src as a macOS-style icon: the original on a rounded-square
// plate with a soft shadow, a rim of light and a shadow under the glyph.
func Render(src image.Image, style string) *image.NRGBA {
	const S = Canvas
	p, ok := plates[style]
	if !ok {
		p = plates[StyleLight]
	}
	cx, cy := S/2.0, S/2.0-2
	mask := squircle(S, body/2, cx, cy, 5)
	canvas := make([]float32, S*S*4)

	// shadow of the plate
	sh := make([]float32, S*S)
	for y := 0; y+5 < S; y++ {
		copy(sh[(y+5)*S:(y+6)*S], mask[y*S:(y+1)*S])
	}
	boxBlur(sh, S, S, 4)
	boxBlur(sh, S, S, 4)
	for i, a := range sh {
		over(canvas, i*4, 0, 0, 0, a*.30)
	}

	// plate
	top := cy - body/2
	for y := 0; y < S; y++ {
		t := clamp01(float32((float64(y) - top) / body))
		r := p.top[0] + (p.bottom[0]-p.top[0])*t
		g := p.top[1] + (p.bottom[1]-p.top[1])*t
		b := p.top[2] + (p.bottom[2]-p.top[2])*t
		for x := 0; x < S; x++ {
			if m := mask[y*S+x]; m > 0 {
				over(canvas, (y*S+x)*4, r, g, b, m)
			}
		}
	}

	// rim: a thin line of light on the upper edge and of shade on the lower
	inner := squircle(S, body/2-1.8, cx, cy, 5)
	light := float32(.9)
	if style == StyleDark {
		light = .28
	}
	for y := 0; y < S; y++ {
		t := clamp01(float32((float64(y) - top) / body))
		for x := 0; x < S; x++ {
			rim := clamp01(mask[y*S+x] - inner[y*S+x])
			if rim == 0 {
				continue
			}
			if t < .55 {
				over(canvas, (y*S+x)*4, 1, 1, 1, rim*light*(1-t/.55))
			} else {
				over(canvas, (y*S+x)*4, 0, 0, 0, rim*.16*((t-.55)/.45))
			}
		}
	}

	// glyph and its shadow
	g := glyph(src, 151) // 64% of the plate
	gb := g.Bounds()
	ox, oy := int(cx)-gb.Dx()/2, int(cy)-gb.Dy()/2
	gs := make([]float32, S*S)
	for y := 0; y < gb.Dy(); y++ {
		for x := 0; x < gb.Dx(); x++ {
			X, Y := ox+x, oy+y+3
			if X >= 0 && X < S && Y >= 0 && Y < S {
				gs[Y*S+X] = float32(g.Pix[g.PixOffset(x, y)+3]) / 255
			}
		}
	}
	boxBlur(gs, S, S, 3)
	boxBlur(gs, S, S, 3)
	for i, a := range gs {
		if a > 0 && mask[i] > 0 {
			over(canvas, i*4, 0, 0, 0, a*.22*mask[i])
		}
	}
	for y := 0; y < gb.Dy(); y++ {
		for x := 0; x < gb.Dx(); x++ {
			X, Y := ox+x, oy+y
			if X < 0 || X >= S || Y < 0 || Y >= S {
				continue
			}
			o := g.PixOffset(x, y)
			if a := float32(g.Pix[o+3]) / 255; a > 0 {
				over(canvas, (Y*S+X)*4, float32(g.Pix[o])/255, float32(g.Pix[o+1])/255, float32(g.Pix[o+2])/255, a)
			}
		}
	}

	out := image.NewNRGBA(image.Rect(0, 0, S, S))
	for i := 0; i < S*S; i++ {
		a := canvas[i*4+3]
		if a <= 0 {
			continue
		}
		out.Pix[i*4] = uint8(math.Round(float64(clamp01(canvas[i*4]/a)) * 255))
		out.Pix[i*4+1] = uint8(math.Round(float64(clamp01(canvas[i*4+1]/a)) * 255))
		out.Pix[i*4+2] = uint8(math.Round(float64(clamp01(canvas[i*4+2]/a)) * 255))
		out.Pix[i*4+3] = uint8(math.Round(float64(clamp01(a)) * 255))
	}
	return out
}

// icoSizes are the sizes Windows asks for: the notification area, the taskbar,
// lists, the desktop and the large view.
var icoSizes = []int{256, 128, 64, 48, 32, 24, 16}

// ICO encodes img (any size; it is fitted to 256 first) as an .ico with PNG
// images at every size.
func ICO(img image.Image) ([]byte, error) {
	n := toNRGBA(img)
	if b := n.Bounds(); b.Dx() != Canvas || b.Dy() != Canvas {
		n = resize(n, Canvas, Canvas)
	}
	var images [][]byte
	for _, s := range icoSizes {
		var buf bytes.Buffer
		im := n
		if s != Canvas {
			im = resize(n, s, s)
		}
		if err := png.Encode(&buf, im); err != nil {
			return nil, err
		}
		images = append(images, buf.Bytes())
	}
	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, [3]uint16{0, 1, uint16(len(icoSizes))})
	offset := 6 + 16*len(icoSizes)
	for i, s := range icoSizes {
		dim := byte(s)
		if s >= 256 {
			dim = 0
		}
		out.Write([]byte{dim, dim, 0, 0})
		binary.Write(&out, binary.LittleEndian, [2]uint16{1, 32})
		binary.Write(&out, binary.LittleEndian, [2]uint32{uint32(len(images[i])), uint32(offset)})
		offset += len(images[i])
	}
	for _, im := range images {
		out.Write(im)
	}
	return out.Bytes(), nil
}

// FromICO returns the largest PNG image of an .ico, which is what the
// preview of a pack icon shows. Icons that only hold bitmaps give false.
func FromICO(data []byte) (image.Image, bool) {
	if len(data) < 6 || binary.LittleEndian.Uint16(data[2:4]) != 1 {
		return nil, false
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	var best image.Image
	bestW := 0
	for i := 0; i < count && 6+16*(i+1) <= len(data); i++ {
		e := data[6+16*i : 6+16*(i+1)]
		size := int(binary.LittleEndian.Uint32(e[8:12]))
		off := int(binary.LittleEndian.Uint32(e[12:16]))
		if off < 0 || size < 8 || off+size > len(data) || !bytes.HasPrefix(data[off:], []byte("\x89PNG")) {
			continue
		}
		im, err := png.Decode(bytes.NewReader(data[off : off+size]))
		if err != nil {
			continue
		}
		if w := im.Bounds().Dx(); w > bestW {
			best, bestW = im, w
		}
	}
	return best, best != nil
}

// DataURL is a PNG of img at side x side, for the preview in the app.
func DataURL(img image.Image, side int) string {
	n := toNRGBA(img)
	if b := n.Bounds(); b.Dx() != side || b.Dy() != side {
		n = resize(n, side, side)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, n); err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}
