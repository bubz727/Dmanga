package image

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"net/url"
	"strings"

	_ "golang.org/x/image/webp"

	"dmanga/internal/image/descramble"
)

// DescrambleGiga reassembles scrambled GigaViewer image tiles
func DescrambleGiga(src image.Image, isSJ bool) (image.Image, error) {
	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Src)

	if isSJ {
		// Specific to Shonen Jump+ (shonenjumpplus.com)
		totalTiles := 16
		cellWidth := (w - 22) / 4
		cellHeight := (h - 16) / 4

		if cellWidth <= 0 || cellHeight <= 0 {
			return dst, nil
		}

		for i := 0; i < 16; i++ {
			row := i / 4
			col := i % 4
			sourceX := col * cellWidth
			sourceY := row * cellHeight

			destIndex := col*totalTiles + row
			destX := (destIndex % totalTiles) * cellWidth
			destY := (destIndex / totalTiles) * cellHeight

			if destX < w && destY < h && sourceX < w && sourceY < h {
				sw := cellWidth
				sh := cellHeight
				if sourceX+sw > w {
					sw = w - sourceX
				}
				if destX+sw > w {
					sw = w - destX
				}
				if sourceY+sh > h {
					sh = h - sourceY
				}
				if destY+sh > h {
					sh = h - destY
				}

				srcRect := image.Rect(sourceX, sourceY, sourceX+sw, sourceY+sh)
				destPt := image.Pt(destX, destY)
				draw.Draw(dst, image.Rectangle{Min: destPt, Max: destPt.Add(srcRect.Size())}, src, srcRect.Min, draw.Src)
			}
		}
	} else {
		// Standard GigaViewer (Tonari no Young Jump, Comic Days, etc.)
		divideNum := 4
		multiple := 8
		totalTiles := divideNum * multiple // 32
		cellWidth := (w / totalTiles) * multiple
		cellHeight := (h / totalTiles) * multiple

		if cellWidth <= 0 || cellHeight <= 0 {
			return dst, nil
		}

		for i := 0; i < totalTiles*totalTiles; i++ {
			row := i / totalTiles
			col := i % totalTiles
			sourceX := col * cellWidth
			sourceY := row * cellHeight

			destIndex := col*totalTiles + row
			destX := (destIndex % totalTiles) * cellWidth
			destY := (destIndex / totalTiles) * cellHeight

			if destX < w && destY < h && sourceX < w && sourceY < h {
				sw := cellWidth
				sh := cellHeight
				if sourceX+sw > w {
					sw = w - sourceX
				}
				if destX+sw > w {
					sw = w - destX
				}
				if sourceY+sh > h {
					sh = h - sourceY
				}
				if destY+sh > h {
					sh = h - destY
				}

				srcRect := image.Rect(sourceX, sourceY, sourceX+sw, sourceY+sh)
				destPt := image.Pt(destX, destY)
				draw.Draw(dst, image.Rectangle{Min: destPt, Max: destPt.Add(srcRect.Size())}, src, srcRect.Min, draw.Src)
			}
		}
	}

	return dst, nil
}

// ProcessImageDescramble is a general helper to process descrambling indicated by URL metadata
func ProcessImageDescramble(rawBytes []byte, imageURL string) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(rawBytes))
	if err != nil {
		// If regular image decoding fails, return raw bytes
		return rawBytes, nil
	}

	var processed image.Image = img

	// Check if Comici format (contains #scramble= metadata)
	if idx := strings.Index(imageURL, "#scramble="); idx != -1 {
		encodedScramble := imageURL[idx+len("#scramble="):]
		if amp := strings.Index(encodedScramble, "&"); amp != -1 {
			encodedScramble = encodedScramble[:amp]
		}
		rawJSON, err := url.QueryUnescape(encodedScramble)
		if err == nil {
			var mapping []int
			if err := json.Unmarshal([]byte(rawJSON), &mapping); err == nil && len(mapping) > 0 {
				processed, _ = descramble.DescrambleComici(img, mapping)
			}
		}
	} else if strings.Contains(imageURL, "#sj=true") {
		// Shonen Jump+
		processed, _ = DescrambleGiga(img, true)
	} else if strings.Contains(imageURL, "#giga=true") {
		// Standard GigaViewer
		processed, _ = DescrambleGiga(img, false)
	}

	var out bytes.Buffer
	err = jpeg.Encode(&out, processed, &jpeg.Options{Quality: 95})
	if err != nil {
		return rawBytes, fmt.Errorf("failed to encode jpeg: %w", err)
	}

	return out.Bytes(), nil
}

// EnsureJPGorPNG ensures the image data is in JPEG or PNG format.
// If it is another format (e.g. WebP, GIF), it is converted to JPEG (.jpg).
// If already JPEG or PNG, the original raw bytes are returned without re-encoding.
func EnsureJPGorPNG(rawBytes []byte, originalExt string) ([]byte, string, error) {
	// 1. Check JPEG magic bytes: FF D8 FF
	if len(rawBytes) >= 3 && rawBytes[0] == 0xFF && rawBytes[1] == 0xD8 && rawBytes[2] == 0xFF {
		return rawBytes, ".jpg", nil
	}

	// 2. Check PNG magic bytes: 89 50 4E 47 0D 0A 1A 0A
	if len(rawBytes) >= 8 && rawBytes[0] == 0x89 && rawBytes[1] == 0x50 && rawBytes[2] == 0x4E && rawBytes[3] == 0x47 &&
		rawBytes[4] == 0x0D && rawBytes[5] == 0x0A && rawBytes[6] == 0x1A && rawBytes[7] == 0x0A {
		return rawBytes, ".png", nil
	}

	// 3. If not JPG or PNG (e.g. WebP or GIF), decode and convert to JPG
	img, _, err := image.Decode(bytes.NewReader(rawBytes))
	if err != nil {
		if originalExt == "" {
			originalExt = ".jpg"
		}
		return rawBytes, originalExt, nil
	}

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95})
	if err != nil {
		return rawBytes, originalExt, nil
	}

	return buf.Bytes(), ".jpg", nil
}
