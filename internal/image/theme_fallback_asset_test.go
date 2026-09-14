package image_test

import (
	"image/color"
	"testing"

	"github.com/pdfrg/must/assets"
	imagepkg "github.com/pdfrg/must/internal/image"
)

func TestEmbeddedFallbackSVGIsRenderable(t *testing.T) {
	img, err := imagepkg.RenderThemedFallbackSVG(
		assets.BubblesLogoSVG,
		1024,
		1024,
		"#1e1e2e",
		"#89b4fa",
		"#cdd6f4",
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Size(); got.X != 1024 || got.Y != 1024 {
		t.Fatalf("rendered size = %v, want (1024,1024)", got)
	}

	assertPixel := func(x, y int, want color.NRGBA) {
		t.Helper()
		if got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA); got != want {
			t.Fatalf("pixel (%d,%d) = %#v, want %#v", x, y, got, want)
		}
	}
	background := color.NRGBA{R: 0x1e, G: 0x1e, B: 0x2e, A: 0xff}
	assertPixel(500, 100, background)
	assertPixel(150, 500, color.NRGBA{R: 0x89, G: 0xb4, B: 0xfa, A: 0xff})
	assertPixel(279, 77, color.NRGBA{R: 0xcd, G: 0xd6, B: 0xf4, A: 0xff})
	assertPixel(279, 107, background)
}
