package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/pdfrg/must/internal/api"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/ctl"
	"github.com/pdfrg/must/internal/db"
	"github.com/pdfrg/must/internal/models"
	"github.com/pdfrg/must/internal/mpv"
	"github.com/pdfrg/must/internal/playlist"
)

func (m Model) handleCtlCommand(cmd string, args []string) (Model, ctl.CtlResult, tea.Cmd) {
	switch cmd {
	case "play":
		return m.ctlPlay(args)
	case "playshuffle":
		m.shuffle = true
		return m.ctlPlay(args)
	case "random":
		return m.ctlPlayRandom(args)
	case "enqueue":
		return m.ctlEnqueue(args, false)
	case "enqueue-next":
		return m.ctlEnqueue(args, true)
	case "pause":
		return m.ctlPause()
	case "clear":
		return m.ctlClear()
	case "next":
		return m.ctlNext()
	case "previous":
		return m.ctlPrevious()
	case "stop":
		return m.ctlStop()
	case "shuffle":
		return m.ctlShuffle()
	case "repeat":
		return m.ctlRepeat(args)
	case "replaygain":
		return m.ctlReplayGain(args)
	case "go":
		return m.ctlGo(args)
	case "status":
		return m, m.ctlStatus(), nil
	case "current":
		return m, m.ctlCurrent(), nil
	case "list":
		return m, m.ctlList(), nil
	case "remove":
		return m.ctlRemove(args)
	case "move":
		return m.ctlMove(args)
	case "save":
		return m, m.ctlSave(args), nil
	case "rescan":
		return m.ctlRescan()
	case "find":
		results, isErr := m.ctlFind(args)
		if isErr != nil {
			return m, *isErr, nil
		}
		m.lastFindResults = results
		return m, m.ctlFindFormat(results), nil
	case "library":
		return m, m.ctlLibrary(), nil
	case "playlists":
		results, result := m.ctlPlaylists()
		m.lastFindResults = results
		return m, result, nil
	default:
		return m, ctl.CtlResult{OK: false, Error: fmt.Sprintf("unknown command: %s", cmd)}, nil
	}
}

func (m Model) ctlPlay(args []string) (Model, ctl.CtlResult, tea.Cmd) {
	if len(args) == 0 {
		if len(m.playlist) == 0 {
			return m, ctl.CtlResult{OK: false, Error: "playlist is empty"}, nil
		}
		if !m.mpvBackend.IsRunning() {
			if m.currentIndex < 0 {
				m.currentIndex = 0
			}
			m.updatePlaylist()
			paths := m.buildMPVPlaylistPaths()
			playIdx := m.playlistIndexToMPVIndex(m.currentIndex)
			return m, ctl.CtlResult{OK: true, Data: "Playing"}, tea.Batch(
				startPlaybackCmd(m.mpvBackend, paths, playIdx),
				m.trackChangedCmds(),
			)
		}
		_ = m.mpvBackend.Pause(false)
		m.paused = false
		m.playing = true
		return m, ctl.CtlResult{OK: true, Data: "Resumed"}, nil
	}

	tracks, label, err := m.resolveTracks(args)
	if err != nil {
		return m, ctl.CtlResult{OK: false, Error: err.Error()}, nil
	}
	if len(tracks) == 0 {
		return m, ctl.CtlResult{OK: false, Error: "no tracks found"}, nil
	}

	m.playlist = tracks
	m.currentIndex = 0
	m.shuffleOrder = nil
	if m.shuffle {
		m.shuffleOrder = shuffleIndices(len(m.playlist))
	}
	m.updatePlaylist()

	paths := m.buildMPVPlaylistPaths()
	playIdx := m.playlistIndexToMPVIndex(0)
	var resultText string
	if len(tracks) == 1 {
		resultText = fmt.Sprintf("Playing %s", label)
	} else {
		resultText = fmt.Sprintf("Playing %s (%d tracks)", label, len(tracks))
	}
	result := ctl.CtlResult{OK: true, Data: resultText}
	return m, result, tea.Batch(
		startPlaybackCmd(m.mpvBackend, paths, playIdx),
		m.trackChangedCmds(),
	)
}

func (m Model) ctlPlayRandom(args []string) (Model, ctl.CtlResult, tea.Cmd) {
	source := ""
	if len(args) > 0 {
		source = args[0]
	}
	resultText := "Playing random album"
	if source != "" {
		resultText = fmt.Sprintf("Playing random %s album", source)
	}
	return m, ctl.CtlResult{OK: true, Data: resultText}, m.randomAlbumCmd(source)
}

func (m Model) ctlEnqueue(args []string, insertNext bool) (Model, ctl.CtlResult, tea.Cmd) {
	if len(args) == 0 {
		return m, ctl.CtlResult{OK: false, Error: "enqueue requires an argument"}, nil
	}

	tracks, label, err := m.resolveTracks(args)
	if err != nil {
		return m, ctl.CtlResult{OK: false, Error: err.Error()}, nil
	}
	if len(tracks) == 0 {
		return m, ctl.CtlResult{OK: false, Error: "no tracks found"}, nil
	}

	if insertNext && m.currentIndex >= 0 {
		insertPos := m.currentIndex + 1
		m.playlist = append(m.playlist, make([]models.Track, len(tracks))...)
		copy(m.playlist[insertPos+len(tracks):], m.playlist[insertPos:])
		copy(m.playlist[insertPos:], tracks)
	} else {
		m.playlist = append(m.playlist, tracks...)
	}

	if m.shuffle && len(m.shuffleOrder) > 0 {
		oldLen := len(m.shuffleOrder)
		for i := 0; i < len(tracks); i++ {
			m.shuffleOrder = append(m.shuffleOrder, oldLen+i)
		}
	}

	m.updatePlaylist()

	if m.playing && m.mpvBackend.IsRunning() {
		var trackPaths []string
		for _, t := range tracks {
			trackPaths = append(trackPaths, t.Path)
		}
		if insertNext && m.currentIndex >= 0 {
			mpvPos := m.playlistIndexToMPVIndex(m.currentIndex)
			if mpvPos >= 0 {
				_ = m.mpvBackend.InsertInPlaylist(trackPaths, mpvPos)
			}
		} else {
			_ = m.mpvBackend.AppendToPlaylist(trackPaths)
		}
	}

	dir := "Enqueued"
	if insertNext {
		dir = "Enqueued next"
	}
	var resultText string
	if len(tracks) == 1 {
		resultText = fmt.Sprintf("%s %s", dir, label)
	} else {
		resultText = fmt.Sprintf("%s %s (%d tracks)", dir, label, len(tracks))
	}
	return m, ctl.CtlResult{OK: true, Data: resultText}, nil
}

func (m Model) ctlPause() (Model, ctl.CtlResult, tea.Cmd) {
	if m.mpvBackend.IsRunning() {
		target := !m.paused
		_ = m.mpvBackend.Pause(target)
		m.paused = target
	}
	if m.paused {
		return m, ctl.CtlResult{OK: true, Data: "Paused"}, nil
	}
	return m, ctl.CtlResult{OK: true, Data: "Playing"}, nil
}

func (m Model) ctlClear() (Model, ctl.CtlResult, tea.Cmd) {
	if m.playing && m.mpvBackend.IsRunning() {
		currentMPVIdx, err := m.mpvBackend.GetPlaylistPosition()
		if err == nil && currentMPVIdx >= 0 {
			count, _ := m.mpvBackend.GetPlaylistCount()
			for i := count - 1; i > currentMPVIdx; i-- {
				_ = m.mpvBackend.RemoveFromPlaylist(i)
			}
			for i := currentMPVIdx - 1; i >= 0; i-- {
				_ = m.mpvBackend.RemoveFromPlaylist(i)
			}
		}
	}

	m.playlist = nil
	m.currentIndex = -1
	m.shuffleOrder = nil
	m.updatePlaylist()
	return m, ctl.CtlResult{OK: true, Data: "Playlist cleared"}, nil
}

