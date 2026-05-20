package tui

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	termimg "github.com/blacktop/go-termimg"
	"github.com/pdfrg/must/internal/api"
	"github.com/pdfrg/must/internal/config"
	imgpkg "github.com/pdfrg/must/internal/image"
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
	if mpvErr == nil && mpvPos >= 0 && !m.restoringPlayback {
		playlistIdx := m.mpvIndexToPlaylistIndex(mpvPos)
		if playlistIdx != m.currentIndex && playlistIdx >= 0 && playlistIdx < len(m.playlist) {
			logf("MPV position changed: mpv=%d playlist=%d", mpvPos, playlistIdx)
			if m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
				track := m.playlist[m.currentIndex]
				m.prevTrack = &track
				m.prevSongStartTime = m.songStartTime
				m.prevScrobbleEligible = m.scrobbleEligible
			}
			m.currentIndex = playlistIdx
			m.updatePlaylist()
			cmds = append(cmds, m.trackChangedCmds())
			// Re-read position after trackChangedCmds zeroed it
			if pos, err := m.mpvBackend.GetPlaybackPosition(); err == nil {
				m.playbackPos = pos
			}
		}
	}

	if len(m.scrobbleStates) > 0 {
		flashDuration := 5 * time.Second
		if time.Since(m.scrobbleFlashAt) >= flashDuration {
			m.scrobbleStates = nil
		} else {
			for svc, state := range m.scrobbleStates {
				if state == 2 || state == 3 { // FlashBlinkOn or FlashBlinkOff
					if int(time.Since(m.scrobbleFlashAt).Seconds())%2 == 0 {
						m.scrobbleStates[svc] = 2 // FlashBlinkOn
					} else {
						m.scrobbleStates[svc] = 3 // FlashBlinkOff
					}
				}
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

	if m.sleepTimerActive {
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
	switch m.repeatMode {
	case "one":
		if m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
			paths := m.buildMPVPlaylistPaths()
			mpvIdx := m.playlistIndexToMPVIndex(m.currentIndex)
			return m, tea.Batch(
				startPlaybackCmd(m.mpvBackend, paths, mpvIdx),
				m.trackChangedCmds(),
			)
		}
	case "all":
		if len(m.playlist) > 0 {
			paths := m.buildMPVPlaylistPaths()
			if m.shuffle {
				m.shuffleOrder = shuffleIndices(len(m.playlist))
			}
			return m, tea.Batch(
				startPlaybackCmd(m.mpvBackend, paths, 0),
				m.trackChangedCmds(),
			)
		}
	}

	m.playing = false
	m.paused = false
	return m, nil
}

func (m Model) skipNext() (tea.Model, tea.Cmd) {
	if len(m.playlist) == 0 {
		return m, nil
	}

	if m.shuffle && len(m.shuffleOrder) > 0 {
		mpvPos, err := m.mpvBackend.GetPlaylistPosition()
		if err == nil && mpvPos >= 0 && mpvPos < len(m.shuffleOrder)-1 {
			nextMPVIdx := mpvPos + 1
			nextPlaylistIdx := m.shuffleOrder[nextMPVIdx]
			m.currentIndex = nextPlaylistIdx
			m.updatePlaylist()
			_ = m.mpvBackend.PlaylistPlayIndex(nextMPVIdx)
			return m, m.trackChangedCmds()
		}
		if m.repeatMode == "all" {
			m.shuffleOrder = shuffleIndices(len(m.playlist))
			m.updatePlaylist()
			paths := m.buildMPVPlaylistPaths()
			return m, tea.Batch(setStatus(&m, "Shuffle: restart", false), startPlaybackCmd(m.mpvBackend, paths, 0), m.trackChangedCmds())
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

	if m.mpvBackend.IsRunning() {
		_ = m.mpvBackend.SkipNext()
	}

	m.currentIndex = next
	m.updatePlaylist()
	return m, m.trackChangedCmds()
}

func (m Model) skipPrev() (tea.Model, tea.Cmd) {
	if len(m.playlist) == 0 {
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

	if m.mpvBackend.IsRunning() {
		_ = m.mpvBackend.SkipPrev()
	}

	m.currentIndex = prev
	m.updatePlaylist()
	return m, m.trackChangedCmds()
}

func (m Model) seekForward() (tea.Model, tea.Cmd) {
	if !m.mpvBackend.IsRunning() {
		return m, nil
	}
	if m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
		dur := m.playlist[m.currentIndex].Duration
		if dur > 0 {
			remaining := dur - m.playbackPos.TimePos - 0.5
			delta := min(10.0, remaining)
			if delta > 0 {
				_ = m.mpvBackend.SeekRelative(delta)
			}
			return m, nil
		}
	}
	_ = m.mpvBackend.SeekRelative(10)
	return m, nil
}

func (m Model) seekBackward() (tea.Model, tea.Cmd) {
	if !m.mpvBackend.IsRunning() {
		return m, nil
	}
	delta := -10.0
	if m.playbackPos.TimePos+delta < 0 {
		delta = -m.playbackPos.TimePos
	}
	_ = m.mpvBackend.SeekRelative(delta)
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

func (m Model) restartSong() (tea.Model, tea.Cmd) {
	if !m.mpvBackend.IsRunning() {
		return m, nil
	}
	_ = m.mpvBackend.SeekAbsolute(0)
	return m, setStatus(&m, "Restarted", false)
}

func (m Model) toggleShuffle() (tea.Model, tea.Cmd) {
	m.shuffle = !m.shuffle
	if m.shuffle {
		m.shuffleOrder = shuffleIndices(len(m.playlist))
		if m.playing && m.mpvBackend.IsRunning() && m.currentIndex >= 0 {
			currentMPVIdx, _ := m.mpvBackend.GetPlaylistPosition()
			if currentMPVIdx >= 0 {
				newOrder := make([]int, 0, len(m.playlist))
				newOrder = append(newOrder, m.currentIndex)
				for _, idx := range m.shuffleOrder {
					if idx != m.currentIndex {
						newOrder = append(newOrder, idx)
					}
				}
				m.shuffleOrder = newOrder
			}
		}
	} else {
		m.shuffleOrder = nil
	}

	shuffleStr := "off"
	if m.shuffle {
		shuffleStr = "on"
	}
	return m, setStatus(&m, "Shuffle "+shuffleStr, false)
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

	if m.playing && m.mpvBackend.IsRunning() {
		mpvIdx := m.playlistIndexToMPVIndex(index)
		if mpvIdx >= 0 {
			_ = m.mpvBackend.PlaylistPlayIndex(mpvIdx)
		}
		return m.trackChangedCmds()
	}

	paths := m.buildMPVPlaylistPaths()
	var playIdx int
	if m.shuffle && len(m.shuffleOrder) > 0 {
		for i, idx := range m.shuffleOrder {
			if idx == index {
				playIdx = i
				break
			}
		}
	} else {
		playIdx = index
	}

	return tea.Batch(
		startPlaybackCmd(m.mpvBackend, paths, playIdx),
		m.trackChangedCmds(),
	)
}

func (m *Model) buildMPVPlaylistPaths() []string {
	if m.shuffle && len(m.shuffleOrder) > 0 {
		paths := make([]string, len(m.playlist))
		for i, idx := range m.shuffleOrder {
			paths[i] = m.playlist[idx].Path
		}
		return paths
	}
	paths := make([]string, len(m.playlist))
	for i, t := range m.playlist {
		paths[i] = t.Path
	}
	return paths
}

func (m *Model) playlistIndexToMPVIndex(playlistIdx int) int {
	if m.shuffle && len(m.shuffleOrder) > 0 {
		for i, idx := range m.shuffleOrder {
			if idx == playlistIdx {
				return i
			}
		}
		return -1
	}
	return playlistIdx
}

func (m *Model) mpvIndexToPlaylistIndex(mpvIdx int) int {
	if m.shuffle && len(m.shuffleOrder) > 0 {
		if mpvIdx >= 0 && mpvIdx < len(m.shuffleOrder) {
			return m.shuffleOrder[mpvIdx]
		}
		return -1
	}
	return mpvIdx
}

func startPlaybackCmd(backend *mpv.MPVBackend, paths []string, startIndex int) tea.Cmd {
	return func() tea.Msg {
		if err := backend.Start(paths); err != nil {
			return statusClearMsg{}
		}
		_ = backend.PlaylistPlayIndex(startIndex)
		return trackChangedMsg{}
	}
}

func (m *Model) trackChangedCmds() tea.Cmd {
	var cmds []tea.Cmd

	if m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
		t := m.playlist[m.currentIndex]
		logf("Track changed: %s - %s (%s)", t.Artist, t.Title, t.Album)
	}

	if m.prevScrobbleEligible && m.prevTrack != nil {
		cmds = append(cmds, scrobbleTrackCmd(m.cfg, m.subsonicClient, *m.prevTrack, m.prevSongStartTime))
	}
	m.prevTrack = nil
	m.prevScrobbleEligible = false
	m.prevSongStartTime = time.Time{}

	m.playing = true
	m.paused = false
	m.playbackPos = mpv.PlaybackPosition{}
	cmds = append(cmds, m.nowPlaying.UpdateProgress(0))
	m.audioInfo = nil

	m.albumArtStr = ""
	m.albumArtLoaded = false
	m.notifSentForSong = false

	if m.vis != nil && m.bottomViewMode == BottomVisualizer {
		m.vis.SetSeed(uint64(m.currentIndex))
	}
	m.visInfoVisible = true
	m.visInfoShownAt = time.Now()

	if m.bottomViewMode != BottomArtistBio {
		m.artistArtStr = ""
		m.artistArtLoaded = false
		m.artistArtEventID = 0
	}

	viewingContent := m.bottomViewMode == BottomLyrics || m.bottomViewMode == BottomArtistBio

	if !viewingContent {
		m.lyrics = ""
		m.syncedLyrics = nil
		m.lyricsLoading = true
		m.viewport.GotoTop()
		m.updateBottomView()
	} else {
		m.hasPendingUpdate = true
		m.syncedLyrics = nil
		m.updateBottomView()
	}

	m.songStartTime = time.Now()
	m.scrobbleEligible = false

	if m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
		t := m.playlist[m.currentIndex]
		if t.Source == models.SourceSubsonic {
			m.audioInfo = &models.AudioInfo{
				Codec:   t.ContentType,
				Bitrate: float64(t.Bitrate),
			}
		}
	}
	if m.audioInfo == nil {
		cmds = append(cmds, fetchAudioInfoCmd(m.mpvBackend))
	}

	if m.imageRenderer != nil && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
		t := m.playlist[m.currentIndex]
		if t.Source == models.SourceSubsonic {
			cmds = append(cmds, loadSubsonicAlbumArtCmd(m.imageRenderer, m.subsonicClient, t))
		} else {
			cmds = append(cmds, loadAlbumArtCmd(m.imageRenderer, t.Path))
		}
	}

	if m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
		track := m.playlist[m.currentIndex]
		cmds = append(cmds, sendNowPlayingCmd(m.cfg, track))
		cmds = append(cmds, fetchLyricsCmd(track))

		m.artistInfoEventID++
		artist := track.Artist
		album := track.Album
		cmds = append(cmds, fetchArtistInfoCmd(m.cfg, artist, album, m.artistInfoEventID, m.artistCache))
	}

	cmds = append(cmds, tickProgressCmd())
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
		if m.cfg.NotificationsEnabled && !m.notifSentForSong && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) && m.playlist[m.currentIndex].Path == msg.trackPath {
			return m, sendNotificationCmd(m.cfg, m.playlist[m.currentIndex], false)
		}
		if msg.trackPath != "" && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) && m.playlist[m.currentIndex].Path == msg.trackPath {
			return m, fetchOnlineArtCmd(m.cfg, m.playlist[m.currentIndex])
		}
		return m, nil
	}

	img, _, err := image.Decode(bytes.NewReader(msg.imageData))
	if err != nil {
		if m.cfg.NotificationsEnabled && !m.notifSentForSong && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) && m.playlist[m.currentIndex].Path == msg.trackPath {
			return m, sendNotificationCmd(m.cfg, m.playlist[m.currentIndex], false)
		}
		return m, nil
	}

	if m.cfg.CopyAlbumArt && m.cfg.AlbumArtPath != "" {
		_ = os.WriteFile(m.cfg.AlbumArtPath, msg.imageData, 0644)
	}

	if m.cfg.NotificationsEnabled && m.cfg.NotificationsShowArt && msg.trackPath != "" {
		api.SaveNotifyArt(msg.imageData)
	}

	if msg.trackPath != "" && m.imageRenderer != nil {
		_ = imgpkg.CacheArtData(msg.trackPath, msg.imageData)
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

	var cmd tea.Cmd
	if m.cfg.NotificationsEnabled && !m.notifSentForSong && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) && m.playlist[m.currentIndex].Path == msg.trackPath {
		cmd = sendNotificationCmd(m.cfg, m.playlist[m.currentIndex], true)
	}

	return m, tea.Batch(renderAlbumArtAfterDelay(), cmd)
}

