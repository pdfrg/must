package tui

import (
	"fmt"
	"log"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/models"
	"github.com/pdfrg/must/internal/playlist"
	"github.com/pdfrg/must/internal/tui/modals"
)

var tuiLogger *log.Logger

func SetLogger(l *log.Logger) {
	tuiLogger = l
}

func logf(format string, args ...any) {
	if tuiLogger != nil {
		tuiLogger.Printf(format, args...)
	}
}

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
		cmds := []tea.Cmd{renderAlbumArtAfterDelay()}
		if m.bottomViewMode == BottomArtistBio && m.artistArtLoaded {
			cmds = append(cmds, renderArtistArtAfterDelay())
		}
		return m, tea.Batch(cmds...)

	case modals.GalleryMsg:
		m.activeModal = ModalNone
		m.galleryModal = nil
		cmds := []tea.Cmd{clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay()}
		if m.bottomViewMode == BottomArtistBio && m.artistArtLoaded {
			cmds = append(cmds, renderArtistArtAfterDelay())
		}
		return m, tea.Batch(cmds...)

	case modals.OptionsMsg:
		return m.handleOptionsModalMsg(msg)

	case modals.GalleryImageLoadedMsg:
		if m.galleryModal != nil {
			cmd := m.galleryModal.HandleImageLoaded(msg)
			return m, cmd
		}
		return m, nil

	case modals.GalleryRenderImageMsg:
		return m, tea.Raw(fmt.Sprintf("\x1b[%d;%dH%s", msg.Row, msg.Col, msg.ImageStr))

	case modals.SearchDebounceMsg:
		if m.activeModal == ModalSearch && m.searchModal != nil {
			return m, m.searchModal.Update(msg)
		}
		return m, nil

	case modals.SearchResultsMsg:
		if m.activeModal == ModalSearch && m.searchModal != nil {
			return m, m.searchModal.Update(msg)
		}
		return m, nil

	case audioInfoMsg:
		m.audioInfo = msg.info
		return m, nil

	case imageLoadedMsg:
		return m.handleImageLoaded(msg)

	case renderAlbumArtMsg:
		return m.handleRenderAlbumArt(msg)

	case artistImageLoadedMsg:
		return m.handleArtistImageLoaded(msg)

	case renderArtistArtMsg:
		return m, m.renderArtistArtCmd()

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
		m.updateBottomView()
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

			m.updateBottomView()

			var trackPath string
			if m.playing && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
				trackPath = m.playlist[m.currentIndex].Path
			}
			cmds = append(cmds, loadArtistImageCmd(msg.eventID, artist, trackPath, msg.info.ThumbnailURL))
		}
		return m, tea.Batch(cmds...)

	case restorePlaybackMsg:
		m.playing = true
		m.paused = false
		m.restoringPlayback = false
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
	if m.savingPlaylist {
		return m.handleSaveInputKey(msg)
	}

	if m.activeModal != ModalNone {
		return m.handleModalKey(msg)
	}

	switch {
	case key.Matches(msg, m.keyMap.Quit):
		var cmds []tea.Cmd
		cmds = append(cmds, clearKittyImagesCmdIf(m.imageProtocol))
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

	case key.Matches(msg, m.keyMap.RestartSong):
		return m.restartSong()

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

	case key.Matches(msg, m.keyMap.MoveTrackUp):
		return m.moveTrackUp()

	case key.Matches(msg, m.keyMap.MoveTrackDown):
		return m.moveTrackDown()

	case key.Matches(msg, m.keyMap.SavePlaylist):
		return m.savePlaylist()

	case key.Matches(msg, m.keyMap.EnqueueNext):
		return m.enqueueHighlightedNext()

	case key.Matches(msg, m.keyMap.Rescan):
		return m.rescanLibrary()

	case key.Matches(msg, m.keyMap.Lyrics):
		if m.bottomViewMode == BottomLyrics {
			m.bottomViewMode = BottomPlaylist
			return m, tea.Batch(clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay())
		}
		if m.bottomViewMode == BottomArtistBio {
			m.artistArtStr = ""
			m.artistArtLoaded = false
			m.artistArtEventID = 0
		}
		m.bottomViewMode = BottomLyrics
		m.viewport.GotoTop()
		m.updateBottomView()
		return m, tea.Batch(clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay())

	case key.Matches(msg, m.keyMap.SyncedLyrics):
		if m.bottomViewMode == BottomSyncedLyrics {
			m.bottomViewMode = BottomPlaylist
			return m, tea.Batch(clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay())
		}
		if m.bottomViewMode == BottomArtistBio {
			m.artistArtStr = ""
			m.artistArtLoaded = false
			m.artistArtEventID = 0
		}
		m.bottomViewMode = BottomSyncedLyrics
		m.viewport.GotoTop()
		return m, tea.Batch(clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay())

	case key.Matches(msg, m.keyMap.ArtistBio):
		if m.bottomViewMode == BottomArtistBio {
			m.artistArtStr = ""
			m.artistArtLoaded = false
			m.artistArtEventID = 0
			m.bottomViewMode = BottomPlaylist
			return m, tea.Batch(clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay())
		}
		return m.openArtistBio()

	case key.Matches(msg, m.keyMap.Gallery):
		return m.openGallery()

	case key.Matches(msg, m.keyMap.Options):
		return m.openOptions()

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
		if m.bottomViewMode == BottomLyrics || m.bottomViewMode == BottomArtistBio {
			m.viewport.ScrollDown(1)
			return m, nil
		}
		return m, m.playlistWidget.Update(msg)

	case key.Matches(msg, m.keyMap.CursorUp):
		if m.bottomViewMode == BottomLyrics || m.bottomViewMode == BottomArtistBio {
			m.viewport.ScrollUp(1)
			return m, nil
		}
		return m, m.playlistWidget.Update(msg)

	case key.Matches(msg, m.keyMap.PageDown):
		if m.bottomViewMode == BottomLyrics || m.bottomViewMode == BottomArtistBio {
			m.viewport.ScrollDown(m.viewport.Height())
			return m, nil
		}
		return m, m.playlistWidget.Update(msg)

	case key.Matches(msg, m.keyMap.PageUp):
		if m.bottomViewMode == BottomLyrics || m.bottomViewMode == BottomArtistBio {
			m.viewport.ScrollUp(m.viewport.Height())
			return m, nil
		}
		return m, m.playlistWidget.Update(msg)

	case key.Matches(msg, m.keyMap.Home):
		if m.bottomViewMode == BottomLyrics || m.bottomViewMode == BottomArtistBio {
			m.viewport.GotoTop()
			return m, nil
		}
		return m, m.playlistWidget.Update(msg)

	case key.Matches(msg, m.keyMap.End):
		if m.bottomViewMode == BottomLyrics || m.bottomViewMode == BottomArtistBio {
			m.viewport.GotoBottom()
			return m, nil
		}
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
	case ModalOptions:
		if m.optionsModal != nil {
			cmd := m.optionsModal.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) handleLibraryModalMsg(msg modals.LibraryModalMsg) (tea.Model, tea.Cmd) {
	if msg.Closed {
		m.activeModal = ModalNone
		cmds := []tea.Cmd{renderAlbumArtAfterDelay()}
		if m.bottomViewMode == BottomArtistBio && m.artistArtLoaded {
			cmds = append(cmds, renderArtistArtAfterDelay())
		}
		return m, tea.Batch(cmds...)
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
			renderAlbumArtAfterDelay(),
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
			return m, tea.Batch(
				setStatus(&m, fmt.Sprintf("Enqueued %d track(s)", len(msg.Enqueue)), false),
				renderAlbumArtAfterDelay(),
			)
		} else if len(m.playlist) > 0 {
			paths := m.buildMPVPlaylistPaths()
			playIdx := m.playlistIndexToMPVIndex(len(m.playlist) - len(msg.Enqueue))
			return m, tea.Batch(
				startPlaybackCmd(m.mpvBackend, paths, playIdx),
				m.trackChangedCmds(),
				setStatus(&m, fmt.Sprintf("Enqueued %d track(s) — playing", len(msg.Enqueue)), false),
				renderAlbumArtAfterDelay(),
			)
		}

		return m, setStatus(&m, fmt.Sprintf("Enqueued %d track(s)", len(msg.Enqueue)), false)
	}

	if len(msg.EnqueueNext) > 0 {
		wasPlaying := m.playing && m.mpvBackend.IsRunning()
		insertAt := m.currentIndex + 1
		if insertAt > len(m.playlist) {
			insertAt = len(m.playlist)
		}

		newTracks := msg.EnqueueNext
		m.playlist = append(m.playlist[:insertAt], append(newTracks, m.playlist[insertAt:]...)...)

		if m.currentIndex >= insertAt {
			m.currentIndex += len(newTracks)
		}

		m.updatePlaylist()
		m.activeModal = ModalNone

		if wasPlaying {
			mpvInsertAt := m.playlistIndexToMPVIndex(m.currentIndex)
			newPaths := make([]string, len(newTracks))
			for i, t := range newTracks {
				newPaths[i] = t.Path
			}
			_ = m.mpvBackend.InsertInPlaylist(newPaths, mpvInsertAt)
			return m, tea.Batch(
				setStatus(&m, fmt.Sprintf("Enqueued next %d track(s)", len(newTracks)), false),
				renderAlbumArtAfterDelay(),
			)
		} else if len(m.playlist) > 0 {
			paths := m.buildMPVPlaylistPaths()
			playIdx := m.playlistIndexToMPVIndex(insertAt)
			return m, tea.Batch(
				startPlaybackCmd(m.mpvBackend, paths, playIdx),
				m.trackChangedCmds(),
				setStatus(&m, fmt.Sprintf("Enqueued next %d track(s) — playing", len(newTracks)), false),
				renderAlbumArtAfterDelay(),
			)
		}

		return m, setStatus(&m, fmt.Sprintf("Enqueued next %d track(s)", len(newTracks)), false)
	}

	return m, nil
}