func (m Model) ctlNext() (Model, ctl.CtlResult, tea.Cmd) {
	newModel, cmd := m.skipNext()
	m2 := newModel.(Model)
	data := "Skipped"
	if m2.currentIndex >= 0 && m2.currentIndex < len(m2.playlist) {
		t := m2.playlist[m2.currentIndex]
		data = fmt.Sprintf("Now playing: %s - %s", t.Artist, t.Title)
	}
	return m2, ctl.CtlResult{OK: true, Data: data}, cmd
}

func (m Model) ctlPrevious() (Model, ctl.CtlResult, tea.Cmd) {
	newModel, cmd := m.skipPrev()
	m2 := newModel.(Model)
	data := "Previous"
	if m2.currentIndex >= 0 && m2.currentIndex < len(m2.playlist) {
		t := m2.playlist[m2.currentIndex]
		data = fmt.Sprintf("Now playing: %s - %s", t.Artist, t.Title)
	}
	return m2, ctl.CtlResult{OK: true, Data: data}, cmd
}

func (m Model) ctlStop() (Model, ctl.CtlResult, tea.Cmd) {
	if m.mpvBackend.IsRunning() {
		_ = m.mpvBackend.Stop()
	}
	m.playing = false
	m.paused = false
	m.playbackPos = mpv.PlaybackPosition{}
	return m, ctl.CtlResult{OK: true, Data: "Stopped"}, nil
}

func (m Model) ctlShuffle() (Model, ctl.CtlResult, tea.Cmd) {
	newModel, cmd := m.toggleShuffle()
	m2 := newModel.(Model)
	state := "off"
	if m2.shuffle {
		state = "on"
	}
	return m2, ctl.CtlResult{OK: true, Data: "Shuffle " + state}, cmd
}

func (m Model) ctlRepeat(args []string) (Model, ctl.CtlResult, tea.Cmd) {
	if len(args) == 0 {
		return m, ctl.CtlResult{OK: true, Data: "Repeat: " + m.repeatMode}, nil
	}
	mode := strings.ToLower(args[0])
	switch mode {
	case "off", "all", "one":
		m.repeatMode = mode
		return m, ctl.CtlResult{OK: true, Data: "Repeat: " + mode}, nil
	default:
		return m, ctl.CtlResult{OK: false, Error: "repeat requires off, all, or one"}, nil
	}
}

func (m Model) ctlReplayGain(args []string) (Model, ctl.CtlResult, tea.Cmd) {
	if len(args) == 0 {
		return m, ctl.CtlResult{OK: true, Data: "ReplayGain: " + m.cfg.ReplayGainMode}, nil
	}
	mode := strings.ToLower(args[0])
	switch mode {
	case "off", "track", "album":
		m.cfg.ReplayGainMode = mode
		if m.mpvBackend != nil {
			_ = m.mpvBackend.SetReplayGainMode(mode)
		}
		if err := m.cfg.Save(); err != nil {
			logf("Failed to save config: %v", err)
		}
		return m, ctl.CtlResult{OK: true, Data: "ReplayGain: " + mode}, setStatus(&m, "ReplayGain: "+mode, false)
	default:
		return m, ctl.CtlResult{OK: false, Error: "replaygain requires off, track, or album"}, nil
	}
}

func (m Model) ctlGo(args []string) (Model, ctl.CtlResult, tea.Cmd) {
	if len(args) == 0 {
		if m.currentIndex < 0 {
			return m, ctl.CtlResult{OK: true, Data: "No track selected"}, nil
		}
		return m, ctl.CtlResult{OK: true, Data: fmt.Sprintf("Position: %d/%d", m.currentIndex+1, len(m.playlist))}, nil
	}

	pos, err := strconv.Atoi(args[0])
	if err != nil || pos < 1 || pos > len(m.playlist) {
		return m, ctl.CtlResult{OK: false, Error: fmt.Sprintf("invalid position: %s (must be 1-%d)", args[0], len(m.playlist))}, nil
	}

	idx := pos - 1
	m.currentIndex = idx
	m.updatePlaylist()

	if m.playing && m.mpvBackend.IsRunning() {
		mpvIdx := m.playlistIndexToMPVIndex(idx)
		if mpvIdx >= 0 {
			_ = m.mpvBackend.PlaylistPlayIndex(mpvIdx)
		}
		t := m.playlist[idx]
		return m, ctl.CtlResult{OK: true, Data: fmt.Sprintf("Now playing: %s - %s", t.Artist, t.Title)}, m.trackChangedCmds()
	}

	paths := m.buildMPVPlaylistPaths()
	playIdx := m.playlistIndexToMPVIndex(idx)
	t := m.playlist[idx]
	return m, ctl.CtlResult{OK: true, Data: fmt.Sprintf("Now playing: %s - %s", t.Artist, t.Title)}, tea.Batch(
		startPlaybackCmd(m.mpvBackend, paths, playIdx),
		m.trackChangedCmds(),
	)
}

func (m Model) ctlStatus() ctl.CtlResult {
	var b strings.Builder

	if m.playing && m.currentIndex >= 0 && m.currentIndex < len(m.playlist) {
		t := m.playlist[m.currentIndex]
		fmt.Fprintf(&b, "Now Playing: %s\n", t.Title)
		fmt.Fprintf(&b, "Artist: %s\n", t.Artist)
		if t.Album != "" {
			albumLine := t.Album
			if t.Year > 0 {
				albumLine = fmt.Sprintf("%s (%d)", albumLine, t.Year)
			}
			fmt.Fprintf(&b, "Album: %s\n", albumLine)
		}
		fmt.Fprintf(&b, "Progress: %s / %s\n",
			formatDuration(int(m.playbackPos.TimePos)),
			t.GetDurationFormatted())
	} else {
		b.WriteString("Not playing\n")
	}

	if m.paused {
		b.WriteString("State: Paused\n")
	} else if m.playing {
		b.WriteString("State: Playing\n")
	} else {
		b.WriteString("State: Stopped\n")
	}

	fmt.Fprintf(&b, "Playlist: %d/%d\n", m.currentIndex+1, len(m.playlist))
	fmt.Fprintf(&b, "Repeat: %s | Shuffle: ", m.repeatMode)
	if m.shuffle {
		b.WriteString("on\n")
	} else {
		b.WriteString("off\n")
	}

	if m.audioInfo != nil {
		var parts []string
		parts = append(parts, fmt.Sprintf("Codec: %s", m.audioInfo.Codec))
		parts = append(parts, fmt.Sprintf("Bitrate: %.0fkbps", m.audioInfo.Bitrate))
		if m.audioInfo.SampleRate > 0 {
			parts = append(parts, fmt.Sprintf("%dHz", m.audioInfo.SampleRate))
		}
		if m.audioInfo.Channels > 0 {
			parts = append(parts, fmt.Sprintf("%dch", m.audioInfo.Channels))
		}
		if m.audioInfo.BitDepth > 0 {
			parts = append(parts, fmt.Sprintf("%dbit", m.audioInfo.BitDepth))
		}
		fmt.Fprintln(&b, strings.Join(parts, " | "))
	}

	return ctl.CtlResult{OK: true, Data: strings.TrimRight(b.String(), "\n")}
}

func (m Model) ctlCurrent() ctl.CtlResult {
	if !m.playing || m.currentIndex < 0 || m.currentIndex >= len(m.playlist) {
		return ctl.CtlResult{OK: false, Error: "not playing"}
	}
	t := m.playlist[m.currentIndex]
	return ctl.CtlResult{OK: true, Data: fmt.Sprintf("%s - %s", t.Artist, t.Title)}
}

func (m Model) ctlList() ctl.CtlResult {
	if len(m.playlist) == 0 {
		return ctl.CtlResult{OK: true, Data: "Playlist is empty"}
	}

	var b strings.Builder
	width := len(strconv.Itoa(len(m.playlist)))
	for i, t := range m.playlist {
		marker := " "
		if i == m.currentIndex {
			marker = "▶"
		}
		fmt.Fprintf(&b, "%s %*d. %s - %s", marker, width, i+1, t.Artist, t.Title)
		if t.Album != "" {
			fmt.Fprintf(&b, " - %s", t.Album)
		}
		b.WriteString("\n")
	}
	return ctl.CtlResult{OK: true, Data: strings.TrimRight(b.String(), "\n")}
}

