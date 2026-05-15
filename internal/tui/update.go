package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/tui/modals"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if cmd := m.nowPlaying.Update(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg, cmds)

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

	case modals.GalleryMsg:
		m.activeModal = ModalNone
		m.galleryModal = nil
		return m, nil

	case modals.GalleryImageLoadedMsg:
		if m.galleryModal != nil {
			cmd := m.galleryModal.HandleImageLoaded(msg)
			return m, cmd
		}
		return m, nil

	case modals.GalleryRenderImageMsg:
		return m, tea.Raw(fmt.Sprintf("\x1b[%d;%dH%s", msg.Row, msg.Col, msg.ImageStr))

	case audioInfoMsg:
		m.audioInfo = msg.info
		return m, nil

	case imageLoadedMsg:
		return m.handleImageLoaded(msg)

	case renderAlbumArtMsg:
		return m.handleRenderAlbumArt(msg)

	case onlineArtFetchedMsg:
		if msg.err == nil && msg.trackPath != "" && m.imageRenderer != nil {
			if m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
				if m.playlist[m.currentIndex].Path == msg.trackPath {
					return m, loadAlbumArtCmd(m.imageRenderer, msg.trackPath)
				}
			}
		}
		return m, nil

	case notificationSentMsg:
		m.notifSentForSong = true
		return m, nil

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

	case artistInfoFetchedMsg:
		m.artistInfoLoading = false
		if msg.eventID != m.artistInfoEventID {
			return m, nil
		}
		if msg.info != nil {
			m.artistInfo = msg.info
			var artist string
			if m.playing && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
				artist = m.playlist[m.currentIndex].Artist
			}
			if artist != "" {
				m.artistCache[strings.ToLower(artist)] = msg.info
			}
		}
		return m, nil

	case restorePlaybackMsg:
		m.playing = true
		m.paused = false
		if msg.position > 0 && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
			m.songStartTime = time.Now().Add(-time.Duration(msg.position * float64(time.Second)))
		}
		cmds := []tea.Cmd{m.trackChangedCmds()}
		cmds = append(cmds, setStatus(&m, "Resumed playback", false))
		return m, tea.Batch(cmds...)

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

	return m, tea.Batch(cmds...)
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

