package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/models"
	"github.com/pdfrg/must/internal/tui/widgets"
)

func TestAutomaticLayoutProfiles(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
		wantMode      string
		wantStacked   bool
		wantBottomMin int
	}{
		{name: "small", width: 80, height: 24, wantMode: "medium", wantBottomMin: 3},
		{name: "medium", width: 120, height: 32, wantMode: "large", wantBottomMin: 3},
		{name: "wide", width: 160, height: 24, wantMode: "medium", wantBottomMin: 3},
		{name: "tall", width: 120, height: 48, wantMode: "narrow", wantStacked: true, wantBottomMin: 3},
		{name: "tiny", width: 40, height: 12, wantMode: "compact"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := planLayout(tt.width, tt.height, layoutPreferences{
				Requested: "auto", ShowHeader: true, ShowFooter: true,
				ShowArtwork: true, ShowBottom: true, NowPlayingRows: 12, CellRatio: 2,
			})
			if plan.Mode != tt.wantMode {
				t.Fatalf("mode = %q, want %q", plan.Mode, tt.wantMode)
			}
			if plan.ArtworkStacked != tt.wantStacked {
				t.Fatalf("stacked artwork = %v, want %v", plan.ArtworkStacked, tt.wantStacked)
			}
			if plan.Bottom.Height < tt.wantBottomMin {
				t.Fatalf("bottom height = %d, want at least %d", plan.Bottom.Height, tt.wantBottomMin)
			}
		})
	}
}

func TestLayoutHonorsExplicitPreferences(t *testing.T) {
	prefs := layoutPreferences{
		Requested: "medium", ShowBottom: true, NowPlayingRows: 12, CellRatio: 2,
	}
	plan := planLayout(120, 48, prefs)
	if plan.Mode != "medium" || !plan.Bottom.Empty() {
		t.Fatalf("explicit medium preference not honored: %+v", plan)
	}

	prefs.Requested = "large"
	prefs.ShowArtwork = true
	plan = planLayout(120, 48, prefs)
	if plan.Artwork.Empty() || plan.Bottom.Empty() || !plan.ArtworkStacked {
		t.Fatalf("explicit large layout should retain features and adapt: %+v", plan)
	}
}

func TestLayoutRectanglesStayWithinTerminal(t *testing.T) {
	for width := 20; width <= 200; width += 7 {
		for height := 8; height <= 60; height += 3 {
			plan := planLayout(width, height, layoutPreferences{
				Requested: "auto", ShowHeader: true, ShowFooter: true,
				ShowArtwork: true, ShowBottom: true, NowPlayingRows: 12, CellRatio: 2,
			})
			for name, rect := range map[string]Rect{
				"header": plan.Header, "now playing": plan.NowPlaying, "artwork": plan.Artwork,
				"bottom": plan.Bottom, "footer": plan.Footer,
			} {
				if rect.Empty() {
					continue
				}
				if rect.X < 0 || rect.Y < 0 || rect.Right() > width || rect.Bottom() > height {
					t.Fatalf("%dx%d %s out of bounds: %+v", width, height, name, rect)
				}
			}
			if !plan.Bottom.Empty() && !plan.Footer.Empty() && plan.Bottom.Bottom() > plan.Footer.Y {
				t.Fatalf("%dx%d bottom overlaps footer: bottom=%+v footer=%+v", width, height, plan.Bottom, plan.Footer)
			}
			if !plan.Artwork.Empty() && !plan.ArtworkStacked && plan.NowPlaying.Right() > plan.Artwork.X {
				t.Fatalf("%dx%d horizontal panes overlap: now=%+v art=%+v", width, height, plan.NowPlaying, plan.Artwork)
			}
		}
	}
}

