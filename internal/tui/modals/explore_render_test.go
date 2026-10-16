package modals

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pdfrg/must/internal/config"
)

func testExploreStyles() *config.ThemeStyles {
	return &config.ThemeStyles{
		ForegroundStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")),
		AccentStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")),
		CursorStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("#0000ff")),
		MutedStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("#777777")),
	}
}

func sampleExploreAlbums(n int) []ExploreAlbum {
	albums := make([]ExploreAlbum, 0, n)
	for i := 0; i < n; i++ {
		albums = append(albums, ExploreAlbum{
			Name:   fmt.Sprintf("Album %02d", i),
			Artist: fmt.Sprintf("Artist %02d", i),
			Year:   1990 + i,
			Source: ExploreLocal,
		})
	}
	return albums
}

func TestExploreVisibleLimitClamps(t *testing.T) {
	e := NewExplore(testExploreStyles(), 2.0, 0, 0)
	e.SetSize(120, 40)
	if got := e.VisibleLimit(); got != 20 {
		t.Fatalf("large terminal limit = %d, want 20", got)
	}
	e.SetSize(100, 14)
	if got := e.VisibleLimit(); got != 6 {
		t.Fatalf("small terminal limit = %d, want 6", got)
	}
	e.SetSize(100, 8)
	if got := e.VisibleLimit(); got != 6 {
		t.Fatalf("tiny terminal limit = %d, want floor 6", got)
	}
}

func TestExploreViewRendersAllAlbums(t *testing.T) {
	e := NewExplore(testExploreStyles(), 2.0, 0, 0)
	e.SetSize(120, 40)
	albums := sampleExploreAlbums(20)
	e.SetAlbums(albums, ExploreAll,
		map[ExploreSource]int{ExploreAll: 1234, ExploreLocal: 1200, ExploreSubsonic: 30, ExploreTemp: 4},
		[]ExploreSource{ExploreLocal, ExploreSubsonic, ExploreTemp},
	)

	view := e.View()
	plain := ansi.Strip(view)

	for _, a := range albums {
		if !strings.Contains(plain, a.Name) {
			t.Fatalf("view missing %q", a.Name)
		}
	}
	if !strings.Contains(plain, "1234 albums") {
		t.Fatalf("view missing total album count: %q", plain)
	}
	if !strings.Contains(plain, "Subsonic 30") {
		t.Fatalf("view missing per-source count: %q", plain)
	}
	if lines := strings.Split(view, "\n"); len(lines) > 40 {
		t.Fatalf("view is %d lines tall, terminal is 40", len(lines))
	}
}

func TestExploreCursorSelectionAndArtRequest(t *testing.T) {
	e := NewExplore(testExploreStyles(), 2.0, 0, 0)
	e.SetSize(120, 40)
	e.SetAlbums(sampleExploreAlbums(5), ExploreAll, map[ExploreSource]int{ExploreAll: 5}, []ExploreSource{ExploreLocal})

	album, seq := e.TakePendingArt()
	if album == nil || album.Name != "Album 00" {
		t.Fatalf("expected first album art request, got %+v", album)
	}
	if seq == 0 {
		t.Fatal("expected non-zero art request sequence")
	}

	// Move down and confirm a new art request with a bumped sequence.
	e.move(1)
	album2, seq2 := e.TakePendingArt()
	if album2 == nil || album2.Name != "Album 01" {
		t.Fatalf("expected Album 01 art request, got %+v", album2)
	}
	if seq2 <= seq {
		t.Fatalf("art sequence did not advance: %d -> %d", seq, seq2)
	}

	// Moving within bounds at the ends does not panic or wrap.
	e.move(-10)
	if got := e.CurrentAlbum(); got == nil || got.Name != "Album 00" {
		t.Fatalf("expected cursor clamped to first album, got %+v", got)
	}
	e.move(100)
	if got := e.CurrentAlbum(); got == nil || got.Name != "Album 04" {
		t.Fatalf("expected cursor clamped to last album, got %+v", got)
	}
}

func TestExploreSourceCycleSkipsUnavailable(t *testing.T) {
	e := NewExplore(testExploreStyles(), 2.0, 0, 0)
	e.SetSize(120, 40)
	e.SetAlbums(sampleExploreAlbums(3), ExploreAll, map[ExploreSource]int{ExploreAll: 3},
		[]ExploreSource{ExploreLocal, ExploreTemp})

	next, ok := e.nextSource()
	if !ok || next != ExploreLocal {
		t.Fatalf("first cycle = %v, %v; want local", next, ok)
	}
	e.source = ExploreLocal
	next, ok = e.nextSource()
	if !ok || next != ExploreTemp {
		t.Fatalf("second cycle = %v, %v; want temp", next, ok)
	}
	e.source = ExploreTemp
	if next, ok := e.nextSource(); !ok || next != ExploreLocal {
		t.Fatalf("wrap cycle = %v, %v; want local", next, ok)
	}
}

func TestExploreSourceBarKeepsSecondColumnStyled(t *testing.T) {
	e := NewExplore(testExploreStyles(), 2.0, 0, 0)
	e.SetSize(60, 20)
	e.SetAlbums(sampleExploreAlbums(3), ExploreLocal, map[ExploreSource]int{ExploreAll: 3},
		[]ExploreSource{ExploreLocal})

	view := e.View()
	if strings.Contains(view, "NaN") {
		t.Fatalf("unexpected NaN in view: %q", view)
	}
}
