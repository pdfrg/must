package modals

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pdfrg/must/internal/config"
)

func searchTestStyles() *config.ThemeStyles {
	return &config.ThemeStyles{
		ForegroundStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")),
		AccentStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")),
		CursorStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("#0000ff")),
		MutedStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("#777777")),
	}
}

func TestSearchAlbumYearUsesMutedStyle(t *testing.T) {
	styles := searchTestStyles()
	s := NewSearch(styles, nil)
	s.SetSize(60, 10)
	s.SetEntries([]searchEntry{{
		Kind: resultAlbum, AlbumName: "The Autumn Effect",
		AlbumArtist: "10 Years", AlbumYear: 2005,
	}})

	got := s.View()
	if plain := ansi.Strip(got); !strings.Contains(plain, "Album: 10 Years — The Autumn Effect (2005)") {
		t.Fatalf("search album row = %q", plain)
	}
	if !strings.Contains(got, styles.MutedStyle.Render(" (2005)")) {
		t.Fatalf("search album year does not use the muted style: %q", got)
	}
}

func TestSearchAlbumTruncationPreservesYear(t *testing.T) {
	styles := searchTestStyles()
	s := NewSearch(styles, nil)
	s.SetSize(40, 10)
	s.SetEntries([]searchEntry{{
		Kind: resultAlbum, AlbumName: "A very long album name that needs to be shortened",
		AlbumArtist: "Some Artist", AlbumYear: 2005,
	}})

	got := s.View()
	plain := ansi.Strip(got)
	if !strings.HasSuffix(strings.TrimSpace(firstResultLine(plain)), "(2005)") {
		t.Fatalf("search album year lost to truncation: %q", plain)
	}
}

func TestSearchAlbumWithoutYearHasNoSuffix(t *testing.T) {
	styles := searchTestStyles()
	s := NewSearch(styles, nil)
	s.SetSize(60, 10)
	s.SetEntries([]searchEntry{{
		Kind: resultAlbum, AlbumName: "Unknown Year Album", AlbumArtist: "Someone",
	}})

	got := s.View()
	if plain := ansi.Strip(got); !strings.Contains(plain, "Album: Someone — Unknown Year Album") {
		t.Fatalf("search album row = %q", plain)
	}
	if strings.Contains(ansi.Strip(got), "(") {
		t.Fatalf("year-less album gained a suffix: %q", ansi.Strip(got))
	}
}

func firstResultLine(view string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "Album:") {
			return line
		}
	}
	return ""
}
