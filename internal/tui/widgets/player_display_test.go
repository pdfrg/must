package widgets

import (
	"strings"
	"testing"

	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/models"
)

func TestNowPlayingUsesConfiguredCompactDetails(t *testing.T) {
	styles := &config.ThemeStyles{}
	player := NewNowPlaying(styles, "#89b4fa", "#cdd6f4", "#6c7086")
	player.SetWidth(80)
	player.SetDisplayOptions(false, config.ProgressRemaining)

	view := player.View(NowPlayingData{
		Track:          &models.Track{Title: "Wasteland", Duration: 230},
		AudioInfo:      &models.AudioInfo{Codec: "aac", Bitrate: 291, SampleRate: 44100},
		TimePos:        20,
		PlaylistPos:    1,
		PlaylistLength: 2,
	})

	for _, want := range []string{"━━━━━━━━", "-03:30", "track 1 of 2"} {
		if !strings.Contains(view, want) {
			t.Fatalf("player view does not contain %q:\n%s", want, view)
		}
	}
	for _, hidden := range []string{"aac", "291k", "44100Hz"} {
		if strings.Contains(view, hidden) {
			t.Fatalf("player view unexpectedly contains %q:\n%s", hidden, view)
		}
	}
}

func TestPlaylistUsesConfiguredColumns(t *testing.T) {
	styles := &config.ThemeStyles{Background: "#1e1e2e"}
	playlist := NewPlaylist(styles)
	playlist.SetSize(100, 4)
	playlist.SetColumns([]string{"position", "title", "artist", "duration"})
	playlist.SetRows(BuildPlaylistRows([]models.Track{{
		Title: "Wasteland", Artist: "10 Years", Album: "The Autumn Effect", Year: 2005, Duration: 230,
	}}, 0))

	view := playlist.View()
	for _, want := range []string{"Song", "Artist", "Time", "Wasteland", "10 Years", "3:50"} {
		if !strings.Contains(view, want) {
			t.Fatalf("playlist view does not contain %q:\n%s", want, view)
		}
	}
	for _, hidden := range []string{"2005", "Album", "The Autumn Effect"} {
		if strings.Contains(view, hidden) {
			t.Fatalf("playlist view unexpectedly contains %q:\n%s", hidden, view)
		}
	}
}