func (m Model) renderImagesCmd() tea.Cmd {
	if m.activeModal != ModalNone {
		return nil
	}

	if m.layoutPromptActive {
		return nil
	}

	if m.visFullscreen && m.bottomViewMode == BottomVisualizer {
		return nil
	}

	layout := m.layoutMode()
	compactLayout := layout == "compact"
	narrowLayout := layout == "narrow"

	hasAlbumArt := m.cfg.ShowAlbumArt && m.albumArtLoaded && m.albumArtStr != "" && !compactLayout
	hasLogoArt := !hasAlbumArt && m.logoArtLoaded && m.logoArtStr != "" && m.cfg.ShowAlbumArt && !compactLayout
	hasArtistArt := m.artistArtLoaded && m.artistArtStr != "" && m.bottomViewMode == BottomArtistBio

	if !hasAlbumArt && !hasLogoArt && !hasArtistArt {
		return nil
	}

	var raw string

	if m.imageProtocol == termimg.Kitty || m.imageProtocol == termimg.Halfblocks ||
		m.imageProtocol == termimg.Sixel || m.imageProtocol == termimg.ITerm2 {

		if hasAlbumArt {
			artCol := m.width - m.albumArtWidth - 2
			if narrowLayout {
				artCol = 2
			}
			raw += fmt.Sprintf("\x1b[s\x1b[%d;%dH%s\x1b[u", 3, artCol, m.albumArtStr)
		} else if hasLogoArt {
			artCol := m.width - m.logoArtWidth - 2
			if narrowLayout {
				artCol = 2
			}
			raw += fmt.Sprintf("\x1b[s\x1b[%d;%dH%s\x1b[u", 3, artCol, m.logoArtStr)
		}

		if hasArtistArt {
			availableSpace := m.height - 20 - 3
			if availableSpace >= m.artistArtHeight {
				const artistRow = 20
				if m.imageProtocol == termimg.Kitty {
					raw += fmt.Sprintf("\x1b[s\x1b[%d;%dH%s\x1b[u", artistRow, 2, m.artistArtStr)
				} else {
					lines := strings.Split(m.artistArtStr, "\n")
					raw += "\x1b[s"
					for i, line := range lines {
						if line != "" {
							raw += fmt.Sprintf("\x1b[%d;%dH%s", artistRow+i, 2, line)
						}
					}
					raw += "\x1b[u"
				}
			}
		}

		return tea.Raw(raw)
	}

	return nil
}

