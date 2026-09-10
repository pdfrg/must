package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/pdfrg/must/internal/config"
)

func TestLibraryMouseMode(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MouseEnabled = true
	cfg.MouseFocusOnHover = true
	m := Model{cfg: cfg, activeModal: ModalLibrary, styles: &config.ThemeStyles{}}

	if got := m.altView("library").MouseMode; got != tea.MouseModeAllMotion {
		t.Fatalf("hover mouse mode = %v, want all motion", got)
	}
	cfg.MouseFocusOnHover = false
	if got := m.altView("library").MouseMode; got != tea.MouseModeCellMotion {
		t.Fatalf("click-only mouse mode = %v, want cell motion", got)
	}
	m.activeModal = ModalNone
	if got := m.altView("player").MouseMode; got != tea.MouseModeNone {
		t.Fatalf("main player mouse mode = %v, want disabled", got)
	}
	cfg.MouseEnabled = false
	m.activeModal = ModalLibrary
	if got := m.altView("library").MouseMode; got != tea.MouseModeNone {
		t.Fatalf("disabled mouse mode = %v, want none", got)
	}
}
