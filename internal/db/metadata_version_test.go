package db

import (
	"testing"

	"github.com/pdfrg/must/internal/models"
)

func TestMetadataScanVersionSchedulesOneTimeRescan(t *testing.T) {
	ld := newTestDB(t)

	version, err := ld.MetadataScanVersion()
	if err != nil {
		t.Fatalf("MetadataScanVersion failed: %v", err)
	}
	if version != 0 {
		t.Fatalf("new database metadata scan version = %d, want 0", version)
	}

	if _, err := ld.InsertTrack(&models.Track{
		Path:        "/music/artist/album/song.m4a",
		Title:       "song.m4a",
		FileModTime: 1234,
	}); err != nil {
		t.Fatalf("InsertTrack failed: %v", err)
	}

	affected, err := ld.ResetTrackModTimes()
	if err != nil {
		t.Fatalf("ResetTrackModTimes failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("ResetTrackModTimes affected %d tracks, want 1", affected)
	}
	track, err := ld.GetTrackByPath("/music/artist/album/song.m4a")
	if err != nil {
		t.Fatalf("GetTrackByPath failed: %v", err)
	}
	if track == nil || track.FileModTime != 0 {
		t.Fatalf("track after reset = %#v, want file_mod_time 0", track)
	}

	if err := ld.SetMetadataScanVersion(1); err != nil {
		t.Fatalf("SetMetadataScanVersion failed: %v", err)
	}
	if err := ld.SetMetadataScanVersion(2); err != nil {
		t.Fatalf("SetMetadataScanVersion update failed: %v", err)
	}
	version, err = ld.MetadataScanVersion()
	if err != nil {
		t.Fatalf("MetadataScanVersion after update failed: %v", err)
	}
	if version != 2 {
		t.Fatalf("metadata scan version = %d, want 2", version)
	}
}
