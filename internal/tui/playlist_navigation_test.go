package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/models"
	"github.com/pdfrg/must/internal/mpv"
	"github.com/pdfrg/must/internal/tui/widgets"
)

func TestMainPlaylistArrowKeysMoveSelection(t *testing.T) {
	tracks := []models.Track{
		{Title: "First"},
		{Title: "Second"},
		{Title: "Third"},
	}
	styles := &config.ThemeStyles{}
	playlistWidget := widgets.NewPlaylist(styles)
	playlistWidget.SetRows(widgets.BuildPlaylistRows(tracks, 0))
	playlistWidget.SetCurrentIndex(0)
	playlistWidget.SetSize(100, 5)

	m := Model{
		keyMap:          DefaultKeyMap,
		layoutCheckDone: true,
		bottomViewMode:  BottomPlaylist,
		playlist:        tracks,
		playlistWidget:  playlistWidget,
		mpvBackend:      mpv.NewMPVBackend(),
		nowPlaying:      widgets.NewNowPlaying(styles, "", "", ""),
		cfg:             config.DefaultConfig(),
	}

	updated, _ := m.handleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m = updated.(Model)
	if got := m.playlistWidget.GetCursor(); got != 1 {
		t.Fatalf("cursor after Down = %d, want 1", got)
	}
	view := m.playlistWidget.View()
	if !strings.Contains(view, "▶") || !strings.Contains(view, "▸") {
		t.Fatalf("playlist view does not distinguish playing and selected rows:\n%s", view)
	}

	updated, cmd := m.handleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updated.(Model)
	if m.currentIndex != 1 || cmd == nil {
		t.Fatalf("Enter selected index/cmd = %d/%v, want 1/non-nil", m.currentIndex, cmd)
	}

	updated, _ = m.handleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	m = updated.(Model)
	if got := m.playlistWidget.GetCursor(); got != 0 {
		t.Fatalf("cursor after Up = %d, want 0", got)
	}
}
