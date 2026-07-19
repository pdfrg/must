package modals

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pdfrg/must/internal/api"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/db"
	"github.com/pdfrg/must/internal/genre"
	"github.com/pdfrg/must/internal/models"
)

type LibraryModalMsg struct {
	PlayTracks  []models.Track
	PlayIndex   int
	Enqueue     []models.Track
	EnqueueNext []models.Track
	Closed      bool
}

type FocusPane int

const (
	FocusArtists FocusPane = iota
	FocusAlbums
	FocusTracks
)

type BrowseMode int

const (
	BrowseArtists BrowseMode = iota
	BrowseGenres
)

type artistDisplay struct {
	Name       string
	IsSubsonic bool
	SubsonicID string
}

type genreDisplay struct {
	Name       string
	IsSubsonic bool
}

type Library struct {
	styles *config.ThemeStyles
	db     *db.LibraryDB
	width  int
	height int

	allArtists  []artistDisplay
	allAlbums   []models.AlbumEntry
	artists     []artistDisplay
	albums      []models.AlbumEntry
	albumTracks []models.Track
	focusPane   FocusPane
	browseMode  BrowseMode

	artistCursor       int
	artistScrollOffset int
	albumCursor        int
	albumScrollOffset  int
	trackCursor        int
	trackScrollOffset  int

	genres            []genreDisplay
	allGenres         []genreDisplay
	genreCursor       int
	genreScrollOffset int

	filterText      string
	filteredArtists []artistDisplay
	filteredAlbums  []models.AlbumEntry
	filteredGenres  []genreDisplay
	enqueueNextMode bool
	albumSort       string
	source          SearchSource

	subsonicBadge string

	localArtistNames       []string
	subsonicArtistEntries  []api.ArtistID3
	subsonicAlbumIDs       []string
	subsonicAlbumsByArtist map[string][]api.AlbumID3
	subsonicTracksByAlbum  map[string][]models.Track

	localGenreNames       []string
	subsonicGenreEntries  []api.GenreID3
	subsonicAlbumsByGenre map[string][]api.AlbumID3

	PendingFetchArtistID  string
	PendingFetchAlbumID   string
	PendingFetchGenreName string
}

func NewLibrary(styles *config.ThemeStyles, libraryDB *db.LibraryDB) *Library {
	return &Library{
		styles:                 styles,
		db:                     libraryDB,
		focusPane:              FocusArtists,
		source:                 SearchBoth,
		subsonicAlbumsByArtist: make(map[string][]api.AlbumID3),
		subsonicTracksByAlbum:  make(map[string][]models.Track),
		subsonicAlbumsByGenre:  make(map[string][]api.AlbumID3),
	}
}

func (l *Library) SetEnqueueNextMode(next bool) {
	l.enqueueNextMode = next
}

func (l *Library) SetSize(width, height int) {
	l.width = width
	l.height = height
}

func (l *Library) SetSubsonicBadge(badge string) { l.subsonicBadge = badge }

func (l *Library) SetAlbumSort(sort string) {
	l.albumSort = sort
	if len(l.allAlbums) > 0 {
		if l.browseMode == BrowseGenres && len(l.genres) > 0 && l.genreCursor < len(l.genres) {
			l.loadAlbumsForGenre()
		} else if len(l.artists) > 0 && l.artistCursor < len(l.artists) {
			l.LoadAlbumsForArtist()
		}
	}
}

func (l *Library) SetSource(src SearchSource) {
	if l.source == src {
		return
	}
	l.source = src
	l.filterText = ""
	if l.browseMode == BrowseGenres {
		l.applyGenreSourceFilter()
	} else {
		l.applySourceFilter()
	}
}

func (l *Library) cycleSource() {
	l.source = (l.source + 1) % 3
	l.filterText = ""
	if l.browseMode == BrowseGenres {
		l.applyGenreSourceFilter()
	} else {
		l.applySourceFilter()
	}
}

func (l *Library) applySourceFilter() {
	var filtered []artistDisplay
	switch l.source {
	case SearchLocal:
		for _, a := range l.allArtists {
			if !a.IsSubsonic {
				filtered = append(filtered, a)
			}
		}
	case SearchSubsonic:
		for _, a := range l.allArtists {
			if a.IsSubsonic {
				filtered = append(filtered, a)
			}
		}
	default:
		filtered = l.allArtists
	}
	l.artists = filtered
	l.filteredArtists = nil
	if l.artistCursor >= len(l.artists) {
		l.artistCursor = 0
		l.artistScrollOffset = 0
	}
	if len(l.artists) > 0 && l.artistCursor < len(l.artists) {
		l.LoadAlbumsForArtist()
	} else {
		l.albums = nil
		l.allAlbums = nil
		l.albumTracks = nil
		l.albumCursor = 0
		l.albumScrollOffset = 0
		l.trackCursor = 0
		l.trackScrollOffset = 0
	}
}

func (l *Library) applyGenreSourceFilter() {
	var filtered []genreDisplay
	switch l.source {
	case SearchLocal:
		for _, g := range l.allGenres {
			if !g.IsSubsonic {
				filtered = append(filtered, g)
			}
		}
	case SearchSubsonic:
		for _, g := range l.allGenres {
			if g.IsSubsonic {
				filtered = append(filtered, g)
			}
		}
	default:
		filtered = l.allGenres
	}
	l.genres = filtered
	l.filteredGenres = nil
	if l.genreCursor >= len(l.genres) {
		l.genreCursor = 0
		l.genreScrollOffset = 0
	}
	if len(l.genres) > 0 && l.genreCursor < len(l.genres) {
		l.loadAlbumsForGenre()
	} else {
		l.albums = nil
		l.allAlbums = nil
		l.albumTracks = nil
		l.albumCursor = 0
		l.albumScrollOffset = 0
		l.trackCursor = 0
		l.trackScrollOffset = 0
	}
}

