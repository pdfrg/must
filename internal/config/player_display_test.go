package config

import "testing"

func TestPlayerDisplayDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.DefaultView != "playlist" {
		t.Fatalf("default view = %q, want playlist", cfg.DefaultView)
	}
	if !cfg.ShowEncodingDetails {
		t.Fatal("encoding details should be shown by default")
	}
	if cfg.ProgressDisplay != ProgressAll {
		t.Fatalf("progress display = %q, want %q", cfg.ProgressDisplay, ProgressAll)
	}
	if len(cfg.PlaylistColumns) != len(DefaultPlaylistColumns) {
		t.Fatalf("playlist columns = %v, want %v", cfg.PlaylistColumns, DefaultPlaylistColumns)
	}
}

func TestPlayerDisplayValuesAreNormalized(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultView = "unknown"
	cfg.ProgressDisplay = "unknown"
	cfg.PlaylistColumns = []string{"title", "year", "title", "unknown", "duration"}

	cfg.applyDefaults()

	if cfg.DefaultView != "playlist" || cfg.ProgressDisplay != ProgressAll {
		t.Fatalf("invalid display values were not reset: view=%q progress=%q", cfg.DefaultView, cfg.ProgressDisplay)
	}
	want := []string{"title", "year", "duration"}
	if len(cfg.PlaylistColumns) != len(want) {
		t.Fatalf("playlist columns = %v, want %v", cfg.PlaylistColumns, want)
	}
	for i := range want {
		if cfg.PlaylistColumns[i] != want[i] {
			t.Fatalf("playlist columns = %v, want %v", cfg.PlaylistColumns, want)
		}
	}
}