func TestUltraWideMiniVisualizerStaysWithPlayer(t *testing.T) {
	plan := planLayout(200, 50, layoutPreferences{
		Requested: "auto", ShowArtwork: true, ShowBottom: true,
		CompactBottom: true, NowPlayingRows: 12, CellRatio: 2,
	})
	if plan.NowPlaying.X < 30 || plan.Artwork.Right() > 172 {
		t.Fatalf("player stage is not centered/constrained: now=%+v art=%+v", plan.NowPlaying, plan.Artwork)
	}
	if plan.Bottom.Height != 6 || plan.Bottom.Width > maximumPlayerWidth {
		t.Fatalf("mini visualizer is not bounded: %+v", plan.Bottom)
	}
	if plan.Bottom.X != plan.NowPlaying.X || plan.Bottom.Y != max(plan.NowPlaying.Bottom(), plan.Artwork.Bottom())+1 {
		t.Fatalf("mini visualizer is detached: bottom=%+v now=%+v art=%+v", plan.Bottom, plan.NowPlaying, plan.Artwork)
	}
	spaceAbove := plan.NowPlaying.Y
	spaceBelow := plan.Height - plan.Bottom.Bottom()
	if difference(spaceAbove, spaceBelow) > 2 {
		t.Fatalf("player group is vertically unbalanced: above=%d below=%d plan=%+v", spaceAbove, spaceBelow, plan)
	}
}

func TestPlayerAndTableShareContentFrame(t *testing.T) {
	plan := planLayout(202, 51, layoutPreferences{
		Requested: "auto", ShowArtwork: true, ShowBottom: true,
		BottomRows: 2, NowPlayingRows: 8, CellRatio: 2,
	})
	if plan.Bottom.X != plan.NowPlaying.X || plan.Bottom.Width != maximumPlayerWidth {
		t.Fatalf("bottom section and player do not share a frame: bottom=%+v now=%+v", plan.Bottom, plan.NowPlaying)
	}
	if plan.Bottom.Height != 2 {
		t.Fatalf("one-track table height = %d, want its natural two rows", plan.Bottom.Height)
	}
	if plan.Artwork.Right() != plan.Bottom.Right() {
		t.Fatalf("artwork does not align to frame right edge: art=%+v bottom=%+v", plan.Artwork, plan.Bottom)
	}
	spaceAbove := plan.NowPlaying.Y
	spaceBelow := plan.Height - plan.Bottom.Bottom()
	if difference(spaceAbove, spaceBelow) > 1 {
		t.Fatalf("short player/table group is not vertically centered: above=%d below=%d plan=%+v", spaceAbove, spaceBelow, plan)
	}
}

func TestLongTableUsesAvailableHeight(t *testing.T) {
	plan := planLayout(202, 51, layoutPreferences{
		Requested: "auto", ShowArtwork: true, ShowBottom: true,
		BottomRows: 101, NowPlayingRows: 8, CellRatio: 2,
	})
	if plan.NowPlaying.Y != 1 {
		t.Fatalf("scrollable composition should remain top-aligned: %+v", plan)
	}
	if plan.Bottom.Bottom() != plan.Height {
		t.Fatalf("long table should use all available rows: %+v", plan.Bottom)
	}
}

func difference(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

func TestResponsiveViewFitsRepresentativeTerminalSizes(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ShowAlbumArt = false
	styles := &config.ThemeStyles{}
	tracks := []models.Track{{
		Title:  "A deliberately long track title that must remain inside the planned pane",
		Artist: "Angels & Airwaves", Album: "I-Empire", Year: 2007, Duration: 275,
	}}
	playlist := widgets.NewPlaylist(styles)
	playlist.SetRows(widgets.BuildPlaylistRows(tracks, 0))
	playlist.SetCurrentIndex(0)
	m := Model{
		cfg: cfg, styles: styles, width: 80, height: 24, layoutCheckDone: true,
		showHeader: true, showFooter: true, playing: true, currentIndex: 0,
		playlist: tracks, bottomViewMode: BottomPlaylist,
		header:         widgets.NewHeader(styles.Header, "must - MUSic TUI"),
		nowPlaying:     widgets.NewNowPlaying(styles, "", "", ""),
		playlistWidget: playlist,
		footer:         widgets.NewFooter(styles.AccentStyle, styles.MutedStyle, styles.ForegroundStyle),
	}

	for _, size := range []struct{ width, height int }{{80, 24}, {120, 32}, {160, 24}, {120, 48}, {40, 12}, {200, 50}} {
		m.width, m.height = size.width, size.height
		m.header.SetWidth(size.width)
		content := m.View().Content
		lines := strings.Split(content, "\n")
		if len(lines) != size.height {
			t.Fatalf("%dx%d rendered %d rows", size.width, size.height, len(lines))
		}
		for row, line := range lines {
			if got := ansi.StringWidth(line); got > size.width {
				t.Fatalf("%dx%d row %d is %d cells wide: %q", size.width, size.height, row, got, line)
			}
		}
	}
}