func (m Model) ctlRemove(args []string) (Model, ctl.CtlResult, tea.Cmd) {
	if len(args) == 0 {
		return m, ctl.CtlResult{OK: false, Error: "remove requires a position"}, nil
	}

	pos, err := strconv.Atoi(args[0])
	if err != nil || pos < 1 || pos > len(m.playlist) {
		return m, ctl.CtlResult{OK: false, Error: fmt.Sprintf("invalid position: %s (must be 1-%d)", args[0], len(m.playlist))}, nil
	}

	idx := pos - 1
	isCurrent := idx == m.currentIndex
	mpvIdx := m.playlistIndexToMPVIndex(idx)

	m.playlist = append(m.playlist[:idx], m.playlist[idx+1:]...)

	if m.shuffle && len(m.shuffleOrder) > 0 {
		var newOrder []int
		for _, si := range m.shuffleOrder {
			if si < idx {
				newOrder = append(newOrder, si)
			} else if si > idx {
				newOrder = append(newOrder, si-1)
			}
		}
		m.shuffleOrder = newOrder
	}

	if len(m.playlist) == 0 {
		m.playing = false
		m.paused = false
		m.currentIndex = -1
		m.updatePlaylist()
		if m.mpvBackend.IsRunning() {
			_ = m.mpvBackend.Stop()
		}
		return m, ctl.CtlResult{OK: true, Data: "Removed last track. Playlist empty."}, nil
	}

	if m.currentIndex >= len(m.playlist) {
		m.currentIndex = len(m.playlist) - 1
	} else if m.currentIndex > idx {
		m.currentIndex--
	}

	m.updatePlaylist()

	var cmd tea.Cmd
	if m.playing && m.mpvBackend.IsRunning() && mpvIdx >= 0 {
		_ = m.mpvBackend.RemoveFromPlaylist(mpvIdx)

		if isCurrent {
			newMPVIdx := m.playlistIndexToMPVIndex(m.currentIndex)
			if newMPVIdx >= 0 {
				_ = m.mpvBackend.PlaylistPlayIndex(newMPVIdx)
			}
			cmd = m.trackChangedCmds()
		}
	}

	return m, ctl.CtlResult{OK: true, Data: fmt.Sprintf("Removed track %d", pos)}, cmd
}

func (m Model) ctlMove(args []string) (Model, ctl.CtlResult, tea.Cmd) {
	if len(args) != 2 {
		return m, ctl.CtlResult{OK: false, Error: "move requires <from> <to> positions"}, nil
	}

	from, err1 := strconv.Atoi(args[0])
	to, err2 := strconv.Atoi(args[1])
	if err1 != nil || err2 != nil || from < 1 || to < 1 || from > len(m.playlist) || to > len(m.playlist) {
		return m, ctl.CtlResult{OK: false, Error: fmt.Sprintf("invalid positions (must be 1-%d)", len(m.playlist))}, nil
	}

	fromIdx := from - 1
	toIdx := to - 1

	mpvFrom := m.playlistIndexToMPVIndex(fromIdx)
	mpvTo := m.playlistIndexToMPVIndex(toIdx)

	track := m.playlist[fromIdx]
	m.playlist = append(m.playlist[:fromIdx], m.playlist[fromIdx+1:]...)
	m.playlist = append(m.playlist[:toIdx], append([]models.Track{track}, m.playlist[toIdx:]...)...)

	if m.currentIndex == fromIdx {
		m.currentIndex = toIdx
	} else if fromIdx < m.currentIndex && toIdx >= m.currentIndex {
		m.currentIndex--
	} else if fromIdx > m.currentIndex && toIdx <= m.currentIndex {
		m.currentIndex++
	}

	if m.shuffle && len(m.shuffleOrder) > 0 {
		m.shuffleOrder = nil
	}

	m.updatePlaylist()

	if m.playing && m.mpvBackend.IsRunning() && mpvFrom >= 0 && mpvTo >= 0 {
		_ = m.mpvBackend.PlaylistMove(mpvFrom, mpvTo)
	}

	return m, ctl.CtlResult{OK: true, Data: fmt.Sprintf("Moved track %d to position %d", from, to)}, nil
}

func (m Model) ctlSave(args []string) ctl.CtlResult {
	if len(args) == 0 {
		return ctl.CtlResult{OK: false, Error: "save requires a playlist name"}
	}
	if len(m.playlist) == 0 {
		return ctl.CtlResult{OK: false, Error: "playlist is empty"}
	}

	name := args[0]
	var trackPaths []string
	for _, t := range m.playlist {
		trackPaths = append(trackPaths, t.Path)
	}

	path := config.GetPlaylistSavePath(name)
	rel := m.cfg.PlaylistPathMode == "relative"
	if err := playlist.Save(path, trackPaths, &playlist.SaveOptions{RelativePaths: rel}); err != nil {
		return ctl.CtlResult{OK: false, Error: fmt.Sprintf("failed to save playlist: %v", err)}
	}

	return ctl.CtlResult{OK: true, Data: fmt.Sprintf("Saved playlist '%s' (%d tracks)", name, len(m.playlist))}
}

func (m Model) ctlRescan() (Model, ctl.CtlResult, tea.Cmd) {
	return m, ctl.CtlResult{OK: true, Data: "Rescan started"}, scanLibraryCmd(m.cfg)
}

