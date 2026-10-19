package tui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pdfrg/must/internal/api"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/models"
)

func TestHandlePlaybackEndedScrobblesOnNaturalEnd(t *testing.T) {
	m := Model{
		cfg:              &config.Config{},
		playlist:         []models.Track{{Source: models.SourceSubsonic, RemoteID: "tr-1", Title: "Song"}},
		currentIndex:     0,
		repeatMode:       "off",
		playing:          true,
		paused:           false,
		scrobbleEligible: true,
		songStartTime:    time.Now().Add(-2 * time.Minute),
	}

	newModel, cmd := m.handlePlaybackEnded()
	nm := newModel.(Model)

	if nm.playing || nm.paused {
		t.Errorf("expected playback stopped, got playing=%v paused=%v", nm.playing, nm.paused)
	}
	if cmd == nil {
		t.Fatal("expected non-nil scrobble cmd for eligible finished track")
	}
	if nm.prevTrack != nil || nm.prevScrobbleEligible {
		t.Error("expected prev stash cleared after firing pending scrobble")
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("scrobble cmd panicked with nil clients: %v", r)
			}
		}()
		cmd()
	}()
}

func TestHandlePlaybackEndedNoScrobbleWhenIneligible(t *testing.T) {
	m := Model{
		cfg:              &config.Config{},
		playlist:         []models.Track{{Source: models.SourceSubsonic, RemoteID: "tr-1"}},
		currentIndex:     0,
		repeatMode:       "off",
		playing:          true,
		scrobbleEligible: false,
		songStartTime:    time.Now(),
	}

	newModel, cmd := m.handlePlaybackEnded()
	nm := newModel.(Model)

	if nm.playing {
		t.Error("expected playing=false")
	}
	if cmd != nil {
		t.Error("expected nil cmd when track not scrobble-eligible")
	}
	if nm.prevTrack != nil || nm.prevScrobbleEligible {
		t.Error("expected stash cleared even when ineligible")
	}
}

func TestStashCurrentForScrobble(t *testing.T) {
	start := time.Now().Add(-time.Minute)
	m := Model{
		playlist:         []models.Track{{Title: "A"}, {Title: "B"}},
		currentIndex:     1,
		scrobbleEligible: true,
		songStartTime:    start,
	}

	m.stashCurrentForScrobble()

	if m.prevTrack == nil || m.prevTrack.Title != "B" {
		t.Fatalf("expected prevTrack=B, got %+v", m.prevTrack)
	}
	if !m.prevScrobbleEligible {
		t.Error("expected eligibility stashed")
	}
	if !m.prevSongStartTime.Equal(start) {
		t.Errorf("expected start time stashed, got %v", m.prevSongStartTime)
	}

	var oob Model
	oob.playlist = []models.Track{{Title: "X"}}
	oob.currentIndex = 5
	oob.stashCurrentForScrobble()
	if oob.prevTrack != nil {
		t.Error("out-of-range index must not stash")
	}
}

func TestSendNowPlayingReportsToSubsonic(t *testing.T) {
	var requests int
	var gotID, gotSubmission, gotTime string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		q := r.URL.Query()
		gotID = q.Get("id")
		gotSubmission = q.Get("submission")
		gotTime = q.Get("time")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"subsonic-response":{"status":"ok","version":"1.16.1"}}`))
	}))
	defer srv.Close()

	client, err := api.NewSubsonicClient(srv.URL, "u", "p", "Navidrome", "ND")
	if err != nil {
		t.Fatal(err)
	}

	track := models.Track{
		Source:   models.SourceSubsonic,
		RemoteID: "tr-1",
		Artist:   "Artist",
		Title:    "Song",
	}
	msg := sendNowPlayingCmd(&config.Config{}, client, track)()
	if msg != nil {
		t.Errorf("expected nil msg, got %T", msg)
	}

	if requests != 1 {
		t.Fatalf("made %d requests, want 1", requests)
	}
	if gotID != track.RemoteID {
		t.Errorf("id = %q, want %q", gotID, track.RemoteID)
	}
	if gotSubmission != "false" {
		t.Errorf("submission = %q, want false", gotSubmission)
	}
	if gotTime == "" {
		t.Error("expected time parameter to be set")
	}
}

func TestSendNowPlayingSkipsNonSubsonicTracks(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"subsonic-response":{"status":"ok","version":"1.16.1"}}`))
	}))
	defer srv.Close()

	client, err := api.NewSubsonicClient(srv.URL, "u", "p", "Navidrome", "ND")
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	tracks := []models.Track{
		{Source: models.SourceLocal, RemoteID: "local-id"},
		{Source: models.SourceSubsonic, RemoteID: ""},
	}
	for _, track := range tracks {
		sendNowPlayingCmd(cfg, client, track)()
	}
	sendNowPlayingCmd(cfg, nil, models.Track{Source: models.SourceSubsonic, RemoteID: "tr-1"})()

	if requests != 0 {
		t.Errorf("made %d requests, want 0", requests)
	}
}
