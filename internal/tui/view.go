package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/pdfrg/must/internal/models"
	"github.com/pdfrg/must/internal/tui/widgets"
)

func (m Model) altView(s string) tea.View {
	v := tea.NewView(s)
	v.AltScreen = true
	if !m.cfg.TransparentBackground && !m.cfg.DisableTheme {
		v.BackgroundColor = m.styles.BackgroundStyle.GetBackground()
		v.ForegroundColor = m.styles.ForegroundStyle.GetForeground()
	}
	return v
}

func (m Model) View() tea.View {
	if m.width == 0 {
		return m.altView("Loading...")
	}

	if m.activeModal != ModalNone {
		modalView := m.renderModal()
		if modalView != "" {
			return m.renderWithModal(modalView)
		}
	}

	var b strings.Builder

	headerView := m.header.View()
	b.WriteString(headerView)
	b.WriteString("\n\n")

	nowPlayingView := m.renderNowPlaying()
	b.WriteString(nowPlayingView)
	if !strings.HasSuffix(nowPlayingView, "\n") {
		b.WriteString("\n")
	}

	artHeight := 16
	hasArt := (m.cfg.ShowAlbumArt && m.albumArtLoaded && m.layoutMode() != "compact") ||
		(m.logoArtLoaded && m.imageRenderer != nil && m.cfg.ShowAlbumArt && m.layoutMode() != "compact")
	if hasArt {
		nowPlayingLines := strings.Count(nowPlayingView, "\n")
		if !strings.HasSuffix(nowPlayingView, "\n") {
			nowPlayingLines++
		}
		if nowPlayingLines < artHeight {
			for i := 0; i < artHeight-nowPlayingLines; i++ {
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n")

	currentHeight := lipgloss.Height(b.String())
	footerView := m.renderFooter()
	footerHeight := lipgloss.Height(footerView)
	if footerHeight == 0 {
		footerHeight = 1
	}
	remainingHeight := m.height - currentHeight - footerHeight

	if remainingHeight > 0 {
		bottomView := m.renderBottomSection(remainingHeight)
		bottomLines := strings.Split(bottomView, "\n")
		for i := 0; i < remainingHeight; i++ {
			if i < len(bottomLines) {
				b.WriteString(bottomLines[i])
			}
			b.WriteString("\n")
		}
	}

	b.WriteString(footerView)

	return m.altView(b.String())
}

func (m Model) renderNowPlaying() string {
	var track *models.Track
	if m.playing && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
		track = &m.playlist[m.currentIndex]
	}

	data := widgets.NowPlayingData{
		Track:       track,
		AudioInfo:   m.audioInfo,
		IsPaused:    m.paused,
		TimePos:     m.playbackPos.TimePos,
		RepeatMode:  m.repeatMode,
		Shuffle:     m.shuffle,
		StatusMsg:   m.statusMsg,
		StatusIsErr: m.statusIsErr,
		SleepActive: m.sleepTimer > 0 && m.sleepRemaining > 0,
		SleepMins:   int(m.sleepRemaining.Minutes()) + 1,
	}

	m.nowPlaying.SetWidth(m.width - 4)
	m.nowPlaying.SetMaxWidth(0)

	hasAlbumArt := m.cfg.ShowAlbumArt && m.albumArtLoaded && m.layoutMode() != "compact"
	hasLogo := !hasAlbumArt && m.logoImage != nil && m.imageRenderer != nil && m.layoutMode() != "compact"

	if hasAlbumArt || hasLogo {
		artHeight := 16
		artWidth := int(float64(artHeight) * m.cellRatio)
		if artWidth < 10 {
			artWidth = 10
		}
		artCol := m.width - artWidth - 2
		if artCol > 10 {
			m.nowPlaying.SetContentWidth(artCol - 2)
		} else {
			m.nowPlaying.SetContentWidth(0)
		}
	} else {
		m.nowPlaying.SetContentWidth(0)
	}

	return m.nowPlaying.View(data)
}

func (m Model) renderBottomSection(height int) string {
	if height <= 0 {
		return ""
	}

	switch m.bottomViewMode {
	case BottomPlaylist:
		m.playlistWidget.SetSize(m.width, height)
		return m.playlistWidget.View()

	case BottomLyrics:
		return m.renderLyrics(height)

	case BottomSyncedLyrics:
		return m.renderSyncedLyrics(height)

	case BottomArtistBio:
		return m.renderArtistBio(height)

	case BottomOff:
		return ""
	}

	return ""
}

func (m Model) renderModal() string {
	switch m.activeModal {
	case ModalLibrary:
		if m.libraryModal != nil {
			m.libraryModal.SetSize(m.width, m.height)
			return m.libraryModal.View()
		}
	case ModalSearch:
		if m.searchModal != nil {
			m.searchModal.SetSize(m.width, m.height)
			return m.searchModal.View()
		}
	case ModalHelp:
		if m.helpModal != nil {
			m.helpModal.SetSize(m.width, m.height)
			return m.helpModal.View()
		}
	case ModalGallery:
		if m.galleryModal != nil {
			m.galleryModal.SetSize(m.width, m.height)
			return m.galleryModal.View()
		}
	}
	return ""
}

func (m Model) renderWithModal(modalView string) tea.View {
	modalLines := strings.Split(modalView, "\n")
	modalHeight := len(modalLines)
	modalWidth := 0
	for _, line := range modalLines {
		if w := lipgloss.Width(line); w > modalWidth {
			modalWidth = w
		}
	}

	padTop := max(0, (m.height-modalHeight)/2)
	padLeft := max(0, (m.width-modalWidth)/2)
	leftPad := strings.Repeat(" ", padLeft)

	var b strings.Builder
	for i := 0; i < padTop; i++ {
		b.WriteString("\n")
	}
	for _, line := range modalLines {
		b.WriteString(leftPad + line + "\n")
	}

	return m.altView(b.String())
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
			b.WriteString(m.styles.MutedStyle.Render(" " + m.syncedLyrics[i].Content))
		}
		if i < endIdx-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m Model) renderArtistBio(height int) string {
	if m.artistInfoLoading {
		return m.styles.MutedStyle.Render(" Loading artist info...")
	}

	if m.artistInfo == nil {
		return m.styles.MutedStyle.Render(" No artist info available")
	}

	info := m.artistInfo
	var b strings.Builder

	var title string
	if m.playing && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
		title = m.playlist[m.currentIndex].Artist
	}
	if title != "" {
		b.WriteString(m.styles.Header.Render(title))
		b.WriteString("\n")
	}

	lineWidth := m.width - 4
	if lineWidth < 20 {
		lineWidth = 20
	}
	lineCount := 0
	maxBioLines := height - 2
	if info.Discography != "" {
		maxBioLines = max(5, height/2-1)
	}

	if info.Bio != "" && info.Bio != "No biography found." {
		words := strings.Fields(info.Bio)
		var line string
		for _, w := range words {
			test := line + " " + w
			if lipgloss.Width(test) > lineWidth && line != "" {
				b.WriteString(m.styles.ForegroundStyle.Render(" " + strings.TrimSpace(line)))
				b.WriteString("\n")
				line = w
				lineCount++
				if lineCount >= maxBioLines {
					break
				}
			} else {
				line = test
			}
		}
		if lineCount < maxBioLines && line != "" {
			b.WriteString(m.styles.ForegroundStyle.Render(" " + strings.TrimSpace(line)))
			lineCount++
		}
	} else if info.Bio == "No biography found." {
		b.WriteString(m.styles.MutedStyle.Render(" No bio available"))
		lineCount++
	}

	if info.BioSource != "" {
		b.WriteString("\n")
		b.WriteString(m.styles.MutedStyle.Render(" Source: " + info.BioSource))
		lineCount++
	}

	if info.Discography != "" && lineCount < height-3 {
		b.WriteString("\n")
		b.WriteString(m.styles.AccentStyle.Render(" Discography"))
		if info.DiscoSource != "" {
			b.WriteString(m.styles.MutedStyle.Render(" (" + info.DiscoSource + ")"))
		}
		b.WriteString("\n")

		discoLines := strings.Split(info.Discography, "\n")
		for _, dl := range discoLines {
			if lineCount >= height-2 {
				break
			}
			b.WriteString(m.styles.ForegroundStyle.Render(" " + dl))
			b.WriteString("\n")
			lineCount++
		}
	}

	if info.PageURL != "" && lineCount < height-1 {
		b.WriteString("\n")
		b.WriteString(m.styles.MutedStyle.Render(" " + info.PageURL))
	}

	if len(info.GalleryURLs) > 0 {
		b.WriteString("\n")
		galleryHint := fmt.Sprintf(" %d images — press I for gallery", len(info.GalleryURLs))
		b.WriteString(m.styles.MutedStyle.Render(galleryHint))
	}

	return b.String()
}

func (m Model) renderFooter() string {
	if m.activeModal != ModalNone {
		m.footer.SetMiniMode(true)
	} else if m.layoutMode() == "compact" || m.layoutMode() == "narrow" {
		m.footer.SetMiniMode(true)
	} else {
		m.footer.SetMiniMode(false)
	}
	m.footer.SetWidth(m.width)

	var services []string
	if m.cfg.LastFM.Enabled && m.cfg.LastFM.SessionKey != "" {
		services = append(services, "LFM")
	}
	if m.cfg.ListenBrainz.Enabled && m.cfg.ListenBrainz.Token != "" {
		services = append(services, "LB")
	}
	m.footer.SetScrobbleServices(services)

	return m.footer.View()
}