func (l *Library) rebuildDisplay() {
	var combined []artistDisplay
	seen := make(map[string]bool)
	for _, name := range l.localArtistNames {
		combined = append(combined, artistDisplay{Name: name})
		seen[strings.ToLower(name)] = true
	}
	for _, a := range l.subsonicArtistEntries {
		combined = append(combined, artistDisplay{
			Name:       a.Name,
			IsSubsonic: true,
			SubsonicID: a.ID,
		})
	}
	sort.Slice(combined, func(i, j int) bool {
		return strings.ToLower(combined[i].Name) < strings.ToLower(combined[j].Name)
	})
	l.allArtists = combined
	l.filteredArtists = nil
	l.applySourceFilter()
}

func (l *Library) SetArtists(artists []string) {
	l.localArtistNames = artists
	l.rebuildDisplay()
	l.filterText = ""
}

func (l *Library) SetSubsonicArtists(artists []api.ArtistID3) {
	l.PendingFetchArtistID = ""
	l.subsonicArtistEntries = artists
	l.rebuildDisplay()
	l.albums = nil
	l.allAlbums = nil
	l.albumTracks = nil
	// Load albums for the currently selected artist if it's Subsonic
	if len(l.artists) > 0 && l.artistCursor < len(l.artists) {
		if l.artists[l.artistCursor].IsSubsonic {
			l.LoadAlbumsForArtist()
		}
	}
}

func (l *Library) SetSubsonicAlbums(albums []api.AlbumID3) {
	l.PendingFetchAlbumID = ""
	if len(l.artists) == 0 || l.artistCursor >= len(l.artists) {
		return
	}
	entry := l.artists[l.artistCursor]
	if !entry.IsSubsonic {
		return
	}
	l.subsonicAlbumsByArtist[entry.SubsonicID] = albums
	l.loadSubsonicAlbumsForArtist(entry.SubsonicID)
}

func (l *Library) SetSubsonicTracks(tracks []models.Track) {
	if l.albumCursor < len(l.subsonicAlbumIDs) {
		l.subsonicTracksByAlbum[l.subsonicAlbumIDs[l.albumCursor]] = tracks
	}
	l.loadSubsonicTracksForAlbum()
}

func (l *Library) loadSubsonicAlbumsForArtist(artistID string) {
	if albums, ok := l.subsonicAlbumsByArtist[artistID]; ok {
		entries := make([]models.AlbumEntry, len(albums))
		ids := make([]string, len(albums))
		for i, a := range albums {
			entries[i] = models.AlbumEntry{Name: a.Name, Year: a.Year}
			ids[i] = a.ID
		}
		sortSubsonicAlbums(entries, ids, l.albumSort)
		l.allAlbums = entries
		l.albums = entries
		l.subsonicAlbumIDs = ids
		l.filteredAlbums = nil
		l.albumCursor = 0
		l.albumScrollOffset = 0
		l.albumTracks = nil
		l.loadSubsonicTracksForAlbum()
	} else {
		l.allAlbums = nil
		l.albums = nil
		l.albumTracks = nil
		l.albumCursor = 0
		l.albumScrollOffset = 0
		l.PendingFetchArtistID = artistID
	}
}

func (l *Library) loadSubsonicTracksForAlbum() {
	if l.albumCursor >= len(l.subsonicAlbumIDs) {
		l.albumTracks = nil
		return
	}
	id := l.subsonicAlbumIDs[l.albumCursor]
	if tracks, ok := l.subsonicTracksByAlbum[id]; ok {
		l.albumTracks = tracks
		l.trackCursor = 0
		l.trackScrollOffset = 0
	} else if l.PendingFetchArtistID == "" {
		l.PendingFetchAlbumID = id
	}
}

func (l *Library) rebuildGenreDisplay() {
	var combined []genreDisplay
	seen := make(map[string]bool)
	for _, name := range l.localGenreNames {
		combined = append(combined, genreDisplay{Name: name})
		seen[strings.ToLower(name)] = true
	}
	for _, g := range l.subsonicGenreEntries {
		for _, part := range genre.Split(g.Value) {
			low := strings.ToLower(part)
			if !seen[low] {
				combined = append(combined, genreDisplay{Name: part, IsSubsonic: true})
				seen[low] = true
			}
		}
	}
	sort.Slice(combined, func(i, j int) bool {
		return strings.ToLower(combined[i].Name) < strings.ToLower(combined[j].Name)
	})
	l.allGenres = combined
	l.filteredGenres = nil
	l.applyGenreSourceFilter()
}

func (l *Library) SetSubsonicGenres(genres []api.GenreID3) {
	l.PendingFetchGenreName = ""
	l.subsonicGenreEntries = genres
	l.rebuildGenreDisplay()
	if l.browseMode == BrowseGenres && len(l.genres) > 0 && l.genreCursor < len(l.genres) {
		if l.genres[l.genreCursor].IsSubsonic {
			l.loadAlbumsForGenre()
		}
	}
}

