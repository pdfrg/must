package tui

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	termimg "github.com/blacktop/go-termimg"
	"github.com/pdfrg/must/internal/api"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/models"
	"github.com/pdfrg/must/internal/mpv"
)

func (m Model) handleProgressTick(msg progressTickMsg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{tickProgressCmd()}

	if !m.mpvBackend.IsRunning() && !m.mpvBackend.IsPaused() {
		if m.playing && m.currentIndex >= 0 {
			return m, tea.Batch(append(cmds, m.handlePlaybackEnded())...)
		}
		return m, tea.Batch(cmds...)
	}

	pos, err := m.mpvBackend.GetPlaybackPosition()
	if err == nil {
		m.playbackPos = pos
	}

	m.paused = m.mpvBackend.QueryPauseState()

	mpvPos, mpvErr := m.mpvBackend.GetPlaylistPosition()
	if mpvErr == nil && mpvPos >= 0 && mpvPos != m.currentIndex {
		if m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
			track := m.playlist[m.currentIndex]
			m.prevTrack = &track
			m.prevSongStartTime = m.songStartTime
			m.prevScrobbleEligible = m.scrobbleEligible
		}
		if m.shuffle && len(m.shuffleOrder) > 0 {
			if mpvPos >= 0 && mpvPos < len(m.shuffleOrder) {
				absIdx := m.shuffleOrder[mpvPos]
				if absIdx != m.currentIndex && absIdx >= 0 && absIdx < len(m.playlist) {
					m.currentIndex = absIdx
					cmds = append(cmds, m.trackChangedCmds())
				}
			}
		} else {
			if mpvPos >= 0 && mpvPos < len(m.playlist) {
				m.currentIndex = mpvPos
				cmds = append(cmds, m.trackChangedCmds())
			}
		}
	}

	if !m.scrobbleEligible && m.playing && !m.paused && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
		elapsed := time.Since(m.songStartTime)
		durSecs := m.playlist[m.currentIndex].Duration
		threshold := time.Duration(min(durSecs/2, 240) * float64(time.Second))
		if elapsed >= threshold && durSecs > 30 {
			m.scrobbleEligible = true
		}
	}

	if m.sleepTimer > 0 && m.sleepRemaining > 0 {
		cmds = append(cmds, tickSleepTimerCmd())
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handlePlaybackEnded() tea.Cmd {
	m.playing = false
	m.paused = false

	switch m.repeatMode {
	case "one":
		if m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
			return m.playTrack(m.currentIndex)
		}
	case "all":
		if len(m.playlist) > 0 {
			next := m.currentIndex + 1
			if next >= len(m.playlist) {
				next = 0
			}
			return m.playTrack(next)
		}
	default:
		if m.currentIndex < len(m.playlist)-1 {
			return m.playTrack(m.currentIndex + 1)
		}
	}

	return nil
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

func (m Model) skipNext() (tea.Model, tea.Cmd) {
	if len(m.playlist) == 0 {
		return m, nil
	}

	if m.shuffle && len(m.shuffleOrder) > 0 {
		mpvPos, err := m.mpvBackend.GetPlaylistPosition()
		if err == nil && mpvPos >= 0 && mpvPos < len(m.shuffleOrder)-1 {
			next := m.shuffleOrder[mpvPos+1]
			return m, m.playTrack(next)
		}
		if m.repeatMode == "all" {
			m.shuffleOrder = shuffleIndices(len(m.playlist))
			paths := make([]string, len(m.playlist))
			for i, idx := range m.shuffleOrder {
				paths[i] = m.playlist[idx].Path
			}
			return m, tea.Batch(setStatus(&m, "Shuffle: restart", false), startPlaybackCmd(m.mpvBackend, paths, 0))
		}
		return m, setStatus(&m, "End of playlist", false)
	}

	next := m.currentIndex + 1
	if next >= len(m.playlist) {
		if m.repeatMode == "all" {
			next = 0
		} else {
			return m, setStatus(&m, "End of playlist", false)
		}
	}

	return m, m.playTrack(next)
}

func (m Model) skipPrev() (tea.Model, tea.Cmd) {
	if len(m.playlist) == 0 {
		return m, nil
	}

	if m.playbackPos.TimePos > 3 {
		_ = m.mpvBackend.SeekAbsolute(0)
		return m, nil
	}

	prev := m.currentIndex - 1
	if prev < 0 {
		if m.repeatMode == "all" {
			prev = len(m.playlist) - 1
		} else {
			prev = 0
		}
	}

	return m, m.playTrack(prev)
}

func (m Model) seekForward() (tea.Model, tea.Cmd) {
	if m.mpvBackend.IsRunning() {
		_ = m.mpvBackend.SeekRelative(5)
	}
	return m, nil
}

func (m Model) seekBackward() (tea.Model, tea.Cmd) {
	if m.mpvBackend.IsRunning() {
		_ = m.mpvBackend.SeekRelative(-5)
	}
	return m, nil
}

func (m Model) volumeUp() (tea.Model, tea.Cmd) {
	m.volume = min(m.volume+5, 100)
	if m.mpvBackend.IsRunning() {
		_ = m.mpvBackend.SetVolume(m.volume)
	}
	return m, nil
}

func (m Model) volumeDown() (tea.Model, tea.Cmd) {
	m.volume = max(m.volume-5, 0)
	if m.mpvBackend.IsRunning() {
		_ = m.mpvBackend.SetVolume(m.volume)
	}
	return m, nil
}

func (m Model) toggleMute() (tea.Model, tea.Cmd) {
	m.muted = !m.muted
	if m.mpvBackend.IsRunning() {
		_ = m.mpvBackend.SetMute(m.muted)
	}
	if m.muted {
		return m, setStatus(&m, "Muted", false)
	}
	return m, setStatus(&m, fmt.Sprintf("Volume: %d%%", int(m.volume)), false)
}

func (m Model) cycleRepeat() (tea.Model, tea.Cmd) {
	switch m.repeatMode {
	case "off":
		m.repeatMode = "all"
	case "all":
		m.repeatMode = "one"
	case "one":
		m.repeatMode = "off"
	}
	repeatStr := m.repeatMode
	if repeatStr == "off" {
		repeatStr = "no repeat"
	} else {
		repeatStr = "repeat " + repeatStr
	}
	return m, setStatus(&m, repeatStr, false)
}

func (m Model) toggleShuffle() (tea.Model, tea.Cmd) {
	m.shuffle = !m.shuffle
	if m.shuffle {
		m.shuffleOrder = shuffleIndices(len(m.playlist))
		return m, setStatus(&m, "Shuffle on", false)
	}
	m.shuffleOrder = nil
	return m, setStatus(&m, "Shuffle off", false)
}

func (m Model) playTrack(index int) tea.Cmd {
	if index < 0 || index >= len(m.playlist) {
		return nil
	}

	if m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
		track := m.playlist[m.currentIndex]
		m.prevTrack = &track
		m.prevSongStartTime = m.songStartTime
		m.prevScrobbleEligible = m.scrobbleEligible
	}

	m.currentIndex = index

	var paths []string
	var playIdx int

	if m.shuffle && len(m.shuffleOrder) > 0 {
		paths = make([]string, len(m.playlist))
		for i, idx := range m.shuffleOrder {
			paths[i] = m.playlist[idx].Path
		}
		for i, idx := range m.shuffleOrder {
			if idx == index {
				playIdx = i
				break
			}
		}
	} else {
		paths = make([]string, len(m.playlist)-index)
		for i := index; i < len(m.playlist); i++ {
			paths[i-index] = m.playlist[i].Path
		}
		playIdx = 0
	}

	return tea.Batch(
		startPlaybackCmd(m.mpvBackend, paths, playIdx),
		m.trackChangedCmds(),
	)
}