func (m Model) handleSearchModalMsg(msg modals.SearchModalMsg) (tea.Model, tea.Cmd) {
	if msg.Closed {
		m.activeModal = ModalNone
		m.searchModal.Blur()
		cmds := []tea.Cmd{renderAlbumArtAfterDelay()}
		if m.bottomViewMode == BottomArtistBio && m.artistArtLoaded {
			cmds = append(cmds, renderArtistArtAfterDelay())
		}
		return m, tea.Batch(cmds...)
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
			renderAlbumArtAfterDelay(),
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
			return m, tea.Batch(
				setStatus(&m, "Track enqueued", false),
				renderAlbumArtAfterDelay(),
			)
		} else if len(m.playlist) > 0 {
			paths := m.buildMPVPlaylistPaths()
			playIdx := m.playlistIndexToMPVIndex(len(m.playlist) - len(msg.Enqueue))
			return m, tea.Batch(
				startPlaybackCmd(m.mpvBackend, paths, playIdx),
				m.trackChangedCmds(),
				setStatus(&m, "Track enqueued — playing", false),
				renderAlbumArtAfterDelay(),
			)
		}

		return m, setStatus(&m, "Track enqueued", false)
	}

	if len(msg.EnqueueNext) > 0 {
		wasPlaying := m.playing && m.mpvBackend.IsRunning()
		insertAt := m.currentIndex + 1
		if insertAt > len(m.playlist) {
			insertAt = len(m.playlist)
		}

		newTracks := msg.EnqueueNext
		m.playlist = append(m.playlist[:insertAt], append(newTracks, m.playlist[insertAt:]...)...)

		if m.currentIndex >= insertAt {
			m.currentIndex += len(newTracks)
		}

		m.updatePlaylist()
		m.activeModal = ModalNone
		m.searchModal.Blur()

		if wasPlaying {
			mpvInsertAt := m.playlistIndexToMPVIndex(m.currentIndex)
			newPaths := make([]string, len(newTracks))
			for i, t := range newTracks {
				newPaths[i] = t.Path
			}
			_ = m.mpvBackend.InsertInPlaylist(newPaths, mpvInsertAt)
			return m, tea.Batch(
				setStatus(&m, "Track enqueued next", false),
				renderAlbumArtAfterDelay(),
			)
		} else if len(m.playlist) > 0 {
			paths := m.buildMPVPlaylistPaths()
			playIdx := m.playlistIndexToMPVIndex(insertAt)
			return m, tea.Batch(
				startPlaybackCmd(m.mpvBackend, paths, playIdx),
				m.trackChangedCmds(),
				setStatus(&m, "Track enqueued next — playing", false),
				renderAlbumArtAfterDelay(),
			)
		}

		return m, setStatus(&m, "Track enqueued next", false)
	}

	return m, nil
}

