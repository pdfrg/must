package widgets

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/models"
)

func TestProgressDisplayModesAndThemeTrackColor(t *testing.T) {
	styles := &config.ThemeStyles{}
	nowPlaying := NewNowPlaying(styles, "#112233", "#445566", "#778899")
	nowPlaying.SetWidth(60)
	data := NowPlayingData{
		Track:          &models.Track{Duration: 185},
		TimePos:        65,
		PlaylistPos:    2,
		PlaylistLength: 10,
	}

	if got := nowPlaying.progress.EmptyColor; got != lipgloss.Color("#778899") {
		t.Fatalf("progress track color = %v, want theme muted color", got)
	}
	if got := nowPlaying.progress.FullColor; got != lipgloss.Color("#112233") {
		t.Fatalf("progress fill color = %v, want theme accent color", got)
	}
	if nowPlaying.progress.Full != '━' || nowPlaying.progress.Empty != '━' {
		t.Fatalf("progress characters = %q/%q, want a consistent timeline", nowPlaying.progress.Full, nowPlaying.progress.Empty)
	}
	if got := nowPlaying.formatProgressTime(data); got != "01:05 / 03:05 (35%)" {
		t.Fatalf("all progress = %q", got)
	}

	nowPlaying.SetDisplayOptions(true, config.ProgressSimple)
	if got := nowPlaying.formatProgressTime(data); got != "01:05 / 03:05" {
		t.Fatalf("simple progress = %q", got)
	}

	nowPlaying.SetDisplayOptions(false, config.ProgressRemaining)
	if got := nowPlaying.formatProgressTime(data); got != "-02:00" {
		t.Fatalf("remaining progress = %q", got)
	}
	view := nowPlaying.View(NowPlayingData{
		Track:          &models.Track{Title: "Song", Duration: 185},
		AudioInfo:      &models.AudioInfo{Codec: "FLAC"},
		TimePos:        65,
		PlaylistPos:    2,
		PlaylistLength: 10,
	})
	if strings.Contains(view, "FLAC") {
		t.Fatalf("encoding details shown when disabled:\n%s", view)
	}
	if !strings.Contains(view, "-02:00") || !strings.Contains(view, "track 2 of 10") {
		t.Fatalf("remaining/track layout missing:\n%s", view)
	}
	details := nowPlaying.progressDetailsLine(data)
	if got, want := lipgloss.Width(details), nowPlaying.progress.Width()+1; got != want {
		t.Fatalf("progress details width = %d, want %d to match indented timeline", got, want)
	}
	if !strings.HasSuffix(details, "track 2 of 10") {
		t.Fatalf("track count is not aligned at the end of progress details: %q", details)
	}
}

func TestPlaylistColumnsAreConfigurable(t *testing.T) {
	playlist := NewPlaylist(&config.ThemeStyles{})
	playlist.SetColumns([]string{"title", "album", "duration"})
	playlist.SetSize(80, 4)
	playlist.SetRows([]TrackRow{{Track: models.Track{
		Title:    "Song",
		Artist:   "Hidden Artist",
		Album:    "Album",
		Year:     2026,
		Duration: 90,
	}}})

	view := playlist.View()
	header := strings.Split(view, "\n")[0]
	if !strings.Contains(header, "Song") || !strings.Contains(header, "Album") || !strings.Contains(header, "Time") {
		t.Fatalf("configured headers missing: %q", header)
	}
	if strings.Contains(header, "Artist") || strings.Contains(header, "Year") || strings.Contains(view, "Hidden Artist") || strings.Contains(view, "2026") {
		t.Fatalf("disabled playlist columns still rendered:\n%s", view)
	}
}
