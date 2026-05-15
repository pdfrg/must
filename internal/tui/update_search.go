package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/pdfrg/must/internal/tui/modals"
)

func (m Model) handleScanComplete(msg scanCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.scanMsg = fmt.Sprintf("Scan error: %v", msg.err)
		return m, nil
	}

	m.libraryReady = true
	m.scanResult = msg.result
	m.libraryDB = msg.db

	m.loadCLIPaths()

	if m.searchModal != nil {
		m.searchModal = modals.NewSearch(m.styles, m.libraryDB)
	}

	if m.randomMode && len(m.paths) == 0 && m.libraryDB != nil {
		return m.handleRandomPlay()
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
		m.scanMsg = fmt.Sprintf("Library: %d tracks (%d new, %d updated, %d removed)",
			r.TotalFiles, r.NewFiles, r.UpdatedFiles, r.RemovedFiles)
	} else {
		count, _ := m.libraryDB.TrackCount()
		m.scanMsg = fmt.Sprintf("Library: %d tracks", count)
	}

	return m, setStatus(&m, m.scanMsg, false)
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
