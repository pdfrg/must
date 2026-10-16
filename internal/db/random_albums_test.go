package db

import (
	"testing"

	"github.com/pdfrg/must/internal/models"
)

func TestRandomAlbumsGroupsByArtistAndAlbum(t *testing.T) {
	ld := newTestDB(t)
	tracks := []*models.Track{
		{Path: "/a1.mp3", Title: "One", Artist: "VA", AlbumArtist: "Artist A", Album: "Greatest Hits"},
		{Path: "/a2.mp3", Title: "Two", Artist: "VA", AlbumArtist: "Artist A", Album: "Greatest Hits"},
		{Path: "/b1.mp3", Title: "Three", Artist: "VA", AlbumArtist: "Artist B", Album: "Greatest Hits"},
		{Path: "/c1.mp3", Title: "Four", Artist: "Artist C", Album: "Other"},
		{Path: "/d1.mp3", Title: "Five", Artist: "Artist D", Album: ""},
	}
	for _, tr := range tracks {
		if _, err := ld.InsertTrack(tr); err != nil {
			t.Fatalf("failed to insert %s: %v", tr.Path, err)
		}
	}

	got, err := ld.RandomAlbums(10)
	if err != nil {
		t.Fatalf("RandomAlbums failed: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 distinct albums (same-name albums split by artist), got %d: %+v", len(got), got)
	}

	seen := map[string]bool{}
	for _, e := range got {
		if e.Name == "" {
			t.Fatalf("empty album name returned: %+v", e)
		}
		seen[e.Artist+"\x1f"+e.Name] = true
	}
	if !seen["Artist A\x1fGreatest Hits"] || !seen["Artist B\x1fGreatest Hits"] {
		t.Fatalf("same-named albums from different artists were not both returned: %+v", seen)
	}

	if n, err := ld.AlbumCount(); err != nil || n != 3 {
		t.Fatalf("AlbumCount = %d, err=%v; want 3", n, err)
	}
}

func TestRandomAlbumsRespectsLimit(t *testing.T) {
	ld := newTestDB(t)
	for _, p := range []string{"/1.mp3", "/2.mp3", "/3.mp3"} {
		if _, err := ld.InsertTrack(&models.Track{Path: p, Title: p, Artist: "A", Album: "Album " + p}); err != nil {
			t.Fatalf("insert %s: %v", p, err)
		}
	}
	got, err := ld.RandomAlbums(1)
	if err != nil {
		t.Fatalf("RandomAlbums: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 album, got %d", len(got))
	}
	if got, err := ld.RandomAlbums(0); err != nil || got != nil {
		t.Fatalf("limit 0 should return nil,nil; got %v,%v", got, err)
	}
}
