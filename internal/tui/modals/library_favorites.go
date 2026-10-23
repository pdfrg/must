package modals

import (
	"sort"
	"strings"

	"github.com/pdfrg/must/internal/models"
)

// Favorites-only view for the library modal.
//
// Toggled with ctrl+f, it reuses the existing artist/album/track panes but
// populates them from favorited tracks only: local favorites from the SQLite
// DB and server favorites from a getStarred2 fetch (answered by the parent
// TUI via SetStarredTracks). The ctrl+t source toggle keeps working
// orthogonally (Local / Both / Subsonic).
//
// Removal is deferred: F flips a track to unstarred in place and it stays
// visible — without its star — until the view rebuilds (leaving favorites
// mode, switching source, navigating artists/albums). Pressing F again
// before then re-stars it, so an accidental keypress is trivially undone.

// FavoritesOnly reports whether the favorites-only lens is active.
func (l *Library) FavoritesOnly() bool { return l.favoritesOnly }

// ToggleFavorites flips the favorites-only lens. Genres/playlists browse
// modes have no favorites data path, so enabling forces BrowseArtists.
func (l *Library) ToggleFavorites() {
	l.favoritesOnly = !l.favoritesOnly
	l.filterText = ""
	l.filtering = false
	l.baseTracks = nil
	l.favNotice = ""
	l.focusPane = FocusArtists
	l.artistCursor = 0
	l.artistScrollOffset = 0
	l.albumCursor = 0
	l.albumScrollOffset = 0
	l.trackCursor = 0
	l.trackScrollOffset = 0
	if l.favoritesOnly {
		l.browseMode = BrowseArtists
		l.filteredArtists = nil
		l.loadFavoriteLocal()
		l.maybeRequestStarred()
		l.rebuildFavorites()
	} else {
		l.rebuildDisplay()
		l.LoadAlbumsForArtist()
	}
}

// exitFavorites leaves favorites mode without changing the browse mode.
// Used when the user presses g: favorites and genre/playlist browsing are
// mutually exclusive, and exiting is less surprising than cycling away.
func (l *Library) exitFavorites() {
	if !l.favoritesOnly {
		return
	}
	l.favoritesOnly = false
	l.filterText = ""
	l.filtering = false
	l.baseTracks = nil
	l.favNotice = ""
	l.rebuildDisplay()
	l.LoadAlbumsForArtist()
}

// DisableFavorites is used when the modal opens: a fresh open always starts
// in the normal view rather than restoring a stale favorites lens.
func (l *Library) DisableFavorites() {
	l.favoritesOnly = false
	l.favNotice = ""
}

func (l *Library) loadFavoriteLocal() {
	l.favLocalTracks = nil
	if l.db == nil {
		return
	}
	tracks, err := l.db.FavoriteTracks()
	if err != nil {
		l.favNotice = "Couldn't load local favorites"
		return
	}
	for _, t := range tracks {
		t.Starred = true
		l.favLocalTracks = append(l.favLocalTracks, t)
	}
}

func (l *Library) maybeRequestStarred() {
	if l.source == SearchLocal {
		return
	}
	l.PendingFetchStarred = true
}

// SetStarredTracks answers a getStarred2 fetch kicked off via
// PendingFetchStarred. Tracks stay in the display cache with their live
// Starred flag (see SetTrackStarred), so an unstarred-then-rebuilt view
// reflects server truth.
func (l *Library) SetStarredTracks(tracks []models.Track, err error) {
	l.PendingFetchStarred = false
	if err != nil {
		if len(l.favLocalTracks) == 0 || l.source == SearchSubsonic {
			l.favNotice = "Couldn't load server favorites"
		}
		return
	}
	l.favSubTracks = nil
	for _, t := range tracks {
		t.Starred = true
		l.favSubTracks = append(l.favSubTracks, t)
	}
	if l.favoritesOnly {
		l.rebuildFavorites()
	}
}

// SetTrackStarred flips the Starred flag on every cached and visible copy of
// a track. Visible rows are deliberately not rebuilt, so an F-unstarred
// track stays on screen (starless) until the next rebuild — pressing F again
// re-stars it.
func (l *Library) SetTrackStarred(path, remoteID string, source models.TrackSource, starred bool) {
	flip := func(t *models.Track) {
		if t.Path == path && (source != models.SourceSubsonic || t.RemoteID == remoteID) {
			t.Starred = starred
		}
	}
	for i := range l.favLocalTracks {
		flip(&l.favLocalTracks[i])
	}
	for i := range l.favSubTracks {
		flip(&l.favSubTracks[i])
	}
	for i := range l.albumTracks {
		flip(&l.albumTracks[i])
	}
}

