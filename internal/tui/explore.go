package tui

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/pdfrg/must/internal/api"
	"github.com/pdfrg/must/internal/models"
	"github.com/pdfrg/must/internal/tui/modals"
)

// exploreTracksMsg carries resolved tracks for a chosen album back to the model.
type exploreTracksMsg struct {
	tracks []models.Track
	action modals.ExploreAction
	album  string
	err    error
}

// exploreAlbumsCmd gathers a fresh randomized batch using hybrid allocation:
// one guaranteed slot per available source, remaining slots proportional to
// each source's album count (largest-remainder rounding).
func (m *Model) exploreAlbumsCmd(source modals.ExploreSource, limit int, seq int) tea.Cmd {
	return func() tea.Msg {
		// Counting subsonic albums hits the network, so it must happen inside
		// the command (async) rather than while handling the keypress.
		localCount := 0
		if m.libraryDB != nil && m.libraryReady {
			if c, err := m.libraryDB.AlbumCount(); err == nil {
				localCount = c
			}
		}
		subsonicCount := 0
		if m.subsonicClient != nil {
			if c, err := subsonicAlbumCount(m.subsonicClient); err == nil {
				subsonicCount = c
			}
		}
		tempCount := len(m.tempAlbumDirs())

		counts := map[modals.ExploreSource]int{
			modals.ExploreLocal:    max(0, localCount),
			modals.ExploreSubsonic: max(0, subsonicCount),
			modals.ExploreTemp:     tempCount,
		}

		var available []modals.ExploreSource
		for _, s := range []modals.ExploreSource{modals.ExploreLocal, modals.ExploreSubsonic, modals.ExploreTemp} {
			if counts[s] > 0 {
				available = append(available, s)
			}
		}

		// "All" total for the header.
		total := 0
		for _, c := range counts {
			total += c
		}
		counts[modals.ExploreAll] = total

		if len(available) == 0 {
			return modals.ExploreAlbumsMsg{Source: source, Counts: counts, Err: fmt.Errorf("no music sources available")}
		}

		targets := available
		if source != modals.ExploreAll {
			targets = nil
			for _, s := range available {
				if s == source {
					targets = append(targets, s)
				}
			}
			if len(targets) == 0 {
				return modals.ExploreAlbumsMsg{Source: source, Counts: counts, Available: available, Err: fmt.Errorf("source unavailable")}
			}
		}

		alloc := allocateExplore(limit, counts, targets)

		var pool []modals.ExploreAlbum
		for _, s := range targets {
			pool = append(pool, m.fetchExploreAlbums(s, alloc[s])...)
		}

		albums := dedupeShuffleExplore(pool, limit)
		if len(albums) < limit {
			albums = m.topUpExplore(albums, targets, counts, limit)
		}
		return modals.ExploreAlbumsMsg{
			Albums:    albums,
			Source:    source,
			Counts:    counts,
			Available: available,
			Seq:       seq,
		}
	}
}

// allocateExplore implements min-1 + proportional (largest remainder) slots.
func allocateExplore(limit int, counts map[modals.ExploreSource]int, available []modals.ExploreSource) map[modals.ExploreSource]int {
	alloc := map[modals.ExploreSource]int{}
	if limit <= 0 || len(available) == 0 {
		return alloc
	}
	if limit < len(available) {
		// Degenerate: fewer slots than sources — give one to the first `limit`.
		for i := 0; i < limit; i++ {
			alloc[available[i]] = 1
		}
		return alloc
	}

	for _, s := range available {
		alloc[s] = 1
	}
	remaining := limit - len(available)

	total := 0
	for _, s := range available {
		total += counts[s]
	}
	if remaining <= 0 || total <= 0 {
		return alloc
	}

	type fracrem struct {
		src  modals.ExploreSource
		frac float64
	}
	rems := make([]fracrem, 0, len(available))
	used := 0
	for _, s := range available {
		exact := float64(remaining) * float64(counts[s]) / float64(total)
		base := int(exact)
		alloc[s] += base
		used += base
		rems = append(rems, fracrem{src: s, frac: exact - float64(base)})
	}
	leftover := remaining - used
	sort.SliceStable(rems, func(i, j int) bool { return rems[i].frac > rems[j].frac })
	for i := 0; i < leftover && i < len(rems); i++ {
		alloc[rems[i].src]++
	}
	return alloc
}

