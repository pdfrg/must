package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/pdfrg/must/internal/api"
	"github.com/pdfrg/must/internal/db"
	"github.com/pdfrg/must/internal/models"
	"github.com/pdfrg/must/internal/playlist"
)

type favoriteResultMsg struct {
	path     string
	remoteID string
	source   models.TrackSource
	starred  bool
	err      error
}

// expandUserPath expands ~ and env vars like temp-dir handling elsewhere.
func expandUserPath(p string) string {
	expanded := os.ExpandEnv(p)
	if strings.HasPrefix(expanded, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			expanded = filepath.Join(home, expanded[2:])
		}
	}
	return expanded
}

// favoritableReason reports why a track cannot be favorited, or "" if OK.
// Temp/download dirs (cfg.TempDirs) and stream URLs are excluded.
func favoritableReason(t models.Track, tempDirs []string) string {
	if t.Source == models.SourceSubsonic {
		if t.RemoteID == "" {
			return "No server ID for this track"
		}
		return ""
	}
	if playlist.IsURL(t.Path) {
		return "Cannot favorite streams"
	}
	if t.Path == "" {
		return "Nothing to favorite"
	}
	abs, err := filepath.Abs(expandUserPath(t.Path))
	if err != nil {
		return "Cannot favorite this track"
	}
	for _, dir := range tempDirs {
		base := expandUserPath(dir)
		baseAbs, err := filepath.Abs(base)
		if err != nil {
			continue
		}
		if abs == baseAbs || strings.HasPrefix(abs, baseAbs+string(os.PathSeparator)) {
			return "Cannot favorite temp/download tracks"
		}
	}
	return ""
}

func toggleFavoriteCmd(client *api.SubsonicClient, libraryDB *db.LibraryDB, track models.Track, starred bool) tea.Cmd {
	return func() tea.Msg {
		msg := favoriteResultMsg{path: track.Path, remoteID: track.RemoteID, source: track.Source, starred: starred}
		if track.Source == models.SourceSubsonic {
			if client == nil {
				msg.err = fmt.Errorf("subsonic not configured")
				return msg
			}
			if starred {
				msg.err = client.Star(track.RemoteID)
			} else {
				msg.err = client.Unstar(track.RemoteID)
			}
			return msg
		}
		if libraryDB == nil {
			msg.err = fmt.Errorf("library not ready")
			return msg
		}
		msg.err = libraryDB.SetFavorite(track.Path, starred)
		return msg
	}
}

func sameFavoriteIdentity(a, b models.Track) bool {
	if a.Source == models.SourceSubsonic && b.Source == models.SourceSubsonic {
		if a.RemoteID != "" && a.RemoteID == b.RemoteID {
			return true
		}
	}
	return a.Path != "" && a.Path == b.Path
}

// toggleFavorite flips the starred state of the playlist cursor track.
func (m Model) toggleFavorite() (tea.Model, tea.Cmd) {
	if len(m.playlist) == 0 {
		return m, setStatus(&m, "Nothing to favorite", true)
	}
	idx := m.playlistWidget.GetCursor()
	if idx < 0 || idx >= len(m.playlist) {
		idx = m.currentIndex
	}
	if idx < 0 || idx >= len(m.playlist) {
		return m, setStatus(&m, "Nothing to favorite", true)
	}
	track := m.playlist[idx]
	if reason := favoritableReason(track, m.cfg.TempDirs); reason != "" {
		return m, setStatus(&m, reason, true)
	}
	starred := !track.Starred
	m.applyFavoriteOptimistic(track, starred)
	verb := "\u2b50 Favorited"
	if !starred {
		verb = "Unfavorited"
	}
	return m, tea.Batch(
		toggleFavoriteCmd(m.subsonicClient, m.libraryDB, track, starred),
		setStatus(&m, verb+": "+track.Title, false),
	)
}

func (m Model) handleFavoriteResult(msg favoriteResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		// Revert optimistic update.
		for i := range m.playlist {
			if m.playlist[i].Path == msg.path && (msg.source != models.SourceSubsonic || m.playlist[i].RemoteID == msg.remoteID) {
				m.playlist[i].Starred = !msg.starred
			}
		}
		if m.libraryModal != nil {
			m.libraryModal.SetTrackStarred(msg.path, msg.remoteID, msg.source, !msg.starred)
		}
		m.updatePlaylist()
		return m, setStatus(&m, "Favorite failed: "+msg.err.Error(), true)
	}
	return m, nil
}

// toggleModalFavorite flips the star on a library-modal track. The modal
// owns no Subsonic client, so the parent validates, fans the change out to
// the main playlist and the modal's visible rows, and runs the async op.
// The modal deliberately does not rebuild its lists, so an F-unstarred
// track stays visible (starless) until the next rebuild — F again undoes it.
func (m Model) toggleModalFavorite(track models.Track) (tea.Model, tea.Cmd) {
	if reason := favoritableReason(track, m.cfg.TempDirs); reason != "" {
		return m, setStatus(&m, reason, true)
	}
	starred := !track.Starred
	m.applyFavoriteOptimistic(track, starred)
	if m.libraryModal != nil {
		m.libraryModal.SetTrackStarred(track.Path, track.RemoteID, track.Source, starred)
	}
	verb := "\u2b50 Favorited"
	if !starred {
		verb = "Unfavorited"
	}
	return m, tea.Batch(
		toggleFavoriteCmd(m.subsonicClient, m.libraryDB, track, starred),
		setStatus(&m, verb+": "+track.Title, false),
	)
}

// applyLocalFavorites marks local playlist tracks starred per the favorites DB.
// Subsonic tracks already carry Starred from the server; temp/download and
// stream tracks are never marked.
func (m *Model) applyLocalFavorites() {
	if m.libraryDB == nil {
		return
	}
	favSet, err := m.libraryDB.FavoritePaths()
	if err != nil || len(favSet) == 0 {
		return
	}
	for i := range m.playlist {
		t := &m.playlist[i]
		if t.Source == models.SourceSubsonic {
			continue
		}
		if favoritableReason(*t, m.cfg.TempDirs) != "" {
			t.Starred = false
			continue
		}
		t.Starred = favSet[t.Path]
	}
}

// applyFavoriteOptimistic flips Starred on all playlist entries matching track.
func (m *Model) applyFavoriteOptimistic(track models.Track, starred bool) {
	for i := range m.playlist {
		if sameFavoriteIdentity(m.playlist[i], track) {
			m.playlist[i].Starred = starred
		}
	}
	// Skip applyLocalFavorites here: the backing store (server/DB) hasn't
	// confirmed yet, and re-reading it would revert this optimistic flip.
	m.refreshPlaylistWidget()
}
