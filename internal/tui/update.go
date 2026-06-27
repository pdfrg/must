package tui

import (
	"fmt"
	"log"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	termimg "github.com/blacktop/go-termimg"
	"github.com/charmbracelet/x/ansi"
	"github.com/pdfrg/must/internal/api"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/ctl"
	"github.com/pdfrg/must/internal/models"
	"github.com/pdfrg/must/internal/playlist"
	"github.com/pdfrg/must/internal/tui/modals"
	"github.com/pdfrg/must/internal/tui/visualizer"
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

	case ctl.CtlMessage:
		newModel, result, cmd := m.handleCtlCommand(msg.Cmd, msg.Args)
		if msg.ResultCh != nil {
			select {
			case msg.ResultCh <- result:
			default:
			}
		}
		cmds = append(cmds, cmd)
		return newModel, tea.Batch(cmds...)

	case visTickMsg:
		return m.handleVisTick(msg)

	case tea.BackgroundColorMsg:
		m.isDark = msg.IsDark()
		logf("Terminal background color: dark=%v", m.isDark)
		return m, nil

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

	case modals.TempDirsModalMsg:
		return m.handleTempDirsModalMsg(msg)

	case modals.SleepTimerMsg:
		m.activeModal = ModalNone
		if msg.Closed {
			return m, renderAlbumArtAfterDelay()
		}
		if msg.Cancelled {
			m.stopSleepTimer()
			return m, tea.Batch(setStatus(&m, "Sleep timer cancelled", false), renderAlbumArtAfterDelay())
		}
		if msg.Duration > 0 {
			m.startSleepTimer(msg.Duration)
			mins := int(msg.Duration.Minutes())
			return m, tea.Batch(
				setStatus(&m, fmt.Sprintf("Sleep timer set for %d min", mins), false),
				tickSleepTimerCmd(),
				renderAlbumArtAfterDelay(),
			)
		}
		return m, renderAlbumArtAfterDelay()

	case modals.GalleryImageLoadedMsg:
		if m.galleryModal != nil {
			cmd := m.galleryModal.HandleImageLoaded(msg)
			return m, cmd
		}
		return m, nil

	case modals.GalleryRenderImageMsg:
		raw := termimg.ClearAllString() + fmt.Sprintf("\x1b[s\x1b[%d;%dH%s\x1b[u", msg.Row, msg.Col, msg.ImageStr)
		return m, tea.Raw(raw)

	case modals.SearchDebounceMsg:
		if m.activeModal == ModalSearch && m.searchModal != nil {
			var cmd tea.Cmd
			if m.subsonicClient != nil && m.searchModal.SourceWantsSubsonic() {
				cmd = subsonicSearchCmd(m.subsonicClient, msg.Query)
			}
			return m, tea.Batch(m.searchModal.Update(msg), cmd)
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
		if m.bottomViewMode == BottomLyrics && m.lyrics != "" {
			if msg.err != nil {
				m.pendingLyrics = "Lyrics not found"
			} else {
				m.pendingLyrics = msg.plain
			}
		} else {
			if msg.err != nil {
				m.lyrics = "Lyrics not found"
			} else {
				m.lyrics = msg.plain
				m.syncedLyrics = msg.synced
			}
			m.updateBottomView()
		}
		return m, nil

	case artistInfoFetchedMsg:
		m.artistInfoLoading = false
		if msg.eventID != m.artistInfoEventID {
			return m, nil
		}
		if msg.info != nil {
			var artist string
			if m.playing && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
				artist = m.playlist[m.currentIndex].Artist
			}
			if artist != "" {
				m.artistCache[strings.ToLower(artist)] = msg.info
			}

			if m.bottomViewMode == BottomArtistBio && m.artistInfo != nil {
				m.pendingArtistInfo = msg.info
			} else {
				m.artistInfo = msg.info
				m.updateBottomView()

				var trackPath string
				if m.playing && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
					trackPath = m.playlist[m.currentIndex].Path
				}
				cmds = append(cmds, loadArtistImageCmd(msg.eventID, artist, trackPath, msg.info.ThumbnailURL))
			}
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

	case subsonicSearchResultsMsg:
		if m.activeModal == ModalSearch && m.searchModal != nil {
			if msg.err != nil {
				logf("Subsonic search error: %v", msg.err)
				return m, nil
			}
			m.searchModal.AddSubsonicResults(msg.artists, msg.albums, msg.tracks)
		}
		return m, nil

	case subsonicArtistsMsg:
		if msg.err != nil {
			logf("Subsonic artists error: %v", msg.err)
			return m, nil
		}
		if m.libraryModal != nil {
			m.libraryModal.SetSubsonicArtists(msg.artists)
		}
		return m, nil

	case subsonicArtistAlbumsMsg:
		if msg.err != nil {
			logf("Subsonic artist albums error: %v", msg.err)
			return m, nil
		}
		if m.libraryModal != nil {
			m.libraryModal.SetSubsonicAlbums(msg.albums)
		}
		return m, nil

	case subsonicAlbumTracksMsg:
		if msg.err != nil {
			logf("Subsonic album tracks error: %v", msg.err)
			return m, nil
		}
		if m.libraryModal != nil {
			m.libraryModal.SetSubsonicTracks(msg.tracks)
		}
		// If search modal is active, this is a search resolve — enqueue/play/enqueue-next
		if m.activeModal == ModalSearch && m.searchModal != nil && len(msg.tracks) > 0 {
			if m.searchModal.ResolveEnqueue {
				m.searchModal.ResolveEnqueue = false
				closeSearch := func() tea.Msg {
					m.activeModal = ModalNone
					return modals.SearchModalMsg{Enqueue: msg.tracks}
				}
				return m, closeSearch
			}
			if m.searchModal.ResolveEnqueueNext {
				m.searchModal.ResolveEnqueueNext = false
				closeSearch := func() tea.Msg {
					m.activeModal = ModalNone
					return modals.SearchModalMsg{EnqueueNext: msg.tracks}
				}
				return m, closeSearch
			}
			m.playlist = msg.tracks
			m.shuffleOrder = nil
			if m.shuffle {
				m.shuffleOrder = shuffleIndices(len(m.playlist))
			}
			m.updatePlaylist()
			m.activeModal = ModalNone
			paths := m.buildMPVPlaylistPaths()
			return m, tea.Batch(
				startPlaybackCmd(m.mpvBackend, paths, 0),
				m.trackChangedCmds(),
				renderAlbumArtAfterDelay(),
			)
		}
		return m, nil

	case subsonicGenresMsg:
		if msg.err != nil {
			logf("Subsonic genres error: %v", msg.err)
			return m, nil
		}
		if m.libraryModal != nil {
			m.libraryModal.SetSubsonicGenres(msg.genres)
		}
		return m, nil

	case subsonicGenreAlbumsMsg:
		if msg.err != nil {
			logf("Subsonic genre albums error: %v", msg.err)
			return m, nil
		}
		if m.libraryModal != nil {
			m.libraryModal.SetSubsonicGenreAlbums(msg.genreName, msg.albums)
		}
		return m, nil

	case scrobbleResultMsg:
		m.scrobbleFlashAt = time.Now()
		m.scrobbleStates = make(map[string]int, len(msg.results))
		for _, r := range msg.results {
			if r.Success {
				m.scrobbleStates[r.Service] = 1 // FlashSolid
			} else {
				m.scrobbleStates[r.Service] = 2 // FlashBlinkOn
			}
		}
		return m, nil

	case sleepTimerTickMsg:
		if !m.sleepTimerActive {
			return m, nil
		}
		if time.Now().After(m.sleepTimerExpiresAt) || time.Now().Equal(m.sleepTimerExpiresAt) {
			m.sleepTimerActive = false
			if err := m.mpvBackend.Pause(true); err == nil {
				m.paused = true
				m.playing = false
			}
			m.quittingActive = true
			m.quittingStartedAt = time.Now()
			return m, tea.Batch(
				setStatus(&m, "Sleep timer expired — quitting in 60s...", false),
				tickQuitCmd(),
			)
		}
		return m, tickSleepTimerCmd()

	case quitTickMsg:
		if !m.quittingActive {
			return m, nil
		}
		elapsed := time.Since(m.quittingStartedAt)
		remaining := 60 - int(elapsed.Seconds())
		if remaining <= 0 {
			if m.mpvBackend != nil {
				_ = m.mpvBackend.Stop()
			}
			if m.libraryDB != nil {
				_ = m.libraryDB.Close()
			}
			if m.themeWatcher != nil {
				m.themeWatcher.Close()
			}
			return m, tea.Batch(clearKittyImagesCmdIf(m.imageProtocol), tea.Quit)
		}
		m.statusMsg = fmt.Sprintf("Sleep timer expired — quitting in %ds...", remaining)
		m.statusIsErr = false
		m.statusSeq++
		return m, tickQuitCmd()
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

	if m.vis != nil {
		m.vis.SetColors(m.styles.Accent, m.styles.Cursor, m.styles.Muted)
	}

	if m.themeWatcher != nil {
		return m, watchThemeCmd(m.themeWatcher)
	}
	return m, nil
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg, priorCmds []tea.Cmd) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.header.SetWidth(m.width)

	if !m.layoutCheckDone {
		fits, suboptimal, _ := checkTerminalSize(m.width, m.height, m.layoutMode())
		m.layoutPromptActive = !fits || suboptimal
		if fits && !suboptimal {
			m.layoutCheckDone = true
		}
	}

	priorCmds = append(priorCmds, renderAlbumArtAfterDelay())
	return m, tea.Batch(priorCmds...)
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if !m.layoutCheckDone {
		layout := m.layoutMode()
		fits, suboptimal, _ := checkTerminalSize(m.width, m.height, layout)
		if !fits || suboptimal {
			switch msg.String() {
			case "l":
				m.layoutPromptActive = false
				m.layoutOverride = "large"
				m.layoutCheckDone = true
				m.cfg.Layout = "large"
				return m, tea.Batch(setStatus(&m, "Layout: large", false), clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay())
			case "m":
				m.layoutPromptActive = false
				m.layoutOverride = "medium"
				m.layoutCheckDone = true
				m.cfg.Layout = "medium"
				return m, tea.Batch(setStatus(&m, "Layout: medium", false), clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay())
			case "c":
				if getFittingLayouts(m.width, m.height) == nil && layout != "" {
					break
				}
				m.layoutPromptActive = false
				m.layoutOverride = "compact"
				m.layoutCheckDone = true
				m.cfg.Layout = "compact"
				return m, tea.Batch(setStatus(&m, "Layout: compact", false), clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay())
			case "n":
				m.layoutPromptActive = false
				m.layoutOverride = "narrow"
				m.layoutCheckDone = true
				m.cfg.Layout = "narrow"
				return m, tea.Batch(setStatus(&m, "Layout: narrow", false), clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay())
			case "enter", " ", "space":
				m.layoutPromptActive = false
				m.layoutCheckDone = true
				return m, tea.Batch(clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay())
			case "q", "ctrl+c":
				if len(m.playlist) > 0 {
					SavePlaybackState(m.playlist, m.currentIndex, m.playbackPos.TimePos, m.shuffle, m.repeatMode)
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
				return m, tea.Batch(clearKittyImagesCmdIf(m.imageProtocol), tea.Quit)
			}
			return m, nil
		}
		m.layoutCheckDone = true
	}

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
		if len(m.playlist) > 0 {
			SavePlaybackState(m.playlist, m.currentIndex, m.playbackPos.TimePos, m.shuffle, m.repeatMode)
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
			cmds = append(cmds, scrobbleTrackCmd(m.cfg, m.subsonicClient, m.playlist[m.currentIndex], m.songStartTime))
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

	case key.Matches(msg, m.keyMap.MoveTrackTop):
		return m.moveTrackTop()

	case key.Matches(msg, m.keyMap.MoveTrackBottom):
		return m.moveTrackBottom()

	case key.Matches(msg, m.keyMap.SavePlaylist):
		return m.savePlaylist()

	case key.Matches(msg, m.keyMap.ReversePlaylist):
		return m.reversePlaylist()

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
		if m.hasPendingUpdate && m.pendingLyrics != "" {
			m.lyrics = m.pendingLyrics
			m.pendingLyrics = ""
			m.hasPendingUpdate = false
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

	case key.Matches(msg, m.keyMap.UpdateView):
		if !m.hasPendingUpdate || (m.bottomViewMode != BottomLyrics && m.bottomViewMode != BottomArtistBio) {
			return m, setStatus(&m, "No pending update", true)
		}
		var uCmds []tea.Cmd
		if m.pendingLyrics != "" {
			m.lyrics = m.pendingLyrics
			m.pendingLyrics = ""
		}
		if m.pendingArtistInfo != nil {
			m.artistInfo = m.pendingArtistInfo
			m.pendingArtistInfo = nil
			var artist string
			var trackPath string
			if m.playing && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
				track := m.playlist[m.currentIndex]
				artist = track.Artist
				trackPath = track.Path
			}
			if artist != "" {
				uCmds = append(uCmds, loadArtistImageCmd(m.artistInfoEventID, artist, trackPath, m.artistInfo.ThumbnailURL))
			}
		}
		m.hasPendingUpdate = false
		m.viewport.GotoTop()
		m.updateBottomView()
		uCmds = append(uCmds, renderAlbumArtAfterDelay())
		return m, tea.Batch(uCmds...)

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

	case key.Matches(msg, m.keyMap.VisualizerView):
		if m.bottomViewMode == BottomVisualizer {
			m.bottomViewMode = BottomPlaylist
			return m, tea.Batch(clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay())
		}
		var cmds []tea.Cmd
		if m.bottomViewMode == BottomArtistBio {
			m.artistArtStr = ""
			m.artistArtLoaded = false
			m.artistArtEventID = 0
			cmds = append(cmds, clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay())
		}
		m.bottomViewMode = BottomVisualizer
		if m.vis == nil {
			seed := uint64(0)
			if m.playing && m.currentIndex >= 0 {
				seed = uint64(m.currentIndex)
			}
			m.vis = visualizer.New(seed)
			m.vis.SetColors(m.styles.Accent, m.styles.Cursor, m.styles.Muted)
			mode := visualizer.ModeFromString(m.cfg.Visualizer.Mode)
			m.vis.SetMode(mode)
		} else {
			m.vis.SetColors(m.styles.Accent, m.styles.Cursor, m.styles.Muted)
		}
		source := m.vis.EnableRealAudio(m.cfg.Visualizer.RealAudio)
		m.vis.RequestRefresh()
		cmds = append(cmds, tickVisCmd(), setStatus(&m, "Visualizer: "+source, false))
		return m, tea.Batch(cmds...)

	case key.Matches(msg, m.keyMap.VisFullscreen):
		return m.toggleVisFullscreen()

	case key.Matches(msg, m.keyMap.LidarrBrowser):
		return m.openLidarr()

	case key.Matches(msg, m.keyMap.CopyClipboard):
		if m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
			track := m.playlist[m.currentIndex]
			return m, tea.Batch(
				setStatus(&m, "Copied: "+track.FormatDisplayInfo(), false),
				copyToClipboardCmd(track),
			)
		}
		return m, setStatus(&m, "Nothing playing", true)

	case key.Matches(msg, m.keyMap.SleepTimer):
		return m.openSleepTimer()

	case key.Matches(msg, m.keyMap.TempDirs):
		return m.openTempDirs()

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
		if m.bottomViewMode == BottomVisualizer && m.vis != nil {
			m.vis.CycleModeReverse()
			return m, setStatus(&m, "Visualizer: "+m.vis.ModeName(), false)
		}
		if m.bottomViewMode == BottomLyrics || m.bottomViewMode == BottomArtistBio {
			m.viewport.ScrollDown(1)
			return m, nil
		}
		return m, m.playlistWidget.Update(msg)

	case key.Matches(msg, m.keyMap.CursorUp):
		if m.bottomViewMode == BottomVisualizer && m.vis != nil {
			m.vis.CycleMode()
			return m, setStatus(&m, "Visualizer: "+m.vis.ModeName(), false)
		}
		if m.bottomViewMode == BottomLyrics || m.bottomViewMode == BottomArtistBio {
			m.viewport.ScrollUp(1)
			return m, nil
		}
		return m, m.playlistWidget.Update(msg)

	case key.Matches(msg, m.keyMap.PageDown):
		if m.bottomViewMode == BottomVisualizer || m.bottomViewMode == BottomLyrics || m.bottomViewMode == BottomArtistBio {
			m.viewport.ScrollDown(m.viewport.Height())
			return m, nil
		}
		return m, m.playlistWidget.Update(msg)

	case key.Matches(msg, m.keyMap.PageUp):
		if m.bottomViewMode == BottomVisualizer || m.bottomViewMode == BottomLyrics || m.bottomViewMode == BottomArtistBio {
			m.viewport.ScrollUp(m.viewport.Height())
			return m, nil
		}
		return m, m.playlistWidget.Update(msg)

	case key.Matches(msg, m.keyMap.Home):
		if m.bottomViewMode == BottomVisualizer || m.bottomViewMode == BottomLyrics || m.bottomViewMode == BottomArtistBio {
			m.viewport.GotoTop()
			return m, nil
		}
		return m, m.playlistWidget.Update(msg)

	case key.Matches(msg, m.keyMap.End):
		if m.bottomViewMode == BottomVisualizer || m.bottomViewMode == BottomLyrics || m.bottomViewMode == BottomArtistBio {
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
			var extra []tea.Cmd
			if m.subsonicClient != nil {
				if id := m.libraryModal.PendingFetchArtistID; id != "" {
					m.libraryModal.PendingFetchArtistID = ""
					extra = append(extra, subsonicArtistAlbumsCmd(m.subsonicClient, id))
				}
				if id := m.libraryModal.PendingFetchAlbumID; id != "" {
					m.libraryModal.PendingFetchAlbumID = ""
					extra = append(extra, subsonicAlbumTracksCmd(m.subsonicClient, id))
				}
				if name := m.libraryModal.PendingFetchGenreName; name != "" {
					m.libraryModal.PendingFetchGenreName = ""
					extra = append(extra, subsonicGenreAlbumsCmd(m.subsonicClient, name))
				}
			}
			if len(extra) > 0 {
				return m, tea.Batch(append([]tea.Cmd{cmd}, extra...)...)
			}
			return m, cmd
		}
	case ModalSearch:
		if m.searchModal != nil {
			cmd := m.searchModal.Update(msg)
			var cmd2 tea.Cmd
			var extra []tea.Cmd
			if m.subsonicClient != nil {
				if id := m.searchModal.PendingSubsonicArtistID; id != "" {
					m.searchModal.PendingSubsonicArtistID = ""
					extra = append(extra, subsonicArtistPlayCmd(m.subsonicClient, id))
				}
				if id := m.searchModal.PendingSubsonicAlbumID; id != "" {
					m.searchModal.PendingSubsonicAlbumID = ""
					extra = append(extra, subsonicAlbumTracksCmd(m.subsonicClient, id))
				}
			}
			if name := m.searchModal.ResolveArtistName; name != "" {
				m.searchModal.ResolveArtistName = ""
				enq := m.searchModal.ResolveEnqueueNext
				m.searchModal.ResolveEnqueueNext = false
				m, cmd2 = m.resolveSearchArtist(name, enq)
				extra = append(extra, cmd2)
			}
			if artist := m.searchModal.ResolveAlbumArtist; artist != "" {
				album := m.searchModal.ResolveAlbumName
				m.searchModal.ResolveAlbumArtist = ""
				m.searchModal.ResolveAlbumName = ""
				enq := m.searchModal.ResolveEnqueueNext
				m.searchModal.ResolveEnqueueNext = false
				m, cmd2 = m.resolveSearchAlbum(artist, album, enq)
				extra = append(extra, cmd2)
			}
			if len(extra) > 0 {
				return m, tea.Batch(append([]tea.Cmd{cmd}, extra...)...)
			}
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
	case ModalSleepTimer:
		if m.sleepTimerModal != nil {
			cmd := m.sleepTimerModal.Update(msg)
			return m, cmd
		}
	case ModalTempDirs:
		if m.tempDirsModal != nil {
			cmd := m.tempDirsModal.Update(msg)
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
		m.currentIndex = msg.PlayIndex
		m.shuffleOrder = nil
		if m.shuffle {
			m.shuffleOrder = shuffleIndices(len(m.playlist))
		}
		m.updatePlaylist()
		m.playlistWidget.SetCursor(msg.PlayIndex)
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

func (m Model) handleVisTick(msg visTickMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if m.vis != nil {
		if m.bottomViewMode == BottomVisualizer {
			m.vis.Tick(m.playing, m.paused)
			cmds = append(cmds, tickVisCmd())

			if m.visFullscreen && m.visInfoVisible && m.cfg.Visualizer.ShowInfo == "fade" {
				elapsed := time.Since(m.visInfoShownAt)
				if elapsed > time.Duration(m.cfg.Visualizer.InfoDuration)*time.Second {
					m.visInfoVisible = false
				}
			}
		} else {
			m.vis.Close()
			m.vis = nil
		}
	}
	return m, tea.Batch(cmds...)
}

func (m Model) cycleView() (tea.Model, tea.Cmd) {
	if m.layoutMode() != "large" {
		return m, setStatus(&m, "Bottom view unavailable in current layout", true)
	}
	var cmds []tea.Cmd
	prevMode := m.bottomViewMode
	m.bottomViewMode = (m.bottomViewMode + 1) % BottomViewModeCount

	if m.bottomViewMode == BottomSyncedLyrics && len(m.syncedLyrics) == 0 {
		m.bottomViewMode = (m.bottomViewMode + 1) % BottomViewModeCount
	}

	// Initialize visualizer when entering the view
	if m.bottomViewMode == BottomVisualizer {
		if prevMode == BottomArtistBio {
			m.artistArtStr = ""
			m.artistArtLoaded = false
			m.artistArtEventID = 0
			cmds = append(cmds, clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay())
		}
		if m.vis == nil {
			seed := uint64(0)
			if m.playing && m.currentIndex >= 0 {
				seed = uint64(m.currentIndex)
			}
			m.vis = visualizer.New(seed)
			m.vis.SetColors(m.styles.Accent, m.styles.Cursor, m.styles.Muted)
			mode := visualizer.ModeFromString(m.cfg.Visualizer.Mode)
			m.vis.SetMode(mode)
		} else {
			m.vis.SetColors(m.styles.Accent, m.styles.Cursor, m.styles.Muted)
		}
		source := m.vis.EnableRealAudio(m.cfg.Visualizer.RealAudio)
		m.vis.RequestRefresh()
		cmds = append(cmds, tickVisCmd())
		cmds = append(cmds, setStatus(&m, "Visualizer: "+source, false))
		return m, tea.Batch(cmds...)
	}

	if m.vis != nil {
		m.vis.Close()
	}

	if m.hasPendingUpdate {
		if m.bottomViewMode == BottomLyrics && m.pendingLyrics != "" {
			m.lyrics = m.pendingLyrics
			m.pendingLyrics = ""
			if m.pendingArtistInfo == nil {
				m.hasPendingUpdate = false
			}
		} else if m.bottomViewMode == BottomArtistBio && m.pendingArtistInfo != nil {
			m.artistInfo = m.pendingArtistInfo
			m.pendingArtistInfo = nil
			if m.pendingLyrics == "" {
				m.hasPendingUpdate = false
			}
		}
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
	case BottomVisualizer:
		viewName = "visualizer"
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
		if m.subsonicClient != nil {
			m.libraryModal.SetSubsonicBadge(m.subsonicClient.ServerBadge())
		}
	}
	m.libraryModal.SetArtists(m.artists)
	m.libraryModal.SetAlbumSort(m.cfg.AlbumSort)
	m.libraryModal.LoadAlbumsForArtist()
	m.libraryModal.SetSize(m.width, m.height)

	var cmds []tea.Cmd
	cmds = append(cmds, clearKittyImagesCmdIf(m.imageProtocol))

	// If Subsonic is configured, fetch artists and genres for Subsonic tabs
	if m.subsonicClient != nil {
		cmds = append(cmds, subsonicArtistsCmd(m.subsonicClient))
		cmds = append(cmds, subsonicGenresCmd(m.subsonicClient))
	}

	return m, tea.Batch(cmds...)
}

func (m Model) resolveSearchArtist(name string, enqueueNext bool) (Model, tea.Cmd) {
	tracks, err := m.libraryDB.GetTracksByArtist(name)
	if err != nil || len(tracks) == 0 {
		return m, nil
	}
	if enqueueNext {
		return m, func() tea.Msg {
			return modals.SearchModalMsg{EnqueueNext: tracks}
		}
	}
	m.playlist = tracks
	m.shuffleOrder = nil
	if m.shuffle {
		m.shuffleOrder = shuffleIndices(len(m.playlist))
	}
	m.updatePlaylist()
	m.activeModal = ModalNone
	paths := m.buildMPVPlaylistPaths()
	return m, tea.Batch(
		startPlaybackCmd(m.mpvBackend, paths, 0),
		m.trackChangedCmds(),
		renderAlbumArtAfterDelay(),
	)
}

func (m Model) resolveSearchAlbum(artist, album string, enqueueNext bool) (Model, tea.Cmd) {
	tracks, err := m.libraryDB.GetTracksByArtistAndAlbum(artist, album)
	if err != nil || len(tracks) == 0 {
		return m, nil
	}
	if enqueueNext {
		return m, func() tea.Msg {
			return modals.SearchModalMsg{EnqueueNext: tracks}
		}
	}
	m.playlist = tracks
	m.shuffleOrder = nil
	if m.shuffle {
		m.shuffleOrder = shuffleIndices(len(m.playlist))
	}
	m.updatePlaylist()
	m.activeModal = ModalNone
	paths := m.buildMPVPlaylistPaths()
	return m, tea.Batch(
		startPlaybackCmd(m.mpvBackend, paths, 0),
		m.trackChangedCmds(),
		renderAlbumArtAfterDelay(),
	)
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

func (m Model) openLidarr() (tea.Model, tea.Cmd) {
	if !m.cfg.Lidarr.Enabled || m.cfg.Lidarr.URL == "" || m.cfg.Lidarr.APIKey == "" {
		return m, setStatus(&m, "Lidarr not configured", true)
	}
	if m.currentIndex < 0 || m.currentIndex >= len(m.playlist) {
		return m, setStatus(&m, "No song playing", true)
	}

	var lidarrURL string
	if m.artistInfo != nil && m.artistInfo.LidarrInLidarr && m.artistInfo.LidarrMBID != "" {
		lidarrClient := api.NewLidarrClient(m.cfg.Lidarr.URL, m.cfg.Lidarr.APIKey, m.cfg.Lidarr.Enabled)
		lidarrURL = lidarrClient.OpenArtistURL(m.artistInfo.LidarrMBID)
	} else if m.artistInfo != nil && m.artistInfo.LidarrMBID != "" {
		lidarrClient := api.NewLidarrClient(m.cfg.Lidarr.URL, m.cfg.Lidarr.APIKey, m.cfg.Lidarr.Enabled)
		lidarrURL = lidarrClient.OpenSearchByMBID(m.artistInfo.LidarrMBID)
	} else {
		lidarrClient := api.NewLidarrClient(m.cfg.Lidarr.URL, m.cfg.Lidarr.APIKey, m.cfg.Lidarr.Enabled)
		lidarrURL = lidarrClient.OpenSearchURL(m.playlist[m.currentIndex].Artist)
	}

	api.OpenBrowser(lidarrURL)
	return m, setStatus(&m, "Opening Lidarr...", false)
}

func (m Model) openSleepTimer() (tea.Model, tea.Cmd) {
	layout := m.layoutMode()
	if layout == "compact" || layout == "narrow" {
		return m, setStatus(&m, "Sleep timer unavailable in compact/narrow layout", true)
	}
	var remaining time.Duration
	if m.sleepTimerActive {
		remaining = time.Until(m.sleepTimerExpiresAt)
	}
	m.sleepTimerModal = modals.NewSleepTimer(m.styles, m.sleepTimerActive, remaining)
	m.activeModal = ModalSleepTimer
	return m, clearKittyImagesCmdIf(m.imageProtocol)
}

func (m *Model) startSleepTimer(duration time.Duration) {
	m.stopSleepTimer()
	m.sleepTimerActive = true
	m.sleepTimer = duration
	m.sleepRemaining = duration
	m.sleepTimerExpiresAt = time.Now().Add(duration)
}

func (m *Model) stopSleepTimer() {
	m.sleepTimerActive = false
	m.sleepTimer = 0
	m.sleepRemaining = 0
	m.sleepTimerExpiresAt = time.Time{}
	m.quittingActive = false
	m.quittingStartedAt = time.Time{}
}

func (m Model) openTempDirs() (tea.Model, tea.Cmd) {
	if len(m.cfg.TempDirs) == 0 {
		return m, setStatus(&m, "No temp dirs configured — set temp_dirs in config", true)
	}
	m.tempDirsModal = modals.NewTempDirs(m.styles)
	m.tempDirsModal.SetSize(m.width, m.height)
	m.tempDirsModal.SetDirs(m.cfg.TempDirs)
	m.activeModal = ModalTempDirs
	return m, clearKittyImagesCmdIf(m.imageProtocol)
}

func (m Model) handleTempDirsModalMsg(msg modals.TempDirsModalMsg) (tea.Model, tea.Cmd) {
	if msg.Closed {
		m.activeModal = ModalNone
		m.tempDirsModal = nil
		return m, tea.Batch(clearKittyImagesCmdIf(m.imageProtocol), renderAlbumArtAfterDelay())
	}

	tracks := loadDirTracks(msg.DirPath, m.libraryDB)
	if len(tracks) == 0 {
		m.activeModal = ModalNone
		m.tempDirsModal = nil
		return m, setStatus(&m, "No audio files found", true)
	}

	switch msg.Action {
	case "play":
		m.playlist = tracks
		m.currentIndex = 0
		m.shuffleOrder = nil
		if m.shuffle {
			m.shuffleOrder = shuffleIndices(len(m.playlist))
		}
		m.updatePlaylist()
		m.playlistWidget.SetCursor(0)
		m.activeModal = ModalNone
		m.tempDirsModal = nil
		paths := m.buildMPVPlaylistPaths()
		return m, tea.Batch(
			startPlaybackCmd(m.mpvBackend, paths, 0),
			m.trackChangedCmds(),
			renderAlbumArtAfterDelay(),
		)

	case "enqueue":
		wasPlaying := m.playing && m.mpvBackend.IsRunning()
		m.playlist = append(m.playlist, tracks...)
		m.updatePlaylist()
		m.activeModal = ModalNone
		m.tempDirsModal = nil

		if wasPlaying {
			newPaths := make([]string, len(tracks))
			for i, t := range tracks {
				newPaths[i] = t.Path
			}
			_ = m.mpvBackend.AppendToPlaylist(newPaths)
			return m, tea.Batch(
				setStatus(&m, fmt.Sprintf("Enqueued %d track(s)", len(tracks)), false),
				renderAlbumArtAfterDelay(),
			)
		} else if len(m.playlist) > 0 {
			paths := m.buildMPVPlaylistPaths()
			playIdx := m.playlistIndexToMPVIndex(len(m.playlist) - len(tracks))
			return m, tea.Batch(
				startPlaybackCmd(m.mpvBackend, paths, playIdx),
				m.trackChangedCmds(),
				setStatus(&m, fmt.Sprintf("Enqueued %d track(s) — playing", len(tracks)), false),
				renderAlbumArtAfterDelay(),
			)
		}

		return m, setStatus(&m, fmt.Sprintf("Enqueued %d track(s)", len(tracks)), false)

	case "enqueue_next":
		wasPlaying := m.playing && m.mpvBackend.IsRunning()
		insertAt := m.currentIndex + 1
		if insertAt > len(m.playlist) {
			insertAt = len(m.playlist)
		}

		m.playlist = append(m.playlist[:insertAt], append(tracks, m.playlist[insertAt:]...)...)

		if m.currentIndex >= insertAt {
			m.currentIndex += len(tracks)
		}

		m.updatePlaylist()
		m.activeModal = ModalNone
		m.tempDirsModal = nil

		if wasPlaying {
			mpvInsertAt := m.playlistIndexToMPVIndex(m.currentIndex)
			newPaths := make([]string, len(tracks))
			for i, t := range tracks {
				newPaths[i] = t.Path
			}
			_ = m.mpvBackend.InsertInPlaylist(newPaths, mpvInsertAt)
			return m, tea.Batch(
				setStatus(&m, fmt.Sprintf("Enqueued next %d track(s)", len(tracks)), false),
				renderAlbumArtAfterDelay(),
			)
		} else if len(m.playlist) > 0 {
			paths := m.buildMPVPlaylistPaths()
			playIdx := m.playlistIndexToMPVIndex(insertAt)
			return m, tea.Batch(
				startPlaybackCmd(m.mpvBackend, paths, playIdx),
				m.trackChangedCmds(),
				setStatus(&m, fmt.Sprintf("Enqueued next %d track(s) — playing", len(tracks)), false),
				renderAlbumArtAfterDelay(),
			)
		}

		return m, setStatus(&m, fmt.Sprintf("Enqueued next %d track(s)", len(tracks)), false)
	}

	m.activeModal = ModalNone
	m.tempDirsModal = nil
	return m, nil
}

func (m Model) toggleVisFullscreen() (tea.Model, tea.Cmd) {
	if m.bottomViewMode != BottomVisualizer {
		return m, setStatus(&m, "Visualizer not active", true)
	}
	m.visFullscreen = !m.visFullscreen
	if m.visFullscreen {
		return m, tea.Batch(
			setStatus(&m, "Visualizer: fullscreen", false),
			clearKittyImagesCmdIf(m.imageProtocol),
		)
	}
	return m, tea.Batch(
		setStatus(&m, "Visualizer: windowed", false),
		clearKittyImagesCmdIf(m.imageProtocol),
		renderAlbumArtAfterDelay(),
	)
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
		m.cfg.ReplayGainMode,
		m.cfg.AlbumSort,
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
	if msg.ReplayGainMode != nil {
		m.cfg.ReplayGainMode = *msg.ReplayGainMode
		if m.mpvBackend != nil {
			_ = m.mpvBackend.SetReplayGainMode(*msg.ReplayGainMode)
		}
		changed = true
	}
	if msg.AlbumSort != nil {
		m.cfg.AlbumSort = *msg.AlbumSort
		if m.libraryModal != nil {
			m.libraryModal.SetAlbumSort(*msg.AlbumSort)
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
	m.artistArtStr = ""
	m.artistArtLoaded = false
	m.artistArtEventID = 0

	if m.hasPendingUpdate && m.pendingArtistInfo != nil {
		m.artistInfo = m.pendingArtistInfo
		m.pendingArtistInfo = nil
		m.hasPendingUpdate = false
		m.viewport.GotoTop()
		m.updateBottomView()
		var trackPath string
		if m.playing && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
			trackPath = m.playlist[m.currentIndex].Path
		}
		return m, tea.Batch(
			loadArtistImageCmd(m.artistInfoEventID, artist, trackPath, m.artistInfo.ThumbnailURL),
			renderAlbumArtAfterDelay(),
		)
	}

	m.artistInfo = nil
	m.artistInfoLoading = true
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
	m.playlistWidget.SetCursor(0)
	return m, setStatus(&m, "Playlist cleared", false)
}

func (m Model) rescanLibrary() (tea.Model, tea.Cmd) {
	if m.libraryDB != nil {
		_ = m.libraryDB.Close()
		m.libraryDB = nil
	}
	m.libraryReady = false
	m.scanning = true
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

func (m Model) moveTrackTop() (tea.Model, tea.Cmd) {
	cursor := m.playlistWidget.GetCursor()
	if len(m.playlist) < 2 || cursor <= 0 {
		return m, nil
	}

	track := m.playlist[cursor]
	m.playlist = append(m.playlist[:cursor], m.playlist[cursor+1:]...)
	m.playlist = append([]models.Track{track}, m.playlist...)

	if m.currentIndex == cursor {
		m.currentIndex = 0
	} else if m.currentIndex < cursor {
		m.currentIndex++
	}

	m.playlistWidget.SetCursor(0)
	m.updatePlaylist()

	if m.mpvBackend.IsRunning() && m.playing {
		mpvFrom := m.playlistIndexToMPVIndex(1)
		mpvTo := 0
		if mpvFrom >= 0 {
			_ = m.mpvBackend.PlaylistMove(mpvFrom, mpvTo)
		}
	}

	return m, setStatus(&m, "Moved to top", false)
}

func (m Model) moveTrackBottom() (tea.Model, tea.Cmd) {
	cursor := m.playlistWidget.GetCursor()
	if len(m.playlist) < 2 || cursor < 0 || cursor >= len(m.playlist)-1 {
		return m, nil
	}

	track := m.playlist[cursor]
	m.playlist = append(m.playlist[:cursor], m.playlist[cursor+1:]...)
	m.playlist = append(m.playlist, track)

	if m.currentIndex == cursor {
		m.currentIndex = len(m.playlist) - 1
	} else if m.currentIndex > cursor {
		m.currentIndex--
	}

	m.playlistWidget.SetCursor(len(m.playlist) - 1)
	m.updatePlaylist()

	if m.mpvBackend.IsRunning() && m.playing {
		mpvFrom := m.playlistIndexToMPVIndex(cursor)
		mpvTo := len(m.playlist) - 1
		if mpvFrom >= 0 {
			_ = m.mpvBackend.PlaylistMove(mpvFrom, mpvTo)
		}
	}

	return m, setStatus(&m, "Moved to bottom", false)
}

func (m Model) reversePlaylist() (tea.Model, tea.Cmd) {
	if len(m.playlist) < 2 {
		return m, setStatus(&m, "Playlist too short to reverse", true)
	}

	currPath := m.playlist[m.currentIndex].Path

	for i, j := 0, len(m.playlist)-1; i < j; i, j = i+1, j-1 {
		m.playlist[i], m.playlist[j] = m.playlist[j], m.playlist[i]
	}

	m.currentIndex = -1
	for i := range m.playlist {
		if m.playlist[i].Path == currPath {
			m.currentIndex = i
			break
		}
	}

	m.shuffleOrder = nil
	if m.shuffle {
		m.shuffleOrder = shuffleIndices(len(m.playlist))
	}

	m.playlistWidget.SetCursor(m.currentIndex)
	m.updatePlaylist()

	paths := m.buildMPVPlaylistPaths()
	playIdx := m.playlistIndexToMPVIndex(m.currentIndex)

	return m, tea.Batch(
		startPlaybackCmd(m.mpvBackend, paths, playIdx),
		m.trackChangedCmds(),
		setStatus(&m, "Playlist reversed", false),
	)
}

func (m Model) savePlaylist() (tea.Model, tea.Cmd) {
	if len(m.playlist) == 0 {
		return m, setStatus(&m, "No tracks to save", true)
	}
	m.savingPlaylist = true
	m.saveAsRelative = m.cfg.PlaylistPathMode == "relative"
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
		opts := &playlist.SaveOptions{
			UseEXTINF:     true,
			RelativePaths: m.saveAsRelative,
			Tracks:        m.playlist,
		}
		if err := playlist.Save(savePath, paths, opts); err != nil {
			return m, setStatus(&m, fmt.Sprintf("Save error: %v", err), true)
		}
		mode := "absolute"
		if m.saveAsRelative {
			mode = "relative"
		}
		return m, setStatus(&m, fmt.Sprintf("Saved (%s): %s", mode, savePath), false)

	case "tab":
		m.saveAsRelative = !m.saveAsRelative
		return m, nil

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
	if m.bottomViewMode == BottomPlaylist || m.bottomViewMode == BottomVisualizer || m.bottomViewMode == BottomOff || m.bottomViewMode == BottomSyncedLyrics {
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

			lidarrConfigured := m.cfg.Lidarr.Enabled && m.cfg.Lidarr.URL != "" && m.cfg.Lidarr.APIKey != ""
			if lidarrConfigured {
				b.WriteString("\n")
				var lidarrLine string
				if info.LidarrError != "" {
					lidarrLine = m.styles.MutedStyle.Render("?") + " error: " + info.LidarrError
				} else if info.LidarrMonitored {
					lidarrLine = m.styles.AccentStyle.Render("●") + " monitored"
				} else if info.LidarrInLidarr {
					lidarrLine = m.styles.ForegroundStyle.Render("○") + " not monitored"
				} else {
					lidarrLine = m.styles.MutedStyle.Render("⊝") + " not in Lidarr"
				}
				b.WriteString(m.styles.ForegroundStyle.Render(indent + "Lidarr artist: "))
				b.WriteString(lidarrLine)
				b.WriteString("\n")

				if len(info.LidarrAlbums) > 0 {
					legend := fmt.Sprintf("%s  %s  %s  %s  %s",
						m.styles.AccentStyle.Render("● downloaded"),
						m.styles.AccentStyle.Render("◐ partial"),
						m.styles.ForegroundStyle.Render("○ wanted"),
						m.styles.MutedStyle.Render("○ unmonitored"),
						m.styles.MutedStyle.Render("⊝ not in Lidarr"))
					b.WriteString(indent + legend)
					b.WriteString("\n\n")
				}
			}

			if info.Discography != "" {
				if !lidarrConfigured || len(info.LidarrAlbums) == 0 {
					b.WriteString("\n\n")
				}
				b.WriteString(m.styles.AccentStyle.Render(indent + "Discography"))
				if info.DiscoSource != "" {
					b.WriteString(m.styles.MutedStyle.Render(" (" + info.DiscoSource + ")"))
				}
				b.WriteString("\n")

				discoLines := strings.Split(info.Discography, "\n")
				for _, dl := range discoLines {
					line := indent
					if lidarrConfigured && len(info.LidarrAlbums) > 0 {
						albumTitle := dl
						if idx := strings.LastIndex(dl, " ("); idx > 0 {
							albumTitle = dl[:idx]
						}
						if albumInfo, ok := info.LidarrAlbums[albumTitle]; ok {
							var indicator string
							switch {
							case albumInfo.PercentOfTracks == 100:
								indicator = m.styles.AccentStyle.Render("● ")
							case albumInfo.PercentOfTracks > 0:
								indicator = m.styles.AccentStyle.Render("◐ ")
							case albumInfo.Monitored:
								indicator = m.styles.ForegroundStyle.Render("○ ")
							case albumInfo.InLidarr:
								indicator = m.styles.MutedStyle.Render("○ ")
							default:
								indicator = m.styles.MutedStyle.Render("⊝ ")
							}
							line += indicator
						} else {
							line += "  "
						}
					} else {
						line += " "
					}
					b.WriteString(m.styles.ForegroundStyle.Render(line + dl))
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

	if m.hasPendingUpdate {
		content += "\n" + strings.Repeat("\n", 2) + m.styles.MutedStyle.Render(" Track changed — press u to update")
	}

	viewWidth := m.viewport.Width()
	if content != "" && viewWidth > 0 {
		content = ansi.Wordwrap(content, viewWidth, "")
	}

	m.viewport.SetContent(content)
	m.viewport.SoftWrap = true
}
