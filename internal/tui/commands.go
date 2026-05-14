package tui

import (
	"bytes"
	"crypto/rand"
	"image"
	"image/jpeg"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pdfrg/must/internal/api"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/db"
	imgpkg "github.com/pdfrg/must/internal/image"
	"github.com/pdfrg/must/internal/models"
	"github.com/pdfrg/must/internal/playlist"
	"github.com/pdfrg/must/internal/scanner"
)

func scanLibraryCmd(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		libraryDB, err := db.NewLibraryDB()
		if err != nil {
			return scanCompleteMsg{err: err}
		}

		s := scanner.NewScanner(libraryDB)
		result, err := s.Scan(cfg.MusicDir)
		if err != nil {
			return scanCompleteMsg{err: err, db: libraryDB}
		}

		return scanCompleteMsg{result: result, err: nil, db: libraryDB}
	}
}

func loadPathsIntoPlaylist(paths []string, libraryDB *db.LibraryDB) []models.Track {
	var tracks []models.Track

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}

		if info.IsDir() {
			tracks = append(tracks, loadDirTracks(p, libraryDB)...)
			continue
		}

		lower := strings.ToLower(p)
		if strings.HasSuffix(lower, ".m3u") || strings.HasSuffix(lower, ".m3u8") {
			pl, err := playlist.Load(p)
			if err != nil {
				continue
			}
			for _, trackPath := range pl.Tracks {
				if t := findTrackByPath(trackPath, libraryDB); t != nil {
					tracks = append(tracks, *t)
				} else {
					tracks = append(tracks, models.Track{Path: trackPath, Title: filepath.Base(trackPath)})
				}
			}
			continue
		}

		if isAudioFile(p) {
			if t := findTrackByPath(p, libraryDB); t != nil {
				tracks = append(tracks, *t)
			} else {
				tracks = append(tracks, models.Track{Path: p, Title: filepath.Base(p)})
			}
		}
	}

	return tracks
}

func loadDirTracks(dir string, libraryDB *db.LibraryDB) []models.Track {
	var tracks []models.Track

	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !isAudioFile(path) {
			return nil
		}
		if t := findTrackByPath(path, libraryDB); t != nil {
			tracks = append(tracks, *t)
		} else {
			tracks = append(tracks, models.Track{Path: path, Title: filepath.Base(path)})
		}
		return nil
	})

	return tracks
}

func findTrackByPath(path string, libraryDB *db.LibraryDB) *models.Track {
	if libraryDB == nil {
		return nil
	}
	t, err := libraryDB.GetTrackByPath(path)
	if err != nil || t == nil {
		return nil
	}
	return t
}

func isAudioFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3", ".flac", ".ogg", ".opus", ".m4a", ".aac", ".wma", ".wav":
		return true
	}
	return false
}

func shuffleIndices(n int) []int {
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}
	for i := n - 1; i > 0; i-- {
		j, _ := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		indices[i], indices[j.Int64()] = indices[j.Int64()], indices[i]
	}
	return indices
}

func randInt(max int) int {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return int(n.Int64())
}

func loadAlbumArtCmd(renderer *imgpkg.Renderer, trackPath string) tea.Cmd {
	return func() tea.Msg {
		img, err := renderer.GetArtForTrack(trackPath)
		if err != nil {
			return imageLoadedMsg{err: err}
		}

		var buf bytes.Buffer
		if err := encodeImage(&buf, img); err != nil {
			return imageLoadedMsg{err: err}
		}

		return imageLoadedMsg{imageData: buf.Bytes()}
	}
}

func renderAlbumArtAfterDelay() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return renderAlbumArtMsg{}
	})
}

func watchThemeCmd(watcher *config.ThemeWatcher) tea.Cmd {
	return func() tea.Msg {
		path := <-watcher.Events()
		return themeChangedMsg{path: path}
	}
}

func encodeImage(buf *bytes.Buffer, img image.Image) error {
	return jpeg.Encode(buf, img, &jpeg.Options{Quality: 90})
}

func fetchLyricsCmd(track models.Track) tea.Cmd {
	return func() tea.Msg {
		result, err := api.GetBestLyrics(track.Artist, track.Title, track.Album, track.Duration)
		if err != nil {
			return lyricsFetchedMsg{err: err}
		}

		var synced []api.SyncedLyric
		if result.SyncedLyrics != "" {
			synced = api.ParseSyncedLyrics(result.SyncedLyrics)
		}

		return lyricsFetchedMsg{
			plain:  result.PlainLyrics,
			synced: synced,
		}
	}
}

func fetchArtistBioCmd(artist string) tea.Cmd {
	return func() tea.Msg {
		summary, err := api.GetArtistSummary(artist)
		if err != nil {
			return artistBioFetchedMsg{err: err}
		}
		return artistBioFetchedMsg{summary: summary}
	}
}
