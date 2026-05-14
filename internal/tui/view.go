package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/pdfrg/must/internal/models"
)

func (m Model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("Loading...")
	}

	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	contentHeight := m.height - 3
	if contentHeight < 5 {
		contentHeight = 5
	}

	switch m.viewMode {
	case ViewHelp:
		b.WriteString(m.renderHelp(contentHeight))
	case ViewLyrics:
		b.WriteString(m.renderLyrics(contentHeight))
	case ViewSyncedLyrics:
		b.WriteString(m.renderSyncedLyrics(contentHeight))
	case ViewArtistBio:
		b.WriteString(m.renderArtistBio(contentHeight))
	case ViewLibrary:
		if m.searching {
			b.WriteString(m.renderSearch(contentHeight))
		} else {
			b.WriteString(m.renderLibrary(contentHeight))
		}
	case ViewPlaylist:
		if m.searching {
			b.WriteString(m.renderSearch(contentHeight))
		} else {
			b.WriteString(m.renderPlaylist(contentHeight))
		}
	}

	b.WriteString("\n")
	b.WriteString(m.renderFooter())

	return tea.NewView(b.String())
}

func (m Model) renderHeader() string {
	title := m.styles.Header.Render("must")

	var info string
	if m.playing && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
		track := m.playlist[m.currentIndex]
		info = m.styles.ForegroundStyle.Render(track.FormatDisplayInfo())
	}

	status := m.styles.MutedStyle.Render(m.statusLine())

	layout := m.layoutMode()
	if layout != "large" && layout != "" {
		status += m.styles.MutedStyle.Render(" | " + layout)
	}

	parts := []string{title}
	if info != "" {
		parts = append(parts, info)
	}
	parts = append(parts, status)

	avail := m.width
	used := 0
	rendered := make([]string, 0, len(parts))
	for _, p := range parts {
		r := len(p)
		if used+r > avail {
			break
		}
		rendered = append(rendered, p)
		used += r + 1
	}

	return strings.Join(rendered, " ")
}

func (m Model) statusLine() string {
	var parts []string

	state := "stopped"
	if m.playing && !m.paused {
		state = "playing"
	} else if m.paused {
		state = "paused"
	}
	parts = append(parts, state)

	switch m.repeatMode {
	case "all":
		parts = append(parts, "repeat all")
	case "one":
		parts = append(parts, "repeat one")
	}

	if m.shuffle {
		parts = append(parts, "shuffle")
	}

	if m.muted {
		parts = append(parts, "muted")
	} else {
		parts = append(parts, fmt.Sprintf("vol:%d%%", int(m.volume)))
	}

	switch m.viewMode {
	case ViewLibrary:
		switch m.focusPane {
		case FocusArtists:
			parts = append(parts, "[artists]")
		case FocusAlbums:
			parts = append(parts, "[albums]")
		case FocusTracks:
			parts = append(parts, "[tracks]")
		}
	case ViewLyrics:
		parts = append(parts, "[lyrics]")
	case ViewSyncedLyrics:
		parts = append(parts, "[synced]")
	case ViewArtistBio:
		parts = append(parts, "[bio]")
	}

	if m.sleepTimer > 0 && m.sleepRemaining > 0 {
		mins := int(m.sleepRemaining.Minutes())
		parts = append(parts, fmt.Sprintf("sleep:%dm", mins))
	}

	return strings.Join(parts, " | ")
}

func (m Model) renderLibrary(height int) string {
	var b strings.Builder

	if !m.libraryReady {
		b.WriteString(m.styles.MutedStyle.Render("Scanning library..."))
		if m.scanMsg != "" {
			b.WriteString("\n")
			b.WriteString(m.styles.MutedStyle.Render(m.scanMsg))
		}
		return b.String()
	}

	leftWidth := m.width / 3
	if leftWidth < 20 {
		leftWidth = 20
	}

	rightWidth := m.width - leftWidth - 3

	showArt := m.cfg.ShowAlbumArt && m.albumArtLoaded && m.layoutMode() != "compact"
	if showArt && m.albumArtWidth > 0 {
		artMargin := m.albumArtWidth + 4
		if rightWidth-artMargin > 20 {
			rightWidth -= artMargin
		}
	}

	artistList := m.renderArtistList(leftWidth, height)
	albumTrackList := m.renderAlbumTrackList(rightWidth, height)

	sep := m.styles.MutedStyle.Render("│")
	if m.focusPane == FocusArtists {
		sep = m.styles.AccentStyle.Render("│")
	}

	b.WriteString(artistList)
	b.WriteString(" ")
	b.WriteString(sep)
	b.WriteString(" ")
	b.WriteString(albumTrackList)

	return b.String()
}