func (l *Library) SetSubsonicGenreAlbums(genreName string, albums []api.AlbumID3) {
	l.PendingFetchGenreName = ""
	if l.browseMode != BrowseGenres || l.genreCursor >= len(l.genres) {
		return
	}
	entry := l.genres[l.genreCursor]
	if !entry.IsSubsonic || !strings.EqualFold(entry.Name, genreName) {
		return
	}
	l.subsonicAlbumsByGenre[genreName] = albums
	l.loadSubsonicAlbumsForGenre(genreName)
}

func (l *Library) loadSubsonicAlbumsForGenre(genreName string) {
	if albums, ok := l.subsonicAlbumsByGenre[genreName]; ok {
		entries := make([]models.AlbumEntry, len(albums))
		ids := make([]string, len(albums))
		for i, a := range albums {
			entries[i] = models.AlbumEntry{Name: a.Artist + " - " + a.Name, Year: a.Year}
			ids[i] = a.ID
		}
		sortSubsonicAlbums(entries, ids, l.albumSort)
		l.allAlbums = entries
		l.albums = entries
		l.subsonicAlbumIDs = ids
		l.filteredAlbums = nil
		l.albumCursor = 0
		l.albumScrollOffset = 0
		l.albumTracks = nil
		l.trackCursor = 0
		l.trackScrollOffset = 0
		l.loadTracksForAlbum()
	} else {
		l.allAlbums = nil
		l.albums = nil
		l.albumTracks = nil
		l.albumCursor = 0
		l.albumScrollOffset = 0
		l.trackCursor = 0
		l.trackScrollOffset = 0
		l.PendingFetchGenreName = genreName
	}
}

func (l *Library) LoadAlbumsForArtist() {
	if len(l.artists) == 0 || l.artistCursor >= len(l.artists) {
		return
	}
	entry := l.artists[l.artistCursor]
	if entry.IsSubsonic {
		l.loadSubsonicAlbumsForArtist(entry.SubsonicID)
		return
	}
	if l.db == nil {
		return
	}
	sortMode := l.albumSort
	if sortMode == "" {
		sortMode = config.SortAlpha
	}
	albums, err := l.db.GetAlbumsByArtistSorted(entry.Name, sortMode)
	if err != nil || len(albums) == 0 {
		l.allAlbums = nil
		l.albums = nil
		l.albumTracks = nil
		l.albumCursor = 0
		l.albumScrollOffset = 0
		l.trackCursor = 0
		l.trackScrollOffset = 0
		return
	}
	l.allAlbums = albums
	l.albums = albums
	l.filteredAlbums = nil
	l.albumCursor = 0
	l.albumScrollOffset = 0
	l.albumTracks = nil
	l.trackCursor = 0
	l.trackScrollOffset = 0
	l.loadTracksForAlbum()
}

func (l *Library) loadTracksForAlbum() {
	if l.browseMode == BrowseGenres && l.genreCursor < len(l.genres) && l.genres[l.genreCursor].IsSubsonic {
		l.loadSubsonicTracksForAlbum()
		return
	}
	if len(l.artists) > 0 && l.artistCursor < len(l.artists) && l.artists[l.artistCursor].IsSubsonic {
		l.loadSubsonicTracksForAlbum()
		return
	}
	if l.db == nil || len(l.albums) == 0 || l.albumCursor >= len(l.albums) {
		l.albumTracks = nil
		l.trackCursor = 0
		l.trackScrollOffset = 0
		return
	}
	if l.browseMode == BrowseGenres {
		albumEntry := l.albums[l.albumCursor]
		tracks, err := l.db.GetTracksByAlbum(albumEntry.Name)
		if err == nil && len(tracks) > 0 {
			l.albumTracks = tracks
			l.trackCursor = 0
			l.trackScrollOffset = 0
		} else {
			l.albumTracks = nil
			l.trackCursor = 0
			l.trackScrollOffset = 0
		}
		return
	}
	entry := l.artists[l.artistCursor]
	albumEntry := l.albums[l.albumCursor]
	tracks, err := l.db.GetTracksByArtistAndAlbum(entry.Name, albumEntry.Name)
	if err == nil && len(tracks) > 0 {
		l.albumTracks = tracks
		l.trackCursor = 0
		l.trackScrollOffset = 0
	} else {
		l.albumTracks = nil
		l.trackCursor = 0
		l.trackScrollOffset = 0
	}
}

func (l *Library) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if l.filterText != "" {
				l.clearFilter()
				return nil
			}
			l.enqueueNextMode = false
			return func() tea.Msg { return LibraryModalMsg{Closed: true} }
		case "q":
			if l.filterText != "" {
				l.clearFilter()
				return nil
			}
			l.enqueueNextMode = false
			return func() tea.Msg { return LibraryModalMsg{Closed: true} }
		case "up", "k":
			l.moveUp()
		case "down", "j":
			l.moveDown()
		case "pgdown":
			l.pageDown()
		case "pgup":
			l.pageUp()
		case "home":
			l.jumpHome()
		case "end":
			l.jumpEnd()
		case "h", "left":
			l.focusLeft()
		case "l", "right":
			if l.filterText != "" {
				l.appendToFilter("l")
				return nil
			}
			l.focusRight()
		case "enter":
			if l.filterText != "" {
				l.clearFilter()
			}
			return l.handleEnter()
		case "e":
			return l.handleEnqueue()
		case "E":
			l.enqueueNextMode = true
			cmd := l.handleEnqueue()
			l.enqueueNextMode = false
			return cmd
		case "g":
			l.toggleBrowseMode()
		case "backspace":
			l.backspaceFilter()
		case "ctrl+t":
			l.cycleSource()
		case "ctrl+l":
			l.SetSource(SearchLocal)
		case "ctrl+s":
			l.SetSource(SearchSubsonic)
		default:
			k := msg.Key()
			if k.Text != "" {
				l.appendToFilter(k.Text)
				return nil
			}
		}
	}
	return nil
}