func startPlaybackCmd(backend *mpv.MPVBackend, paths []string, startIndex int) tea.Cmd {
	return func() tea.Msg {
		if startIndex > 0 {
			temp := make([]string, len(paths)-startIndex)
			copy(temp, paths[startIndex:])
			paths = temp
		}
		if err := backend.Start(paths); err != nil {
			return statusClearMsg{}
		}
		return trackChangedMsg{}
	}
}

func (m *Model) trackChangedCmds() tea.Cmd {
	var cmds []tea.Cmd

	if m.prevScrobbleEligible && m.prevTrack != nil {
		cmds = append(cmds, scrobbleTrackCmd(m.cfg, *m.prevTrack, m.prevSongStartTime))
	}
	m.prevTrack = nil
	m.prevScrobbleEligible = false
	m.prevSongStartTime = time.Time{}

	m.playing = true
	m.paused = false
	m.playbackPos = mpv.PlaybackPosition{}
	m.audioInfo = nil

	m.albumArtStr = ""
	m.albumArtLoaded = false

	m.lyrics = ""
	m.syncedLyrics = nil
	m.lyricsLoading = true

	m.songStartTime = time.Now()
	m.scrobbleEligible = false

	cmds = append(cmds, fetchAudioInfoCmd(m.mpvBackend))

	if m.imageRenderer != nil && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
		trackPath := m.playlist[m.currentIndex].Path
		cmds = append(cmds, loadAlbumArtCmd(m.imageRenderer, trackPath))
	}

	if m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
		track := m.playlist[m.currentIndex]
		cmds = append(cmds, sendNowPlayingCmd(m.cfg, track))
		cmds = append(cmds, fetchLyricsCmd(track))
	}

	return tea.Batch(cmds...)
}

