// gen_icon generates a minimal 16x16 ICO file for the systray icon.
// Run from the daemon/ directory: go run ./gen_icon
// Output: daemon/icon.ico
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	cx, cy, r := 8.0, 8.0, 6.0
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, color.NRGBA{0x00, 0xCC, 0x44, 0xFF}) // Claudial green
			}
		}
	}

	var pngBuf bytes.Buffer
	must(png.Encode(&pngBuf, img))
	pngData := pngBuf.Bytes()

	// カレントディレクトリに icon.ico を書き込む。
	// daemon/ から "go run ./gen_icon" で実行すること（コメント冒頭参照）。
	// Write icon.ico to the current working directory.
	// Run as "go run ./gen_icon" from the daemon/ directory (see file header).
	f, err := os.Create("icon.ico")
	must(err)
	defer f.Close()

	pngSize := uint32(len(pngData))
	dataOffset := uint32(6 + 16) // ICONDIR(6) + ICONDIRENTRY(16)

	write := func(v any) { must(binary.Write(f, binary.LittleEndian, v)) }
	writeB := func(b []byte) { _, err := f.Write(b); must(err) }

	// ICONDIR
	write(uint16(0)) // reserved
	write(uint16(1)) // type: ICO
	write(uint16(1)) // count

	// ICONDIRENTRY
	writeB([]byte{16, 16, 0, 0}) // width, height, colorCount, reserved
	write(uint16(1))             // planes
	write(uint16(32))            // bitCount
	write(pngSize)               // size
	write(dataOffset)            // offset

	writeB(pngData)
	fmt.Println("icon.ico written")
}