func (m Model) ctlFind(args []string) ([]ctl.SearchResult, *ctl.CtlResult) {
	if len(args) == 0 {
		return nil, &ctl.CtlResult{OK: false, Error: "find requires a query"}
	}
	if m.libraryDB == nil {
		return nil, &ctl.CtlResult{OK: false, Error: "library not loaded yet"}
	}

	query := normalizeSubsonicPrefix(m.subsonicClient, strings.Join(args, " "))
	field, fieldVal := parseQueryPrefix(query)

	var results []ctl.SearchResult

	switch field {
	case "artist":
		artists, err := m.libraryDB.SearchArtistsLike(fieldVal)
		if err != nil {
			return nil, &ctl.CtlResult{OK: false, Error: fmt.Sprintf("search failed: %v", err)}
		}
		for _, a := range artists {
			count, _ := trackCountByArtist(m.libraryDB, a)
			results = append(results, ctl.SearchResult{
				Type: ctl.ResultArtist, ArtistName: a,
				Display: fmt.Sprintf("Artist: %s (%d tracks)", a, count), TrackCount: count,
			})
		}
		for _, a := range artists {
			albumNames, _ := m.libraryDB.GetAlbumsByArtist(a)
			for _, albumName := range albumNames {
				tracks, _ := m.libraryDB.GetTracksByArtistAndAlbum(a, albumName)
				year := 0
				if len(tracks) > 0 {
					year = tracks[0].Year
				}
				results = append(results, ctl.SearchResult{
					Type: ctl.ResultAlbum, AlbumName: albumName, ArtistName: a,
					TrackCount: len(tracks), Year: year,
					Display: fmt.Sprintf("Album: %s - %s (%d tracks)", a, albumName, len(tracks)),
				})
			}
		}
		tracks, _ := m.libraryDB.SearchFTS(query)
		for _, t := range tracks {
			results = append(results, ctl.SearchResult{
				Type: ctl.ResultTrack, TrackID: t.ID, Title: t.Title,
				ArtistName: t.Artist, AlbumName: t.Album, Year: t.Year,
				Display: fmt.Sprintf("Track: \"%s\" - %s - %s", t.Title, t.Artist, t.Album),
			})
		}

	case "album":
		albums, err := m.libraryDB.SearchAlbumsLike(fieldVal)
		if err != nil {
			return nil, &ctl.CtlResult{OK: false, Error: fmt.Sprintf("search failed: %v", err)}
		}
		for _, a := range albums {
			tracks, _ := m.libraryDB.GetTracksByArtistAndAlbum(a.Artist, a.Album)
			year := 0
			if len(tracks) > 0 {
				year = tracks[0].Year
			}
			yearStr := ""
			if year > 0 {
				yearStr = fmt.Sprintf(" (%d)", year)
			}
			results = append(results, ctl.SearchResult{
				Type: ctl.ResultAlbum, AlbumName: a.Album, ArtistName: a.Artist,
				TrackCount: len(tracks), Year: year,
				Display: fmt.Sprintf("Album: %s - %s%s (%d tracks)", a.Artist, a.Album, yearStr, len(tracks)),
			})
		}
		tracks, _ := m.libraryDB.SearchFTS(query)
		for _, t := range tracks {
			results = append(results, ctl.SearchResult{
				Type: ctl.ResultTrack, TrackID: t.ID, Title: t.Title,
				ArtistName: t.Artist, AlbumName: t.Album, Year: t.Year,
				Display: fmt.Sprintf("Track: \"%s\" - %s - %s", t.Title, t.Artist, t.Album),
			})
		}

	case "genre":
		genres, err := m.libraryDB.GetGenres()
		if err == nil {
			for _, g := range genres {
				if strings.Contains(strings.ToLower(g), strings.ToLower(fieldVal)) {
					count := trackCountByGenre(m.libraryDB, g)
					results = append(results, ctl.SearchResult{
						Type: ctl.ResultGenre, GenreName: g, TrackCount: count,
						Display: fmt.Sprintf("Genre: %s (%d tracks)", g, count),
					})
				}
			}
		}

	case "subsonic":
		if m.subsonicClient == nil {
			return nil, &ctl.CtlResult{OK: false, Error: "subsonic not configured"}
		}
		badge := m.subsonicClient.ServerBadge()

		subField, subVal := parseQueryPrefix(fieldVal)
		switch subField {
		case "song", "track":
			subField = "title"
		}

		switch subField {
		case "title":
			result, err := m.subsonicClient.Search3(subVal, 0, 0, 50)
			if err != nil {
				return nil, &ctl.CtlResult{OK: false, Error: fmt.Sprintf("subsonic search failed: %v", err)}
			}
			for _, s := range result.Song {
				tracks := m.subsonicClient.ChildrenToTracks([]api.Child{s})
				if len(tracks) > 0 {
					t := tracks[0]
					results = append(results, ctl.SearchResult{
						Type:          ctl.ResultSubsonicTrack,
						SubsonicTrack: &ctl.TrackRef{Track: t},
						Title:         t.Title,
						ArtistName:    t.Artist,
						AlbumName:     t.Album,
						Year:          t.Year,
						Display:       fmt.Sprintf("[%s] Track: \"%s\" - %s - %s", badge, t.Title, t.Artist, t.Album),
					})
				}
			}

		case "genre":
			songs, err := m.subsonicClient.GetSongsByGenre(subVal, 50)
			if err != nil {
				return nil, &ctl.CtlResult{OK: false, Error: fmt.Sprintf("subsonic genre search failed: %v", err)}
			}
			genreAlbums, _ := m.subsonicClient.GetAlbumList2("byGenre", 0, 0, 20, subVal)
			results = append(results, ctl.SearchResult{
				Type: ctl.ResultGenre, GenreName: subVal, TrackCount: len(songs),
				Display: fmt.Sprintf("[%s] Genre: %s (%d songs, %d albums)", badge, subVal, len(songs), len(genreAlbums)),
			})
			for _, a := range genreAlbums {
				results = append(results, ctl.SearchResult{
					Type: ctl.ResultAlbum, SubsonicAlbumID: a.ID,
					AlbumName: a.Name, ArtistName: a.Artist,
					TrackCount: a.SongCount,
					Display:    fmt.Sprintf("[%s] Album: %s - %s (%d tracks)", badge, a.Artist, a.Name, a.SongCount),
				})
			}
			for _, s := range songs {
				tracks := m.subsonicClient.ChildrenToTracks([]api.Child{s})
				if len(tracks) > 0 {
					t := tracks[0]
					results = append(results, ctl.SearchResult{
						Type:          ctl.ResultSubsonicTrack,
						SubsonicTrack: &ctl.TrackRef{Track: t},
						Title:         t.Title,
						ArtistName:    t.Artist,
						AlbumName:     t.Album,
						Year:          t.Year,
						Display:       fmt.Sprintf("[%s] Track: \"%s\" - %s - %s", badge, t.Title, t.Artist, t.Album),
					})
				}
			}

		case "artist":
			result, err := m.subsonicClient.Search3(subVal, 3, 30, 100)
			if err != nil {
				return nil, &ctl.CtlResult{OK: false, Error: fmt.Sprintf("subsonic search failed: %v", err)}
			}
			for _, a := range result.Artist {
				results = append(results, ctl.SearchResult{
					Type: ctl.ResultArtist, SubsonicArtistID: a.ID, ArtistName: a.Name,
					Display: fmt.Sprintf("[%s] Artist: %s (%d albums)", badge, a.Name, a.AlbumCount),
				})
			}
			for _, a := range result.Album {
				results = append(results, ctl.SearchResult{
					Type: ctl.ResultAlbum, SubsonicAlbumID: a.ID,
					AlbumName: a.Name, ArtistName: a.Artist,
					TrackCount: a.SongCount,
					Display:    fmt.Sprintf("[%s] Album: %s - %s (%d tracks)", badge, a.Artist, a.Name, a.SongCount),
				})
			}
			for _, s := range result.Song {
				tracks := m.subsonicClient.ChildrenToTracks([]api.Child{s})
				if len(tracks) > 0 {
					t := tracks[0]
					results = append(results, ctl.SearchResult{
						Type:          ctl.ResultSubsonicTrack,
						SubsonicTrack: &ctl.TrackRef{Track: t},
						Title:         t.Title,
						ArtistName:    t.Artist,
						AlbumName:     t.Album,
						Year:          t.Year,
						Display:       fmt.Sprintf("[%s] Track: \"%s\" - %s - %s", badge, t.Title, t.Artist, t.Album),
					})
				}
			}

		case "album":
			result, err := m.subsonicClient.Search3(subVal, 0, 3, 50)
			if err != nil {
				return nil, &ctl.CtlResult{OK: false, Error: fmt.Sprintf("subsonic search failed: %v", err)}
			}
			for _, a := range result.Album {
				results = append(results, ctl.SearchResult{
					Type: ctl.ResultAlbum, SubsonicAlbumID: a.ID,
					AlbumName: a.Name, ArtistName: a.Artist,
					TrackCount: a.SongCount,
					Display:    fmt.Sprintf("[%s] Album: %s - %s (%d tracks)", badge, a.Artist, a.Name, a.SongCount),
				})
			}
			for _, s := range result.Song {
				tracks := m.subsonicClient.ChildrenToTracks([]api.Child{s})
				if len(tracks) > 0 {
					t := tracks[0]
					results = append(results, ctl.SearchResult{
						Type:          ctl.ResultSubsonicTrack,
						SubsonicTrack: &ctl.TrackRef{Track: t},
						Title:         t.Title,
						ArtistName:    t.Artist,
						AlbumName:     t.Album,
						Year:          t.Year,
						Display:       fmt.Sprintf("[%s] Track: \"%s\" - %s - %s", badge, t.Title, t.Artist, t.Album),
					})
				}
			}

		case "year":
			yearMin, yearMax := parseYearRange(subVal)
			if yearMin == 0 {
				return nil, &ctl.CtlResult{OK: false, Error: fmt.Sprintf("invalid year: %s", subVal)}
			}
			albums, err := m.subsonicClient.GetAlbumList2("byYear", yearMin, yearMax, 50, "")
			if err != nil {
				return nil, &ctl.CtlResult{OK: false, Error: fmt.Sprintf("subsonic year search failed: %v", err)}
			}
			results = append(results, ctl.SearchResult{
				Type: ctl.ResultYear, Year: yearMin, YearEnd: yearMax,
				TrackCount: len(albums),
				Display:    fmt.Sprintf("[%s] Year: %s (%d albums)", badge, subVal, len(albums)),
			})
			for _, a := range albums {
				album, err := m.subsonicClient.GetAlbum(a.ID)
				if err != nil {
					continue
				}
				for _, s := range album.Song {
					tracks := m.subsonicClient.ChildrenToTracks([]api.Child{s})
					if len(tracks) > 0 {
						t := tracks[0]
						results = append(results, ctl.SearchResult{
							Type:          ctl.ResultSubsonicTrack,
							SubsonicTrack: &ctl.TrackRef{Track: t},
							Title:         t.Title,
							ArtistName:    t.Artist,
							AlbumName:     t.Album,
							Year:          t.Year,
							Display:       fmt.Sprintf("[%s] Track: \"%s\" - %s - %s", badge, t.Title, t.Artist, t.Album),
						})
					}
				}
			}

		default:
			result, err := m.subsonicClient.Search3(fieldVal, 5, 10, 50)
			if err != nil {
				return nil, &ctl.CtlResult{OK: false, Error: fmt.Sprintf("subsonic search failed: %v", err)}
			}
			for _, a := range result.Artist {
				results = append(results, ctl.SearchResult{
					Type: ctl.ResultArtist, SubsonicArtistID: a.ID, ArtistName: a.Name,
					Display: fmt.Sprintf("[%s] Artist: %s (%d albums)", badge, a.Name, a.AlbumCount),
				})
			}
			for _, a := range result.Album {
				results = append(results, ctl.SearchResult{
					Type: ctl.ResultAlbum, SubsonicAlbumID: a.ID,
					AlbumName: a.Name, ArtistName: a.Artist,
					TrackCount: a.SongCount,
					Display:    fmt.Sprintf("[%s] Album: %s - %s (%d tracks)", badge, a.Artist, a.Name, a.SongCount),
				})
			}
			for _, s := range result.Song {
				tracks := m.subsonicClient.ChildrenToTracks([]api.Child{s})
				if len(tracks) > 0 {
					t := tracks[0]
					results = append(results, ctl.SearchResult{
						Type:          ctl.ResultSubsonicTrack,
						SubsonicTrack: &ctl.TrackRef{Track: t},
						Title:         t.Title,
						ArtistName:    t.Artist,
						AlbumName:     t.Album,
						Year:          t.Year,
						Display:       fmt.Sprintf("[%s] Track: \"%s\" - %s - %s", badge, t.Title, t.Artist, t.Album),
					})
				}
			}
		}

	case "year":
		yearMin, yearMax := parseYearRange(fieldVal)
		tracks, err := m.libraryDB.SearchWithYearRange("", yearMin, yearMax)
		if err == nil {
			results = append(results, ctl.SearchResult{
				Type: ctl.ResultYear, Year: yearMin, YearEnd: yearMax,
				TrackCount: len(tracks),
				Display:    fmt.Sprintf("Year: %s (%d tracks)", fieldVal, len(tracks)),
			})
			for _, t := range tracks {
				results = append(results, ctl.SearchResult{
					Type: ctl.ResultTrack, TrackID: t.ID, Title: t.Title,
					ArtistName: t.Artist, AlbumName: t.Album, Year: t.Year,
					Display: fmt.Sprintf("Track: \"%s\" - %s - %s", t.Title, t.Artist, t.Album),
				})
			}
		}

	default:
		seenAlbum := make(map[string]bool)

		artists, _ := m.libraryDB.SearchArtistsLike(query)
		for _, a := range artists {
			count, _ := trackCountByArtist(m.libraryDB, a)
			results = append(results, ctl.SearchResult{
				Type: ctl.ResultArtist, ArtistName: a, TrackCount: count,
				Display: fmt.Sprintf("Artist: %s (%d tracks)", a, count),
			})
			if len(results) >= 10 {
				break
			}
		}

		// Albums by matching artists
		for _, a := range artists {
			albumNames, _ := m.libraryDB.GetAlbumsByArtist(a)
			for _, albumName := range albumNames {
				key := a + "|" + albumName
				if seenAlbum[key] {
					continue
				}
				seenAlbum[key] = true
				tracks, _ := m.libraryDB.GetTracksByArtistAndAlbum(a, albumName)
				year := 0
				if len(tracks) > 0 {
					year = tracks[0].Year
				}
				results = append(results, ctl.SearchResult{
					Type: ctl.ResultAlbum, AlbumName: albumName, ArtistName: a,
					TrackCount: len(tracks), Year: year,
					Display: fmt.Sprintf("Album: %s - %s (%d tracks)", a, albumName, len(tracks)),
				})
				if len(results) >= 30 {
					break
				}
			}
			if len(results) >= 30 {
				break
			}
		}

		// Albums whose name directly matches the query
		albums, _ := m.libraryDB.SearchAlbumsLike(query)
		for _, a := range albums {
			key := a.Artist + "|" + a.Album
			if seenAlbum[key] {
				continue
			}
			seenAlbum[key] = true
			tracks, _ := m.libraryDB.GetTracksByArtistAndAlbum(a.Artist, a.Album)
			year := 0
			if len(tracks) > 0 {
				year = tracks[0].Year
			}
			results = append(results, ctl.SearchResult{
				Type: ctl.ResultAlbum, AlbumName: a.Album, ArtistName: a.Artist,
				TrackCount: len(tracks), Year: year,
				Display: fmt.Sprintf("Album: %s - %s (%d tracks)", a.Artist, a.Album, len(tracks)),
			})
			if len(results) >= 30 {
				break
			}
		}

		tracks, _ := m.libraryDB.SearchFTS(query)
		for _, t := range tracks {
			results = append(results, ctl.SearchResult{
				Type: ctl.ResultTrack, TrackID: t.ID, Title: t.Title,
				ArtistName: t.Artist, AlbumName: t.Album, Year: t.Year,
				Display: fmt.Sprintf("Track: \"%s\" - %s - %s (%d)",
					t.Title, t.Artist, t.Album, t.Year),
			})
		}
	}

	if len(results) == 0 {
		likeTracks, err := m.libraryDB.SearchLike(query)
		if err == nil && len(likeTracks) > 0 {
			for _, t := range likeTracks {
				results = append(results, ctl.SearchResult{
					Type: ctl.ResultTrack, TrackID: t.ID, Title: t.Title,
					ArtistName: t.Artist, AlbumName: t.Album, Year: t.Year,
					Display: fmt.Sprintf("Track: \"%s\" - %s - %s", t.Title, t.Artist, t.Album),
				})
			}
		}
	}

	if len(results) == 0 {
		return nil, &ctl.CtlResult{OK: false, Error: "no results found"}
	}

	return results, nil
}

