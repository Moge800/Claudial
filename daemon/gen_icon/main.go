// gen_icon generates a minimal 16x16 ICO file for the systray icon.
// Run: go run ./gen_icon
package main

import (
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"bytes"
	"os"
)

func main() {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	cx, cy, r := 8.0, 8.0, 6.0
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, color.NRGBA{0x00, 0xCC, 0x44, 0xFF}) // Clawdial green
			}
		}
	}

	// PNG encode
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		panic(err)
	}
	pngData := pngBuf.Bytes()

	// ICO format: ICONDIR + ICONDIRENTRY + PNG data
	f, err := os.Create("../icon.ico")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	pngSize := uint32(len(pngData))
	dataOffset := uint32(6 + 16) // ICONDIR(6) + ICONDIRENTRY(16)

	// ICONDIR
	binary.Write(f, binary.LittleEndian, uint16(0))     // reserved
	binary.Write(f, binary.LittleEndian, uint16(1))     // type: ICO
	binary.Write(f, binary.LittleEndian, uint16(1))     // count

	// ICONDIRENTRY
	f.Write([]byte{16, 16, 0, 0})                       // width, height, colorCount, reserved
	binary.Write(f, binary.LittleEndian, uint16(1))     // planes
	binary.Write(f, binary.LittleEndian, uint16(32))    // bitCount
	binary.Write(f, binary.LittleEndian, pngSize)       // size
	binary.Write(f, binary.LittleEndian, dataOffset)    // offset

	// PNG image data
	f.Write(pngData)
}
