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

func (m *Model) renderLogoArt(img image.Image) {
	termimg.ClearResizeCache()

	const targetHeight = 16
	var width, height int
	if m.imageProtocol == termimg.Halfblocks {
		targetWidth := int(float64(targetHeight) * m.cellRatio)
		if targetWidth < 10 {
			targetWidth = 10
		}
		width = targetWidth * 2
		height = targetHeight * 2
	} else {
		height = targetHeight
		width = int(float64(height) * m.cellRatio)
		if width < 10 {
			width = 10
		}
	}

	tiImg := termimg.New(img).Size(width, height).
		Scale(termimg.ScaleFit).Protocol(m.imageProtocol).UseUnicode(false)

	rendered, err := tiImg.Render()
	if err != nil {
		return
	}

	m.logoArtStr = rendered
	m.logoArtLoaded = true

	if m.imageProtocol == termimg.Halfblocks {
		m.logoArtWidth = width / 2
		m.logoArtHeight = height / 2
	} else {
		m.logoArtWidth = width
		m.logoArtHeight = height
	}
}

func (m Model) handleProgressTick(msg progressTickMsg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{tickProgressCmd()}

	if !m.mpvBackend.IsRunning() && !m.mpvBackend.IsPaused() {
		if m.playing && m.currentIndex >= 0 {
			return m.handlePlaybackEnded()
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
					m.updatePlaylist()
					cmds = append(cmds, m.trackChangedCmds())
				}
			}
		} else {
			if mpvPos >= 0 && mpvPos < len(m.playlist) {
				m.currentIndex = mpvPos
				m.updatePlaylist()
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

	if m.playing && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
		percent := 0.0
		if m.playlist[m.currentIndex].Duration > 0 {
			percent = m.playbackPos.TimePos / m.playlist[m.currentIndex].Duration
		}
		cmds = append(cmds, m.nowPlaying.UpdateProgress(percent))
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handlePlaybackEnded() (tea.Model, tea.Cmd) {
	m.playing = false
	m.paused = false

	switch m.repeatMode {
	case "one":
		if m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
			return m, m.playTrack(m.currentIndex)
		}
	case "all":
		if len(m.playlist) > 0 {
			next := m.currentIndex + 1
			if next >= len(m.playlist) {
				next = 0
			}
			return m, m.playTrack(next)
		}
	default:
		if m.currentIndex < len(m.playlist)-1 {
			return m, m.playTrack(m.currentIndex + 1)
		}
	}

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
	m.updatePlaylist()

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
	var width, height int
	if m.imageProtocol == termimg.Halfblocks {
		targetWidth := int(float64(targetHeight) * m.cellRatio)
		if targetWidth < 10 {
			targetWidth = 10
		}
		width = targetWidth * 2
		height = targetHeight * 2
	} else {
		height = targetHeight
		width = int(float64(height) * m.cellRatio)
		if width < 10 {
			width = 10
		}
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
	hasLogoArt := !hasAlbumArt && m.logoArtLoaded && m.logoArtStr != "" && m.cfg.ShowAlbumArt && m.cfg.Layout != "compact"

	if !hasAlbumArt && !hasLogoArt {
		return nil
	}

	var raw string

	if m.imageProtocol == termimg.Kitty || m.imageProtocol == termimg.Halfblocks ||
		m.imageProtocol == termimg.Sixel || m.imageProtocol == termimg.ITerm2 {

		if hasAlbumArt {
			artCol := m.width - m.albumArtWidth - 2
			raw += fmt.Sprintf("\x1b[s\x1b[%d;%dH%s\x1b[u", 3, artCol, m.albumArtStr)
		} else if hasLogoArt {
			artCol := m.width - m.logoArtWidth - 2
			raw += fmt.Sprintf("\x1b[s\x1b[%d;%dH%s\x1b[u", 3, artCol, m.logoArtStr)
		}

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