func fetchAudioInfoCmd(backend *mpv.MPVBackend) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(500 * time.Millisecond)
		info, err := backend.GetAudioInfo()
		if err != nil {
			return nil
		}
		return audioInfoMsg{info: info}
	}
}

func (m Model) handleTrackChanged(msg trackChangedMsg) (tea.Model, tea.Cmd) {
	return m, m.trackChangedCmds()
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	if !m.libraryReady || m.libraryDB == nil {
		return m, setStatus(&m, "Library not ready", true)
	}

	switch m.viewMode {
	case ViewLibrary:
		switch m.focusPane {
		case FocusArtists:
			if len(m.artists) == 0 || m.artistCursor >= len(m.artists) {
				return m, nil
			}
			artist := m.artists[m.artistCursor]
			albums, err := m.libraryDB.GetAlbumsByArtist(artist)
			if err != nil {
				return m, setStatus(&m, fmt.Sprintf("Error loading albums: %v", err), true)
			}
			m.albums = albums
			m.albumCursor = 0
			m.albumScrollOffset = 0
			m.albumTracks = nil
			m.focusPane = FocusAlbums
			return m, nil

		case FocusAlbums:
			if len(m.albums) == 0 || m.albumCursor >= len(m.albums) {
				return m, nil
			}
			artist := m.artists[m.artistCursor]
			album := m.albums[m.albumCursor]
			tracks, err := m.libraryDB.GetTracksByArtistAndAlbum(artist, album)
			if err != nil {
				return m, setStatus(&m, fmt.Sprintf("Error loading tracks: %v", err), true)
			}
			m.albumTracks = tracks
			m.albumCursor = 0
			m.albumScrollOffset = 0
			m.focusPane = FocusTracks
			return m, nil

		case FocusTracks:
			if len(m.albumTracks) > 0 && m.albumCursor < len(m.albumTracks) {
				m.playlist = m.albumTracks
				return m, m.playTrack(m.albumCursor)
			}
		}

	case ViewPlaylist:
		if len(m.playlist) > 0 && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
			return m, m.playTrack(m.currentIndex)
		}
	}

	return m, nil
}

