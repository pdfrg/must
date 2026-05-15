package tui

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/tui/modals"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case progressTickMsg:
		return m.handleProgressTick(msg)

	case scanCompleteMsg:
		return m.handleScanComplete(msg)

	case trackChangedMsg:
		return m.handleTrackChanged(msg)

	case modals.LibraryModalMsg:
		return m.handleLibraryModalMsg(msg)

	case modals.SearchModalMsg:
		return m.handleSearchModalMsg(msg)

	case modals.HelpModalMsg:
		m.activeModal = ModalNone
		return m, nil

	case audioInfoMsg:
		m.audioInfo = msg.info
		return m, nil

	case imageLoadedMsg:
		return m.handleImageLoaded(msg)

	case renderAlbumArtMsg:
		return m.handleRenderAlbumArt(msg)

	case themeChangedMsg:
		return m.handleThemeChanged(msg)

	case lyricsFetchedMsg:
		m.lyricsLoading = false
		if msg.err != nil {
			m.lyrics = "Lyrics not found"
		} else {
			m.lyrics = msg.plain
			m.syncedLyrics = msg.synced
		}
		return m, nil

	case artistBioFetchedMsg:
		m.artistBioLoading = false
		if msg.err != nil || msg.summary == nil {
			m.artistBio = "Artist bio not found"
		} else {
			m.artistBio = msg.summary.Extract
			m.artistBioTitle = msg.summary.Title
			if msg.summary.URL != "" {
				m.artistBioURL = msg.summary.URL
			}
		}
		return m, nil

	case statusClearMsg:
		if msg.seq == m.statusSeq {
			m.statusMsg = ""
			m.statusIsErr = false
		}
		return m, nil

	case sleepTimerTickMsg:
		if m.sleepRemaining > 0 {
			m.sleepRemaining -= time.Minute
			if m.sleepRemaining <= 0 {
				return m, tea.Quit
			}
			return m, tickSleepTimerCmd()
		}
		return m, nil
	}

	return m, nil
}

