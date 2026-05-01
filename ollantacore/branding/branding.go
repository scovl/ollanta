// Package branding provides shared Ollanta brand assets for runtime UIs.
package branding

import (
	"embed"
)

//go:embed assets/ollanta-mark.png
var assetFiles embed.FS

var markPNG = mustReadMark()

// MarkPNG returns the shared Ollanta brand mark bytes.
func MarkPNG() []byte {
	return markPNG
}

func mustReadMark() []byte {
	data, err := assetFiles.ReadFile("assets/ollanta-mark.png")
	if err != nil {
		panic("branding: read mark asset: " + err.Error())
	}
	return data
}