func (m *Model) fetchExploreAlbums(source modals.ExploreSource, n int) []modals.ExploreAlbum {
	if n <= 0 {
		return nil
	}
	switch source {
	case modals.ExploreLocal:
		if m.libraryDB == nil || !m.libraryReady {
			return nil
		}
		entries, err := m.libraryDB.RandomAlbums(n)
		if err != nil {
			return nil
		}
		out := make([]modals.ExploreAlbum, 0, len(entries))
		for _, e := range entries {
			out = append(out, modals.ExploreAlbum{
				Name:   e.Name,
				Artist: e.Artist,
				Year:   e.Year,
				Source: modals.ExploreLocal,
			})
		}
		return out

	case modals.ExploreSubsonic:
		if m.subsonicClient == nil {
			return nil
		}
		albums, err := m.subsonicClient.GetAlbumList2("random", 0, 0, n, "")
		if err != nil {
			return nil
		}
		out := make([]modals.ExploreAlbum, 0, len(albums))
		for _, a := range albums {
			out = append(out, modals.ExploreAlbum{
				Name:    a.Name,
				Artist:  a.Artist,
				Year:    a.Year,
				Source:  modals.ExploreSubsonic,
				AlbumID: a.ID,
			})
		}
		return out

	case modals.ExploreTemp:
		dirs := m.tempAlbumDirs()
		for i := len(dirs) - 1; i > 0; i-- {
			j := randInt(i + 1)
			dirs[i], dirs[j] = dirs[j], dirs[i]
		}
		if n < len(dirs) {
			dirs = dirs[:n]
		}
		out := make([]modals.ExploreAlbum, 0, len(dirs))
		for _, dir := range dirs {
			out = append(out, modals.ExploreAlbum{
				Name:      filepath.Base(dir),
				Artist:    "temp",
				Source:    modals.ExploreTemp,
				TrackPath: firstAudioFile(dir),
			})
		}
		return out
	}
	return nil
}

func dedupeShuffleExplore(pool []modals.ExploreAlbum, limit int) []modals.ExploreAlbum {
	seen := make(map[string]bool, len(pool))
	out := make([]modals.ExploreAlbum, 0, len(pool))
	for _, a := range pool {
		key := strings.ToLower(a.Artist) + "\x1f" + strings.ToLower(a.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, a)
	}
	for i := len(out) - 1; i > 0; i-- {
		j := randInt(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// topUpExplore recovers from cross-source duplicates dropping the batch below
// the target size. It fetches a few more random candidates per source.
func (m *Model) topUpExplore(have []modals.ExploreAlbum, targets []modals.ExploreSource, counts map[modals.ExploreSource]int, limit int) []modals.ExploreAlbum {
	seen := make(map[string]bool, len(have))
	for _, a := range have {
		seen[strings.ToLower(a.Artist)+"\x1f"+strings.ToLower(a.Name)] = true
	}
	out := have
	for attempt := 0; attempt < 2 && len(out) < limit; attempt++ {
		for _, s := range targets {
			if len(out) >= limit {
				break
			}
			extra := m.fetchExploreAlbums(s, limit)
			for _, a := range extra {
				key := strings.ToLower(a.Artist) + "\x1f" + strings.ToLower(a.Name)
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, a)
				if len(out) >= limit {
					break
				}
			}
		}
	}
	return out
}

// tempAlbumDirs returns every immediate subdirectory (under the configured temp
// dirs) that contains audio files.
func (m *Model) tempAlbumDirs() []string {
	var dirs []string
	for _, dir := range m.cfg.TempDirs {
		expanded := expandTempPath(dir)
		entries, err := os.ReadDir(expanded)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			sub := filepath.Join(expanded, e.Name())
			if countAudioFilesInDir(sub) > 0 {
				dirs = append(dirs, sub)
			}
		}
	}
	return dirs
}

func expandTempPath(dir string) string {
	expanded := os.ExpandEnv(dir)
	if strings.HasPrefix(expanded, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			expanded = filepath.Join(home, expanded[2:])
		}
	}
	return expanded
}

