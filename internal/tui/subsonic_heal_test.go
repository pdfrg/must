package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pdfrg/must/internal/api"
	"github.com/pdfrg/must/internal/models"
)

func TestHealSubsonicTracks(t *testing.T) {
	const (
		oldID = "e3b7fc2ae9447bbec37a13bf916e3cf6"
		newID = "6VHl3uR4kss6sUPKA8Cwnk"
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("id") != newID {
			_, _ = w.Write([]byte(`{"subsonic-response":{"status":"failed","version":"1.16.1","error":{"code":70,"message":"Song not found"}}}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"subsonic-response": map[string]any{
				"status":  "ok",
				"version": "1.16.1",
				"song":    map[string]any{"id": newID, "title": "Healed", "artist": "Artist", "albumId": newID},
			},
		})
	}))
	defer srv.Close()

	client, err := api.NewSubsonicClient(srv.URL, "u", "p", "Navidrome", "ND")
	if err != nil {
		t.Fatal(err)
	}
	m := Model{subsonicClient: client}

	tracks := []models.Track{
		{Path: client.StreamURL(oldID), Title: "stream.view?id=" + oldID},
		{Path: "/music/local.flac", Title: "Local"},
	}
	got := m.healSubsonicTracks(tracks)

	if got[0].Source != models.SourceSubsonic || got[0].RemoteID != newID {
		t.Errorf("healed track = {source:%q remoteID:%q}, want subsonic/%q", got[0].Source, got[0].RemoteID, newID)
	}
	if got[0].Title != "Healed" {
		t.Errorf("healed title = %q, want Healed", got[0].Title)
	}
	if got[1].Path != "/music/local.flac" || got[1].Title != "Local" {
		t.Errorf("local track was modified: %+v", got[1])
	}
}
