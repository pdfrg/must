package modals

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/pdfrg/must/internal/config"
)

type OptionsMsg struct {
	ShowAlbumArt          *bool
	CopyAlbumArt          *bool
	NotificationsEnabled  *bool
	NotificationsShowArt  *bool
	TransparentBackground *bool
	DisableTheme          *bool
	VisualizerMode        *string
	VisualizerShowInfo    *string
	RealAudio             *bool
	Theme                 *string
	ReplayGainMode        *string
	Closed                bool
}

const (
	optShowAlbumArt = iota
	optCopyAlbumArt
	optNotificationsEnabled
	optNotificationsShowArt
	optTransparentBackground
	optDisableTheme
	optVisualizerMode
	optVisualizerShowInfo
	optRealAudio
	optTheme
	optReplayGain
)

var visualizerModeNames = []string{
	"Bars", "Braille", "ClassicPeak", "Wave",
	"Stars", "BrailleBars", "Rain", "Segmented", "Binary",
}

var visualizerShowInfoOptions = []string{"fade", "on", "off"}

var replayGainModeNames = []string{"Off", "Track", "Album"}

func themeOptions() []string {
	names := config.ThemeNames()
	opts := make([]string, 0, len(names)+1)
	opts = append(opts, "default")
	opts = append(opts, names...)
	return opts
}

func themeIndexFromConfig(themeName string) int {
	if themeName == "" {
		return 0
	}
	opts := themeOptions()
	for i, t := range opts {
		if t == themeName {
			return i
		}
	}
	return 0
}

type Options struct {
	styles *config.ThemeStyles
	width  int
	height int
	cursor int

	showAlbumArt         bool
	copyAlbumArt         bool
	notificationsEnabled bool
	notificationsShowArt bool
	transparentBg        bool
	disableTheme         bool
	visModeIdx           int
	visShowInfoIdx       int
	realAudio            bool
	themeIdx             int
	replayGainIdx        int

	origShowAlbumArt         bool
	origCopyAlbumArt         bool
	origNotificationsEnabled bool
	origNotificationsShowArt bool
	origTransparentBg        bool
	origDisableTheme         bool
	origVisModeIdx           int
	origVisShowInfoIdx       int
	origRealAudio            bool
	origThemeIdx             int
	origReplayGainIdx        int
}

func NewOptions(
	styles *config.ThemeStyles,
	showAlbumArt, copyAlbumArt bool,
	notificationsEnabled, notificationsShowArt bool,
	transparentBg, disableTheme bool,
	visMode, visShowInfo string,
	realAudio bool,
	themeName string,
	replayGainMode string,
) *Options {
	visModeIdx := 0
	for i, name := range visualizerModeNames {
		if name == visMode {
			visModeIdx = i
			break
		}
	}

	visShowInfoIdx := 0
	for i, name := range visualizerShowInfoOptions {
		if name == visShowInfo {
			visShowInfoIdx = i
			break
		}
	}

	themeIdx := themeIndexFromConfig(themeName)

	replayGainIdx := 0
	for i, name := range replayGainModeNames {
		if strings.EqualFold(name, replayGainMode) {
			replayGainIdx = i
			break
		}
	}

	return &Options{
		styles: styles,

		showAlbumArt:         showAlbumArt,
		copyAlbumArt:         copyAlbumArt,
		notificationsEnabled: notificationsEnabled,
		notificationsShowArt: notificationsShowArt,
		transparentBg:        transparentBg,
		disableTheme:         disableTheme,
		visModeIdx:           visModeIdx,
		visShowInfoIdx:       visShowInfoIdx,
		realAudio:            realAudio,
		themeIdx:             themeIdx,
		replayGainIdx:        replayGainIdx,

		origShowAlbumArt:         showAlbumArt,
		origCopyAlbumArt:         copyAlbumArt,
		origNotificationsEnabled: notificationsEnabled,
		origNotificationsShowArt: notificationsShowArt,
		origTransparentBg:        transparentBg,
		origDisableTheme:         disableTheme,
		origVisModeIdx:           visModeIdx,
		origVisShowInfoIdx:       visShowInfoIdx,
		origRealAudio:            realAudio,
		origThemeIdx:             themeIdx,
		origReplayGainIdx:        replayGainIdx,
	}
}

func (o *Options) SetSize(width, height int) {
	o.width = width
	o.height = height
}