func (m Model) cycleView() (tea.Model, tea.Cmd) {
	prevMode := m.bottomViewMode
	m.bottomViewMode = (m.bottomViewMode + 1) % BottomViewModeCount

	if m.bottomViewMode == BottomSyncedLyrics && len(m.syncedLyrics) == 0 {
		m.bottomViewMode = (m.bottomViewMode + 1) % BottomViewModeCount
	}

	m.viewport.GotoTop()
	m.updateBottomView()

	if prevMode == BottomArtistBio && m.bottomViewMode != BottomArtistBio {
		m.artistArtStr = ""
		m.artistArtLoaded = false
		m.artistArtEventID = 0
		return m, tea.Batch(clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay())
	}

	if m.bottomViewMode == BottomArtistBio {
		if m.artistInfo == nil {
			newM, cmd := m.openArtistBio()
			return newM, cmd
		}
		if m.artistArtLoaded && m.artistArtStr != "" {
			return m, tea.Batch(renderAlbumArtAfterDelay(), renderArtistArtAfterDelay())
		}
		if m.artistInfo.ThumbnailURL != "" || m.playing {
			m.artistArtStr = ""
			m.artistArtLoaded = false
			m.artistInfoEventID++
			var trackPath string
			if m.playing && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
				trackPath = m.playlist[m.currentIndex].Path
			}
			var artist string
			if m.playing && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
				artist = m.playlist[m.currentIndex].Artist
			}
			return m, tea.Batch(
				loadArtistImageCmd(m.artistInfoEventID, artist, trackPath, m.artistInfo.ThumbnailURL),
				renderAlbumArtAfterDelay(),
			)
		}
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
	if m.libraryDB != nil {
		m.searchModal.SetDB(m.libraryDB)
	}
	cmd := m.searchModal.Focus()
	return m, tea.Batch(clearKittyImagesCmdIf(m.imageProtocol), cmd)
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
	return m, clearKittyImagesCmdIf(m.imageProtocol)
}