func (m Model) ctlFindFormat(results []ctl.SearchResult) ctl.CtlResult {
	var b strings.Builder
	width := len(strconv.Itoa(len(results)))
	for i, r := range results {
		fmt.Fprintf(&b, "%*d. %s\n", width, i+1, r.Display)
		r.Index = i + 1
		results[i] = r
	}
	return ctl.CtlResult{OK: true, Data: strings.TrimRight(b.String(), "\n")}
}

func (m Model) ctlLibrary() ctl.CtlResult {
	var b strings.Builder
	for i, dir := range m.cfg.MusicDirs {
		fmt.Fprintf(&b, "Music directory %d: %s\n", i+1, dir)
	}

	if m.libraryDB != nil {
		count, _ := m.libraryDB.TrackCount()
		fmt.Fprintf(&b, "Total tracks: %d\n", count)
		dur, _ := m.libraryDB.TotalDuration()
		if dur > 0 {
			fmt.Fprintf(&b, "Total duration: %s\n", formatDuration(int(dur)))
		}
	} else {
		b.WriteString("Library not loaded yet\n")
	}

	if m.subsonicClient != nil {
		b.WriteString("\n")
		status := "Connected"
		if err := m.subsonicClient.Ping(); err != nil {
			status = fmt.Sprintf("Disconnected (%v)", err)
		}
		fmt.Fprintf(&b, "Subsonic: %s [%s] — %s\n", m.subsonicClient.ServerName(), m.subsonicClient.ServerBadge(), status)
		fmt.Fprintf(&b, "  Server: %s\n", m.subsonicClient.BaseURL())

		artists, err := m.subsonicClient.GetArtists()
		if err == nil {
			var artistCount, albumCount int
			for _, idx := range artists.Index {
				for _, a := range idx.Artist {
					artistCount++
					albumCount += a.AlbumCount
				}
			}
			fmt.Fprintf(&b, "  Artists: %d  Albums: %d\n", artistCount, albumCount)
		}
	}

	return ctl.CtlResult{OK: true, Data: strings.TrimRight(b.String(), "\n")}
}

