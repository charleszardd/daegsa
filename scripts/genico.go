package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

func main() {
	sizes := []int{16, 32, 48, 64, 128, 256}
	var pngBuffers [][]byte

	for _, s := range sizes {
		img := createIconImage(s)
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			panic(err)
		}
		pngBuffers = append(pngBuffers, buf.Bytes())
	}

	// Build ICO file
	var icoBuf bytes.Buffer

	// ICONDIR header
	_ = binary.Write(&icoBuf, binary.LittleEndian, uint16(0))           // Reserved
	_ = binary.Write(&icoBuf, binary.LittleEndian, uint16(1))           // Type: ICO
	_ = binary.Write(&icoBuf, binary.LittleEndian, uint16(len(sizes))) // Image count

	offset := uint32(6 + len(sizes)*16)

	for i, s := range sizes {
		data := pngBuffers[i]
		w := byte(s)
		h := byte(s)
		if s >= 256 {
			w = 0
			h = 0
		}

		_ = binary.Write(&icoBuf, binary.LittleEndian, w)
		_ = binary.Write(&icoBuf, binary.LittleEndian, h)
		_ = binary.Write(&icoBuf, binary.LittleEndian, byte(0)) // Color count
		_ = binary.Write(&icoBuf, binary.LittleEndian, byte(0)) // Reserved
		_ = binary.Write(&icoBuf, binary.LittleEndian, uint16(1)) // Planes
		_ = binary.Write(&icoBuf, binary.LittleEndian, uint16(32)) // Bit count
		_ = binary.Write(&icoBuf, binary.LittleEndian, uint32(len(data))) // Size
		_ = binary.Write(&icoBuf, binary.LittleEndian, offset) // Offset
		offset += uint32(len(data))
	}

	for _, data := range pngBuffers {
		icoBuf.Write(data)
	}

	_ = os.MkdirAll("assets", 0755)
	_ = os.WriteFile(filepath.Join("assets", "icon.ico"), icoBuf.Bytes(), 0644)
	_ = os.WriteFile(filepath.Join("cmd", "daegsa-gui", "icon.ico"), icoBuf.Bytes(), 0644)
}

func createIconImage(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	s := float64(size)

	bgDark := color.RGBA{13, 17, 23, 255}
	bgSurface := color.RGBA{22, 27, 34, 255}
	cyan := color.RGBA{88, 166, 255, 255}
	purple := color.RGBA{163, 113, 247, 255}
	amber := color.RGBA{210, 153, 34, 255}

	draw.Draw(img, img.Bounds(), &image.Uniform{bgDark}, image.Point{}, draw.Src)

	// Crest badge
	pad := s * 0.05
	cornerRadius := s * 0.22
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if isInsideRounded(float64(x), float64(y), pad, pad, s-pad, s-pad, cornerRadius) {
				img.Set(x, y, bgSurface)
			}
		}
	}

	// Bar 1 (Left Pillar)
	fillRRect(img, s*0.22, s*0.22, s*0.38, s*0.80, s*0.06, cyan)

	// Bar 2 (Middle Surge Bar)
	fillRRect(img, s*0.45, s*0.42, s*0.59, s*0.80, s*0.05, purple)

	// Bar 3 (Right Crest Bar)
	fillRRect(img, s*0.65, s*0.30, s*0.79, s*0.80, s*0.05, amber)

	return img
}

func isInsideRounded(x, y, minX, minY, maxX, maxY, r float64) bool {
	if x < minX || x > maxX || y < minY || y > maxY {
		return false
	}
	if x < minX+r && y < minY+r {
		return math.Hypot(x-(minX+r), y-(minY+r)) <= r
	}
	if x > maxX-r && y < minY+r {
		return math.Hypot(x-(maxX-r), y-(minY+r)) <= r
	}
	if x < minX+r && y > maxY-r {
		return math.Hypot(x-(minX+r), y-(maxY-r)) <= r
	}
	if x > maxX-r && y > maxY-r {
		return math.Hypot(x-(maxX-r), y-(maxY-r)) <= r
	}
	return true
}

func fillRRect(img *image.RGBA, minX, minY, maxX, maxY, r float64, c color.RGBA) {
	for y := int(minY); y <= int(maxY); y++ {
		for x := int(minX); x <= int(maxX); x++ {
			if isInsideRounded(float64(x), float64(y), minX, minY, maxX, maxY, r) {
				img.Set(x, y, c)
			}
		}
	}
}