func (l *Library) appendToFilter(ch string) {
	l.filterText += ch
	l.applyFilter()
}

func (l *Library) backspaceFilter() {
	if len(l.filterText) > 0 {
		l.filterText = l.filterText[:len(l.filterText)-1]
		l.applyFilter()
	}
}

func (l *Library) clearFilter() {
	l.filterText = ""
	if l.browseMode == BrowseGenres {
		l.applyGenreSourceFilter()
	} else {
		l.applySourceFilter()
	}
	l.albums = l.allAlbums
	l.filteredAlbums = nil
	if l.albumCursor >= len(l.albums) && len(l.albums) > 0 {
		l.albumCursor = 0
		l.albumScrollOffset = 0
	}
}

func (l *Library) applyFilter() {
	if l.filterText == "" {
		if l.browseMode == BrowseGenres {
			l.applyGenreSourceFilter()
		} else {
			l.applySourceFilter()
		}
		l.albums = l.allAlbums
		l.filteredAlbums = nil
		return
	}

	if l.browseMode == BrowseGenres {
		l.filteredGenres = filterGenreDisplays(l.allGenres, l.filterText)
		l.genres = l.filteredGenres
		if l.genreCursor >= len(l.genres) {
			l.genreCursor = 0
			l.genreScrollOffset = 0
		}
	} else {
		l.filteredArtists = filterArtistDisplays(l.allArtists, l.filterText)
		l.artists = l.filteredArtists
		if l.artistCursor >= len(l.artists) {
			l.artistCursor = 0
			l.artistScrollOffset = 0
		}
	}

	if l.focusPane >= FocusAlbums {
		l.filteredAlbums = filterAlbumEntries(l.allAlbums, l.filterText)
		l.albums = l.filteredAlbums
		if l.albumCursor >= len(l.albums) {
			l.albumCursor = 0
			l.albumScrollOffset = 0
		}
	}
}

func filterAlbumEntries(items []models.AlbumEntry, query string) []models.AlbumEntry {
	query = strings.ToLower(query)
	var result []models.AlbumEntry
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Name), query) {
			result = append(result, item)
		}
	}
	return result
}

func filterArtistDisplays(items []artistDisplay, query string) []artistDisplay {
	query = strings.ToLower(query)
	var result []artistDisplay
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Name), query) {
			result = append(result, item)
		}
	}
	return result
}

func filterGenreDisplays(items []genreDisplay, query string) []genreDisplay {
	query = strings.ToLower(query)
	var result []genreDisplay
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Name), query) {
			result = append(result, item)
		}
	}
	return result
}

func (l *Library) handleEnter() tea.Cmd {
	switch l.focusPane {
	case FocusArtists:
		if l.browseMode == BrowseGenres {
			if len(l.genres) > 0 && l.genreCursor < len(l.genres) && len(l.albums) > 0 {
				l.focusPane = FocusAlbums
			}
		} else {
			if len(l.artists) > 0 && l.artistCursor < len(l.artists) && len(l.albums) > 0 {
				l.focusPane = FocusAlbums
			}
		}
	case FocusAlbums:
		if len(l.albumTracks) > 0 {
			return func() tea.Msg {
				return LibraryModalMsg{
					PlayTracks: l.albumTracks,
					PlayIndex:  0,
				}
			}
		}
	case FocusTracks:
		if len(l.albumTracks) > 0 && l.trackCursor < len(l.albumTracks) {
			return func() tea.Msg {
				return LibraryModalMsg{
					PlayTracks: []models.Track{l.albumTracks[l.trackCursor]},
					PlayIndex:  0,
				}
			}
		}
	}
	return nil
}

func (l *Library) handleEnqueue() tea.Cmd {
	var tracks []models.Track

	switch l.focusPane {
	case FocusTracks:
		if len(l.albumTracks) > 0 && l.trackCursor < len(l.albumTracks) {
			tracks = []models.Track{l.albumTracks[l.trackCursor]}
		}
	case FocusAlbums:
		if len(l.albums) > 0 && l.albumCursor < len(l.albums) {
			if len(l.albumTracks) > 0 {
				tracks = l.albumTracks
			}
		}
	case FocusArtists:
		if l.browseMode == BrowseGenres {
			if len(l.genres) > 0 && l.genreCursor < len(l.genres) {
				entry := l.genres[l.genreCursor]
				if entry.IsSubsonic {
					for _, albumID := range l.subsonicAlbumIDs {
						if at, ok := l.subsonicTracksByAlbum[albumID]; ok {
							tracks = append(tracks, at...)
						}
					}
				} else if l.db != nil && len(l.albums) > 0 {
					for _, album := range l.albums {
						t, err := l.db.GetTracksByAlbum(album.Name)
						if err == nil {
							tracks = append(tracks, t...)
						}
					}
				}
			}
		} else if len(l.artists) > 0 && l.artistCursor < len(l.artists) {
			entry := l.artists[l.artistCursor]
			if entry.IsSubsonic {
				for _, albumID := range l.subsonicAlbumIDs {
					if at, ok := l.subsonicTracksByAlbum[albumID]; ok {
						tracks = append(tracks, at...)
					}
				}
			} else if l.db != nil {
				t, err := l.db.GetTracksByArtist(entry.Name)
				if err == nil && len(t) > 0 {
					tracks = t
				}
			}
		}
	}

	if len(tracks) == 0 {
		return nil
	}

	if l.enqueueNextMode {
		return func() tea.Msg { return LibraryModalMsg{EnqueueNext: tracks} }
	}
	return func() tea.Msg { return LibraryModalMsg{Enqueue: tracks} }
}

