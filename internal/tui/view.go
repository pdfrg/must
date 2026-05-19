package tui

import (
	"strings"
	"time"

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

	// Fullscreen visualizer replaces everything
	if m.visFullscreen && m.bottomViewMode == BottomVisualizer && m.vis != nil {
		return m.renderFullscreenVis()
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
		nowPlayingLines := lipgloss.Height(nowPlayingView)
		if nowPlayingLines < artHeight {
			for i := 0; i < artHeight-nowPlayingLines; i++ {
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n")

	currentHeight := lipgloss.Height(b.String())
	m.bottomSectionStartRow = currentHeight + 1
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

	b.WriteString("\n")
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
		SleepActive: m.sleepTimerActive,
		SleepMins:   int(time.Until(m.sleepTimerExpiresAt).Minutes()) + 1,
	}
	if m.savingPlaylist {
		modeStr := "[absolute]"
		if m.saveAsRelative {
			modeStr = "[relative]"
		}
		data.StatusMsg = "Save: " + m.saveInput.View() + "  " + modeStr + " (tab)"
		data.StatusIsErr = false
	} else {
		data.StatusMsg = m.statusMsg
		data.StatusIsErr = m.statusIsErr
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
		m.viewport.SetHeight(height)
		m.viewportReady = true
		return m.viewport.View()

	case BottomSyncedLyrics:
		return m.renderSyncedLyrics(height)

	case BottomArtistBio:
		m.viewport.SetHeight(height)
		m.viewportReady = true

		viewContent := m.viewport.View()
		if m.artistArtLoaded && m.artistArtStr != "" {
			availableSpace := m.height - 20 - 3
			if availableSpace >= m.artistArtHeight {
				leftPad := strings.Repeat(" ", m.artistArtWidth+5)
				vpLines := strings.Split(viewContent, "\n")
				for i, line := range vpLines {
					vpLines[i] = leftPad + line
				}
				viewContent = strings.Join(vpLines, "\n")
			} else {
				viewContent = "Increase terminal height to view artist info and image."
			}
		}
		return viewContent

	case BottomVisualizer:
		if m.vis == nil {
			return ""
		}
		m.vis.SetRows(max(3, height))
		if m.vis.AudioReady() {
			return m.vis.Render(m.width)
		}
		modeName := m.vis.ModeName()
		source := m.vis.AudioSource()
		retryStatus := m.vis.RetryStatus()
		var lines []string
		lines = append(lines, "")
		if retryStatus != "" {
			lines = append(lines, retryStatus)
		} else {
			lines = append(lines, "Loading "+modeName+" visualization...")
			lines = append(lines, "Connecting to "+source+" audio...")
		}
		lines = append(lines, "")
		return strings.Join(lines, "\n")

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
	case ModalOptions:
		if m.optionsModal != nil {
			m.optionsModal.SetSize(m.width, m.height)
			return m.optionsModal.View()
		}
	case ModalSleepTimer:
		if m.sleepTimerModal != nil {
			m.sleepTimerModal.SetSize(m.width, m.height)
			return m.sleepTimerModal.View()
		}
	}
	return ""
}

func (m Model) renderWithModal(modalView string) tea.View {
	return m.altView(modalView)
}

func (m Model) renderFullscreenVis() tea.View {
	if m.vis == nil {
		return m.altView("Visualizer not initialized")
	}
	rows := max(3, m.height)
	m.vis.SetRows(rows)

	var b strings.Builder

	if !m.vis.AudioReady() {
		modeName := m.vis.ModeName()
		source := m.vis.AudioSource()
		retryStatus := m.vis.RetryStatus()
		b.WriteString("\n\n")
		if retryStatus != "" {
			b.WriteString(retryStatus + "\n")
		} else {
			b.WriteString("Loading " + modeName + " visualization...\n")
			b.WriteString("Connecting to " + source + " audio...\n")
		}
		return m.altView(b.String())
	}

	vizContent := m.vis.Render(m.width)
	b.WriteString(vizContent)

	return m.altView(b.String())
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
		services = append(services, "fm")
	}
	if m.cfg.ListenBrainz.Enabled && m.cfg.ListenBrainz.Token != "" {
		services = append(services, "lb")
	}
	m.footer.SetScrobbleServices(services)
	m.footer.SetFlashStateByService(m.scrobbleStates)

	lidarrConfigured := m.cfg.Lidarr.Enabled && m.cfg.Lidarr.URL != "" && m.cfg.Lidarr.APIKey != ""
	m.footer.SetLidarrConfigured(lidarrConfigured)
	if lidarrConfigured && m.artistInfo != nil {
		switch {
		case m.artistInfo.LidarrError != "":
			m.footer.SetLidarrState(widgets.LidarrStateError)
		case m.artistInfo.LidarrMonitored:
			m.footer.SetLidarrState(widgets.LidarrStateMonitored)
		case m.artistInfo.LidarrInLidarr:
			m.footer.SetLidarrState(widgets.LidarrStateInLidarr)
		default:
			m.footer.SetLidarrState(widgets.LidarrStateNotInLidarr)
		}
	} else {
		m.footer.SetLidarrState(widgets.LidarrStateNotInLidarr)
	}

	return m.footer.View()
}
