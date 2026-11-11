package modals

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/pdfrg/must/internal/api"
	"github.com/pdfrg/must/internal/models"
)

func TestSubsonicPlaylistsAppearBeforeTracks(t *testing.T) {
	styles := searchTestStyles()
	s := NewSearch(styles, nil)
	s.SetSize(80, 10)
	s.SetEntries([]searchEntry{{
		Kind: resultTrack, IsSubsonic: true,
		Track: models.Track{Artist: "Mac Quayle", Album: "Mr. Robot", Title: "Track"},
	}})

	s.AddSubsonicPlaylists([]api.PlaylistInfo{{
		ID: "playlist-1", Name: "Mr. Robot - Mac Quayle", SongCount: 204,
	}})

	if len(s.entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(s.entries))
	}
	if s.entries[0].Kind != resultPlaylist {
		t.Fatalf("first entry kind = %v, want playlist", s.entries[0].Kind)
	}
	if s.entries[1].Kind != resultTrack {
		t.Fatalf("second entry kind = %v, want track", s.entries[1].Kind)
	}

	view := ansi.Strip(s.View())
	if !strings.Contains(view, "Playlist: Mr. Robot - Mac Quayle (204 tracks)") {
		t.Fatalf("playlist missing from visible results: %q", view)
	}
}
