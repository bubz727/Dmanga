package descramble

import (
	"image"
	"image/draw"
)

// DescrambleComici reassembles scrambled Comici 4x4 image tiles
func DescrambleComici(src image.Image, mapping []int) (image.Image, error) {
	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Src)

	if len(mapping) < 16 {
		return dst, nil
	}

	cols := 4
	rows := 4
	tileW := w / cols
	tileH := h / rows

	for i := 0; i < 16 && i < len(mapping); i++ {
		sourceIndex := mapping[i]

		// Column-major Source coordinates
		sx := (sourceIndex / rows) * tileW
		sy := (sourceIndex % rows) * tileH

		// Column-major Destination coordinates
		dx := (i / rows) * tileW
		dy := (i % rows) * tileH

		if sx < w && sy < h && dx < w && dy < h {
			sw := tileW
			sh := tileH
			if sx+sw > w {
				sw = w - sx
			}
			if dx+sw > w {
				sw = w - dx
			}
			if sy+sh > h {
				sh = h - sy
			}
			if dy+sh > h {
				sh = h - dy
			}

			srcRect := image.Rect(sx, sy, sx+sw, sy+sh)
			destPt := image.Pt(dx, dy)
			draw.Draw(dst, image.Rectangle{Min: destPt, Max: destPt.Add(srcRect.Size())}, src, srcRect.Min, draw.Src)
		}
	}

	return dst, nil
}
