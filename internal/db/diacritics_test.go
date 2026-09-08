package db

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/pdfrg/must/internal/models"
)

// Regression coverage for diacritic-insensitive FTS search.
// The schema pins tokenize='porter unicode61 remove_diacritics 1' so that
// ASCII-folded queries (beyonce) match diacritic-containing tags (Beyoncé).
// Current bundled SQLite already folds by default, so these tests guard the
// pin rather than catching a live bug.
func newTestDB(t *testing.T) *LibraryDB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	ld := &LibraryDB{db: sqlDB}
	if err := ld.initSchema(); err != nil {
		t.Fatalf("initSchema failed: %v", err)
	}
	return ld
}

func seedDiacriticTracks(t *testing.T, ld *LibraryDB) {
	t.Helper()
	tracks := []*models.Track{
		{Path: "/test/beyonce.mp3", Title: "Halo", Artist: "Beyoncé", Album: "I Am..."},
		{Path: "/test/sigur.mp3", Title: "Hoppípolla", Artist: "Sigur Rós", Album: "Takk..."},
		{Path: "/test/bjork.mp3", Title: "Jóga", Artist: "Björk", Album: "Homogenic"},
		{Path: "/test/control.mp3", Title: "Control Song", Artist: "Control Artist", Album: "Control Album"},
	}
	for _, tr := range tracks {
		if _, err := ld.InsertTrack(tr); err != nil {
			t.Fatalf("failed to insert %s: %v", tr.Path, err)
		}
	}
}

func TestFTSSchemaPinsRemoveDiacritics(t *testing.T) {
	ld := newTestDB(t)
	var sqlText string
	err := ld.db.QueryRow(`SELECT sql FROM sqlite_master WHERE name='tracks_fts'`).Scan(&sqlText)
	if err != nil {
		t.Fatalf("failed to read tracks_fts schema: %v", err)
	}
	if !strings.Contains(sqlText, "remove_diacritics") {
		t.Fatalf("tracks_fts schema lacks remove_diacritics pin:\n%s", sqlText)
	}
}

func TestLikeSearchDiacriticFolding(t *testing.T) {
	ld := newTestDB(t)
	seedDiacriticTracks(t, ld)

	artists, err := ld.SearchArtistsLike("beyonce")
	if err != nil {
		t.Fatalf("SearchArtistsLike failed: %v", err)
	}
	if len(artists) != 1 || artists[0] != "Beyoncé" {
		t.Errorf("SearchArtistsLike(%q) = %v, want [Beyoncé]", "beyonce", artists)
	}

	// Accented input still matches, and plain-ASCII rows are unaffected.
	artists, err = ld.SearchArtistsLike("beyoncé")
	if err != nil {
		t.Fatalf("SearchArtistsLike failed: %v", err)
	}
	if len(artists) != 1 || artists[0] != "Beyoncé" {
		t.Errorf("SearchArtistsLike(%q) = %v, want [Beyoncé]", "beyoncé", artists)
	}

	artists, err = ld.SearchArtistsLike("control")
	if err != nil {
		t.Fatalf("SearchArtistsLike failed: %v", err)
	}
	if len(artists) != 1 || artists[0] != "Control Artist" {
		t.Errorf("SearchArtistsLike(%q) = %v, want [Control Artist]", "control", artists)
	}

	artists, err = ld.SearchArtistsLike("xyzzy")
	if err != nil {
		t.Fatalf("SearchArtistsLike failed: %v", err)
	}
	if len(artists) != 0 {
		t.Errorf("SearchArtistsLike(%q) = %v, want []", "xyzzy", artists)
	}

	// Album search folds too (album titles, not just artist names).
	if _, err := ld.InsertTrack(&models.Track{Path: "/test/cafe.mp3", Title: "La Ingrata", Artist: "Café Tacvba", Album: "Café", Genre: "Rock"}); err != nil {
		t.Fatalf("failed to insert café track: %v", err)
	}
	albums, err := ld.SearchAlbumsLike("cafe")
	if err != nil {
		t.Fatalf("SearchAlbumsLike failed: %v", err)
	}
	found := false
	for _, a := range albums {
		if a.Album == "Café" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("SearchAlbumsLike(%q) missing Café, got %v", "cafe", albums)
	}
}

func TestFTSDiacriticFolding(t *testing.T) {
	ld := newTestDB(t)
	seedDiacriticTracks(t, ld)

	cases := []struct {
		query  string
		artist string // expected artist in results
	}{
		{"beyonce", "Beyoncé"},     // ASCII-folded query finds diacritic tag
		{"beyoncé", "Beyoncé"},     // exact-diacritic input still matches
		{"BEYONCE", "Beyoncé"},     // case-insensitive + folded
		{"sigur ros", "Sigur Rós"}, // multi-token folding
		{"bjork", "Björk"},
		{"beyonc*", "Beyoncé"}, // prefix search unaffected
	}
	for _, tc := range cases {
		results, err := ld.SearchFTS(tc.query)
		if err != nil {
			t.Fatalf("SearchFTS(%q) failed: %v", tc.query, err)
		}
		found := false
		for _, r := range results {
			if r.Artist == tc.artist {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SearchFTS(%q): expected artist %q in results, got %v", tc.query, tc.artist, results)
		}
	}

	// Control: unrelated query stays empty.
	results, err := ld.SearchFTS("xyzzy")
	if err != nil {
		t.Fatalf("SearchFTS control query failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("SearchFTS(%q): expected 0 results, got %d", "xyzzy", len(results))
	}
}