func (o *Options) visibleItems() []int {
	return []int{
		optShowAlbumArt,
		optCopyAlbumArt,
		optNotificationsEnabled,
		optNotificationsShowArt,
		optTransparentBackground,
		optDisableTheme,
		optVisualizerMode,
		optVisualizerShowInfo,
		optRealAudio,
		optTheme,
		optReplayGain,
	}
}

func (o *Options) currentOptID() int {
	items := o.visibleItems()
	if o.cursor >= 0 && o.cursor < len(items) {
		return items[o.cursor]
	}
	return items[0]
}

func (o *Options) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "q":
			return func() tea.Msg { return OptionsMsg{Closed: true} }

		case "up", "k":
			if o.cursor > 0 {
				o.cursor--
			}

		case "down", "j":
			if o.cursor < len(o.visibleItems())-1 {
				o.cursor++
			}

		case "left", "h":
			o.cycleLeft()

		case "right", "l":
			o.cycleRight()

		case "enter", " ":
			switch o.currentOptID() {
			case optShowAlbumArt:
				o.showAlbumArt = !o.showAlbumArt
			case optCopyAlbumArt:
				o.copyAlbumArt = !o.copyAlbumArt
			case optNotificationsEnabled:
				o.notificationsEnabled = !o.notificationsEnabled
			case optNotificationsShowArt:
				o.notificationsShowArt = !o.notificationsShowArt
			case optTransparentBackground:
				o.transparentBg = !o.transparentBg
			case optDisableTheme:
				o.disableTheme = !o.disableTheme
			case optVisualizerMode:
				o.cycleRight()
			case optVisualizerShowInfo:
				o.cycleRight()
			case optRealAudio:
				o.realAudio = !o.realAudio
			case optTheme:
				o.cycleRight()
			case optReplayGain:
				o.cycleRight()
			}

		case "a":
			return o.applyChanges()
		}
	}
	return nil
}

func (o *Options) cycleLeft() {
	switch o.currentOptID() {
	case optShowAlbumArt:
		o.showAlbumArt = !o.showAlbumArt
	case optCopyAlbumArt:
		o.copyAlbumArt = !o.copyAlbumArt
	case optNotificationsEnabled:
		o.notificationsEnabled = !o.notificationsEnabled
	case optNotificationsShowArt:
		o.notificationsShowArt = !o.notificationsShowArt
	case optTransparentBackground:
		o.transparentBg = !o.transparentBg
	case optDisableTheme:
		o.disableTheme = !o.disableTheme
	case optVisualizerMode:
		if o.visModeIdx > 0 {
			o.visModeIdx--
		} else {
			o.visModeIdx = len(visualizerModeNames) - 1
		}
	case optVisualizerShowInfo:
		if o.visShowInfoIdx > 0 {
			o.visShowInfoIdx--
		} else {
			o.visShowInfoIdx = len(visualizerShowInfoOptions) - 1
		}
	case optRealAudio:
		o.realAudio = !o.realAudio
	case optTheme:
		themeOpts := themeOptions()
		if o.themeIdx > 0 {
			o.themeIdx--
		} else {
			o.themeIdx = len(themeOpts) - 1
		}
	case optReplayGain:
		if o.replayGainIdx > 0 {
			o.replayGainIdx--
		} else {
			o.replayGainIdx = len(replayGainModeNames) - 1
		}
	}
}

func (o *Options) cycleRight() {
	switch o.currentOptID() {
	case optShowAlbumArt:
		o.showAlbumArt = !o.showAlbumArt
	case optCopyAlbumArt:
		o.copyAlbumArt = !o.copyAlbumArt
	case optNotificationsEnabled:
		o.notificationsEnabled = !o.notificationsEnabled
	case optNotificationsShowArt:
		o.notificationsShowArt = !o.notificationsShowArt
	case optTransparentBackground:
		o.transparentBg = !o.transparentBg
	case optDisableTheme:
		o.disableTheme = !o.disableTheme
	case optVisualizerMode:
		if o.visModeIdx < len(visualizerModeNames)-1 {
			o.visModeIdx++
		} else {
			o.visModeIdx = 0
		}
	case optVisualizerShowInfo:
		if o.visShowInfoIdx < len(visualizerShowInfoOptions)-1 {
			o.visShowInfoIdx++
		} else {
			o.visShowInfoIdx = 0
		}
	case optRealAudio:
		o.realAudio = !o.realAudio
	case optTheme:
		themeOpts := themeOptions()
		if o.themeIdx < len(themeOpts)-1 {
			o.themeIdx++
		} else {
			o.themeIdx = 0
		}
	case optReplayGain:
		if o.replayGainIdx < len(replayGainModeNames)-1 {
			o.replayGainIdx++
		} else {
			o.replayGainIdx = 0
		}
	}
}

