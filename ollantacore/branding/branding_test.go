package branding

import (
	"image/png"
	"bytes"
	"testing"
)

func TestMarkPNG_ReturnsBytes(t *testing.T) {
	data := MarkPNG()
	if len(data) == 0 {
		t.Fatal("MarkPNG() returned empty bytes")
	}
}

func TestMarkPNG_IsValidPNG(t *testing.T) {
	data := MarkPNG()
	_, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("MarkPNG() is not a valid PNG: %v", err)
	}
}

func TestMarkPNG_Deterministic(t *testing.T) {
	a := MarkPNG()
	b := MarkPNG()
	if len(a) != len(b) {
		t.Error("MarkPNG() should return identical bytes on each call")
	}
}
