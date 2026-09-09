package config

import (
	"reflect"
	"testing"
)

func TestDisplayConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.ShowEncodingDetails {
		t.Fatal("ShowEncodingDetails default = false, want true")
	}
	if cfg.ProgressDisplay != ProgressAll {
		t.Fatalf("ProgressDisplay default = %q, want %q", cfg.ProgressDisplay, ProgressAll)
	}
	if !reflect.DeepEqual(cfg.PlaylistColumns, DefaultPlaylistColumns) {
		t.Fatalf("PlaylistColumns default = %v, want %v", cfg.PlaylistColumns, DefaultPlaylistColumns)
	}
}

func TestDisplayConfigValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ProgressDisplay = "invalid"
	cfg.PlaylistColumns = []string{"title", "year", "title", "invalid", "duration"}
	cfg.applyDefaults()

	if cfg.ProgressDisplay != ProgressAll {
		t.Fatalf("invalid ProgressDisplay normalized to %q, want %q", cfg.ProgressDisplay, ProgressAll)
	}
	wantColumns := []string{"title", "year", "duration"}
	if !reflect.DeepEqual(cfg.PlaylistColumns, wantColumns) {
		t.Fatalf("PlaylistColumns normalized to %v, want %v", cfg.PlaylistColumns, wantColumns)
	}
}