func (m Model) cycleView() (tea.Model, tea.Cmd) {
	switch m.viewMode {
	case ViewLibrary:
		m.viewMode = ViewPlaylist
	case ViewPlaylist:
		m.viewMode = ViewLyrics
	case ViewLyrics:
		if len(m.syncedLyrics) > 0 {
			m.viewMode = ViewSyncedLyrics
		} else {
			m.viewMode = ViewLibrary
		}
	case ViewSyncedLyrics:
		m.viewMode = ViewLibrary
	default:
		m.viewMode = ViewLibrary
	}
	return m, nil
}

func (m Model) openArtistBio() (tea.Model, tea.Cmd) {
	var artist string
	if m.playing && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
		artist = m.playlist[m.currentIndex].Artist
	} else if len(m.artists) > 0 && m.artistCursor < len(m.artists) {
		artist = m.artists[m.artistCursor]
	}
	if artist == "" {
		return m, setStatus(&m, "No artist selected", true)
	}

	m.viewMode = ViewArtistBio
	m.artistBio = ""
	m.artistBioTitle = artist
	m.artistBioURL = ""
	m.artistBioLoading = true

	return m, fetchArtistBioCmd(artist)
}

func (m Model) moveCursorDown() (tea.Model, tea.Cmd) {
	switch m.viewMode {
	case ViewLibrary:
		switch m.focusPane {
		case FocusArtists:
			if len(m.artists) > 0 && m.artistCursor < len(m.artists)-1 {
				m.artistCursor++
				maxVisible := m.height - 3
				if maxVisible < 1 {
					maxVisible = 1
				}
				if m.artistCursor >= m.artistScrollOffset+maxVisible {
					m.artistScrollOffset = m.artistCursor - maxVisible + 1
				}
				m.albums = nil
				m.albumTracks = nil
			}
		case FocusAlbums:
			if len(m.albumTracks) > 0 && m.albumCursor < len(m.albumTracks)-1 {
				m.albumCursor++
				maxVisible := m.height - 3
				if maxVisible < 1 {
					maxVisible = 1
				}
				if m.albumCursor >= m.albumScrollOffset+maxVisible {
					m.albumScrollOffset = m.albumCursor - maxVisible + 1
				}
			} else if len(m.albums) > 0 && m.albumCursor < len(m.albums)-1 {
				m.albumCursor++
				maxVisible := m.height - 3
				if maxVisible < 1 {
					maxVisible = 1
				}
				if m.albumCursor >= m.albumScrollOffset+maxVisible {
					m.albumScrollOffset = m.albumCursor - maxVisible + 1
				}
				m.albumTracks = nil
			}
		case FocusTracks:
			if len(m.albumTracks) > 0 && m.albumCursor < len(m.albumTracks)-1 {
				m.albumCursor++
				maxVisible := m.height - 3
				if maxVisible < 1 {
					maxVisible = 1
				}
				if m.albumCursor >= m.albumScrollOffset+maxVisible {
					m.albumScrollOffset = m.albumCursor - maxVisible + 1
				}
			}
		}
	case ViewPlaylist:
		if m.currentIndex < len(m.playlist)-1 {
			m.currentIndex++
		}
	}
	return m, nil
}