func (o *Options) applyChanges() tea.Cmd {
	albumArtChanged := o.showAlbumArt != o.origShowAlbumArt
	copyArtChanged := o.copyAlbumArt != o.origCopyAlbumArt
	notifEnabledChanged := o.notificationsEnabled != o.origNotificationsEnabled
	notifShowArtChanged := o.notificationsShowArt != o.origNotificationsShowArt
	transparentBgChanged := o.transparentBg != o.origTransparentBg
	disableThemeChanged := o.disableTheme != o.origDisableTheme
	visModeChanged := o.visModeIdx != o.origVisModeIdx
	visShowInfoChanged := o.visShowInfoIdx != o.origVisShowInfoIdx
	realAudioChanged := o.realAudio != o.origRealAudio
	themeChanged := o.themeIdx != o.origThemeIdx
	replayGainChanged := o.replayGainIdx != o.origReplayGainIdx

	if !albumArtChanged && !copyArtChanged && !notifEnabledChanged && !notifShowArtChanged &&
		!transparentBgChanged && !disableThemeChanged && !visModeChanged && !visShowInfoChanged &&
		!realAudioChanged && !themeChanged && !replayGainChanged {
		return func() tea.Msg { return OptionsMsg{Closed: true} }
	}

	var msg OptionsMsg
	if albumArtChanged {
		v := o.showAlbumArt
		msg.ShowAlbumArt = &v
	}
	if copyArtChanged {
		v := o.copyAlbumArt
		msg.CopyAlbumArt = &v
	}
	if notifEnabledChanged {
		v := o.notificationsEnabled
		msg.NotificationsEnabled = &v
	}
	if notifShowArtChanged {
		v := o.notificationsShowArt
		msg.NotificationsShowArt = &v
	}
	if transparentBgChanged {
		v := o.transparentBg
		msg.TransparentBackground = &v
	}
	if disableThemeChanged {
		v := o.disableTheme
		msg.DisableTheme = &v
	}
	if visModeChanged {
		v := visualizerModeNames[o.visModeIdx]
		msg.VisualizerMode = &v
	}
	if visShowInfoChanged {
		v := visualizerShowInfoOptions[o.visShowInfoIdx]
		msg.VisualizerShowInfo = &v
	}
	if realAudioChanged {
		v := o.realAudio
		msg.RealAudio = &v
	}
	if themeChanged {
		themeOpts := themeOptions()
		v := themeOpts[o.themeIdx]
		if v == "default" {
			v = ""
		}
		msg.Theme = &v
	}
	if replayGainChanged {
		v := strings.ToLower(replayGainModeNames[o.replayGainIdx])
		msg.ReplayGainMode = &v
	}
	return func() tea.Msg { return msg }
}

