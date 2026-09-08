//go:build ignore

// 售后示例的报错屏幕图生成器：设备屏幕风格的 E-13 / E-99 报错图。
// 纯标准库 + 手写 5x7 点阵字体，运行：go run examples/after-sales-support/genscreen.go out1.png out2.png
package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

var glyphs = map[rune][7]string{
	'E': {"11111", "10000", "10000", "11110", "10000", "10000", "11111"},
	'-': {"00000", "00000", "00000", "11111", "00000", "00000", "00000"},
	'1': {"00100", "01100", "00100", "00100", "00100", "00100", "01110"},
	'3': {"11110", "00001", "00001", "01110", "00001", "00001", "11110"},
	'9': {"01110", "10001", "10001", "01111", "00001", "00001", "01110"},
	'B': {"11110", "10001", "10001", "11110", "10001", "10001", "11110"},
	'R': {"11110", "10001", "10001", "11110", "10100", "10010", "10001"},
	'U': {"10001", "10001", "10001", "10001", "10001", "10001", "01110"},
	'S': {"01111", "10000", "10000", "01110", "00001", "00001", "11110"},
	'H': {"10001", "10001", "10001", "11111", "10001", "10001", "10001"},
	'O': {"01110", "10001", "10001", "10001", "10001", "10001", "01110"},
	'V': {"10001", "10001", "10001", "10001", "10001", "01010", "00100"},
	'L': {"10000", "10000", "10000", "10000", "10000", "10000", "11111"},
	'A': {"01110", "10001", "10001", "11111", "10001", "10001", "10001"},
	'D': {"11110", "10001", "10001", "10001", "10001", "10001", "11110"},
	'M': {"10001", "11011", "10101", "10101", "10001", "10001", "10001"},
	'T': {"11111", "00100", "00100", "00100", "00100", "00100", "00100"},
	'N': {"10001", "11001", "10101", "10011", "10001", "10001", "10001"},
	'I': {"01110", "00100", "00100", "00100", "00100", "00100", "01110"},
}

func drawText(img *image.RGBA, text string, scale, x, y int, c color.RGBA) {
	for _, r := range text {
		g, ok := glyphs[r]
		if !ok {
			x += 6 * scale
			continue
		}
		for row := 0; row < 7; row++ {
			for col := 0; col < 5; col++ {
				if g[row][col] == '1' {
					for dy := 0; dy < scale; dy++ {
						for dx := 0; dx < scale; dx++ {
							img.Set(x+col*scale+dx, y+row*scale+dy, c)
						}
					}
				}
			}
		}
		x += 6 * scale
	}
}

func centeredWidth(text string, scale int) int { return len(text)*6*scale - scale }

func render(path, code, sub string) {
	const W, H = 480, 320
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	bg := color.RGBA{12, 16, 28, 255}
	amber := color.RGBA{255, 176, 32, 255}
	dim := color.RGBA{120, 130, 150, 255}
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			img.Set(x, y, bg)
		}
	}
	for x := 8; x < W-8; x++ {
		img.Set(x, 8, dim)
		img.Set(x, H-9, dim)
	}
	for y := 8; y < H-8; y++ {
		img.Set(8, y, dim)
		img.Set(W-9, y, dim)
	}
	for i := 0; i < 14; i++ {
		for j := 0; j <= i*2; j++ {
			img.Set(W/2-40-i+j-14, 40+i, amber)
		}
	}
	drawText(img, code, 10, (W-centeredWidth(code, 10))/2, 120, amber)
	drawText(img, sub, 4, (W-centeredWidth(sub, 4))/2, 220, dim)
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}

func main() {
	render(os.Args[1], "E-13", "BRUSH OVERLOAD")
	render(os.Args[2], "E-99", "MOTOR BURNOUT")
}