func (m Model) openHelp() (tea.Model, tea.Cmd) {
	m.activeModal = ModalHelp
	m.helpModal.SetSize(m.width, m.height)
	return m, clearKittyImagesCmdIf(m.imageProtocol)
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
	return m, tea.Batch(clearKittyImagesCmdIf(m.imageProtocol), m.galleryModal.PrefetchImages())
}

func (m Model) openOptions() (tea.Model, tea.Cmd) {
	m.activeModal = ModalOptions
	m.optionsModal = modals.NewOptions(
		m.styles,
		m.cfg.ShowAlbumArt,
		m.cfg.CopyAlbumArt,
		m.cfg.NotificationsEnabled,
		m.cfg.NotificationsShowArt,
		m.cfg.TransparentBackground,
		m.cfg.DisableTheme,
		m.cfg.Visualizer.Mode,
		m.cfg.Visualizer.ShowInfo,
		m.cfg.Visualizer.RealAudio,
		m.cfg.Theme,
	)
	return m, clearKittyImagesCmdIf(m.imageProtocol)
}

func (m Model) handleOptionsModalMsg(msg modals.OptionsMsg) (tea.Model, tea.Cmd) {
	if msg.Closed {
		m.activeModal = ModalNone
		m.optionsModal = nil
		return m, tea.Batch(clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay())
	}

	changed := false

	if msg.ShowAlbumArt != nil {
		m.cfg.ShowAlbumArt = *msg.ShowAlbumArt
		changed = true
	}
	if msg.CopyAlbumArt != nil {
		m.cfg.CopyAlbumArt = *msg.CopyAlbumArt
		changed = true
	}
	if msg.NotificationsEnabled != nil {
		m.cfg.NotificationsEnabled = *msg.NotificationsEnabled
		changed = true
	}
	if msg.NotificationsShowArt != nil {
		m.cfg.NotificationsShowArt = *msg.NotificationsShowArt
		changed = true
	}
	if msg.TransparentBackground != nil {
		m.cfg.TransparentBackground = *msg.TransparentBackground
		changed = true
	}
	if msg.DisableTheme != nil {
		m.cfg.DisableTheme = *msg.DisableTheme
		changed = true
	}
	if msg.VisualizerMode != nil {
		m.cfg.Visualizer.Mode = *msg.VisualizerMode
		changed = true
	}
	if msg.VisualizerShowInfo != nil {
		m.cfg.Visualizer.ShowInfo = *msg.VisualizerShowInfo
		changed = true
	}
	if msg.RealAudio != nil {
		m.cfg.Visualizer.RealAudio = *msg.RealAudio
		changed = true
	}
	if msg.Theme != nil {
		m.cfg.Theme = *msg.Theme
		if *msg.Theme == "" {
			m.cfg.ColorsFile = ""
		}
		changed = true
	}

	if !changed {
		m.activeModal = ModalNone
		m.optionsModal = nil
		return m, tea.Batch(clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay())
	}

	if err := m.cfg.Save(); err != nil {
		logf("Failed to save config: %v", err)
	}

	m.activeModal = ModalNone
	m.optionsModal = nil

	cmds := []tea.Cmd{clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay()}

	// If theme or display settings changed, reload theme and styles
	if msg.Theme != nil || msg.TransparentBackground != nil || msg.DisableTheme != nil {
		newTheme, err := config.LoadTheme(m.cfg.ColorsFile, m.cfg.Theme)
		if err == nil {
			m.theme = newTheme
			m.styles = config.NewThemeStyles(newTheme, m.cfg.TransparentBackground, m.cfg.DisableTheme, m.cfg.TerminalPalette)
			m.header.UpdateStyles(m.styles.Header)
			m.nowPlaying.UpdateStyles(m.styles, m.styles.Accent, m.styles.Cursor, m.styles.Background)
			m.playlistWidget.UpdateStyles(m.styles)
			m.footer.UpdateStyles(m.styles.AccentStyle, m.styles.MutedStyle)
		}
	}

	return m, tea.Batch(cmds...)
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
	m.artistArtStr = ""
	m.artistArtLoaded = false
	m.artistArtEventID = 0
	m.artistInfoEventID++
	m.viewport.GotoTop()
	m.updateBottomView()

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

func (m Model) moveTrackUp() (tea.Model, tea.Cmd) {
	cursor := m.playlistWidget.GetCursor()
	if len(m.playlist) < 2 || cursor <= 0 {
		return m, nil
	}

	m.playlist[cursor-1], m.playlist[cursor] = m.playlist[cursor], m.playlist[cursor-1]

	if m.currentIndex == cursor {
		m.currentIndex = cursor - 1
	} else if m.currentIndex == cursor-1 {
		m.currentIndex = cursor
	}

	m.playlistWidget.SetCursor(cursor - 1)
	m.updatePlaylist()

	if m.mpvBackend.IsRunning() && m.playing {
		mpvFrom := m.playlistIndexToMPVIndex(cursor)
		mpvTo := m.playlistIndexToMPVIndex(cursor - 1)
		if mpvFrom >= 0 && mpvTo >= 0 {
			_ = m.mpvBackend.PlaylistMove(mpvFrom, mpvTo)
		}
	}

	return m, setStatus(&m, "Moved up", false)
}

func (m Model) moveTrackDown() (tea.Model, tea.Cmd) {
	cursor := m.playlistWidget.GetCursor()
	if len(m.playlist) < 2 || cursor < 0 || cursor >= len(m.playlist)-1 {
		return m, nil
	}

	m.playlist[cursor], m.playlist[cursor+1] = m.playlist[cursor+1], m.playlist[cursor]

	if m.currentIndex == cursor {
		m.currentIndex = cursor + 1
	} else if m.currentIndex == cursor+1 {
		m.currentIndex = cursor
	}

	m.playlistWidget.SetCursor(cursor + 1)
	m.updatePlaylist()

	if m.mpvBackend.IsRunning() && m.playing {
		mpvFrom := m.playlistIndexToMPVIndex(cursor)
		mpvTo := m.playlistIndexToMPVIndex(cursor+1) + 1
		if mpvFrom >= 0 && mpvTo >= 1 {
			_ = m.mpvBackend.PlaylistMove(mpvFrom, mpvTo)
		}
	}

	return m, setStatus(&m, "Moved down", false)
}

func (m Model) savePlaylist() (tea.Model, tea.Cmd) {
	if len(m.playlist) == 0 {
		return m, setStatus(&m, "No tracks to save", true)
	}
	m.savingPlaylist = true
	m.saveInput.SetValue("")
	cmd := m.saveInput.Focus()
	return m, cmd
}

func (m Model) handleSaveInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := m.saveInput.Value()
		if name == "" {
			name = time.Now().Format("20060102-150405")
		}
		m.savingPlaylist = false
		m.saveInput.Blur()

		paths := make([]string, len(m.playlist))
		for i, t := range m.playlist {
			paths[i] = t.Path
		}
		savePath := config.GetPlaylistSavePath(name)
		if err := playlist.Save(savePath, paths); err != nil {
			return m, setStatus(&m, fmt.Sprintf("Save error: %v", err), true)
		}
		return m, setStatus(&m, fmt.Sprintf("Saved: %s", savePath), false)

	case "esc":
		m.savingPlaylist = false
		m.saveInput.Blur()
		return m, nil

	default:
		var cmd tea.Cmd
		m.saveInput, cmd = m.saveInput.Update(msg)
		return m, cmd
	}
}

