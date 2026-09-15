package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSongHealsLegacyID(t *testing.T) {
	const (
		oldID = "e3b7fc2ae9447bbec37a13bf916e3cf6" // pre-0.64 32-hex
		newID = "6VHl3uR4kss6sUPKA8Cwnk"           // canonical 0.64 base62
	)
	var requested []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		requested = append(requested, id)
		w.Header().Set("Content-Type", "application/json")
		if id != newID {
			_, _ = w.Write([]byte(`{"subsonic-response":{"status":"failed","version":"1.16.1","error":{"code":70,"message":"Song not found"}}}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"subsonic-response": map[string]any{
				"status":  "ok",
				"version": "1.16.1",
				"song":    map[string]any{"id": newID, "title": "Healed"},
			},
		})
	}))
	defer srv.Close()

	c, err := NewSubsonicClient(srv.URL, "u", "p", "Navidrome", "ND")
	if err != nil {
		t.Fatal(err)
	}

	song, err := c.GetSong(oldID)
	if err != nil {
		t.Fatalf("GetSong: %v", err)
	}
	if song.ID != newID {
		t.Errorf("song id = %q, want %q", song.ID, newID)
	}
	if len(requested) != 2 || requested[0] != oldID || requested[1] != newID {
		t.Errorf("requested ids = %v, want [%s %s]", requested, oldID, newID)
	}
}

func TestGetSongDoesNotRetryUnrecognizedID(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"subsonic-response":{"status":"failed","version":"1.16.1","error":{"code":70,"message":"Song not found"}}}`))
	}))
	defer srv.Close()

	c, err := NewSubsonicClient(srv.URL, "u", "p", "Jellyfin", "JF")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := c.GetSong("aB3xY9kQz1"); err == nil {
		t.Fatal("expected error")
	}
	if requests != 1 {
		t.Errorf("made %d requests, want 1 (no retry for unrecognized shape)", requests)
	}
}
