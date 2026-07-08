package illustrator

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// createTestPNG 创建一个 4x4 红色 PNG 用于测试。
func createTestPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}
