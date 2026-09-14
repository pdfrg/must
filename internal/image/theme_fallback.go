package image

import (
	"bytes"
	"fmt"
	"image"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

const (
	fallbackBackground = "#000000"
	fallbackAccent     = "#00ff00"
	fallbackForeground = "#ffffff"
)

// RenderThemedFallbackSVG injects the active theme's semantic colours into the
// fallback cover template, then rasterizes it for the terminal image renderer.
// Real album artwork never passes through this function.
func RenderThemedFallbackSVG(template []byte, width, height int, backgroundHex, accentHex, foregroundHex string) (image.Image, error) {
	if len(template) == 0 {
		return nil, fmt.Errorf("fallback SVG template is empty")
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid fallback render size %dx%d", width, height)
	}

	svg := string(template)
	replacements := map[string]string{
		"{{BACKGROUND}}": normalizeThemeColor(backgroundHex, fallbackBackground),
		"{{ACCENT}}":     normalizeThemeColor(accentHex, fallbackAccent),
		"{{FOREGROUND}}": normalizeThemeColor(foregroundHex, fallbackForeground),
	}
	for token, value := range replacements {
		svg = strings.ReplaceAll(svg, token, value)
	}
	if strings.Contains(svg, "{{") {
		return nil, fmt.Errorf("fallback SVG contains an unresolved theme token")
	}

	icon, err := oksvg.ReadIconStream(bytes.NewBufferString(svg), oksvg.StrictErrorMode)
	if err != nil {
		return nil, fmt.Errorf("parse fallback SVG: %w", err)
	}
	icon.SetTarget(0, 0, float64(width), float64(height))

	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	scanner := rasterx.NewScannerGV(width, height, dst, dst.Bounds())
	icon.Draw(rasterx.NewDasher(width, height, scanner), 1)
	return dst, nil
}

func normalizeThemeColor(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "transparent" {
		return fallback
	}
	value = strings.TrimPrefix(value, "#")
	if len(value) != 6 {
		return fallback
	}
	var r, g, b uint8
	if _, err := fmt.Sscanf(value, "%02x%02x%02x", &r, &g, &b); err != nil {
		return fallback
	}
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}