func (m Model) handleRenderAlbumArt(msg renderAlbumArtMsg) (tea.Model, tea.Cmd) {
	return m, m.renderImagesCmd()
}

func (m Model) handleArtistImageLoaded(msg artistImageLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.eventID != m.artistInfoEventID {
		return m, nil
	}
	if msg.err != nil || msg.imageData == nil {
		return m, nil
	}

	img, _, err := image.Decode(bytes.NewReader(msg.imageData))
	if err != nil {
		return m, nil
	}

	termimg.ClearResizeCache()

	const displayWidth = 30
	imgBounds := img.Bounds()
	imgW := float64(imgBounds.Dx())
	imgH := float64(imgBounds.Dy())

	displayHeight := int(float64(displayWidth) * (imgH / imgW) / m.cellRatio)
	if displayHeight < 4 {
		displayHeight = 4
	}
	if displayHeight > 20 {
		displayHeight = 20
	}

	var renderWidth, renderHeight int
	if m.imageProtocol == termimg.Halfblocks {
		renderWidth = displayWidth * 2
		renderHeight = displayHeight * 2
	} else {
		renderWidth = displayWidth
		renderHeight = displayHeight
	}

	tiImg := termimg.New(img).Size(renderWidth, renderHeight).
		Scale(termimg.ScaleFit).Protocol(m.imageProtocol).ZIndex(1).UseUnicode(false)

	rendered, err := tiImg.Render()
	if err != nil {
		return m, nil
	}

	m.artistArtStr = rendered
	m.artistArtLoaded = true
	m.artistArtEventID = msg.eventID
	m.artistArtWidth = displayWidth
	m.artistArtHeight = displayHeight

	if m.bottomViewMode == BottomArtistBio {
		m.viewport.GotoTop()
		m.updateBottomView()
	}

	return m, renderArtistArtAfterDelay()
}