func (m Model) ctlPlaylists() ([]ctl.SearchResult, ctl.CtlResult) {
	var results []ctl.SearchResult

	dir := config.GetPlaylistSaveDir()
	entries, err := os.ReadDir(dir)
	if err == nil {
		var names []string
		for _, e := range entries {
			if !e.IsDir() && (strings.HasSuffix(e.Name(), ".m3u") || strings.HasSuffix(e.Name(), ".m3u8")) {
				name := strings.TrimSuffix(e.Name(), ".m3u")
				name = strings.TrimSuffix(name, ".m3u8")
				names = append(names, name)
			}
		}
		sort.Strings(names)
		for _, n := range names {
			path := config.GetPlaylistSavePath(n)
			pl, err := playlist.Load(path)
			count := 0
			if err == nil && pl != nil {
				count = len(pl.Tracks)
			}
			results = append(results, ctl.SearchResult{
				Type: ctl.ResultPlaylist, PlaylistName: n, TrackCount: count,
				Display: fmt.Sprintf("%s (%d tracks)", n, count),
			})
		}
	}

	if m.subsonicClient != nil {
		subsonicPlaylists, err := m.subsonicClient.GetPlaylists()
		if err == nil {
			badge := m.subsonicClient.ServerBadge()
			serverName := m.subsonicClient.ServerName()
			for _, p := range subsonicPlaylists {
				results = append(results, ctl.SearchResult{
					Type: ctl.ResultSubsonicPlaylist, SubsonicPlaylistID: p.ID,
					PlaylistName: p.Name, TrackCount: p.SongCount,
					Display: fmt.Sprintf("[%s] %s: %s (%d tracks)", badge, serverName, p.Name, p.SongCount),
				})
			}
		}
	}

	if len(results) == 0 {
		return nil, ctl.CtlResult{OK: true, Data: "No playlists found"}
	}

	var b strings.Builder
	for i, r := range results {
		fmt.Fprintf(&b, "%d. %s\n", i+1, r.Display)
	}
	return results, ctl.CtlResult{OK: true, Data: strings.TrimRight(b.String(), "\n")}
}

// normalizeSubsonicPrefix rewrites the configured server name prefix (first word, case-insensitive)
// or server badge to "subsonic:", so e.g. "navidrome:radiohead" or "n:radiohead" becomes "subsonic:radiohead".
func normalizeSubsonicPrefix(client *api.SubsonicClient, query string) string {
	if client == nil {
		return query
	}
	name := strings.ToLower(strings.TrimSpace(client.ServerName()))
	if idx := strings.Index(name, " "); idx > 0 {
		name = name[:idx]
	}
	if name != "" && name != "subsonic" {
		prefix := name + ":"
		if strings.HasPrefix(strings.ToLower(query), prefix) {
			return "subsonic:" + query[len(prefix):]
		}
	}
	badge := strings.ToLower(strings.TrimSpace(client.ServerBadge()))
	if badge != "" && badge != name {
		prefix := badge + ":"
		if strings.HasPrefix(strings.ToLower(query), prefix) {
			return "subsonic:" + query[len(prefix):]
		}
	}
	return query
}

func parseQueryPrefix(query string) (field, value string) {
	idx := strings.Index(query, ":")
	if idx < 0 {
		return "", query
	}
	field = strings.ToLower(strings.TrimSpace(query[:idx]))
	value = strings.TrimSpace(query[idx+1:])
	switch field {
	case "artist", "album", "genre", "year", "title", "song", "track", "subsonic":
		return field, value
	default:
		return "", query
	}
}

func parseYearRange(s string) (int, int) {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) == 2 {
		y1, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		y2, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
		if y1 > 0 && y2 > 0 && y2 >= y1 {
			return y1, y2
		}
	}
	y, _ := strconv.Atoi(strings.TrimSpace(s))
	if y > 0 {
		return y, y
	}
	return 0, 0
}

func trackCountByArtist(libraryDB *db.LibraryDB, artist string) (int, error) {
	if libraryDB == nil {
		return 0, nil
	}
	tracks, err := libraryDB.GetTracksByArtist(artist)
	if err != nil {
		return 0, err
	}
	return len(tracks), nil
}

func trackCountByGenre(libraryDB *db.LibraryDB, genre string) int {
	if libraryDB == nil {
		return 0
	}
	tracks, err := libraryDB.GetTracksByField("genre", []string{genre})
	if err != nil {
		return 0
	}
	return len(tracks)
}

func fieldPrefix(arg string) string {
	for _, p := range []string{"artist:", "album:", "genre:", "year:", "playlist:"} {
		if strings.HasPrefix(arg, p) {
			return p
		}
	}
	return ""
}

func isNewArgStart(arg string) bool {
	if _, err := strconv.Atoi(arg); err == nil {
		return true
	}
	if strings.HasPrefix(arg, "/") || strings.HasPrefix(arg, "~") || strings.HasPrefix(arg, ".") {
		return true
	}
	if fieldPrefix(arg) != "" {
		return true
	}
	return false
}

func recombineFieldQueries(args []string) []string {
	var result []string
	for i := 0; i < len(args); i++ {
		if prefix := fieldPrefix(args[i]); prefix != "" {
			joined := args[i]
			for j := i + 1; j < len(args) && !isNewArgStart(args[j]); j++ {
				joined += " " + args[j]
				i = j
			}
			result = append(result, joined)
		} else if isNewArgStart(args[i]) {
			result = append(result, args[i])
		} else {
			joined := args[i]
			for j := i + 1; j < len(args) && !isNewArgStart(args[j]); j++ {
				joined += " " + args[j]
				i = j
			}
			result = append(result, joined)
		}
	}
	return result
}

func (m *Model) resolveTracks(args []string) ([]models.Track, string, error) {
	args = recombineFieldQueries(args)
	var allTracks []models.Track
	var labels []string

	for _, arg := range args {
		tracks, label, err := m.resolveSingleArg(arg)
		if err != nil {
			return nil, "", err
		}
		if len(tracks) == 0 {
			continue
		}
		allTracks = append(allTracks, tracks...)
		labels = append(labels, label)
	}

	label := strings.Join(labels, ", ")
	return allTracks, label, nil
}

func (m *Model) resolveSingleArg(arg string) ([]models.Track, string, error) {
	if n, err := strconv.Atoi(arg); err == nil {
		return m.resolveResultIndex(n)
	}

	if strings.HasPrefix(arg, "/") || strings.HasPrefix(arg, "~") || strings.HasPrefix(arg, ".") {
		return m.resolvePathOrPlaylist(arg)
	}

	if strings.HasPrefix(arg, "playlist:") {
		return m.resolveSavedPlaylist(arg[len("playlist:"):])
	}

	if strings.HasPrefix(arg, "subsonic:") {
		return m.resolveSubsonicQuery(arg[len("subsonic:"):])
	}

	normArg := normalizeSubsonicPrefix(m.subsonicClient, arg)
	if normArg != arg && strings.HasPrefix(normArg, "subsonic:") {
		return m.resolveSubsonicQuery(normArg[len("subsonic:"):])
	}

	if strings.HasPrefix(arg, "artist:") || strings.HasPrefix(arg, "album:") ||
		strings.HasPrefix(arg, "genre:") || strings.HasPrefix(arg, "year:") {
		return m.resolveFieldQuery(arg)
	}

	return m.resolveFTSQuery(arg)
}