func (m Model) openLibraryEnqueueNext() (tea.Model, tea.Cmd) {
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
	m.libraryModal.SetEnqueueNextMode(true)
	return m, clearKittyImagesCmdIf(m.imageProtocol)
}

func (m Model) enqueueHighlightedNext() (tea.Model, tea.Cmd) {
	if len(m.playlist) == 0 || m.currentIndex < 0 {
		return m, setStatus(&m, "Nothing playing", true)
	}
	cursor := m.playlistWidget.GetCursor()
	if cursor < 0 || cursor >= len(m.playlist) || cursor == m.currentIndex {
		return m, setStatus(&m, "Highlight a track to enqueue next", true)
	}

	track := m.playlist[cursor]
	insertAt := m.currentIndex + 1

	m.playlist = append(m.playlist[:insertAt], append([]models.Track{track}, m.playlist[insertAt:]...)...)

	if cursor < insertAt {
		m.currentIndex++
	} else {
	}

	if cursor >= insertAt {
		m.playlist = append(m.playlist[:cursor+1], m.playlist[cursor+2:]...)
	} else {
		m.playlist = append(m.playlist[:cursor], m.playlist[cursor+1:]...)
	}

	if m.currentIndex > cursor && cursor < insertAt {
		m.currentIndex--
	}

	if m.mpvBackend.IsRunning() && m.playing {
		mpvInsertAt := m.playlistIndexToMPVIndex(m.currentIndex)
		_ = m.mpvBackend.InsertInPlaylist([]string{track.Path}, mpvInsertAt)
		mpvRemoveFrom := m.playlistIndexToMPVIndex(cursor)
		if mpvRemoveFrom > mpvInsertAt {
			mpvRemoveFrom++
		}
		_ = m.mpvBackend.RemoveFromPlaylist(mpvRemoveFrom)
	}

	m.updatePlaylist()
	return m, setStatus(&m, fmt.Sprintf("Enqueued next: %s", track.Title), false)
}