func (m Model) renderArtistArtCmd() tea.Cmd {
	return m.renderImagesCmd()
}

func sendNowPlayingCmd(cfg *config.Config, track models.Track) tea.Cmd {
	return func() tea.Msg {
		apiKey := cfg.LastFM.APIKey
		if apiKey == "" {
			apiKey = api.LastFMAPIKey
		}
		secret := cfg.LastFM.SharedSecret
		if secret == "" {
			secret = api.LastFMSharedSecret
		}
		if cfg.LastFM.Enabled && cfg.LastFM.SessionKey != "" && apiKey != "" {
			lfmTrack := api.LastFMTrack{
				Artist:   track.Artist,
				Title:    track.Title,
				Album:    track.Album,
				Duration: int(track.Duration),
			}
			if err := api.UpdateNowPlayingLastFM(apiKey, secret, cfg.LastFM.SessionKey, lfmTrack); err != nil {
				logf("Now-playing to last.fm failed: %v", err)
			}
		}

		if cfg.ListenBrainz.Enabled && cfg.ListenBrainz.Token != "" {
			lbTrack := api.ListenBrainzTrack{
				Artist: track.Artist,
				Title:  track.Title,
				Album:  track.Album,
			}
			if err := api.SubmitPlayingNowListenBrainz(cfg.ListenBrainz.Token, lbTrack); err != nil {
				logf("Now-playing to listenbrainz failed: %v", err)
			}
		}

		return nil
	}
}