func (m Model) handleWindowSize(msg tea.WindowSizeMsg, priorCmds []tea.Cmd) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.header.SetWidth(m.width)
	priorCmds = append(priorCmds, renderAlbumArtAfterDelay())
	return m, tea.Batch(priorCmds...)
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.activeModal != ModalNone {
		return m.handleModalKey(msg)
	}

	switch {
	case key.Matches(msg, m.keyMap.Quit):
		var cmds []tea.Cmd
		if m.playing && m.currentIndex >= 0 && len(m.playlist) > 0 {
			SavePlaybackState(m.playlist, m.currentIndex, m.playbackPos.TimePos, m.shuffle, m.repeatMode)
		} else {
			ClearPlaybackState()
		}
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

	case key.Matches(msg, m.keyMap.Enqueue):
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

	case key.Matches(msg, m.keyMap.Gallery):
		return m.openGallery()

	case key.Matches(msg, m.keyMap.Enter):
		if len(m.playlist) > 0 {
			cursor := m.playlistWidget.GetCursor()
			if cursor >= 0 && cursor < len(m.playlist) {
				if m.playing && m.mpvBackend.IsRunning() {
					mpvIdx := m.playlistIndexToMPVIndex(cursor)
					if mpvIdx >= 0 {
						_ = m.mpvBackend.PlaylistPlayIndex(mpvIdx)
					}
				}
				if m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
					track := m.playlist[m.currentIndex]
					m.prevTrack = &track
					m.prevSongStartTime = m.songStartTime
					m.prevScrobbleEligible = m.scrobbleEligible
				}
				m.currentIndex = cursor
				if !m.mpvBackend.IsRunning() {
					paths := m.buildMPVPlaylistPaths()
					playIdx := m.playlistIndexToMPVIndex(cursor)
					return m, tea.Batch(
						startPlaybackCmd(m.mpvBackend, paths, playIdx),
						m.trackChangedCmds(),
					)
				}
				return m, m.trackChangedCmds()
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
	case ModalGallery:
		if m.galleryModal != nil {
			cmd := m.galleryModal.Update(msg)
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
		paths := m.buildMPVPlaylistPaths()
		playIdx := m.playlistIndexToMPVIndex(msg.PlayIndex)
		return m, tea.Batch(
			startPlaybackCmd(m.mpvBackend, paths, playIdx),
			m.trackChangedCmds(),
		)
	}

	if len(msg.Enqueue) > 0 {
		wasPlaying := m.playing && m.mpvBackend.IsRunning()
		m.playlist = append(m.playlist, msg.Enqueue...)
		m.updatePlaylist()
		m.activeModal = ModalNone

		if wasPlaying {
			newPaths := make([]string, len(msg.Enqueue))
			for i, t := range msg.Enqueue {
				newPaths[i] = t.Path
			}
			_ = m.mpvBackend.AppendToPlaylist(newPaths)
		} else if len(m.playlist) > 0 {
			paths := m.buildMPVPlaylistPaths()
			playIdx := m.playlistIndexToMPVIndex(len(m.playlist) - len(msg.Enqueue))
			return m, tea.Batch(
				startPlaybackCmd(m.mpvBackend, paths, playIdx),
				m.trackChangedCmds(),
				setStatus(&m, fmt.Sprintf("Enqueued %d track(s) — playing", len(msg.Enqueue)), false),
			)
		}

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
		paths := m.buildMPVPlaylistPaths()
		playIdx := m.playlistIndexToMPVIndex(msg.PlayIndex)
		return m, tea.Batch(
			startPlaybackCmd(m.mpvBackend, paths, playIdx),
			m.trackChangedCmds(),
		)
	}

	if len(msg.Enqueue) > 0 {
		wasPlaying := m.playing && m.mpvBackend.IsRunning()
		m.playlist = append(m.playlist, msg.Enqueue...)
		m.updatePlaylist()
		m.activeModal = ModalNone
		m.searchModal.Blur()

		if wasPlaying {
			newPaths := make([]string, len(msg.Enqueue))
			for i, t := range msg.Enqueue {
				newPaths[i] = t.Path
			}
			_ = m.mpvBackend.AppendToPlaylist(newPaths)
		} else if len(m.playlist) > 0 {
			paths := m.buildMPVPlaylistPaths()
			playIdx := m.playlistIndexToMPVIndex(len(m.playlist) - len(msg.Enqueue))
			return m, tea.Batch(
				startPlaybackCmd(m.mpvBackend, paths, playIdx),
				m.trackChangedCmds(),
				setStatus(&m, "Track enqueued — playing", false),
			)
		}

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

func (m Model) openGallery() (tea.Model, tea.Cmd) {
	if m.artistInfo == nil || len(m.artistInfo.GalleryURLs) == 0 {
		return m, setStatus(&m, "No gallery images available", true)
	}

	m.activeModal = ModalGallery
	m.galleryModal = modals.NewGallery(
		m.styles, m.artistInfo.GalleryURLs, m.artistInfo.GallerySource,
		m.width, m.height, m.cellRatio, m.fontW, m.fontH,
	)
	m.galleryModal.SetProtocol(m.imageProtocol)
	return m, m.galleryModal.PrefetchImages()
}

func (m Model) openArtistBio() (tea.Model, tea.Cmd) {
	var artist string
	var album string
	if m.playing && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
		artist = m.playlist[m.currentIndex].Artist
		album = m.playlist[m.currentIndex].Album
	} else if len(m.artists) > 0 {
		artist = m.artists[0]
	}
	if artist == "" {
		return m, setStatus(&m, "No artist selected", true)
	}

	m.bottomViewMode = BottomArtistBio
	m.artistInfo = nil
	m.artistInfoLoading = true
	m.artistInfoEventID++

	return m, fetchArtistInfoCmd(m.cfg, artist, album, m.artistInfoEventID, m.artistCache)
}

func (m Model) deleteCurrentTrack() (tea.Model, tea.Cmd) {
	cursor := m.playlistWidget.GetCursor()
	if len(m.playlist) > 0 && cursor >= 0 && cursor < len(m.playlist) {
		mpvIdx := m.playlistIndexToMPVIndex(cursor)
		isCurrentTrack := cursor == m.currentIndex

		m.playlist = append(m.playlist[:cursor], m.playlist[cursor+1:]...)
		if cursor >= len(m.playlist) && len(m.playlist) > 0 {
			m.playlistWidget.SetCursor(len(m.playlist) - 1)
		}
		if m.currentIndex >= len(m.playlist) {
			m.currentIndex = len(m.playlist) - 1
		} else if cursor < m.currentIndex {
			m.currentIndex--
		}

		if m.shuffle && len(m.shuffleOrder) > 0 {
			newOrder := make([]int, 0, len(m.playlist))
			for _, idx := range m.shuffleOrder {
				if idx == cursor {
					continue
				}
				if idx > cursor {
					idx--
				}
				newOrder = append(newOrder, idx)
			}
			m.shuffleOrder = newOrder
		}

		if m.mpvBackend.IsRunning() && mpvIdx >= 0 {
			_ = m.mpvBackend.RemoveFromPlaylist(mpvIdx)
		}

		m.updatePlaylist()

		if isCurrentTrack && m.mpvBackend.IsRunning() && len(m.playlist) > 0 {
			newMPVIdx := m.playlistIndexToMPVIndex(m.currentIndex)
			if newMPVIdx >= 0 {
				_ = m.mpvBackend.PlaylistPlayIndex(newMPVIdx)
			}
			return m, tea.Batch(setStatus(&m, "Track removed", false), m.trackChangedCmds())
		}

		if len(m.playlist) == 0 {
			m.playing = false
			m.paused = false
			if m.mpvBackend != nil {
				_ = m.mpvBackend.Stop()
			}
		}

		return m, setStatus(&m, "Track removed", false)
	}
	return m, nil
}

func (m Model) clearPlaylist() (tea.Model, tea.Cmd) {
	m.playlist = nil
	m.currentIndex = -1
	m.shuffleOrder = nil
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
