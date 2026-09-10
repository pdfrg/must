package modals

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/models"
)

func mouseTestLibrary() *Library {
	library := NewLibrary(&config.ThemeStyles{}, nil)
	library.SetSize(90, 12)
	library.SetMouseFocusOnHover(true)
	library.artists = []artistDisplay{{Name: "Artist 1"}, {Name: "Artist 2"}}
	library.albums = []models.AlbumEntry{{Name: "Album 1"}, {Name: "Album 2"}}
	library.albumTracks = []models.Track{{Title: "Track 1"}, {Title: "Track 2"}}
	return library
}

func TestLibraryMouseHoverFocusesColumns(t *testing.T) {
	library := mouseTestLibrary()
	panes := library.paneBounds()

	library.Update(tea.MouseMotionMsg{X: panes[1].x, Y: libraryContentStartRow})
	if library.focusPane != FocusAlbums {
		t.Fatalf("album hover focus = %v, want FocusAlbums", library.focusPane)
	}

	library.Update(tea.MouseMotionMsg{X: panes[2].x, Y: libraryContentStartRow})
	if library.focusPane != FocusTracks {
		t.Fatalf("track hover focus = %v, want FocusTracks", library.focusPane)
	}

	// Separators and the bars above the lists do not change focus.
	library.Update(tea.MouseMotionMsg{X: panes[2].x - 1, Y: libraryContentStartRow})
	library.Update(tea.MouseMotionMsg{X: panes[0].x, Y: 0})
	if library.focusPane != FocusTracks {
		t.Fatalf("non-pane hover changed focus to %v", library.focusPane)
	}
}

func TestLibraryMouseWheelUsesHoveredColumn(t *testing.T) {
	library := mouseTestLibrary()
	artistPane := library.paneBounds()[0]

	library.Update(tea.MouseWheelMsg{
		X:      artistPane.x,
		Y:      libraryContentStartRow,
		Button: tea.MouseWheelDown,
	})
	if library.focusPane != FocusArtists || library.artistCursor != 1 {
		t.Fatalf("wheel result focus/cursor = %v/%d, want artists/1", library.focusPane, library.artistCursor)
	}
}

func TestLibraryMouseClickSelectsThenPlaysTrack(t *testing.T) {
	library := mouseTestLibrary()
	trackPane := library.paneBounds()[2]
	click := tea.MouseClickMsg{X: trackPane.x, Y: libraryContentStartRow + 1, Button: tea.MouseLeft}

	if cmd := library.Update(click); cmd != nil {
		t.Fatal("first click on an unselected track unexpectedly played it")
	}
	if library.focusPane != FocusTracks || library.trackCursor != 1 {
		t.Fatalf("click result focus/cursor = %v/%d, want tracks/1", library.focusPane, library.trackCursor)
	}

	cmd := library.Update(click)
	if cmd == nil {
		t.Fatal("second click on the selected track did not play it")
	}
	msg, ok := cmd().(LibraryModalMsg)
	if !ok || len(msg.PlayTracks) != 1 || msg.PlayTracks[0].Title != "Track 2" {
		t.Fatalf("second click message = %#v, want Track 2 playback", msg)
	}
}

func TestLibraryMouseTargetsMatchColumnGeometry(t *testing.T) {
	library := mouseTestLibrary()
	panes := library.paneBounds()
	for _, pane := range panes {
		got, _, ok := library.mouseTarget(pane.x, libraryContentStartRow)
		if !ok || got != pane.pane {
			t.Fatalf("mouse target at x=%d = %v/%v, want pane %v", pane.x, got, ok, pane.pane)
		}
	}

	library.browseMode = BrowsePlaylists
	panes = library.paneBounds()
	if len(panes) != 2 || panes[0].pane != FocusArtists || panes[1].pane != FocusTracks {
		t.Fatalf("playlist pane geometry = %#v, want playlists and tracks", panes)
	}
}

func TestLibraryHoverCanBeDisabled(t *testing.T) {
	library := mouseTestLibrary()
	library.SetMouseFocusOnHover(false)
	albumPane := library.paneBounds()[1]
	library.Update(tea.MouseMotionMsg{X: albumPane.x, Y: libraryContentStartRow})
	if library.focusPane != FocusArtists {
		t.Fatalf("disabled hover changed focus to %v", library.focusPane)
	}
}

func TestLibraryMouseWheelThrottle(t *testing.T) {
	library := mouseTestLibrary()
	artistPane := library.paneBounds()[0]
	wheel := func() tea.Msg {
		return tea.MouseWheelMsg{X: artistPane.x, Y: libraryContentStartRow, Button: tea.MouseWheelDown}
	}
	library.Update(wheel())
	library.Update(wheel()) // same instant: throttled
	if library.artistCursor != 1 {
		t.Fatalf("throttled wheel cursor = %d, want 1", library.artistCursor)
	}
}

func TestLibraryMouseClickPlaysAlbumOnSecondClick(t *testing.T) {
	library := mouseTestLibrary()
	albumPane := library.paneBounds()[1]
	click := tea.MouseClickMsg{X: albumPane.x, Y: libraryContentStartRow + 1, Button: tea.MouseLeft}
	if cmd := library.Update(click); cmd != nil {
		t.Fatal("first click on unselected album should only select")
	}
	// Fixture has no DB, so reselecting clears tracks; restore for play check.
	library.albumTracks = []models.Track{{Title: "Track 1"}, {Title: "Track 2"}}
	if cmd := library.Update(click); cmd == nil {
		t.Fatal("second click on selected album should play")
	}
}

func TestLibraryMouseWheelAcceleration(t *testing.T) {
	library := mouseTestLibrary()
	// More artists so a multi-row step has room.
	library.artists = append(library.artists,
		artistDisplay{Name: "Artist 3"}, artistDisplay{Name: "Artist 4"},
		artistDisplay{Name: "Artist 5"}, artistDisplay{Name: "Artist 6"})
	artistPane := library.paneBounds()[0]
	// Simulate a fast flick: 4 events already coalesced as pending.
	library.wheelPending = 4
	library.lastWheelAt = library.lastWheelAt.Add(-time.Hour) // force honor
	library.Update(tea.MouseWheelMsg{X: artistPane.x, Y: libraryContentStartRow, Button: tea.MouseWheelDown})
	if want := 1 + 4/2; library.artistCursor != want {
		t.Fatalf("accelerated wheel cursor = %d, want %d", library.artistCursor, want)
	}
}