func (m *Model) resolveResultIndex(n int) ([]models.Track, string, error) {
	if len(m.lastFindResults) == 0 {
		return nil, "", fmt.Errorf("no search results cached. Run 'must find' first")
	}
	idx := n - 1
	if idx < 0 || idx >= len(m.lastFindResults) {
		return nil, "", fmt.Errorf("result %d out of range (1-%d)", n, len(m.lastFindResults))
	}

	r := m.lastFindResults[idx]
	switch r.Type {
	case ctl.ResultTrack:
		if m.libraryDB == nil {
			return nil, "", fmt.Errorf("library not loaded")
		}
		t, err := m.libraryDB.GetTrackByID(r.TrackID)
		if err != nil || t == nil {
			return nil, "", fmt.Errorf("track not found")
		}
		return []models.Track{*t}, r.Display, nil

	case ctl.ResultArtist:
		if r.SubsonicArtistID != "" && m.subsonicClient != nil {
			artist, err := m.subsonicClient.GetArtist(r.SubsonicArtistID)
			if err != nil {
				return nil, "", fmt.Errorf("failed to get subsonic artist: %v", err)
			}
			var allTracks []models.Track
			for _, album := range artist.Album {
				albumDetail, err := m.subsonicClient.GetAlbum(album.ID)
				if err != nil {
					continue
				}
				allTracks = append(allTracks, m.subsonicClient.ChildrenToTracks(albumDetail.Song)...)
			}
			return allTracks, r.Display, nil
		}
		if m.libraryDB == nil {
			return nil, "", fmt.Errorf("library not loaded")
		}
		tracks, err := m.libraryDB.GetTracksByArtist(r.ArtistName)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get tracks: %v", err)
		}
		return tracks, r.Display, nil

	case ctl.ResultAlbum:
		if r.SubsonicAlbumID != "" && m.subsonicClient != nil {
			album, err := m.subsonicClient.GetAlbum(r.SubsonicAlbumID)
			if err != nil {
				return nil, "", fmt.Errorf("failed to get subsonic album: %v", err)
			}
			return m.subsonicClient.ChildrenToTracks(album.Song), r.Display, nil
		}
		if m.libraryDB == nil {
			return nil, "", fmt.Errorf("library not loaded")
		}
		tracks, err := m.libraryDB.GetTracksByArtistAndAlbum(r.ArtistName, r.AlbumName)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get tracks: %v", err)
		}
		return tracks, r.Display, nil

	case ctl.ResultGenre:
		if m.libraryDB == nil {
			return nil, "", fmt.Errorf("library not loaded")
		}
		tracks, err := m.libraryDB.GetTracksByField("genre", []string{r.GenreName})
		if err != nil {
			return nil, "", fmt.Errorf("failed to get tracks: %v", err)
		}
		return tracks, r.Display, nil

	case ctl.ResultYear:
		if m.libraryDB == nil {
			return nil, "", fmt.Errorf("library not loaded")
		}
		tracks, err := m.libraryDB.SearchWithYearRange("", r.Year, r.YearEnd)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get tracks: %v", err)
		}
		return tracks, r.Display, nil

	case ctl.ResultSubsonicTrack:
		if r.SubsonicTrack == nil {
			return nil, "", fmt.Errorf("subsonic track data missing")
		}
		return []models.Track{r.SubsonicTrack.Track}, r.Display, nil

	case ctl.ResultPlaylist:
		if m.libraryDB == nil {
			return nil, "", fmt.Errorf("library not loaded")
		}
		path := config.GetPlaylistSavePath(r.PlaylistName)
		pl, err := playlist.Load(path)
		if err != nil {
			return nil, "", fmt.Errorf("failed to load playlist: %v", err)
		}
		var tracks []models.Track
		for _, p := range pl.Tracks {
			t, err := m.libraryDB.GetTrackByPath(p)
			if err != nil || t == nil {
				tracks = append(tracks, models.Track{Path: p, Title: filepath.Base(p)})
				continue
			}
			tracks = append(tracks, *t)
		}
		return tracks, r.Display, nil

	case ctl.ResultSubsonicPlaylist:
		if m.subsonicClient == nil {
			return nil, "", fmt.Errorf("subsonic not configured")
		}
		pl, err := m.subsonicClient.GetPlaylist(r.SubsonicPlaylistID)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get subsonic playlist: %v", err)
		}
		tracks := m.subsonicClient.ChildrenToTracks(pl.Entry)
		return tracks, r.Display, nil

	default:
		return nil, "", fmt.Errorf("unknown result type: %s", r.Type)
	}
}

func (m *Model) resolvePathOrPlaylist(arg string) ([]models.Track, string, error) {
	info, err := os.Stat(arg)
	if err != nil {
		return nil, "", fmt.Errorf("path not found: %s", arg)
	}

	if info.IsDir() {
		tracks := loadDirTracks(arg, m.libraryDB)
		if len(tracks) == 0 {
			return nil, "", fmt.Errorf("no audio files in directory: %s", arg)
		}
		return tracks, filepath.Base(arg), nil
	}

	lower := strings.ToLower(arg)
	if strings.HasSuffix(lower, ".m3u") || strings.HasSuffix(lower, ".m3u8") {
		pl, err := playlist.Load(arg)
		if err != nil {
			return nil, "", fmt.Errorf("failed to load playlist: %v", err)
		}
		var tracks []models.Track
		for _, tp := range pl.Tracks {
			if t := findTrackByPath(tp, m.libraryDB); t != nil {
				tracks = append(tracks, *t)
			} else {
				tracks = append(tracks, models.Track{Path: tp, Title: filepath.Base(tp)})
			}
		}
		return tracks, filepath.Base(arg), nil
	}

	if isAudioFile(arg) {
		if t := findTrackByPath(arg, m.libraryDB); t != nil {
			return []models.Track{*t}, filepath.Base(arg), nil
		}
		return []models.Track{{
			Path:  arg,
			Title: strings.TrimSuffix(filepath.Base(arg), filepath.Ext(arg)),
		}}, filepath.Base(arg), nil
	}

	return nil, "", fmt.Errorf("unsupported file: %s", arg)
}

func (m *Model) resolveSavedPlaylist(name string) ([]models.Track, string, error) {
	path := config.GetPlaylistSavePath(name)
	if _, err := os.Stat(path); err != nil {
		return nil, "", fmt.Errorf("playlist '%s' not found", name)
	}
	return m.resolvePathOrPlaylist(path)
}

func (m *Model) resolveFieldQuery(arg string) ([]models.Track, string, error) {
	if m.libraryDB == nil {
		return nil, "", fmt.Errorf("library not loaded")
	}

	field, value := parseQueryPrefix(arg)
	switch field {
	case "artist":
		results, err := m.libraryDB.SearchArtistsLike(value)
		if err != nil || len(results) == 0 {
			return nil, "", fmt.Errorf("no artists matching '%s'", value)
		}
		tracks, err := m.libraryDB.GetTracksByArtist(results[0])
		if err != nil {
			return nil, "", fmt.Errorf("failed to get tracks: %v", err)
		}
		return tracks, fmt.Sprintf("artist: %s", results[0]), nil

	case "album":
		results, err := m.libraryDB.SearchAlbumsLike(value)
		if err != nil || len(results) == 0 {
			return nil, "", fmt.Errorf("no albums matching '%s'", value)
		}
		r := results[0]
		tracks, err := m.libraryDB.GetTracksByArtistAndAlbum(r.Artist, r.Album)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get tracks: %v", err)
		}
		return tracks, fmt.Sprintf("album: %s - %s", r.Artist, r.Album), nil

	case "genre":
		genres, err := m.libraryDB.GetGenres()
		if err != nil {
			return nil, "", fmt.Errorf("failed to search genres: %v", err)
		}
		for _, g := range genres {
			if strings.Contains(strings.ToLower(g), strings.ToLower(value)) {
				tracks, err := m.libraryDB.GetTracksByField("genre", []string{g})
				if err != nil {
					return nil, "", fmt.Errorf("failed to get tracks: %v", err)
				}
				return tracks, fmt.Sprintf("genre: %s", g), nil
			}
		}
		return nil, "", fmt.Errorf("no genres matching '%s'", value)

	case "year":
		yearMin, yearMax := parseYearRange(value)
		if yearMin == 0 {
			return nil, "", fmt.Errorf("invalid year: %s", value)
		}
		tracks, err := m.libraryDB.SearchWithYearRange("", yearMin, yearMax)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get tracks: %v", err)
		}
		return tracks, fmt.Sprintf("year: %s", value), nil

	default:
		return nil, "", fmt.Errorf("unknown field: %s", field)
	}
}