func (o Options) View() string {
	modalWidth := 60
	if o.width-8 < modalWidth {
		modalWidth = o.width - 8
	}
	if modalWidth < 40 {
		modalWidth = 40
	}
	contentWidth := modalWidth - 6

	accentStyle := o.styles.AccentStyle
	mutedStyle := o.styles.MutedStyle
	cursorStyle := o.styles.CursorStyle

	var b strings.Builder

	title := accentStyle.Render("OPTIONS")
	b.WriteString(centerStyled(title, contentWidth))
	b.WriteString("\n\n")

	visModeName := visualizerModeNames[0]
	if o.visModeIdx >= 0 && o.visModeIdx < len(visualizerModeNames) {
		visModeName = visualizerModeNames[o.visModeIdx]
	}

	visShowInfoName := visualizerShowInfoOptions[0]
	if o.visShowInfoIdx >= 0 && o.visShowInfoIdx < len(visualizerShowInfoOptions) {
		visShowInfoName = visualizerShowInfoOptions[o.visShowInfoIdx]
	}

	themeOpts := themeOptions()
	themeName := themeOpts[0]
	if o.themeIdx >= 0 && o.themeIdx < len(themeOpts) {
		themeName = themeOpts[o.themeIdx]
	}

	replayGainName := replayGainModeNames[0]
	if o.replayGainIdx >= 0 && o.replayGainIdx < len(replayGainModeNames) {
		replayGainName = replayGainModeNames[o.replayGainIdx]
	}

	type optItem struct {
		id    int
		label string
		value string
	}

	visibleItems := o.visibleItems()
	items := make([]optItem, 0, len(visibleItems))
	for _, id := range visibleItems {
		switch id {
		case optShowAlbumArt:
			items = append(items, optItem{id, "Show album art", o.renderToggle(o.showAlbumArt, o.cursor == len(items))})
		case optCopyAlbumArt:
			items = append(items, optItem{id, "Copy album art", o.renderToggle(o.copyAlbumArt, o.cursor == len(items))})
		case optNotificationsEnabled:
			items = append(items, optItem{id, "Notifications", o.renderToggle(o.notificationsEnabled, o.cursor == len(items))})
		case optNotificationsShowArt:
			items = append(items, optItem{id, " Art in notifications", o.renderToggle(o.notificationsShowArt, o.cursor == len(items))})
		case optTransparentBackground:
			items = append(items, optItem{id, "Transparent bg", o.renderToggle(o.transparentBg, o.cursor == len(items))})
		case optDisableTheme:
			items = append(items, optItem{id, "Disable theme", o.renderToggle(o.disableTheme, o.cursor == len(items))})
		case optVisualizerMode:
			items = append(items, optItem{id, "Visualizer", o.renderPicker(visModeName, o.cursor == len(items))})
		case optVisualizerShowInfo:
			items = append(items, optItem{id, " Info overlay", o.renderPicker(visShowInfoName, o.cursor == len(items))})
		case optRealAudio:
			items = append(items, optItem{id, " Real audio", o.renderToggle(o.realAudio, o.cursor == len(items))})
		case optTheme:
			items = append(items, optItem{id, "Theme", o.renderPicker(themeName, o.cursor == len(items))})
		case optReplayGain:
			items = append(items, optItem{id, "ReplayGain", o.renderPicker(replayGainName, o.cursor == len(items))})
		}
	}

	labelColWidth := 22

	for i, item := range items {
		prefix := " "
		label := mutedStyle.Render(item.label)
		if i == o.cursor {
			prefix = cursorStyle.Render("▸ ")
			label = lipgloss.NewStyle().
				Foreground(o.styles.ForegroundStyle.GetForeground()).
				Render(item.label)
		}

		labelVisualWidth := lipgloss.Width(label)
		padCount := labelColWidth - labelVisualWidth
		if padCount < 0 {
			padCount = 0
		}
		row := prefix + label + strings.Repeat(" ", padCount) + item.value
		b.WriteString(row)
		b.WriteString("\n")
	}

	b.WriteString("\n\n")
	helpText := accentStyle.Render("←/→") + mutedStyle.Render(" change ") +
		accentStyle.Render("↑/↓") + mutedStyle.Render(" navigate ") +
		accentStyle.Render("a") + mutedStyle.Render(" apply ") +
		accentStyle.Render("esc") + mutedStyle.Render(" close")
	b.WriteString(centerStyled(helpText, contentWidth))

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(o.styles.AccentStyle.GetForeground()).
		Padding(1, 2).
		Width(modalWidth)

	rendered := modalStyle.Render(b.String())

	visWidth := lipgloss.Width(rendered)
	visHeight := lipgloss.Height(rendered)
	padLeft := max(0, (o.width-visWidth)/2)
	padTop := max(0, (o.height-visHeight)/2)

	if padTop > 0 || padLeft > 0 {
		padStr := strings.Repeat(" ", padLeft)
		var sb strings.Builder
		for i := 0; i < padTop; i++ {
			sb.WriteString("\n")
		}
		for i, line := range strings.Split(rendered, "\n") {
			if i > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(padStr)
			sb.WriteString(line)
		}
		return sb.String()
	}
	return rendered
}

func (o *Options) renderPicker(value string, selected bool) string {
	if selected {
		arrow := o.styles.MutedStyle.Render("◂ ")
		arrowR := o.styles.MutedStyle.Render(" ▸")
		val := o.styles.CursorStyle.Render(value)
		return arrow + val + arrowR
	}
	return o.styles.MutedStyle.Render(value)
}

func (o *Options) renderToggle(on bool, selected bool) string {
	var indicator string
	if on {
		indicator = o.styles.AccentStyle.Render("●")
	} else {
		indicator = o.styles.MutedStyle.Render("○")
	}

	if selected {
		return o.styles.CursorStyle.Render("[") + indicator + o.styles.CursorStyle.Render("]")
	}
	return o.styles.MutedStyle.Render("[") + indicator + o.styles.MutedStyle.Render("]")
}