func (l *Library) moveDown() {
	switch l.focusPane {
	case FocusArtists:
		if l.browseMode == BrowseGenres {
			if len(l.genres) > 0 && l.genreCursor < len(l.genres)-1 {
				l.genreCursor++
				ensureVisible(&l.genreCursor, &l.genreScrollOffset, len(l.genres), l.paneHeight())
				l.loadAlbumsForGenre()
			}
		} else {
			if len(l.artists) > 0 && l.artistCursor < len(l.artists)-1 {
				l.artistCursor++
				ensureVisible(&l.artistCursor, &l.artistScrollOffset, len(l.artists), l.paneHeight())
				l.LoadAlbumsForArtist()
			}
		}
	case FocusAlbums:
		if len(l.albums) > 0 && l.albumCursor < len(l.albums)-1 {
			l.albumCursor++
			ensureVisible(&l.albumCursor, &l.albumScrollOffset, len(l.albums), l.paneHeight())
			l.loadTracksForAlbum()
		}
	case FocusTracks:
		if len(l.albumTracks) > 0 && l.trackCursor < len(l.albumTracks)-1 {
			l.trackCursor++
			ensureVisible(&l.trackCursor, &l.trackScrollOffset, len(l.albumTracks), l.paneHeight())
		}
	}
}

func (l *Library) moveUp() {
	switch l.focusPane {
	case FocusArtists:
		if l.browseMode == BrowseGenres {
			if l.genreCursor > 0 {
				l.genreCursor--
				ensureVisible(&l.genreCursor, &l.genreScrollOffset, len(l.genres), l.paneHeight())
				l.loadAlbumsForGenre()
			}
		} else {
			if l.artistCursor > 0 {
				l.artistCursor--
				ensureVisible(&l.artistCursor, &l.artistScrollOffset, len(l.artists), l.paneHeight())
				l.LoadAlbumsForArtist()
			}
		}
	case FocusAlbums:
		if l.albumCursor > 0 {
			l.albumCursor--
			ensureVisible(&l.albumCursor, &l.albumScrollOffset, len(l.albums), l.paneHeight())
			l.loadTracksForAlbum()
		}
	case FocusTracks:
		if l.trackCursor > 0 {
			l.trackCursor--
			ensureVisible(&l.trackCursor, &l.trackScrollOffset, len(l.albumTracks), l.paneHeight())
		}
	}
}

func (l *Library) pageDown() {
	ps := max(l.paneHeight()-1, 1)
	switch l.focusPane {
	case FocusArtists:
		if l.browseMode == BrowseGenres {
			if len(l.genres) > 0 {
				l.genreCursor = min(l.genreCursor+ps, len(l.genres)-1)
				ensureVisible(&l.genreCursor, &l.genreScrollOffset, len(l.genres), l.paneHeight())
				l.loadAlbumsForGenre()
			}
		} else {
			if len(l.artists) > 0 {
				l.artistCursor = min(l.artistCursor+ps, len(l.artists)-1)
				ensureVisible(&l.artistCursor, &l.artistScrollOffset, len(l.artists), l.paneHeight())
				l.LoadAlbumsForArtist()
			}
		}
	case FocusAlbums:
		if len(l.albums) > 0 {
			l.albumCursor = min(l.albumCursor+ps, len(l.albums)-1)
			ensureVisible(&l.albumCursor, &l.albumScrollOffset, len(l.albums), l.paneHeight())
			l.loadTracksForAlbum()
		}
	case FocusTracks:
		if len(l.albumTracks) > 0 {
			l.trackCursor = min(l.trackCursor+ps, len(l.albumTracks)-1)
			ensureVisible(&l.trackCursor, &l.trackScrollOffset, len(l.albumTracks), l.paneHeight())
		}
	}
}

func (l *Library) pageUp() {
	ps := max(l.paneHeight()-1, 1)
	switch l.focusPane {
	case FocusArtists:
		if l.browseMode == BrowseGenres {
			l.genreCursor = max(l.genreCursor-ps, 0)
			ensureVisible(&l.genreCursor, &l.genreScrollOffset, len(l.genres), l.paneHeight())
			l.loadAlbumsForGenre()
		} else {
			l.artistCursor = max(l.artistCursor-ps, 0)
			ensureVisible(&l.artistCursor, &l.artistScrollOffset, len(l.artists), l.paneHeight())
			l.LoadAlbumsForArtist()
		}
	case FocusAlbums:
		l.albumCursor = max(l.albumCursor-ps, 0)
		ensureVisible(&l.albumCursor, &l.albumScrollOffset, len(l.albums), l.paneHeight())
		l.loadTracksForAlbum()
	case FocusTracks:
		l.trackCursor = max(l.trackCursor-ps, 0)
		ensureVisible(&l.trackCursor, &l.trackScrollOffset, len(l.albumTracks), l.paneHeight())
	}
}

func (l *Library) jumpHome() {
	switch l.focusPane {
	case FocusArtists:
		if l.browseMode == BrowseGenres {
			l.genreCursor = 0
			l.genreScrollOffset = 0
			l.loadAlbumsForGenre()
		} else {
			l.artistCursor = 0
			l.artistScrollOffset = 0
			l.LoadAlbumsForArtist()
		}
	case FocusAlbums:
		l.albumCursor = 0
		l.albumScrollOffset = 0
		l.loadTracksForAlbum()
	case FocusTracks:
		l.trackCursor = 0
		l.trackScrollOffset = 0
	}
}

