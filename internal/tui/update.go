package tui

import (
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/pdfrg/must/internal/config"
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

	case searchResultsMsg:
		m.searchResults = msg.results
		m.searchCursor = 0
		m.searchScrollOffset = 0
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

	if m.themeWatcher != nil {
		return m, watchThemeCmd(m.themeWatcher)
	}
	return m, nil
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.searching {
		return m.handleSearchKey(msg)
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

	case key.Matches(msg, m.keyMap.VolumeUp):
		return m.volumeUp()

	case key.Matches(msg, m.keyMap.VolumeDown):
		return m.volumeDown()

	case key.Matches(msg, m.keyMap.Mute):
		return m.toggleMute()

	case key.Matches(msg, m.keyMap.Repeat):
		return m.cycleRepeat()

	case key.Matches(msg, m.keyMap.Shuffle):
		return m.toggleShuffle()

	case key.Matches(msg, m.keyMap.CursorDown):
		return m.moveCursorDown()

	case key.Matches(msg, m.keyMap.CursorUp):
		return m.moveCursorUp()

	case key.Matches(msg, m.keyMap.Enter):
		return m.handleEnter()

	case key.Matches(msg, m.keyMap.FocusLeft):
		return m.focusLeft()

	case key.Matches(msg, m.keyMap.FocusRight):
		return m.focusRight()

	case key.Matches(msg, m.keyMap.CycleView):
		return m.cycleView()

	case key.Matches(msg, m.keyMap.Search):
		return m.openSearch()

	case key.Matches(msg, m.keyMap.Help):
		m.viewMode = ViewHelp
		return m, nil

	case key.Matches(msg, m.keyMap.Escape):
		if m.viewMode == ViewHelp || m.viewMode == ViewLyrics || m.viewMode == ViewSyncedLyrics || m.viewMode == ViewArtistBio {
			m.viewMode = ViewLibrary
		} else if m.searching {
			return m.closeSearch()
		}
		return m, nil

	case key.Matches(msg, m.keyMap.DeleteTrack):
		return m.deleteCurrentTrack()

	case key.Matches(msg, m.keyMap.ClearPlaylist):
		return m.clearPlaylist()

	case key.Matches(msg, m.keyMap.Rescan):
		return m.rescanLibrary()

	case key.Matches(msg, m.keyMap.Lyrics):
		m.viewMode = ViewLyrics
		return m, nil

	case key.Matches(msg, m.keyMap.SyncedLyrics):
		m.viewMode = ViewSyncedLyrics
		return m, nil

	case key.Matches(msg, m.keyMap.ArtistBio):
		return m.openArtistBio()
	}

	return m, nil
}
