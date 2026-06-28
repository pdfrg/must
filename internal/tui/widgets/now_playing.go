package widgets

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/models"
)

type NowPlaying struct {
	foregroundStyle lipgloss.Style
	accentStyle     lipgloss.Style
	mutedStyle      lipgloss.Style
	cursorStyle     lipgloss.Style
	width           int
	maxWidth        int
	contentWidth    int

	sleepTimerActive bool
	sleepTimerMins   int

	accentColor string
	cursorColor string

	progress progress.Model
}

func NewNowPlaying(styles *config.ThemeStyles, accentColor, cursorColor, progressBgColor string) *NowPlaying {
	cursorStyle := styles.ForegroundStyle
	if cursorColor != "" {
		cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cursorColor))
	}

	var emptyColor color.Color
	if progressBgColor != "" && len(progressBgColor) == 7 && progressBgColor[0] == '#' {
		emptyColor = lipgloss.Color(darkenColor(progressBgColor, 0.3))
	} else {
		emptyColor = lipgloss.Color("#1a1a1a")
	}

	p := buildProgress(40, accentColor, cursorColor, emptyColor)

	return &NowPlaying{
		foregroundStyle: styles.ForegroundStyle,
		accentStyle:     styles.AccentStyle,
		mutedStyle:      styles.MutedStyle,
		cursorStyle:     cursorStyle,
		accentColor:     accentColor,
		cursorColor:     cursorColor,
		progress:        p,
	}
}

func (n *NowPlaying) SetWidth(width int) {
	n.width = width
	progWidth := min(40, width-2)
	progWidth = max(20, progWidth)
	n.progress.SetWidth(progWidth)
}

func (n *NowPlaying) GetWidth() int {
	return n.width
}

func (n *NowPlaying) SetMaxWidth(maxWidth int) {
	n.maxWidth = maxWidth
}

func (n *NowPlaying) SetContentWidth(width int) {
	n.contentWidth = width
}

func (n *NowPlaying) UpdateStyles(styles *config.ThemeStyles, accentColor, cursorColor, bgColor string) {
	n.foregroundStyle = styles.ForegroundStyle
	n.accentStyle = styles.AccentStyle
	n.mutedStyle = styles.MutedStyle
	n.cursorStyle = styles.ForegroundStyle
	if cursorColor != "" {
		n.cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cursorColor))
	}

	n.accentColor = accentColor
	n.cursorColor = cursorColor

	var emptyColor color.Color
	if bgColor != "" && len(bgColor) == 7 && bgColor[0] == '#' {
		emptyColor = lipgloss.Color(darkenColor(bgColor, 0.3))
	}

	n.progress = buildProgress(n.width, n.accentColor, n.cursorColor, emptyColor)
}

func buildProgress(width int, accentColor, cursorColor string, emptyColor color.Color) progress.Model {
	progWidth := min(40, width-2)
	progWidth = max(20, progWidth)
	p := progress.New(
		progress.WithWidth(progWidth),
		progress.WithColors(lipgloss.Color(cursorColor), lipgloss.Color(accentColor)),
		progress.WithoutPercentage(),
		progress.WithFillCharacters('▀', '▀'),
	)
	p.EmptyColor = emptyColor
	return p
}

func (n *NowPlaying) UpdateProgress(percent float64) tea.Cmd {
	return n.progress.SetPercent(percent)
}

func (n *NowPlaying) SnapProgress(percent float64) {
	emptyColor := n.progress.EmptyColor
	n.progress = buildProgress(n.width, n.accentColor, n.cursorColor, emptyColor)
	_ = n.progress.SetPercent(percent)
}

func (n *NowPlaying) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	n.progress, cmd = n.progress.Update(msg)
	return cmd
}

func (n *NowPlaying) SetSleepTimer(active bool, mins int) {
	n.sleepTimerActive = active
	n.sleepTimerMins = mins
}

type NowPlayingData struct {
	Track          *models.Track
	AudioInfo      *models.AudioInfo
	IsPaused       bool
	TimePos        float64
	RepeatMode     string
	Shuffle        bool
	ReplayGainMode string
	PlaylistPos    int
	PlaylistLength int
	StatusMsg      string
	StatusIsErr    bool
	SleepActive    bool
	SleepMins      int
}

func buildStatusLine(n NowPlaying, data NowPlayingData) string {
	if data.StatusMsg != "" {
		if data.StatusIsErr {
			return n.accentStyle.Render(data.StatusMsg)
		}
		return n.foregroundStyle.Render(data.StatusMsg)
	}
	if data.PlaylistLength > 0 {
		return n.foregroundStyle.Render(fmt.Sprintf("track %d of %d", data.PlaylistPos, data.PlaylistLength))
	}
	return n.mutedStyle.Render("must")
}

