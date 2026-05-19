package widgets

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type KeyBinding struct {
	Key   string
	Icon  string
	Label string
	Dim   bool
}

type Footer struct {
	accentStyle     lipgloss.Style
	mutedStyle      lipgloss.Style
	foregroundStyle lipgloss.Style
	dimStyle        lipgloss.Style
	width           int

	topKeys         []KeyBinding
	bottomLeftKeys  []KeyBinding
	bottomRightKeys []KeyBinding
	miniKeys        []KeyBinding

	miniMode bool

	scrobbleServices     []string
	flashStatesByService map[string]int

	lidarrConfigured bool
	lidarrState      int
}

const (
	FlashOff      = 0
	FlashSolid    = 1
	FlashBlinkOn  = 2
	FlashBlinkOff = 3
)

const (
	LidarrStateNotInLidarr = 0
	LidarrStateInLidarr    = 1
	LidarrStateMonitored   = 2
	LidarrStateError       = 3
)

func NewFooter(accentStyle, mutedStyle, foregroundStyle lipgloss.Style) *Footer {
	dimStyle := mutedStyle.Bold(false)
	return &Footer{
		accentStyle:     accentStyle,
		mutedStyle:      mutedStyle,
		foregroundStyle: foregroundStyle,
		dimStyle:        dimStyle,
		topKeys: []KeyBinding{
			{Key: "p", Icon: "󰒮", Label: ""},
			{Key: "Space", Icon: "󰐎", Label: ""},
			{Key: "\u25c0 \u25b6", Icon: "", Label: "Seek"},
			{Key: "n", Icon: "󰒭", Label: ""},
			{Key: "r", Icon: "󰑖", Label: ""},
			{Key: "s", Icon: "", Label: "󰒟"},
			{Key: "v", Icon: "", Label: "View"},
			{Key: "y", Icon: "", Label: "Lyrics"},
			{Key: "Y", Icon: "", Label: "Sync"},
			{Key: "i", Icon: "", Label: "Bio"},
			{Key: "I", Icon: "", Label: "Gal"},
			{Key: "V", Icon: "", Label: "Vis"},
		},
		bottomLeftKeys: []KeyBinding{
			{Key: "E", Icon: "", Label: "EnqN"},
			{Key: "d", Icon: "", Label: "Rem"},
			{Key: "D", Icon: "", Label: "Clr"},
			{Key: "S", Icon: "", Label: "Save"},
			{Key: "J", Icon: "", Label: "↓"},
			{Key: "K", Icon: "", Label: "↑"},
			{Key: "g", Icon: "", Label: "Top"},
			{Key: "G", Icon: "", Label: "Bot"},
		},
		bottomRightKeys: []KeyBinding{
			//			{Key: "c", Icon: "", Label: "Copy"},
			{Key: "/", Icon: "", Label: "Search"},
			{Key: "l", Icon: "", Label: "Lib"},
			{Key: "L", Icon: "", Label: "Lidarr"},
			{Key: "o", Icon: "", Label: "Opt"},
			{Key: "z", Icon: "", Label: "Zzz"},
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
	f.dimStyle = mutedStyle.Bold(false)
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

func (f *Footer) SetLidarrConfigured(configured bool) {
	f.lidarrConfigured = configured
}

func (f *Footer) SetLidarrState(state int) {
	f.lidarrState = state
}

func (f Footer) scrobbleIndicator() string {
	if len(f.scrobbleServices) == 0 {
		return ""
	}
	var parts []string
	for _, svc := range f.scrobbleServices {
		state := FlashOff
		if f.flashStatesByService != nil {
			state = f.flashStatesByService[svc]
		}
		var style lipgloss.Style
		switch state {
		case FlashSolid, FlashBlinkOn:
			style = f.accentStyle
		default:
			style = f.mutedStyle
		}
		parts = append(parts, style.Render("["+svc+"]"))
	}
	return strings.Join(parts, "")
}

func (f Footer) lidarrIndicator() string {
	if !f.lidarrConfigured {
		return ""
	}
	var style lipgloss.Style
	switch f.lidarrState {
	case LidarrStateMonitored:
		style = f.accentStyle
	case LidarrStateInLidarr:
		style = f.foregroundStyle
	case LidarrStateError:
		style = f.mutedStyle
	default:
		style = f.mutedStyle
	}
	return style.Render("[L]")
}

func (f Footer) View() string {
	renderLine := func(bindings []KeyBinding) string {
		var parts []string
		for _, kb := range bindings {
			if kb.Key == "L" && !f.lidarrConfigured {
				continue
			}
			keyStyle := f.accentStyle
			descStyle := f.mutedStyle
			if kb.Dim {
				keyStyle = f.dimStyle
				descStyle = f.dimStyle
			}
			keyPart := keyStyle.Render(kb.Key)
			var descPart string
			if kb.Icon != "" {
				descPart = descStyle.Render(kb.Icon)
			} else if kb.Label != "" {
				descPart = descStyle.Render(kb.Label)
			}
			if descPart != "" {
				parts = append(parts, keyPart+" "+descPart)
			} else {
				parts = append(parts, keyPart)
			}
		}
		return strings.Join(parts, " ")
	}

	centerLine := func(content string) string {
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
		if ind := f.lidarrIndicator(); ind != "" {
			line += " " + ind
		}
		return centerLine(line)
	}

	topLine := renderLine(f.topKeys)
	if ind := f.scrobbleIndicator(); ind != "" {
		topLine += " " + ind
	}
	if ind := f.lidarrIndicator(); ind != "" {
		topLine += " " + ind
	}

	leftPart := renderLine(f.bottomLeftKeys)
	rightPart := renderLine(f.bottomRightKeys)
	separator := f.mutedStyle.Render(" | ")
	bottomLine := leftPart + " " + separator + " " + rightPart

	return centerLine(topLine) + "\n" + centerLine(bottomLine)
}
