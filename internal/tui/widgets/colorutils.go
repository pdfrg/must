package widgets

import "fmt"

func darkenColor(hex string, factor float64) string {
	if hex == "default" || len(hex) != 7 || hex[0] != '#' {
		return hex
	}
	var r, g, b int
	_, _ = fmt.Sscanf(hex[1:3], "%x", &r)
	_, _ = fmt.Sscanf(hex[3:5], "%x", &g)
	_, _ = fmt.Sscanf(hex[5:7], "%x", &b)
	r = int(float64(r) * (1 - factor))
	g = int(float64(g) * (1 - factor))
	b = int(float64(b) * (1 - factor))
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func lightenColor(hex string, factor float64) string {
	if hex == "default" || len(hex) != 7 || hex[0] != '#' {
		return hex
	}
	var r, g, b int
	_, _ = fmt.Sscanf(hex[1:3], "%x", &r)
	_, _ = fmt.Sscanf(hex[3:5], "%x", &g)
	_, _ = fmt.Sscanf(hex[5:7], "%x", &b)
	r = min(255, int(float64(r)*(1+factor)))
	g = min(255, int(float64(g)*(1+factor)))
	b = min(255, int(float64(b)*(1+factor)))
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}
