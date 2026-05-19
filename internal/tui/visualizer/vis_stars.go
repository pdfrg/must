package visualizer

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func (v *Visualizer) renderStars(width int) string {
	height := v.rows

	energy := 0.0
	for _, b := range v.bands {
		energy += b
	}
	energy /= float64(len(v.bands))

	styles := []lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color(v.colorHigh)),
		lipgloss.NewStyle().Foreground(lipgloss.Color(v.colorLow)),
		lipgloss.NewStyle().Foreground(lipgloss.Color(v.interpolateColor(v.colorLow, v.colorHigh, 0.5))),
	}

	lines := make([]string, height)
	for row := range height {
		var b strings.Builder
		for col := range width {
			locSeed := uint64(row)*73856093 ^ uint64(col)*19349663
			locSeed = locSeed*6364136223846793005 + 1442695040888963407
			locHash := locSeed >> 56

			if locHash > 102 {
				b.WriteString(" ")
				continue
			}

			timeSeed := locHash ^ uint64(v.frame/8)*83492791
			timeSeed = timeSeed*6364136223846793005 + 1442695040888963407
			timeHash := timeSeed >> 56

			visibleThreshold := uint64(3 + energy*61)
			if timeHash >= visibleThreshold {
				b.WriteString(" ")
				continue
			}

			styleIdx := int(locHash>>4) % 3
			b.WriteString(styles[styleIdx].Render("·"))
		}
		lines[row] = b.String()
	}
	return strings.Join(lines, "\n")
}
