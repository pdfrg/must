package modals

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/models"
)

func favTestStyles() *config.ThemeStyles {
	return config.NewThemeStyles(config.DefaultTheme(), false, false, config.TerminalPaletteConfig{})
}

func TestRebuildFavoritesGroupsByArtistAndAlbum(t *testing.T) {
	l := NewLibrary(favTestStyles(), nil)
	l.source = SearchBoth
	l.favoritesOnly = true
	l.favLocalTracks = []models.Track{
		{Path: "/a1.mp3", Title: "One", Artist: "Artist A", Album: "Album X", TrackNum: 1, Starred: true},
		{Path: "/a2.mp3", Title: "Two", Artist: "Artist A", Album: "Album X", TrackNum: 2, Starred: true},
	}
	l.favSubTracks = []models.Track{
		{Source: models.SourceSubsonic, RemoteID: "s1", Title: "Three", Artist: "Artist B", Album: "Album Y", TrackNum: 1, Starred: true},
	}
	l.rebuildFavorites()

	if len(l.artists) != 2 || l.artists[0].Name != "Artist A" || l.artists[1].Name != "Artist B" {
		t.Fatalf("artists = %+v, want [Artist A Artist B]", l.artists)
	}
	if !l.artists[1].IsSubsonic || l.artists[0].IsSubsonic {
		t.Fatalf("origin flags wrong: %+v", l.artists)
	}
	if len(l.albums) != 1 || l.albums[0].Name != "Album X" {
		t.Fatalf("albums = %+v, want [Album X]", l.albums)
	}
	if len(l.albumTracks) != 2 {
		t.Fatalf("albumTracks = %d, want 2", len(l.albumTracks))
	}

	// Deferred removal: unstar keeps the row visible but starless.
	l.SetTrackStarred("/a1.mp3", "", "", false)
	if len(l.albumTracks) != 2 || l.albumTracks[0].Starred {
		t.Fatalf("unstarred track dropped or still starred: %+v", l.albumTracks)
	}
	// And re-starring restores the flag in place.
	l.SetTrackStarred("/a1.mp3", "", "", true)
	if !l.albumTracks[0].Starred {
		t.Fatalf("re-star did not restore flag: %+v", l.albumTracks)
	}
}

func TestRebuildFavoritesRespectsSource(t *testing.T) {
	l := NewLibrary(favTestStyles(), nil)
	l.favLocalTracks = []models.Track{
		{Path: "/a.mp3", Title: "One", Artist: "Artist A", Album: "X", Starred: true},
	}
	l.favSubTracks = []models.Track{
		{Source: models.SourceSubsonic, RemoteID: "s1", Title: "Two", Artist: "Artist B", Album: "Y", Starred: true},
	}
	l.favoritesOnly = true

	l.source = SearchLocal
	l.rebuildFavorites()
	if len(l.artists) != 1 || l.artists[0].Name != "Artist A" {
		t.Fatalf("local-only artists = %+v", l.artists)
	}
	l.source = SearchSubsonic
	l.rebuildFavorites()
	if len(l.artists) != 1 || l.artists[0].Name != "Artist B" {
		t.Fatalf("subsonic-only artists = %+v", l.artists)
	}
}

func TestTrackColumnShowsStar(t *testing.T) {
	l := NewLibrary(favTestStyles(), nil)
	l.albumTracks = []models.Track{
		{TrackNum: 1, Title: "Fav", Starred: true},
		{TrackNum: 2, Title: "Plain"},
	}
	got := ansi.Strip(l.renderTrackColumn(40, 5))
	if !strings.Contains(got, "⭐ Fav") {
		t.Fatalf("starred track missing star: %q", got)
	}
	if strings.Contains(got, "⭐ Plain") {
		t.Fatalf("plain track wrongly starred: %q", got)
	}
}