func (l *Library) jumpEnd() {
	switch l.focusPane {
	case FocusArtists:
		if l.browseMode == BrowseGenres {
			if len(l.genres) > 0 {
				l.genreCursor = len(l.genres) - 1
				ensureVisible(&l.genreCursor, &l.genreScrollOffset, len(l.genres), l.paneHeight())
				l.loadAlbumsForGenre()
			}
		} else {
			if len(l.artists) > 0 {
				l.artistCursor = len(l.artists) - 1
				ensureVisible(&l.artistCursor, &l.artistScrollOffset, len(l.artists), l.paneHeight())
				l.LoadAlbumsForArtist()
			}
		}
	case FocusAlbums:
		if len(l.albums) > 0 {
			l.albumCursor = len(l.albums) - 1
			ensureVisible(&l.albumCursor, &l.albumScrollOffset, len(l.albums), l.paneHeight())
			l.loadTracksForAlbum()
		}
	case FocusTracks:
		if len(l.albumTracks) > 0 {
			l.trackCursor = len(l.albumTracks) - 1
			ensureVisible(&l.trackCursor, &l.trackScrollOffset, len(l.albumTracks), l.paneHeight())
		}
	}
}

func (l *Library) focusLeft() {
	if l.focusPane > FocusArtists {
		l.focusPane--
	}
}

func (l *Library) focusRight() {
	if l.focusPane < FocusTracks {
		switch l.focusPane {
		case FocusArtists:
			if len(l.albums) > 0 {
				l.focusPane = FocusAlbums
			}
		case FocusAlbums:
			if len(l.albumTracks) > 0 {
				l.focusPane = FocusTracks
			}
		}
	}
}

func (l *Library) toggleBrowseMode() {
	l.filterText = ""
	if l.browseMode == BrowseArtists {
		l.browseMode = BrowseGenres
		l.focusPane = FocusArtists
		l.loadGenres()
	} else {
		l.browseMode = BrowseArtists
		l.focusPane = FocusArtists
		l.filteredArtists = nil
		l.albums = nil
		l.allAlbums = nil
		l.albumTracks = nil
		l.albumCursor = 0
		l.albumScrollOffset = 0
		l.trackCursor = 0
		l.trackScrollOffset = 0
		l.applySourceFilter()
	}
}

func (l *Library) loadGenres() {
	if l.db == nil {
		return
	}
	names, err := l.db.GetGenres()
	if err != nil {
		l.localGenreNames = nil
	} else {
		l.localGenreNames = names
	}
	l.rebuildGenreDisplay()
	if len(l.genres) > 0 {
		l.loadAlbumsForGenre()
	} else {
		l.albums = nil
		l.allAlbums = nil
		l.albumTracks = nil
		l.albumCursor = 0
		l.albumScrollOffset = 0
		l.trackCursor = 0
		l.trackScrollOffset = 0
	}
}

func (l *Library) loadAlbumsForGenre() {
	if l.genreCursor >= len(l.genres) {
		return
	}
	entry := l.genres[l.genreCursor]
	if entry.IsSubsonic {
		l.loadSubsonicAlbumsForGenre(entry.Name)
		return
	}
	if l.db == nil {
		return
	}
	sortMode := l.albumSort
	if sortMode == "" {
		sortMode = config.SortAlpha
	}
	albums, err := l.db.GetAlbumsByGenreSorted(entry.Name, sortMode)
	if err != nil || len(albums) == 0 {
		l.albums = nil
		l.allAlbums = nil
		l.albumTracks = nil
		l.albumCursor = 0
		l.albumScrollOffset = 0
		l.trackCursor = 0
		l.trackScrollOffset = 0
		return
	}
	l.allAlbums = albums
	l.albums = albums
	l.filteredAlbums = nil
	l.albumCursor = 0
	l.albumScrollOffset = 0
	l.albumTracks = nil
	l.trackCursor = 0
	l.trackScrollOffset = 0
	l.loadTracksForAlbum()
}

func (l *Library) paneHeight() int {
	return l.height - 4
}