func firstAudioFile(dir string) string {
	var found string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || found != "" {
			return nil
		}
		if isAudioFile(path) {
			found = path
		}
		return nil
	})
	return found
}

// exploreTracksCmd resolves a chosen album to a track list and applies the
// requested play/enqueue action.
func (m *Model) exploreTracksCmd(album modals.ExploreAlbum, action modals.ExploreAction) tea.Cmd {
	return func() tea.Msg {
		tracks, err := m.resolveExploreTracks(album)
		if err != nil || len(tracks) == 0 {
			if err == nil {
				err = fmt.Errorf("no tracks found for %s", album.Name)
			}
			return exploreTracksMsg{action: action, album: album.Name, err: err}
		}
		return exploreTracksMsg{tracks: tracks, action: action, album: album.Name}
	}
}

func (m *Model) resolveExploreTracks(album modals.ExploreAlbum) ([]models.Track, error) {
	switch album.Source {
	case modals.ExploreSubsonic:
		if m.subsonicClient == nil {
			return nil, fmt.Errorf("subsonic not configured")
		}
		full, err := m.subsonicClient.GetAlbum(album.AlbumID)
		if err != nil {
			return nil, err
		}
		return m.subsonicClient.ChildrenToTracks(full.Song), nil
	case modals.ExploreTemp:
		if album.TrackPath == "" {
			return nil, fmt.Errorf("temp album has no tracks")
		}
		return loadDirTracks(filepath.Dir(album.TrackPath), m.libraryDB), nil
	default:
		if m.libraryDB == nil {
			return nil, fmt.Errorf("library not available")
		}
		return m.libraryDB.GetTracksByArtistAndAlbum(album.Artist, album.Name)
	}
}

// exploreArtCmd loads cover art bytes for the highlighted album.
func (m *Model) exploreArtCmd(album modals.ExploreAlbum, seq int) tea.Cmd {
	return func() tea.Msg {
		key := album.Key()
		if album.Source == modals.ExploreSubsonic {
			if m.subsonicClient == nil {
				return modals.ExploreArtLoadedMsg{Key: key, Seq: seq, Err: fmt.Errorf("subsonic not configured")}
			}
			if msg := trySubsonicArt(m.subsonicClient, album.AlbumID, key); msg != nil {
				return modals.ExploreArtLoadedMsg{Key: key, Seq: seq, ImageData: msg.imageData}
			}
			return modals.ExploreArtLoadedMsg{Key: key, Seq: seq, Err: fmt.Errorf("no subsonic art")}
		}

		if m.imageRenderer == nil {
			return modals.ExploreArtLoadedMsg{Key: key, Seq: seq, Err: fmt.Errorf("album art disabled")}
		}
		path := album.TrackPath
		if path == "" && album.Source == modals.ExploreLocal && m.libraryDB != nil {
			if tracks, err := m.libraryDB.GetTracksByArtistAndAlbum(album.Artist, album.Name); err == nil && len(tracks) > 0 {
				path = tracks[0].Path
			}
		}
		if path == "" {
			return modals.ExploreArtLoadedMsg{Key: key, Seq: seq, Err: fmt.Errorf("no track path")}
		}
		img, err := m.imageRenderer.GetArtForTrack(path)
		if err != nil {
			return modals.ExploreArtLoadedMsg{Key: key, Seq: seq, Err: err}
		}
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
			return modals.ExploreArtLoadedMsg{Key: key, Seq: seq, Err: err}
		}
		return modals.ExploreArtLoadedMsg{Key: key, Seq: seq, ImageData: buf.Bytes()}
	}
}

// subsonicAlbumCount returns the total album count across all artists.
func subsonicAlbumCount(client *api.SubsonicClient) (int, error) {
	artists, err := client.GetArtists()
	if err != nil {
		return 0, err
	}
	total := 0
	for _, idx := range artists.Index {
		for _, a := range idx.Artist {
			total += a.AlbumCount
		}
	}
	return total, nil
}
