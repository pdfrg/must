package tui

import (
	"testing"
	"time"

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