func favArtistName(t models.Track) string {
	if t.AlbumArtist != "" {
		return t.AlbumArtist
	}
	return t.Artist
}

func (l *Library) favSources() (local, sub []models.Track) {
	if l.source != SearchSubsonic {
		local = l.favLocalTracks
	}
	if l.source != SearchLocal {
		sub = l.favSubTracks
	}
	return local, sub
}

func (l *Library) rebuildFavorites() {
	local, sub := l.favSources()
	var artists []artistDisplay
	seenLocal := make(map[string]bool)
	seenSub := make(map[string]bool)
	for _, t := range local {
		name := favArtistName(t)
		if name == "" || seenLocal[strings.ToLower(name)] {
			continue
		}
		seenLocal[strings.ToLower(name)] = true
		artists = append(artists, artistDisplay{Name: name})
	}
	for _, t := range sub {
		name := favArtistName(t)
		if name == "" || seenSub[strings.ToLower(name)] {
			continue
		}
		seenSub[strings.ToLower(name)] = true
		artists = append(artists, artistDisplay{Name: name, IsSubsonic: true})
	}
	sort.Slice(artists, func(i, j int) bool {
		return strings.ToLower(artists[i].Name) < strings.ToLower(artists[j].Name)
	})
	l.allArtists = artists
	l.artists = artists
	l.filteredArtists = nil
	l.artistCursor = 0
	l.artistScrollOffset = 0
	l.albumCursor = 0
	l.albumScrollOffset = 0
	l.trackCursor = 0
	l.trackScrollOffset = 0
	if len(l.artists) > 0 {
		l.loadFavoriteAlbums()
	} else {
		l.albums = nil
		l.allAlbums = nil
		l.albumTracks = nil
	}
}

// loadFavoriteAlbums fills the album pane from the favorites caches for the
// selected artist row. Origin (local vs Subsonic) follows the row, so the
// two worlds never merge into one album entry.
func (l *Library) loadFavoriteAlbums() {
	if len(l.artists) == 0 || l.artistCursor >= len(l.artists) {
		l.albums = nil
		l.allAlbums = nil
		l.albumTracks = nil
		return
	}
	entry := l.artists[l.artistCursor]
	var pool []models.Track
	if entry.IsSubsonic {
		_, pool = l.favSources()
	} else {
		pool, _ = l.favSources()
	}
	seen := make(map[string]bool)
	var albums []models.AlbumEntry
	years := make(map[string]int)
	for _, t := range pool {
		if !strings.EqualFold(favArtistName(t), entry.Name) || t.Album == "" {
			continue
		}
		key := strings.ToLower(t.Album)
		if t.Year > years[key] {
			years[key] = t.Year
		}
		if !seen[key] {
			seen[key] = true
			albums = append(albums, models.AlbumEntry{Name: t.Album})
		}
	}
	sort.Slice(albums, func(i, j int) bool {
		return strings.ToLower(albums[i].Name) < strings.ToLower(albums[j].Name)
	})
	for i := range albums {
		albums[i].Year = years[strings.ToLower(albums[i].Name)]
	}
	l.allAlbums = albums
	l.albums = albums
	l.filteredAlbums = nil
	l.albumCursor = 0
	l.albumScrollOffset = 0
	l.loadFavoriteTracks()
}

// loadFavoriteTracks fills the track pane from the favorites caches for the
// selected artist/album. Entries keep their live Starred flag: recently
// F-unstarred tracks remain visible (starless) until the next rebuild.
func (l *Library) loadFavoriteTracks() {
	if len(l.artists) == 0 || l.artistCursor >= len(l.artists) ||
		len(l.albums) == 0 || l.albumCursor >= len(l.albums) {
		l.albumTracks = nil
		l.trackCursor = 0
		l.trackScrollOffset = 0
		return
	}
	artistEntry := l.artists[l.artistCursor]
	albumEntry := l.albums[l.albumCursor]
	var pool []models.Track
	if artistEntry.IsSubsonic {
		_, pool = l.favSources()
	} else {
		pool, _ = l.favSources()
	}
	var tracks []models.Track
	for _, t := range pool {
		if strings.EqualFold(favArtistName(t), artistEntry.Name) &&
			strings.EqualFold(t.Album, albumEntry.Name) {
			tracks = append(tracks, t)
		}
	}
	sort.Slice(tracks, func(i, j int) bool {
		if tracks[i].DiscNum != tracks[j].DiscNum {
			return tracks[i].DiscNum < tracks[j].DiscNum
		}
		return tracks[i].TrackNum < tracks[j].TrackNum
	})
	l.albumTracks = tracks
	l.trackCursor = 0
	l.trackScrollOffset = 0
}