func (m Model) moveCursorUp() (tea.Model, tea.Cmd) {
	switch m.viewMode {
	case ViewLibrary:
		switch m.focusPane {
		case FocusArtists:
			if m.artistCursor > 0 {
				m.artistCursor--
				if m.artistCursor < m.artistScrollOffset {
					m.artistScrollOffset = m.artistCursor
				}
				m.albums = nil
				m.albumTracks = nil
			}
		case FocusAlbums:
			if len(m.albumTracks) > 0 && m.albumCursor > 0 {
				m.albumCursor--
				if m.albumCursor < m.albumScrollOffset {
					m.albumScrollOffset = m.albumCursor
				}
			} else if len(m.albums) > 0 && m.albumCursor > 0 {
				m.albumCursor--
				if m.albumCursor < m.albumScrollOffset {
					m.albumScrollOffset = m.albumCursor
				}
				m.albumTracks = nil
			}
		case FocusTracks:
			if len(m.albumTracks) > 0 && m.albumCursor > 0 {
				m.albumCursor--
				if m.albumCursor < m.albumScrollOffset {
					m.albumScrollOffset = m.albumCursor
				}
			}
		}
	case ViewPlaylist:
		if m.currentIndex > 0 {
			m.currentIndex--
		}
	}
	return m, nil
}

func (m Model) focusLeft() (tea.Model, tea.Cmd) {
	if m.viewMode == ViewLibrary {
		if m.focusPane > FocusArtists {
			m.focusPane--
		}
	}
	return m, nil
}

func (m Model) focusRight() (tea.Model, tea.Cmd) {
	if m.viewMode == ViewLibrary {
		switch m.focusPane {
		case FocusArtists:
			if len(m.albums) > 0 || len(m.albumTracks) > 0 {
				m.focusPane = FocusAlbums
			}
		case FocusAlbums:
			if len(m.albumTracks) > 0 {
				m.focusPane = FocusTracks
			}
		}
	}
	return m, nil
}

