package modals

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/models"
)

func TestAlbumYearUsesMutedStyle(t *testing.T) {
	styles := &config.ThemeStyles{
		ForegroundStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")),
		AccentStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")),
		CursorStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("#0000ff")),
		MutedStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("#777777")),
	}
	library := NewLibrary(styles, nil)
	library.albums = []models.AlbumEntry{{Name: "The Autumn Effect", Year: 2005}}

	got := library.renderAlbumColumn(40, 1)
	if plain := ansi.Strip(got); plain != " The Autumn Effect (2005)" {
		t.Fatalf("rendered album = %q", plain)
	}
	if !strings.Contains(got, styles.MutedStyle.Render(" (2005)")) {
		t.Fatalf("album year does not use the muted style: %q", got)
	}
}

func TestAlbumYearStillMutedOnFocusedRow(t *testing.T) {
	styles := &config.ThemeStyles{
		ForegroundStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")),
		AccentStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")),
		CursorStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("#0000ff")),
		MutedStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("#777777")),
	}
	library := NewLibrary(styles, nil)
	library.focusPane = FocusAlbums
	library.albums = []models.AlbumEntry{{Name: "The Autumn Effect", Year: 2005}}

	got := library.renderAlbumColumn(40, 1)
	if plain := ansi.Strip(got); plain != "> The Autumn Effect (2005)" {
		t.Fatalf("rendered focused album = %q", plain)
	}
	if !strings.Contains(got, styles.MutedStyle.Render(" (2005)")) {
		t.Fatalf("focused album year does not use the muted style: %q", got)
	}
}
