package tui

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/pdfrg/must/internal/models"
)

func (m Model) handleScanComplete(msg scanCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.scanMsg = fmt.Sprintf("Scan error: %v", msg.err)
		return m, nil
	}

	m.libraryReady = true
	m.scanning = false
	m.scanResult = msg.result
	m.libraryDB = msg.db
	logf("Library scan complete: %v", msg.result)

	m.loadCLIPaths()

	// Handle playQuery: auto-started from "must p <query>" or "must ps <query>"
	if m.playQuery != "" {
		tracks, label, startIdx, err := m.resolvePlayQuery(m.playQuery)
		if err != nil {
			m.scanMsg = err.Error()
			return m, nil
		}
		m.playlist = tracks
		m.currentIndex = startIdx
		if m.shuffleMode {
			m.shuffle = true
			m.shuffleOrder = shuffleIndices(len(m.playlist))
		} else {
			m.shuffle = false
			m.shuffleOrder = nil
		}
		m.updatePlaylist()

		return m, tea.Batch(
			m.playTrack(m.currentIndex),
			setStatus(&m, "Playing "+label, false),
		)
	}

	var restoreCmd tea.Cmd
	if len(m.playlist) == 0 && len(m.paths) == 0 {
		if !m.noRestore && m.cfg.RestoreOnStart {
			restoreCmd = m.restorePlaybackState()
		}
	}

	if m.searchModal != nil {
		m.searchModal.SetDB(m.libraryDB)
	}

	if m.randomMode && len(m.paths) == 0 && m.libraryDB != nil && restoreCmd == nil {
		return m.handleRandomPlay()
	}

	if m.autoplay && len(m.paths) == 0 && restoreCmd == nil && !m.randomMode && m.libraryDB != nil {
		return m.handleAutoplay()
	}

	artists, err := m.libraryDB.GetAllArtists()
	if err != nil {
		m.scanMsg = fmt.Sprintf("Error loading artists: %v", err)
		return m, nil
	}
	m.artists = artists

	if m.libraryModal != nil {
		m.libraryModal.SetArtists(artists)
	}

	if msg.result != nil {
		r := msg.result
		m.scanMsg = fmt.Sprintf("Library: %d tracks (%d new, %d updated, %d removed)", r.TotalFiles, r.NewFiles, r.UpdatedFiles, r.RemovedFiles)
	} else {
		count, _ := m.libraryDB.TrackCount()
		m.scanMsg = fmt.Sprintf("Library: %d tracks", count)
	}

	var cmds []tea.Cmd
	if restoreCmd != nil {
		cmds = append(cmds, restoreCmd)
	}
	if len(m.playlist) > 0 && len(m.paths) > 0 {
		cmds = append(cmds, m.playTrack(0))
	}
	cmds = append(cmds, setStatus(&m, m.scanMsg, false))
	return m, tea.Batch(cmds...)
}

func (m *Model) restorePlaybackState() tea.Cmd {
	state := LoadPlaybackState()
	if state == nil || len(state.PlaylistPaths) == 0 {
		return nil
	}

	var tracks []models.Track
	var hasSubsonic bool

	for i, p := range state.PlaylistPaths {
		if len(state.PlaylistSources) > i && state.PlaylistSources[i] == "subsonic" {
			hasSubsonic = true
			id := state.PlaylistRemoteIDs[i]
			track := models.Track{
				Source:   models.SourceSubsonic,
				RemoteID: id,
				Path:     p,
			}
			if m.subsonicClient != nil {
				track.ServerName = m.subsonicClient.ServerName()
				track.ServerBadge = m.subsonicClient.ServerBadge()
				if song, err := m.subsonicClient.GetSong(id); err == nil {
					track = m.subsonicClient.ChildToTrack(*song)
				}
			}
			if track.Title == "" {
				track.Title = filepath.Base(p)
			}
			tracks = append(tracks, track)
			continue
		}

		if _, err := os.Stat(p); err != nil {
			continue
		}

		if t := findTrackByPath(p, m.libraryDB); t != nil {
			tracks = append(tracks, *t)
		} else {
			tracks = append(tracks, readTrackFromFile(p))
		}
	}

	if len(tracks) == 0 {
		ClearPlaybackState()
		return nil
	}

	m.playlist = tracks

	if state.Shuffle {
		m.shuffle = true
		m.shuffleOrder = shuffleIndices(len(m.playlist))
		logf("Restored shuffle state")
	} else {
		m.shuffle = false
		m.shuffleOrder = nil
	}

	switch state.RepeatMode {
	case "off", "all", "one":
		m.repeatMode = state.RepeatMode
	default:
		m.repeatMode = "off"
	}

	if state.CurrentIndex < 0 || state.CurrentIndex >= len(tracks) {
		state.CurrentIndex = 0
	}

	m.currentIndex = state.CurrentIndex
	m.restoringPlayback = true
	m.updatePlaylist()
	m.playlistWidget.SetCursor(state.CurrentIndex)

	paths := m.buildMPVPlaylistPaths()
	playIdx := m.playlistIndexToMPVIndex(state.CurrentIndex)
	if playIdx < 0 {
		playIdx = 0
	}

	savedPos := state.Position

	if hasSubsonic {
		SavePlaybackState(m.playlist, m.currentIndex, savedPos, m.shuffle, m.repeatMode)
	}

	return func() tea.Msg {
		logf("Restoring playback: %d tracks, index=%d, pos=%.1f", len(paths), playIdx, savedPos)
		if err := m.mpvBackend.Start(paths); err != nil {
			ClearPlaybackState()
			return statusClearMsg{}
		}
		_ = m.mpvBackend.PlaylistPlayIndex(playIdx)
		if savedPos > 0 {
			_ = m.mpvBackend.SeekAbsolute(savedPos)
		}
		return restorePlaybackMsg{position: savedPos}
	}
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

		tracks, err := m.libraryDB.GetTracksByArtistAndAlbum(artist, album)
		if err != nil || len(tracks) == 0 {
			return m, setStatus(&m, "No tracks found for random album", true)
		}
		m.playlist = tracks
	}

	if m.shuffle {
		m.shuffleOrder = shuffleIndices(len(m.playlist))
	}

	m.updatePlaylist()
	return m, m.playTrack(0)
}

func (m Model) handleAutoplay() (tea.Model, tea.Cmd) {
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

	albums, err := m.libraryDB.GetAlbumsByArtist(artist)
	if err != nil || len(albums) == 0 {
		return m, setStatus(&m, "No albums found for autoplay", true)
	}

	album := albums[0]
	tracks, err := m.libraryDB.GetTracksByArtistAndAlbum(artist, album)
	if err != nil || len(tracks) == 0 {
		return m, setStatus(&m, "No tracks found for autoplay", true)
	}

	m.playlist = tracks
	if m.shuffle {
		m.shuffleOrder = shuffleIndices(len(m.playlist))
	}

	m.updatePlaylist()
	return m, m.playTrack(0)
}