func (l Library) View() string {
	if l.browseMode == BrowseArtists && len(l.allArtists) == 0 {
		return l.styles.MutedStyle.Render("Library empty - press R to rescan")
	}
	if l.browseMode == BrowseGenres && len(l.allGenres) == 0 {
		return l.styles.MutedStyle.Render("Library empty - press R to rescan")
	}

	avail := l.width - 4
	var col1Width, col2Width, col3Width int
	if l.browseMode == BrowseGenres {
		col1Width = avail * 25 / 100
		col2Width = avail * 45 / 100
		col3Width = avail - col1Width - col2Width
	} else {
		col1Width = avail / 3
		col2Width = avail / 3
		col3Width = avail / 3
	}
	minWidth := 10
	if col1Width < minWidth {
		col1Width = minWidth
	}
	if col2Width < minWidth {
		col2Width = minWidth
	}
	if col3Width < minWidth {
		col3Width = minWidth
	}
	height := l.paneHeight()
	if height < 3 {
		height = 3
	}

	var col1, col2, col3 string
	if l.browseMode == BrowseGenres {
		col1 = l.renderGenreList(col1Width, height)
	} else {
		col1 = l.renderArtistList(col1Width, height)
	}
	col2 = l.renderAlbumColumn(col2Width, height)
	col3 = l.renderTrackColumn(col3Width, height)

	sep1 := l.styles.MutedStyle.Render("│")
	if l.focusPane == FocusArtists {
		sep1 = l.styles.AccentStyle.Render("│")
	}
	sep2 := l.styles.MutedStyle.Render("│")
	if l.focusPane == FocusAlbums || l.focusPane == FocusTracks {
		sep2 = l.styles.AccentStyle.Render("│")
	}

	col1Lines := strings.Split(col1, "\n")
	col2Lines := strings.Split(col2, "\n")
	col3Lines := strings.Split(col3, "\n")
	maxLines := max(len(col1Lines), len(col2Lines), len(col3Lines), height)

	var b strings.Builder
	for i := 0; i < maxLines; i++ {
		var c1, c2, c3 string
		if i < len(col1Lines) {
			c1 = col1Lines[i]
		}
		if i < len(col2Lines) {
			c2 = col2Lines[i]
		}
		if i < len(col3Lines) {
			c3 = col3Lines[i]
		}
		c1 = l.padOrTruncateLine(c1, col1Width)
		c2 = l.padOrTruncateLine(c2, col2Width)
		c3 = l.padOrTruncateLine(c3, col3Width)
		b.WriteString(c1)
		b.WriteString(" ")
		b.WriteString(sep1)
		b.WriteString(" ")
		b.WriteString(c2)
		b.WriteString(" ")
		b.WriteString(sep2)
		b.WriteString(" ")
		b.WriteString(c3)
		if i < maxLines-1 {
			b.WriteString("\n")
		}
	}

	topBar := l.renderTopBar()
	sourceBar := l.renderSourceBar()
	helpLine := l.renderHelpLine()
	inner := topBar + sourceBar + b.String() + "\n" + helpLine
	return lipgloss.NewStyle().Width(l.width).Render(inner)
}

func (l Library) padOrTruncateLine(line string, width int) string {
	visualWidth := lipgloss.Width(line)
	if visualWidth > width {
		return ansi.Truncate(line, width, "")
	}
	if visualWidth < width {
		return line + strings.Repeat(" ", width-visualWidth)
	}
	return line
}

func (l Library) renderTopBar() string {
	modeLabel := "artists"
	if l.browseMode == BrowseGenres {
		modeLabel = "genres"
	}
	if l.filterText != "" {
		filterDisplay := l.filterText
		if len(filterDisplay) > 30 {
			filterDisplay = filterDisplay[:30] + "…"
		}
		return l.styles.AccentStyle.Render(fmt.Sprintf("filter: %s", filterDisplay)) +
			l.styles.MutedStyle.Render(fmt.Sprintf(" [%s]", modeLabel)) + "\n"
	}
	return l.styles.MutedStyle.Render(fmt.Sprintf("[%s]", modeLabel)) + "\n"
}

func (l Library) renderSourceBar() string {
	labels := []string{"Local", "Both", "Subsonic"}
	var parts []string
	for i, label := range labels {
		if SearchSource(i) == l.source {
			parts = append(parts, l.styles.AccentStyle.Render("["+label+"]"))
		} else {
			parts = append(parts, l.styles.MutedStyle.Render(label))
		}
	}
	return strings.Join(parts, "  ") + "\n"
}

func (l Library) renderHelpLine() string {
	type helpItem struct{ key, desc string }
	helpPairs := []helpItem{
		{"↑/↓", "nav"},
		{"←/→", "focus"},
		{"enter", "play"},
		{"e", "enqueue"},
		{"^t", "source"},
		{"g", "genre"},
		{"esc", "close"},
	}

	if l.filterText != "" {
		helpPairs = []helpItem{
			{"type", "filter"},
			{"bksp", "del"},
			{"esc", "clear"},
		}
	}
	focusName := "artists"
	if l.browseMode == BrowseGenres {
		focusName = "genres"
	}
	switch l.focusPane {
	case FocusAlbums:
		focusName = "albums"
	case FocusTracks:
		focusName = "tracks"
	}

	var b strings.Builder
	for i, p := range helpPairs {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(l.styles.AccentStyle.Render(p.key))
		b.WriteString(" ")
		b.WriteString(l.styles.MutedStyle.Render(p.desc))
	}
	b.WriteString(" ")
	b.WriteString(l.styles.MutedStyle.Render(fmt.Sprintf("[%s]", focusName)))
	return b.String()
}