func (m *Model) resolveSubsonicQuery(arg string) ([]models.Track, string, error) {
	if m.subsonicClient == nil {
		return nil, "", fmt.Errorf("subsonic not configured")
	}

	field, value := parseQueryPrefix(arg)
	switch field {
	case "artist":
		result, err := m.subsonicClient.Search3(value, 1, 0, 0)
		if err != nil || len(result.Artist) == 0 {
			return nil, "", fmt.Errorf("no subsonic artist matching '%s'", value)
		}
		artist, err := m.subsonicClient.GetArtist(result.Artist[0].ID)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get subsonic artist: %v", err)
		}
		var allTracks []models.Track
		for _, album := range artist.Album {
			albumDetail, err := m.subsonicClient.GetAlbum(album.ID)
			if err != nil {
				continue
			}
			allTracks = append(allTracks, m.subsonicClient.ChildrenToTracks(albumDetail.Song)...)
		}
		return allTracks, fmt.Sprintf("subsonic artist: %s", artist.Name), nil

	case "album":
		result, err := m.subsonicClient.Search3(value, 0, 1, 0)
		if err != nil || len(result.Album) == 0 {
			return nil, "", fmt.Errorf("no subsonic album matching '%s'", value)
		}
		album, err := m.subsonicClient.GetAlbum(result.Album[0].ID)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get subsonic album: %v", err)
		}
		tracks := m.subsonicClient.ChildrenToTracks(album.Song)
		return tracks, fmt.Sprintf("subsonic album: %s - %s", album.Artist, album.Name), nil

	default:
		result, err := m.subsonicClient.Search3(arg, 3, 5, 20)
		if err != nil {
			return nil, "", fmt.Errorf("subsonic search failed: %v", err)
		}
		if len(result.Song) == 0 && len(result.Album) == 0 && len(result.Artist) == 0 {
			return nil, "", fmt.Errorf("no subsonic results for '%s'", arg)
		}
		if len(result.Song) > 0 {
			tracks := m.subsonicClient.ChildrenToTracks(result.Song)
			return tracks, fmt.Sprintf("subsonic: %s", arg), nil
		}
		if len(result.Album) > 0 {
			album, err := m.subsonicClient.GetAlbum(result.Album[0].ID)
			if err != nil {
				return nil, "", fmt.Errorf("failed to get subsonic album: %v", err)
			}
			tracks := m.subsonicClient.ChildrenToTracks(album.Song)
			return tracks, fmt.Sprintf("subsonic album: %s - %s", album.Artist, album.Name), nil
		}
		artist, err := m.subsonicClient.GetArtist(result.Artist[0].ID)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get subsonic artist: %v", err)
		}
		var allTracks []models.Track
		for _, album := range artist.Album {
			albumDetail, err := m.subsonicClient.GetAlbum(album.ID)
			if err != nil {
				continue
			}
			allTracks = append(allTracks, m.subsonicClient.ChildrenToTracks(albumDetail.Song)...)
		}
		return allTracks, fmt.Sprintf("subsonic artist: %s", artist.Name), nil
	}
}

func (m *Model) resolveFTSQuery(query string) ([]models.Track, string, error) {
	if m.libraryDB == nil {
		return nil, "", fmt.Errorf("library not loaded")
	}

	tracks, err := m.libraryDB.SearchFTS(query)
	if err != nil || len(tracks) == 0 {
		tracks, err = m.libraryDB.SearchLike(query)
	}
	if err != nil {
		return nil, "", fmt.Errorf("search failed: %v", err)
	}
	if len(tracks) == 0 {
		return nil, "", fmt.Errorf("no results for '%s'", query)
	}

	var label string
	if len(tracks) == 1 {
		label = fmt.Sprintf("%s - %s", tracks[0].Artist, tracks[0].Title)
	} else {
		label = fmt.Sprintf("'%s'", query)
	}
	return tracks, label, nil
}

func (m *Model) resolvePlayQuery(query string) ([]models.Track, string, int, error) {
	// Normalize subsonic server name/badge prefix
	if m.subsonicClient != nil {
		query = normalizeSubsonicPrefix(m.subsonicClient, query)
	}

	// Tier 0: explicit prefix via field query or subsonic
	switch {
	case strings.HasPrefix(query, "subsonic:"):
		rest := strings.TrimPrefix(query, "subsonic:")
		tracks, label, err := m.resolveSubsonicQuery(rest)
		return tracks, label, 0, err
	case strings.HasPrefix(query, "artist:"):
		tracks, label, err := m.resolveFieldQuery(query)
		return tracks, label, 0, err
	case strings.HasPrefix(query, "album:"):
		tracks, label, err := m.resolveFieldQuery(query)
		return tracks, label, 0, err
	case strings.HasPrefix(query, "genre:") || strings.HasPrefix(query, "year:"):
		tracks, label, err := m.resolveFieldQuery(query)
		return tracks, label, 0, err
	case strings.HasPrefix(query, "playlist:"):
		name := strings.TrimPrefix(query, "playlist:")
		tracks, label, err := m.resolveSavedPlaylist(name)
		return tracks, label, 0, err
	}

	if m.libraryDB == nil {
		return nil, "", 0, fmt.Errorf("library not loaded")
	}

	// Tier 1: exact artist match
	if artists, err := m.libraryDB.SearchArtistsLike(query); err == nil {
		for _, a := range artists {
			if strings.EqualFold(a, query) {
				tracks, err := m.libraryDB.GetTracksByArtist(a)
				if err == nil && len(tracks) > 0 {
					return tracks, "Artist: " + a, 0, nil
				}
			}
		}
	}

	// Tier 2: exact album match
	if albums, err := m.libraryDB.SearchAlbumsLike(query); err == nil {
		for _, a := range albums {
			if strings.EqualFold(a.Album, query) {
				tracks, err := m.libraryDB.GetTracksByArtistAndAlbum(a.Artist, a.Album)
				if err == nil && len(tracks) > 0 {
					return tracks, "Album: " + a.Artist + " - " + a.Album, 0, nil
				}
			}
		}
	}

	// Tier 3: exact track match → play its album from that track
	if ftsTracks, ftsErr := m.libraryDB.SearchFTS(query); ftsErr == nil && len(ftsTracks) > 0 {
		if len(ftsTracks) == 1 {
			track := ftsTracks[0]
			if track.Artist != "" && track.Album != "" {
				albumTracks, err := m.libraryDB.GetTracksByArtistAndAlbum(track.Artist, track.Album)
				if err == nil && len(albumTracks) > 0 {
					for i, t := range albumTracks {
						if strings.EqualFold(t.Title, track.Title) {
							return albumTracks, track.Artist + " - " + track.Album, i, nil
						}
					}
					return albumTracks, track.Artist + " - " + track.Album, 0, nil
				}
			}
		} else {
			// Multiple FTS results — check if best match has matching title
			first := ftsTracks[0]
			queryLower := strings.ToLower(query)
			titleLower := strings.ToLower(first.Title)
			if strings.EqualFold(first.Title, query) || strings.Contains(titleLower, queryLower) || strings.Contains(queryLower, titleLower) {
				if first.Artist != "" && first.Album != "" {
					albumTracks, err := m.libraryDB.GetTracksByArtistAndAlbum(first.Artist, first.Album)
					if err == nil && len(albumTracks) > 0 {
						for i, t := range albumTracks {
							if strings.EqualFold(t.Title, first.Title) {
								return albumTracks, first.Artist + " - " + first.Album, i, nil
							}
						}
						return albumTracks, first.Artist + " - " + first.Album, 0, nil
					}
				}
			}
		}
	}

	// Tier 4: FTS5 fallback (flat list)
	tracks, label, err := m.resolveFTSQuery(query)
	return tracks, label, 0, err
}

func formatDuration(totalSeconds int) string {
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}
