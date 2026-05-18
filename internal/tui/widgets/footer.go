package widgets

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type KeyBinding struct {
	Key   string
	Icon  string
	Label string
}

type Footer struct {
	accentStyle lipgloss.Style
	mutedStyle  lipgloss.Style
	width       int

	keys     []KeyBinding
	miniKeys []KeyBinding

	miniMode bool

	scrobbleServices     []string
	flashStatesByService map[string]int
}

const (
	flashOff     = 0
	flashSolid   = 1
	flashBlinkOn = 2
)

func NewFooter(accentStyle, mutedStyle lipgloss.Style) *Footer {
	return &Footer{
		accentStyle: accentStyle,
		mutedStyle:  mutedStyle,
		keys: []KeyBinding{
			{Key: "p", Icon: "󰒮", Label: ""},
			{Key: "Space", Icon: "󰐎", Label: ""},
			{Key: "\u25c0 \u25b6", Icon: "", Label: "Seek"},
			{Key: "n", Icon: "󰒭", Label: ""},
			{Key: "r", Icon: "󰑖", Label: ""},
			{Key: "s", Icon: "", Label: "󰒟"},
			{Key: "v", Icon: "", Label: "View"},
			{Key: "/", Icon: "", Label: "Search"},
			{Key: "l", Icon: "", Label: "Lib"},
			{Key: "E", Icon: "", Label: "EnqN"},
			{Key: "d", Icon: "", Label: "Rem"},
			{Key: "D", Icon: "", Label: "Clr"},
			{Key: "S", Icon: "", Label: "Save"},
			{Key: "R", Icon: "", Label: "Rescan"},
			{Key: "?", Icon: "", Label: "Help"},
			{Key: "q", Icon: "", Label: "Quit"},
		},
		miniKeys: []KeyBinding{
			{Key: "p", Icon: "󰒮", Label: ""},
			{Key: "Space", Icon: "󰐎", Label: ""},
			{Key: "n", Icon: "󰒭", Label: ""},
			{Key: "q", Icon: "", Label: "Quit"},
		},
	}
}

func (f *Footer) SetWidth(width int) {
	f.width = width
}

func (f *Footer) GetWidth() int {
	return f.width
}

func (f *Footer) UpdateStyles(accentStyle, mutedStyle lipgloss.Style) {
	f.accentStyle = accentStyle
	f.mutedStyle = mutedStyle
}

func (f *Footer) SetScrobbleServices(services []string) {
	f.scrobbleServices = services
}

func (f *Footer) SetFlashStateByService(states map[string]int) {
	f.flashStatesByService = states
}

func (f *Footer) SetMiniMode(mini bool) {
	f.miniMode = mini
}

func (f Footer) scrobbleIndicator() string {
	if len(f.scrobbleServices) == 0 {
		return ""
	}
	var parts []string
	for _, svc := range f.scrobbleServices {
		state := flashOff
		if f.flashStatesByService != nil {
			state = f.flashStatesByService[svc]
		}
		var style lipgloss.Style
		switch state {
		case flashSolid:
			style = f.accentStyle
		case flashBlinkOn:
			style = f.accentStyle
		default:
			style = f.mutedStyle
		}
		parts = append(parts, style.Render("["+svc+"]"))
	}
	return strings.Join(parts, "")
}

func (f Footer) View() string {
	renderLine := func(bindings []KeyBinding) string {
		var parts []string
		for _, kb := range bindings {
			keyPart := f.accentStyle.Render(kb.Key)
			var descPart string
			if kb.Icon != "" {
				descPart = f.mutedStyle.Render(kb.Icon)
			} else if kb.Label != "" {
				descPart = f.mutedStyle.Render(kb.Label)
			}
			if descPart != "" {
				parts = append(parts, keyPart+" "+descPart)
			} else {
				parts = append(parts, keyPart)
			}
		}
		content := strings.Join(parts, " ")
		if f.width > 0 {
			contentWidth := lipgloss.Width(content)
			if f.width > contentWidth {
				padding := (f.width - contentWidth) / 2
				content = strings.Repeat(" ", padding) + content
			}
		}
		return content
	}

	if f.miniMode {
		line := renderLine(f.miniKeys)
		if ind := f.scrobbleIndicator(); ind != "" {
			line += " " + ind
		}
		return line
	}

	controlsLine := renderLine(f.keys)
	if ind := f.scrobbleIndicator(); ind != "" {
		controlsLine += " " + ind
	}
	return controlsLine
}