func (l Library) renderArtistList(width, height int) string {
	var b strings.Builder
	items := l.artists
	maxRows := min(height, len(items)-l.artistScrollOffset)
	if maxRows < 1 {
		if len(items) == 0 {
			return l.styles.MutedStyle.Render("  No artists")
		}
		maxRows = 1
	}
	focused := l.focusPane == FocusArtists
	for i := 0; i < maxRows; i++ {
		idx := l.artistScrollOffset + i
		if idx >= len(items) {
			break
		}
		entry := items[idx]
		display := entry.Name
		if entry.IsSubsonic {
			badge := l.subsonicBadge
			if badge == "" {
				badge = "S"
			}
			display = "[" + badge + "] " + display
		}
		display = ansi.Truncate(display, width-2, "…")
		if idx == l.artistCursor && focused {
			display = l.styles.CursorStyle.Render("> " + display)
		} else if idx == l.artistCursor {
			display = l.styles.AccentStyle.Render(" " + display)
		} else {
			display = l.styles.MutedStyle.Render(" " + display)
		}
		b.WriteString(display)
		if i < maxRows-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (l Library) renderGenreList(width, height int) string {
	var b strings.Builder
	items := l.genres
	maxRows := min(height, len(items)-l.genreScrollOffset)
	if maxRows < 1 {
		if len(items) == 0 {
			return l.styles.MutedStyle.Render("  No genres")
		}
		maxRows = 1
	}
	focused := l.focusPane == FocusArtists
	for i := 0; i < maxRows; i++ {
		idx := l.genreScrollOffset + i
		if idx >= len(items) {
			break
		}
		entry := items[idx]
		display := entry.Name
		if entry.IsSubsonic {
			badge := l.subsonicBadge
			if badge == "" {
				badge = "S"
			}
			display = "[" + badge + "] " + display
		}
		display = ansi.Truncate(display, width-2, "…")
		if idx == l.genreCursor && focused {
			display = l.styles.CursorStyle.Render("> " + display)
		} else if idx == l.genreCursor {
			display = l.styles.AccentStyle.Render(" " + display)
		} else {
			display = l.styles.MutedStyle.Render(" " + display)
		}
		b.WriteString(display)
		if i < maxRows-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (l Library) renderAlbumColumn(width, height int) string {
	if len(l.albums) == 0 {
		if l.browseMode == BrowseGenres {
			return l.styles.MutedStyle.Render("  Select a genre")
		}
		return l.styles.MutedStyle.Render("  Select an artist")
	}

	var b strings.Builder
	items := l.albums
	maxRows := min(height, len(items)-l.albumScrollOffset)
	if maxRows < 1 {
		maxRows = 1
	}
	focused := l.focusPane == FocusAlbums
	for i := 0; i < maxRows; i++ {
		idx := l.albumScrollOffset + i
		if idx >= len(items) {
			break
		}
		entry := items[idx]
		displayName := entry.Name
		if entry.Year > 0 {
			displayName = entry.Name + " (" + fmt.Sprintf("%d", entry.Year) + ")"
		}
		if l.browseMode == BrowseGenres {
			if parts := strings.SplitN(displayName, " - ", 2); len(parts) == 2 {
				artistMax := width/2 - 3
				if artistMax < 5 {
					artistMax = 5
				}
				artist := ansi.Truncate(parts[0], artistMax, "…")
				displayName = artist + " - " + parts[1]
			}
		}
		displayName = ansi.Truncate(displayName, width-2, "…")
		if idx == l.albumCursor && focused {
			displayName = l.styles.CursorStyle.Render("> " + displayName)
		} else if idx == l.albumCursor {
			displayName = l.styles.AccentStyle.Render(" " + displayName)
		} else {
			displayName = l.styles.ForegroundStyle.Render(" " + displayName)
		}
		b.WriteString(displayName)
		if i < maxRows-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (l Library) renderTrackColumn(width, height int) string {
	if len(l.albumTracks) == 0 {
		if len(l.albums) > 0 {
			return l.styles.MutedStyle.Render("  Select an album")
		}
		return ""
	}

	focused := l.focusPane == FocusTracks
	tracks := l.albumTracks
	start := l.trackScrollOffset
	maxRows := min(height, len(tracks)-start)
	if maxRows < 1 {
		maxRows = 1
	}
	var b strings.Builder
	for i := 0; i < maxRows; i++ {
		idx := start + i
		if idx >= len(tracks) {
			break
		}
		t := tracks[idx]
		dur := t.GetDurationFormatted()
		label := fmt.Sprintf("%2d. %s", t.TrackNum, t.Title)
		avail := width - len(dur) - 5
		if avail < 10 {
			avail = 10
		}
		label = ansi.Truncate(label, avail, "…")
		var prefix string
		if idx == l.trackCursor && focused {
			prefix = l.styles.CursorStyle.Render("> ")
		} else if idx == l.trackCursor {
			prefix = l.styles.AccentStyle.Render(" ")
		} else {
			prefix = " "
		}
		line := fmt.Sprintf("%s%s %s", prefix, label, l.styles.MutedStyle.Render(dur))
		if idx == l.trackCursor && focused {
			b.WriteString(line)
		} else {
			b.WriteString(l.styles.ForegroundStyle.Render(line))
		}
		if i < maxRows-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func sortSubsonicAlbums(entries []models.AlbumEntry, ids []string, sortMode string) {
	if len(entries) == 0 {
		return
	}
	perm := make([]int, len(entries))
	for i := range perm {
		perm[i] = i
	}
	sort.SliceStable(perm, func(i, j int) bool {
		a, b := entries[perm[i]], entries[perm[j]]
		switch sortMode {
		case config.SortYearDesc:
			if a.Year != b.Year {
				if a.Year == 0 {
					return false
				}
				if b.Year == 0 {
					return true
				}
				return a.Year > b.Year
			}
		case config.SortYearAsc:
			if a.Year != b.Year {
				if a.Year == 0 {
					return false
				}
				if b.Year == 0 {
					return true
				}
				return a.Year < b.Year
			}
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	newEntries := make([]models.AlbumEntry, len(entries))
	newIDs := make([]string, len(ids))
	for i, p := range perm {
		newEntries[i] = entries[p]
		newIDs[i] = ids[p]
	}
	copy(entries, newEntries)
	copy(ids, newIDs)
}

func ensureVisible(cursor, offset *int, total, visibleHeight int) {
	if visibleHeight < 1 {
		visibleHeight = 1
	}
	if *cursor < *offset {
		*offset = *cursor
	}
	if *cursor >= *offset+visibleHeight {
		*offset = *cursor - visibleHeight + 1
	}
	if *offset < 0 {
		*offset = 0
	}
}