func (m *Model) updateBottomView() {
	if m.bottomViewMode == BottomPlaylist || m.bottomViewMode == BottomOff || m.bottomViewMode == BottomSyncedLyrics {
		return
	}

	if m.bottomViewMode != BottomArtistBio && m.width > 0 {
		m.viewport.SetWidth(m.width)
	}

	var content string

	switch m.bottomViewMode {
	case BottomLyrics:
		if m.lyricsLoading {
			content = m.styles.MutedStyle.Render(" Loading lyrics...")
		} else if m.lyrics == "" {
			content = m.styles.MutedStyle.Render(" No lyrics available")
		} else {
			var b strings.Builder
			lines := strings.Split(m.lyrics, "\n")
			for i, line := range lines {
				b.WriteString(" ")
				b.WriteString(m.styles.ForegroundStyle.Render(line))
				if i < len(lines)-1 {
					b.WriteString("\n")
				}
			}
			content = b.String()
		}
		content += strings.Repeat("\n", 10)

	case BottomArtistBio:
		if m.artistArtLoaded && m.artistArtStr != "" {
			imgGap := m.artistArtWidth + 5
			newWidth := m.width - imgGap
			if newWidth < 30 {
				newWidth = 30
			}
			m.viewport.SetWidth(newWidth)
		} else {
			m.viewport.SetWidth(m.width)
		}

		if m.artistInfoLoading {
			content = m.styles.MutedStyle.Render("Loading artist info...")
		} else if m.artistInfo == nil {
			content = m.styles.MutedStyle.Render("No artist info available")
		} else {
			info := m.artistInfo
			var b strings.Builder

			indent := " "
			if m.artistArtLoaded && m.artistArtStr != "" {
				indent = ""
			}

			var title string
			if m.playing && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
				title = m.playlist[m.currentIndex].Artist
			}
			if title != "" {
				b.WriteString(m.styles.Header.Render(title))
				b.WriteString("\n")
			}

			lineWidth := m.viewport.Width() - 4
			if lineWidth < 20 {
				lineWidth = 20
			}

			if info.Bio != "" && info.Bio != "No biography found." {
				words := strings.Fields(info.Bio)
				var line string
				for _, w := range words {
					test := line + " " + w
					if lipgloss.Width(test) > lineWidth && line != "" {
						b.WriteString(m.styles.ForegroundStyle.Render(indent + strings.TrimSpace(line)))
						b.WriteString("\n")
						line = w
					} else {
						line = test
					}
				}
				if line != "" {
					b.WriteString(m.styles.ForegroundStyle.Render(indent + strings.TrimSpace(line)))
				}
			} else if info.Bio == "No biography found." {
				b.WriteString(m.styles.MutedStyle.Render(indent + "No bio available"))
			}

			if info.BioSource != "" {
				b.WriteString("\n")
				b.WriteString(m.styles.MutedStyle.Render(indent + "Source: " + info.BioSource))
			}

			if info.Discography != "" {
				b.WriteString("\n\n")
				b.WriteString(m.styles.AccentStyle.Render(indent + "Discography"))
				if info.DiscoSource != "" {
					b.WriteString(m.styles.MutedStyle.Render(" (" + info.DiscoSource + ")"))
				}
				b.WriteString("\n")

				discoLines := strings.Split(info.Discography, "\n")
				for _, dl := range discoLines {
					b.WriteString(m.styles.ForegroundStyle.Render(indent + dl))
					b.WriteString("\n")
				}
			}

			if info.PageURL != "" {
				b.WriteString("\n")
				b.WriteString(m.styles.MutedStyle.Render(indent + info.PageURL))
			}

			if len(info.GalleryURLs) > 0 {
				b.WriteString("\n")
				galleryHint := fmt.Sprintf("%s%d images — press I for gallery", indent, len(info.GalleryURLs))
				b.WriteString(m.styles.MutedStyle.Render(galleryHint))
			}

			content = b.String()
		}
		content += strings.Repeat("\n", 10)
	}

	viewWidth := m.viewport.Width()
	if content != "" && viewWidth > 0 {
		content = ansi.Wordwrap(content, viewWidth, "")
	}

	m.viewport.SetContent(content)
	m.viewport.SoftWrap = true
}