func (m Model) handleThemeChanged(msg themeChangedMsg) (tea.Model, tea.Cmd) {
	newTheme, err := config.LoadTheme(m.cfg.ColorsFile, m.cfg.Theme)
	if err != nil {
		if m.themeWatcher != nil {
			return m, watchThemeCmd(m.themeWatcher)
		}
		return m, nil
	}
	m.theme = newTheme
	m.styles = config.NewThemeStyles(newTheme, m.cfg.TransparentBackground, m.cfg.DisableTheme, m.cfg.TerminalPalette)

	m.header.UpdateStyles(m.styles.Header)
	m.nowPlaying.UpdateStyles(m.styles, m.styles.Accent, m.styles.Cursor, m.styles.Background)
	m.playlistWidget.UpdateStyles(m.styles)
	m.footer.UpdateStyles(m.styles.AccentStyle, m.styles.MutedStyle)

	if m.themeWatcher != nil {
		return m, watchThemeCmd(m.themeWatcher)
	}
	return m, nil
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.header.SetWidth(m.width)
	return m, renderAlbumArtAfterDelay()
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.activeModal != ModalNone {
		return m.handleModalKey(msg)
	}

	switch {
	case key.Matches(msg, m.keyMap.Quit):
		var cmds []tea.Cmd
		if m.mpvBackend != nil {
			_ = m.mpvBackend.Stop()
		}
		if m.libraryDB != nil {
			_ = m.libraryDB.Close()
		}
		if m.themeWatcher != nil {
			m.themeWatcher.Close()
		}
		if m.scrobbleEligible && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
			cmds = append(cmds, scrobbleTrackCmd(m.cfg, m.playlist[m.currentIndex], m.songStartTime))
		}
		cmds = append(cmds, tea.Quit)
		return m, tea.Batch(cmds...)

	case key.Matches(msg, m.keyMap.PlayPause):
		return m.togglePause()

	case key.Matches(msg, m.keyMap.Next):
		return m.skipNext()

	case key.Matches(msg, m.keyMap.Prev):
		return m.skipPrev()

	case key.Matches(msg, m.keyMap.SeekForward):
		return m.seekForward()

	case key.Matches(msg, m.keyMap.SeekBackward):
		return m.seekBackward()

	case key.Matches(msg, m.keyMap.Repeat):
		return m.cycleRepeat()

	case key.Matches(msg, m.keyMap.Shuffle):
		return m.toggleShuffle()

	case key.Matches(msg, m.keyMap.CycleView):
		return m.cycleView()

	case key.Matches(msg, m.keyMap.Search):
		return m.openSearch()

	case key.Matches(msg, m.keyMap.Library):
		return m.openLibrary()

	case key.Matches(msg, m.keyMap.Help):
		return m.openHelp()

	case key.Matches(msg, m.keyMap.Escape):
		return m, nil

	case key.Matches(msg, m.keyMap.DeleteTrack):
		return m.deleteCurrentTrack()

	case key.Matches(msg, m.keyMap.ClearPlaylist):
		return m.clearPlaylist()

	case key.Matches(msg, m.keyMap.Rescan):
		return m.rescanLibrary()

	case key.Matches(msg, m.keyMap.Lyrics):
		m.bottomViewMode = BottomLyrics
		return m, nil

	case key.Matches(msg, m.keyMap.SyncedLyrics):
		m.bottomViewMode = BottomSyncedLyrics
		return m, nil

	case key.Matches(msg, m.keyMap.ArtistBio):
		return m.openArtistBio()

	case key.Matches(msg, m.keyMap.Enter):
		if len(m.playlist) > 0 && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
			cursor := m.playlistWidget.GetCursor()
			if cursor >= 0 && cursor < len(m.playlist) {
				m.currentIndex = cursor
				return m, m.playTrack(cursor)
			}
		}
		return m, nil

	case key.Matches(msg, m.keyMap.CursorDown):
		return m, m.playlistWidget.Update(msg)

	case key.Matches(msg, m.keyMap.CursorUp):
		return m, m.playlistWidget.Update(msg)

	case key.Matches(msg, m.keyMap.PageDown):
		return m, m.playlistWidget.Update(msg)

	case key.Matches(msg, m.keyMap.PageUp):
		return m, m.playlistWidget.Update(msg)

	case key.Matches(msg, m.keyMap.Home):
		return m, m.playlistWidget.Update(msg)

	case key.Matches(msg, m.keyMap.End):
		return m, m.playlistWidget.Update(msg)
	}

	return m, nil
}