func scrobbleTrackCmd(cfg *config.Config, subsonicClient *api.SubsonicClient, track models.Track, startTime time.Time) tea.Cmd {
	return func() tea.Msg {
		var results []scrobbleResult

		apiKey := cfg.LastFM.APIKey
		if apiKey == "" {
			apiKey = api.LastFMAPIKey
		}
		secret := cfg.LastFM.SharedSecret
		if secret == "" {
			secret = api.LastFMSharedSecret
		}
		if cfg.LastFM.Enabled && cfg.LastFM.SessionKey != "" && apiKey != "" {
			lfmTrack := api.LastFMTrack{
				Artist:   track.Artist,
				Title:    track.Title,
				Album:    track.Album,
				Duration: int(track.Duration),
			}
			err := api.ScrobbleLastFM(apiKey, secret, cfg.LastFM.SessionKey, lfmTrack, startTime.Unix())
			results = append(results, scrobbleResult{Service: "fm", Success: err == nil})
			if err != nil {
				logf("Scrobble to last.fm failed: %v", err)
			} else {
				logf("Scrobbled to last.fm: %s - %s", track.Artist, track.Title)
			}
		}

		if cfg.ListenBrainz.Enabled && cfg.ListenBrainz.Token != "" {
			lbTrack := api.ListenBrainzTrack{
				Artist: track.Artist,
				Title:  track.Title,
				Album:  track.Album,
			}
			err := api.SubmitListenBrainz(cfg.ListenBrainz.Token, lbTrack, startTime.Unix())
			results = append(results, scrobbleResult{Service: "lb", Success: err == nil})
			if err != nil {
				logf("Scrobble to listenbrainz failed: %v", err)
			} else {
				logf("Scrobbled to listenbrainz: %s - %s", track.Artist, track.Title)
			}
		}

		if subsonicClient != nil && track.Source == models.SourceSubsonic && track.RemoteID != "" {
			err := subsonicClient.Scrobble(track.RemoteID, true, startTime.Unix())
			if err != nil {
				logf("Scrobble to subsonic failed: %v", err)
			} else {
				logf("Scrobbled to subsonic: %s - %s", track.Artist, track.Title)
			}
		}

		if len(results) > 0 {
			return scrobbleResultMsg{results: results}
		}
		return nil
	}
}
