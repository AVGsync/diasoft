package qr

import (
	"fmt"
	"strings"

	"github.com/skip2/go-qrcode"
)

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) PNG(content string, size int) ([]byte, error) {
	code, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return nil, err
	}
	return code.PNG(size)
}

func (g *Generator) SVG(content string, moduleSize int) ([]byte, error) {
	code, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return nil, err
	}

	bitmap := code.Bitmap()
	dimension := len(bitmap) * moduleSize

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`,
		dimension,
		dimension,
	))
	builder.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="#ffffff"/>`, dimension, dimension))

	for y, row := range bitmap {
		for x, value := range row {
			if !value {
				continue
			}

			builder.WriteString(fmt.Sprintf(
				`<rect x="%d" y="%d" width="%d" height="%d" fill="#000000"/>`,
				x*moduleSize,
				y*moduleSize,
				moduleSize,
				moduleSize,
			))
		}
	}

	builder.WriteString(`</svg>`)
	return []byte(builder.String()), nil
}
