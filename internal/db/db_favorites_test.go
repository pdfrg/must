package db

import (
	"testing"

	"github.com/pdfrg/must/internal/models"
)

func TestFavoriteTracksJoinsTracks(t *testing.T) {
	ld := newTestDB(t)
	for _, tr := range []*models.Track{
		{Path: "/a.mp3", Title: "Fav One", Artist: "Artist A", Album: "Album A"},
		{Path: "/b.mp3", Title: "Plain", Artist: "Artist A", Album: "Album A"},
	} {
		if _, err := ld.InsertTrack(tr); err != nil {
			t.Fatalf("insert %s: %v", tr.Path, err)
		}
	}
	if err := ld.SetFavorite("/a.mp3", true); err != nil {
		t.Fatalf("SetFavorite: %v", err)
	}

	got, err := ld.FavoriteTracks()
	if err != nil {
		t.Fatalf("FavoriteTracks: %v", err)
	}
	if len(got) != 1 || got[0].Path != "/a.mp3" || got[0].Title != "Fav One" {
		t.Fatalf("FavoriteTracks = %+v, want only /a.mp3", got)
	}

	if err := ld.SetFavorite("/a.mp3", false); err != nil {
		t.Fatalf("unset: %v", err)
	}
	got, err = ld.FavoriteTracks()
	if err != nil {
		t.Fatalf("FavoriteTracks: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("FavoriteTracks after unstar = %+v, want empty", got)
	}
}