func (m Model) renderArtistList(width, height int) string {
	if len(m.artists) == 0 {
		return m.styles.MutedStyle.Render("No artists")
	}

	var b strings.Builder
	maxRows := min(height, len(m.artists)-m.artistScrollOffset)
	if maxRows < 1 {
		maxRows = 1
	}

	focused := m.focusPane == FocusArtists

	for i := 0; i < maxRows; i++ {
		idx := m.artistScrollOffset + i
		if idx >= len(m.artists) {
			break
		}

		name := truncate(m.artists[idx], width-2)
		if idx == m.artistCursor && focused {
			name = m.styles.CursorStyle.Render("▸ " + name)
		} else if idx == m.artistCursor {
			name = m.styles.AccentStyle.Render("▹ " + name)
		} else {
			name = m.styles.MutedStyle.Render("  " + name)
		}
		b.WriteString(name)
		if i < maxRows-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m Model) renderAlbumTrackList(width, height int) string {
	if len(m.albumTracks) > 0 && m.focusPane == FocusTracks {
		return m.renderTrackList(m.albumTracks, width, height, true)
	}

	if len(m.albums) > 0 {
		return m.renderAlbumList(width, height)
	}

	if m.focusPane == FocusTracks && len(m.albumTracks) > 0 {
		return m.renderTrackList(m.albumTracks, width, height, true)
	}

	if len(m.artists) > 0 && m.artistCursor < len(m.artists) {
		artist := m.artists[m.artistCursor]
		tracks, err := m.libraryDB.GetTracksByArtist(artist)
		if err == nil && len(tracks) > 0 {
			return m.renderTrackList(tracks, width, height, false)
		}
	}

	return m.styles.MutedStyle.Render("Select an artist")
}

func (m Model) renderAlbumList(width, height int) string {
	var b strings.Builder
	maxRows := min(height, len(m.albums)-m.albumScrollOffset)
	if maxRows < 1 {
		maxRows = 1
	}

	focused := m.focusPane == FocusAlbums

	for i := 0; i < maxRows; i++ {
		idx := m.albumScrollOffset + i
		if idx >= len(m.albums) {
			break
		}

		name := truncate(m.albums[idx], width-2)
		if idx == m.albumCursor && focused {
			name = m.styles.CursorStyle.Render("▸ " + name)
		} else if idx == m.albumCursor {
			name = m.styles.AccentStyle.Render("▹ " + name)
		} else {
			name = m.styles.ForegroundStyle.Render("  " + name)
		}
		b.WriteString(name)
		if i < maxRows-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m Model) renderTrackList(tracks []models.Track, width, height int, focused bool) string {
	var b strings.Builder
	start := m.albumScrollOffset
	if focused {
		start = m.albumScrollOffset
	}
	maxRows := min(height, len(tracks)-start)
	if maxRows < 1 {
		maxRows = 1
	}

	for i := 0; i < maxRows; i++ {
		idx := start + i
		if idx >= len(tracks) {
			break
		}

		t := tracks[idx]

		dur := t.GetDurationFormatted()
		label := fmt.Sprintf("%2d. %s", t.TrackNum, t.Title)
		avail := width - len(dur) - 4
		if avail < 10 {
			avail = 10
		}
		label = truncate(label, avail)

		var prefix string
		if idx == m.albumCursor && focused {
			prefix = m.styles.CursorStyle.Render("▸ ")
		} else if idx == m.albumCursor {
			prefix = m.styles.AccentStyle.Render("▹ ")
		} else {
			prefix = "  "
		}

		line := fmt.Sprintf("%s%s %s", prefix, label, m.styles.MutedStyle.Render(dur))

		if idx == m.albumCursor && focused {
			b.WriteString(line)
		} else {
			b.WriteString(m.styles.ForegroundStyle.Render(line))
		}
		if i < maxRows-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m Model) renderSearch(height int) string {
	var b strings.Builder

	searchBar := m.searchInput.View()
	b.WriteString(m.styles.AccentStyle.Render("Search: "))
	b.WriteString(searchBar)
	b.WriteString("\n")

	if len(m.searchResults) > 0 {
		maxVisible := height - 2
		if maxVisible < 1 {
			maxVisible = 1
		}

		for i := 0; i < maxVisible; i++ {
			idx := m.searchScrollOffset + i
			if idx >= len(m.searchResults) {
				break
			}

			t := m.searchResults[idx]
			dur := t.GetDurationFormatted()
			label := fmt.Sprintf("%s - %s - %s", t.Artist, t.Album, t.Title)
			avail := m.width - len(dur) - 6
			if avail < 10 {
				avail = 10
			}
			label = truncate(label, avail)

			var prefix string
			if idx == m.searchCursor {
				prefix = m.styles.CursorStyle.Render("▸ ")
			} else {
				prefix = "  "
			}

			line := fmt.Sprintf("%s%s %s", prefix, label, m.styles.MutedStyle.Render(dur))

			if idx == m.searchCursor {
				b.WriteString(line)
			} else {
				b.WriteString(m.styles.ForegroundStyle.Render(line))
			}
			if i < maxVisible-1 {
				b.WriteString("\n")
			}
		}

		b.WriteString("\n")
		b.WriteString(m.styles.MutedStyle.Render(fmt.Sprintf("%d results  ↑/↓ navigate  enter play  esc close", len(m.searchResults))))
	} else if m.searchInput.Value() != "" {
		b.WriteString(m.styles.MutedStyle.Render("No results found"))
	} else {
		b.WriteString(m.styles.MutedStyle.Render("Type to search (supports artist:name, album:name, genre:term, year:1997)"))
	}

	return b.String()
}

func (m Model) renderLyrics(height int) string {
	if m.lyricsLoading {
		return m.styles.MutedStyle.Render(" Loading lyrics...")
	}
	if m.lyrics == "" {
		return m.styles.MutedStyle.Render(" No lyrics available")
	}

	var b strings.Builder
	lines := strings.Split(m.lyrics, "\n")
	maxLines := min(height, len(lines))
	for i := 0; i < maxLines; i++ {
		b.WriteString(" ")
		b.WriteString(m.styles.ForegroundStyle.Render(lines[i]))
		if i < maxLines-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m Model) renderSyncedLyrics(height int) string {
	if len(m.syncedLyrics) == 0 {
		if m.lyricsLoading {
			return m.styles.MutedStyle.Render(" Loading lyrics...")
		}
		return m.styles.MutedStyle.Render(" No synced lyrics available")
	}

	currentLineIdx := -1
	for i, line := range m.syncedLyrics {
		if line.Time <= m.playbackPos.TimePos {
			currentLineIdx = i
		}
	}

	startIdx := currentLineIdx - 3
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := currentLineIdx + 4
	if endIdx > len(m.syncedLyrics) {
		endIdx = len(m.syncedLyrics)
	}

	var b strings.Builder
	for i := startIdx; i < endIdx; i++ {
		if i == currentLineIdx {
			b.WriteString(m.styles.CursorStyle.Render("▶ " + m.syncedLyrics[i].Content))
		} else {
			b.WriteString(m.styles.MutedStyle.Render("  " + m.syncedLyrics[i].Content))
		}
		if i < endIdx-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m Model) renderArtistBio(height int) string {
	if m.artistBioLoading {
		return m.styles.MutedStyle.Render(" Loading artist bio...")
	}

	var b strings.Builder

	if m.artistBioTitle != "" {
		b.WriteString(m.styles.Header.Render(m.artistBioTitle))
		b.WriteString("\n")
	}

	if m.artistBio == "" {
		b.WriteString(m.styles.MutedStyle.Render(" No bio available"))
		return b.String()
	}

	words := strings.Fields(m.artistBio)
	lineWidth := m.width - 4
	if lineWidth < 20 {
		lineWidth = 20
	}

	var line string
	lineCount := 0
	for _, w := range words {
		test := line + " " + w
		if len(test) > lineWidth && line != "" {
			b.WriteString(m.styles.ForegroundStyle.Render(" " + strings.TrimSpace(line)))
			b.WriteString("\n")
			line = w
			lineCount++
			if lineCount >= height-2 {
				break
			}
		} else {
			line = test
		}
	}
	if lineCount < height-2 && line != "" {
		b.WriteString(m.styles.ForegroundStyle.Render(" " + strings.TrimSpace(line)))
	}

	if m.artistBioURL != "" {
		b.WriteString("\n")
		b.WriteString(m.styles.MutedStyle.Render(" " + m.artistBioURL))
	}

	return b.String()
}

func (m Model) renderPlaylist(height int) string {
	if len(m.playlist) == 0 {
		return m.styles.MutedStyle.Render("Playlist is empty")
	}

	var b strings.Builder
	maxRows := min(height, len(m.playlist))

	start := 0
	if m.currentIndex >= maxRows {
		start = m.currentIndex - maxRows + 1
	}

	for i := 0; i < maxRows; i++ {
		idx := start + i
		if idx >= len(m.playlist) {
			break
		}

		t := m.playlist[idx]
		dur := t.GetDurationFormatted()

		var prefix string
		if idx == m.currentIndex && m.playing {
			if m.paused {
				prefix = "||"
			} else {
				prefix = " >"
			}
		} else {
			prefix = " "
		}

		avail := m.width - len(dur) - 6
		if avail < 10 {
			avail = 10
		}
		label := truncate(fmt.Sprintf("%s - %s - %s", t.Artist, t.Album, t.Title), avail)

		line := fmt.Sprintf("%s %s %s", prefix, label, m.styles.MutedStyle.Render(dur))

		if idx == m.currentIndex {
			b.WriteString(m.styles.CursorStyle.Render(line))
		} else {
			b.WriteString(m.styles.ForegroundStyle.Render(line))
		}

		if i < maxRows-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m Model) renderFooter() string {
	var parts []string

	if m.playing && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
		track := m.playlist[m.currentIndex]

		var posStr string
		if m.playbackPos.TimePos > 0 {
			posStr = formatSeconds(m.playbackPos.TimePos)
		} else {
			posStr = "00:00"
		}
		totalStr := track.GetDurationFormatted()

		progressWidth := m.width - len(posStr) - len(totalStr) - 4
		if m.layoutMode() == "compact" {
			progressWidth = m.width - len(posStr) - len(totalStr) - 4
		}
		if progressWidth < 10 {
			progressWidth = 10
		}

		bar := m.renderProgressBar(progressWidth, m.playbackPos.PercentPos/100.0)

		parts = append(parts, posStr, bar, totalStr)

		if m.audioInfo != nil && m.layoutMode() != "compact" {
			audioStr := formatAudioInfo(m.audioInfo)
			parts = append(parts, m.styles.MutedStyle.Render(audioStr))
		}
	} else if m.statusMsg != "" {
		if m.statusIsErr {
			parts = append(parts, m.styles.CursorStyle.Render(m.statusMsg))
		} else {
			parts = append(parts, m.styles.MutedStyle.Render(m.statusMsg))
		}
	}

	sep := " "
	return m.styles.Footer.Render(strings.Join(parts, sep))
}

func (m Model) renderProgressBar(width int, progress float64) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	filled := int(progress * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("━", filled) + strings.Repeat("─", width-filled)
	return m.styles.AccentStyle.Render(bar)
}

func (m Model) renderHelp(height int) string {
	bindings := append(append(m.keyMap.PlaybackBindings(), m.keyMap.NavigationBindings()...), m.keyMap.GlobalBindings()...)

	var b strings.Builder
	b.WriteString(m.styles.Header.Render("Keyboard Shortcuts"))
	b.WriteString("\n\n")

	for _, bnd := range bindings {
		if !bnd.Enabled() {
			continue
		}
		help := bnd.Help()
		line := fmt.Sprintf(" %-14s %s", help.Key, help.Desc)
		b.WriteString(m.styles.ForegroundStyle.Render(line))
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) layoutMode() string {
	if m.layoutOverride != "" {
		return m.layoutOverride
	}
	return m.cfg.Layout
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func formatSeconds(secs float64) string {
	total := int(secs)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func formatAudioInfo(info *models.AudioInfo) string {
	parts := []string{}

	if info.Codec != "" {
		parts = append(parts, info.Codec)
	}
	if info.Bitrate > 0 {
		parts = append(parts, fmt.Sprintf("%.0fk", info.Bitrate))
	}
	if info.SampleRate > 0 {
		parts = append(parts, fmt.Sprintf("%dHz", info.SampleRate))
	}
	if info.BitDepth > 0 {
		parts = append(parts, fmt.Sprintf("%dbit", info.BitDepth))
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}