func (n NowPlaying) renderIdleView(data NowPlayingData) string {
	title := n.accentStyle.Bold(true).Render("No song playing")
	artist := n.mutedStyle.Render("—")
	album := n.mutedStyle.Render("—")

	progView := n.progress.View()
	timeStr := n.mutedStyle.Render("00:00 / 00:00 (0%)")
	audioLine := n.mutedStyle.Render("󰎇 —")
	modeLine := n.mutedStyle.Render("󰓛 stopped")
	statusLine := buildStatusLine(n, data)

	output := fmt.Sprintf(" %s\n %s\n %s\n\n %s\n %s\n\n %s\n\n %s\n\n %s\n",
		title, artist, album, progView, timeStr, modeLine, audioLine, statusLine)

	if n.contentWidth > 0 {
		lines := strings.Split(output, "\n")
		for i, line := range lines {
			originalWidth := lipgloss.Width(line)
			if originalWidth > n.contentWidth {
				line = ansi.Truncate(line, n.contentWidth-3, "...")
			}
			if lipgloss.Width(line) < n.contentWidth {
				line = lipgloss.NewStyle().Width(n.contentWidth).Render(line)
			}
			lines[i] = line
		}
		output = strings.Join(lines, "\n")
	}

	return output
}

func (n NowPlaying) View(data NowPlayingData) string {
	if data.Track == nil {
		return n.renderIdleView(data)
	}

	titleText := data.Track.Title
	artistText := data.Track.Artist
	if data.Track.ServerBadge != "" {
		artistText = "[" + data.Track.ServerBadge + "] " + artistText
	}
	albumText := fmt.Sprintf("%s (%d)", data.Track.Album, data.Track.Year)

	if n.maxWidth > 0 {
		titleText = ansi.Truncate(titleText, n.maxWidth, "...")
		artistText = ansi.Truncate(artistText, n.maxWidth, "...")
		albumText = ansi.Truncate(albumText, n.maxWidth, "...")
	}

	title := n.accentStyle.Bold(true).Render(titleText)
	artist := n.foregroundStyle.Render(artistText)
	album := n.mutedStyle.Render(albumText)

	progView := n.progress.View()

	percentPos := 0.0
	if data.Track.Duration > 0 {
		percentPos = data.TimePos / data.Track.Duration * 100
	}
	timeStr := n.mutedStyle.Render(fmt.Sprintf("%s / %s (%.0f%%)",
		formatDuration(data.TimePos),
		formatDuration(data.Track.Duration),
		percentPos))

	var audioLine string
	if data.AudioInfo != nil {
		audioLine = formatAudioLine(data.AudioInfo, n.mutedStyle, n.foregroundStyle)
	}

	var modeLine string
	modeParts := []string{}
	if data.RepeatMode != "" && data.RepeatMode != "off" {
		modeParts = append(modeParts, n.foregroundStyle.Render("󰑖 "+data.RepeatMode))
	}
	if data.ReplayGainMode != "" && data.ReplayGainMode != "off" {
		modeParts = append(modeParts, n.foregroundStyle.Render("󰇼 "+data.ReplayGainMode))
	}
	if data.Shuffle {
		modeParts = append(modeParts, n.foregroundStyle.Render("󰒟 shuffle"))
	}
	if data.IsPaused {
		modeParts = append(modeParts, n.mutedStyle.Render("󰏤 paused"))
	} else {
		modeParts = append(modeParts, n.mutedStyle.Render("▶ playing"))
	}
	if data.Track != nil && data.Track.ServerBadge != "" {
		modeParts = append(modeParts, n.mutedStyle.Render(data.Track.ServerName))
	}
	modeLine = strings.Join(modeParts, "  ")

	statusLine := buildStatusLine(n, data)

	if n.sleepTimerActive || data.SleepActive {
		mins := data.SleepMins
		if n.sleepTimerActive {
			mins = n.sleepTimerMins
		}
		statusLine += " " + n.mutedStyle.Render("•") + " " + n.accentStyle.Render(fmt.Sprintf("Sleep in %dm", mins))
	}

	output := fmt.Sprintf(" %s\n %s\n %s\n\n %s\n %s\n\n %s\n", title, artist, album, progView, timeStr, modeLine)
	if audioLine != "" {
		output += fmt.Sprintf("\n %s\n", audioLine)
	}
	output += fmt.Sprintf("\n %s\n", statusLine)

	if n.contentWidth > 0 {
		lines := strings.Split(output, "\n")
		for i, line := range lines {
			originalWidth := lipgloss.Width(line)
			if originalWidth > n.contentWidth {
				line = ansi.Truncate(line, n.contentWidth-3, "...")
			}
			if lipgloss.Width(line) < n.contentWidth {
				line = lipgloss.NewStyle().Width(n.contentWidth).Render(line)
			}
			lines[i] = line
		}
		output = strings.Join(lines, "\n")
	}

	return output
}

func formatAudioLine(info *models.AudioInfo, mutedStyle, fgStyle lipgloss.Style) string {
	parts := []string{}
	if info.Codec != "" {
		parts = append(parts, fgStyle.Render(info.Codec))
	}
	if info.Bitrate > 0 {
		parts = append(parts, fgStyle.Render(fmt.Sprintf("%.0fk", info.Bitrate)))
	}
	if info.SampleRate > 0 {
		parts = append(parts, fgStyle.Render(fmt.Sprintf("%dHz", info.SampleRate)))
	}
	if info.BitDepth > 0 {
		parts = append(parts, fgStyle.Render(fmt.Sprintf("%dbit", info.BitDepth)))
	}
	if len(parts) == 0 {
		return ""
	}
	return mutedStyle.Render("󰎇") + " " + strings.Join(parts, mutedStyle.Render(" • "))
}

func formatDuration(seconds float64) string {
	totalSeconds := int(seconds)
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	secs := totalSeconds % 60
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%02d:%02d", minutes, secs)
}