func (m Model) deleteCurrentTrack() (tea.Model, tea.Cmd) {
	if m.viewMode == ViewPlaylist && len(m.playlist) > 0 && m.currentIndex >= 0 {
		idx := m.currentIndex
		m.playlist = append(m.playlist[:idx], m.playlist[idx+1:]...)
		if m.currentIndex >= len(m.playlist) {
			m.currentIndex = len(m.playlist) - 1
		}
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
	return m, setStatus(&m, "Playlist cleared", false)
}

func (m Model) rescanLibrary() (tea.Model, tea.Cmd) {
	if m.libraryDB != nil {
		_ = m.libraryDB.Close()
		m.libraryDB = nil
	}
	m.libraryReady = false
	m.artists = nil
	m.albums = nil
	m.albumTracks = nil
	return m, scanLibraryCmd(m.cfg)
}

func (m Model) handleRandomPlay() (tea.Model, tea.Cmd) {
	if m.libraryDB == nil {
		return m, nil
	}

	artists, err := m.libraryDB.GetAllArtists()
	if err != nil || len(artists) == 0 {
		return m, setStatus(&m, "No artists in library", true)
	}
	m.artists = artists

	randArtistIdx := randInt(len(artists))
	artist := artists[randArtistIdx]
	m.artistCursor = randArtistIdx

	albums, err := m.libraryDB.GetAlbumsByArtist(artist)
	if err != nil || len(albums) == 0 {
		tracks, err := m.libraryDB.GetTracksByArtist(artist)
		if err != nil || len(tracks) == 0 {
			return m, setStatus(&m, "No tracks found for random artist", true)
		}
		m.playlist = tracks
	} else {
		randAlbumIdx := randInt(len(albums))
		album := albums[randAlbumIdx]
		m.albums = albums
		m.albumCursor = randAlbumIdx

		tracks, err := m.libraryDB.GetTracksByArtistAndAlbum(artist, album)
		if err != nil || len(tracks) == 0 {
			return m, setStatus(&m, "No tracks found for random album", true)
		}
		m.playlist = tracks
	}

	if m.shuffle {
		m.shuffleOrder = shuffleIndices(len(m.playlist))
	}

	return m, m.playTrack(0)
}

func (m Model) handleImageLoaded(msg imageLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil || msg.imageData == nil {
		return m, nil
	}

	img, _, err := image.Decode(bytes.NewReader(msg.imageData))
	if err != nil {
		return m, nil
	}

	if m.cfg.CopyAlbumArt && m.cfg.AlbumArtPath != "" {
		_ = os.WriteFile(m.cfg.AlbumArtPath, msg.imageData, 0644)
	}

	termimg.ClearResizeCache()

	const targetHeight = 16
	width := targetHeight
	height := targetHeight

	if m.imageProtocol == termimg.Halfblocks {
		width = targetHeight * 2
		height = targetHeight * 2
	}

	tiImg := termimg.New(img).Size(width, height).
		Scale(termimg.ScaleFit).Protocol(m.imageProtocol).UseUnicode(false)

	rendered, err := tiImg.Render()
	if err != nil {
		return m, nil
	}

	m.albumArtStr = rendered
	m.albumArtLoaded = true

	if m.imageProtocol == termimg.Halfblocks {
		m.albumArtWidth = width / 2
		m.albumArtHeight = height / 2
	} else {
		m.albumArtWidth = width
		m.albumArtHeight = height
	}

	return m, renderAlbumArtAfterDelay()
}

func (m Model) renderImagesCmd() tea.Cmd {
	hasAlbumArt := m.cfg.ShowAlbumArt && m.albumArtLoaded && m.albumArtStr != "" && m.cfg.Layout != "compact"

	if !hasAlbumArt {
		return nil
	}

	var raw string

	if m.imageProtocol == termimg.Kitty || m.imageProtocol == termimg.Halfblocks ||
		m.imageProtocol == termimg.Sixel || m.imageProtocol == termimg.ITerm2 {
		artCol := m.width - m.albumArtWidth - 2
		raw += fmt.Sprintf("\x1b[s\x1b[%d;%dH%s\x1b[u", 3, artCol, m.albumArtStr)
		return tea.Raw(raw)
	}

	return nil
}

func (m Model) handleRenderAlbumArt(msg renderAlbumArtMsg) (tea.Model, tea.Cmd) {
	return m, m.renderImagesCmd()
}

func sendNowPlayingCmd(cfg *config.Config, track models.Track) tea.Cmd {
	return func() tea.Msg {
		if cfg.LastFM.Enabled && cfg.LastFM.SessionKey != "" && cfg.LastFM.APIKey != "" {
			lfmTrack := api.LastFMTrack{
				Artist:   track.Artist,
				Title:    track.Title,
				Album:    track.Album,
				Duration: int(track.Duration),
			}
			_ = api.UpdateNowPlayingLastFM(cfg.LastFM.APIKey, cfg.LastFM.SharedSecret, cfg.LastFM.SessionKey, lfmTrack)
		}

		if cfg.ListenBrainz.Enabled && cfg.ListenBrainz.Token != "" {
			lbTrack := api.ListenBrainzTrack{
				Artist: track.Artist,
				Title:  track.Title,
				Album:  track.Album,
			}
			_ = api.SubmitPlayingNowListenBrainz(cfg.ListenBrainz.Token, lbTrack)
		}

		return nil
	}
}

func scrobbleTrackCmd(cfg *config.Config, track models.Track, startTime time.Time) tea.Cmd {
	return func() tea.Msg {
		if cfg.LastFM.Enabled && cfg.LastFM.SessionKey != "" && cfg.LastFM.APIKey != "" {
			lfmTrack := api.LastFMTrack{
				Artist:   track.Artist,
				Title:    track.Title,
				Album:    track.Album,
				Duration: int(track.Duration),
			}
			_ = api.ScrobbleLastFM(cfg.LastFM.APIKey, cfg.LastFM.SharedSecret, cfg.LastFM.SessionKey, lfmTrack, startTime.Unix())
		}

		if cfg.ListenBrainz.Enabled && cfg.ListenBrainz.Token != "" {
			lbTrack := api.ListenBrainzTrack{
				Artist: track.Artist,
				Title:  track.Title,
				Album:  track.Album,
			}
			_ = api.SubmitListenBrainz(cfg.ListenBrainz.Token, lbTrack, startTime.Unix())
		}

		return nil
	}
}