func (m Model) handleModalKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.activeModal {
	case ModalLibrary:
		if m.libraryModal != nil {
			cmd := m.libraryModal.Update(msg)
			return m, cmd
		}
	case ModalSearch:
		if m.searchModal != nil {
			cmd := m.searchModal.Update(msg)
			return m, cmd
		}
	case ModalHelp:
		if m.helpModal != nil {
			cmd := m.helpModal.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) handleLibraryModalMsg(msg modals.LibraryModalMsg) (tea.Model, tea.Cmd) {
	if msg.Closed {
		m.activeModal = ModalNone
		return m, nil
	}

	if len(msg.PlayTracks) > 0 {
		m.playlist = msg.PlayTracks
		m.shuffleOrder = nil
		if m.shuffle {
			m.shuffleOrder = shuffleIndices(len(m.playlist))
		}
		m.updatePlaylist()
		m.activeModal = ModalNone
		return m, m.playTrack(msg.PlayIndex)
	}

	if len(msg.Enqueue) > 0 {
		m.playlist = append(m.playlist, msg.Enqueue...)
		m.updatePlaylist()
		m.activeModal = ModalNone
		return m, setStatus(&m, fmt.Sprintf("Enqueued %d track(s)", len(msg.Enqueue)), false)
	}

	return m, nil
}

func (m Model) handleSearchModalMsg(msg modals.SearchModalMsg) (tea.Model, tea.Cmd) {
	if msg.Closed {
		m.activeModal = ModalNone
		m.searchModal.Blur()
		return m, nil
	}

	if len(msg.PlayTracks) > 0 {
		m.playlist = msg.PlayTracks
		m.shuffleOrder = nil
		if m.shuffle {
			m.shuffleOrder = shuffleIndices(len(m.playlist))
		}
		m.updatePlaylist()
		m.activeModal = ModalNone
		m.searchModal.Blur()
		return m, m.playTrack(msg.PlayIndex)
	}

	if len(msg.Enqueue) > 0 {
		m.playlist = append(m.playlist, msg.Enqueue...)
		m.updatePlaylist()
		m.activeModal = ModalNone
		m.searchModal.Blur()
		return m, setStatus(&m, "Track enqueued", false)
	}

	return m, nil
}

func (m Model) cycleView() (tea.Model, tea.Cmd) {
	m.bottomViewMode = (m.bottomViewMode + 1) % BottomViewModeCount

	if m.bottomViewMode == BottomSyncedLyrics && len(m.syncedLyrics) == 0 {
		m.bottomViewMode = (m.bottomViewMode + 1) % BottomViewModeCount
	}

	viewName := "playlist"
	switch m.bottomViewMode {
	case BottomLyrics:
		viewName = "lyrics"
	case BottomSyncedLyrics:
		viewName = "synced lyrics"
	case BottomArtistBio:
		viewName = "artist bio"
	case BottomOff:
		viewName = "off"
	}
	return m, setStatus(&m, "View: "+viewName, false)
}

func (m Model) openSearch() (tea.Model, tea.Cmd) {
	m.activeModal = ModalSearch
	m.searchModal.Reset()
	m.searchModal.SetSize(m.width, m.height)
	cmd := m.searchModal.Focus()
	return m, cmd
}

func (m Model) openLibrary() (tea.Model, tea.Cmd) {
	if !m.libraryReady || m.libraryDB == nil {
		return m, setStatus(&m, "Library not ready", true)
	}
	m.activeModal = ModalLibrary
	if m.libraryModal == nil {
		m.libraryModal = modals.NewLibrary(m.styles, m.libraryDB)
	}
	m.libraryModal.SetArtists(m.artists)
	m.libraryModal.LoadAlbumsForArtist()
	m.libraryModal.SetSize(m.width, m.height)
	return m, nil
}

func (m Model) openHelp() (tea.Model, tea.Cmd) {
	m.activeModal = ModalHelp
	m.helpModal.SetSize(m.width, m.height)
	return m, nil
}

func (m Model) openArtistBio() (tea.Model, tea.Cmd) {
	var artist string
	if m.playing && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
		artist = m.playlist[m.currentIndex].Artist
	} else if len(m.artists) > 0 {
		artist = m.artists[0]
	}
	if artist == "" {
		return m, setStatus(&m, "No artist selected", true)
	}

	m.bottomViewMode = BottomArtistBio
	m.artistBio = ""
	m.artistBioTitle = artist
	m.artistBioURL = ""
	m.artistBioLoading = true

	return m, fetchArtistBioCmd(artist)
}

func (m Model) deleteCurrentTrack() (tea.Model, tea.Cmd) {
	cursor := m.playlistWidget.GetCursor()
	if len(m.playlist) > 0 && cursor >= 0 && cursor < len(m.playlist) {
		m.playlist = append(m.playlist[:cursor], m.playlist[cursor+1:]...)
		if cursor >= len(m.playlist) {
			m.playlistWidget.SetCursor(len(m.playlist) - 1)
		}
		if m.currentIndex >= len(m.playlist) {
			m.currentIndex = len(m.playlist) - 1
		}
		m.updatePlaylist()
		return m, setStatus(&m, "Track removed", false)
	}
	return m, nil
}

func (m Model) clearPlaylist() (tea.Model, tea.Cmd) {
	m.playlist = nil
	m.currentIndex = -1
	m.playing = false
	m.paused = false
	if m.mpvBackend != nil {
		_ = m.mpvBackend.Stop()
	}
	m.updatePlaylist()
	return m, setStatus(&m, "Playlist cleared", false)
}

func (m Model) rescanLibrary() (tea.Model, tea.Cmd) {
	if m.libraryDB != nil {
		_ = m.libraryDB.Close()
		m.libraryDB = nil
	}
	m.libraryReady = false
	m.artists = nil
	return m, scanLibraryCmd(m.cfg)
}

func (m Model) togglePause() (tea.Model, tea.Cmd) {
	if !m.mpvBackend.IsRunning() {
		return m, nil
	}
	if err := m.mpvBackend.TogglePause(); err != nil {
		return m, setStatus(&m, fmt.Sprintf("Pause error: %v", err), true)
	}
	m.paused = !m.paused
	return m, nil
}
