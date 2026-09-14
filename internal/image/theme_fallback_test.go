package image

import (
	"image/color"
	"testing"
)

const fallbackTestSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 30 10">
<rect width="10" height="10" fill="{{BACKGROUND}}"/>
<rect x="10" width="10" height="10" fill="{{ACCENT}}"/>
<rect x="20" width="10" height="10" fill="{{FOREGROUND}}"/>
</svg>`

func TestRenderThemedFallbackSVGUsesThemePalette(t *testing.T) {
	got, err := RenderThemedFallbackSVG([]byte(fallbackTestSVG), 30, 10, "#112233", "#445566", "#778899")
	if err != nil {
		t.Fatal(err)
	}

	assertPixel(t, got, 5, 5, color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff})
	assertPixel(t, got, 15, 5, color.NRGBA{R: 0x44, G: 0x55, B: 0x66, A: 0xff})
	assertPixel(t, got, 25, 5, color.NRGBA{R: 0x77, G: 0x88, B: 0x99, A: 0xff})
}

func TestRenderThemedFallbackSVGUsesSafeDefaults(t *testing.T) {
	got, err := RenderThemedFallbackSVG([]byte(fallbackTestSVG), 30, 10, "transparent", "invalid", "")
	if err != nil {
		t.Fatal(err)
	}

	assertPixel(t, got, 5, 5, color.NRGBA{A: 0xff})
	assertPixel(t, got, 15, 5, color.NRGBA{G: 0xff, A: 0xff})
	assertPixel(t, got, 25, 5, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
}

func TestRenderThemedFallbackSVGRejectsUnresolvedTokens(t *testing.T) {
	_, err := RenderThemedFallbackSVG([]byte(`<svg viewBox="0 0 1 1"><path fill="{{UNKNOWN}}" d="M0 0h1v1z"/></svg>`), 1, 1, "", "", "")
	if err == nil {
		t.Fatal("expected unresolved token error")
	}
}

func assertPixel(t *testing.T, img interface{ At(int, int) color.Color }, x, y int, want color.NRGBA) {
	t.Helper()
	if got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA); got != want {
		t.Fatalf("pixel (%d,%d) = %#v, want %#v", x, y, got, want)
	}
}
