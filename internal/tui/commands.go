package tui

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/exec"
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

type PlaybackState struct {
	PlaylistPaths []string `json:"playlist_paths"`
	CurrentIndex  int      `json:"current_index"`
	Position      float64  `json:"position"`
	Shuffle       bool     `json:"shuffle"`
	RepeatMode    string   `json:"repeat_mode"`
}

func SavePlaybackState(playlist []models.Track, currentIndex int, position float64, shuffle bool, repeatMode string) {
	state := PlaybackState{
		PlaylistPaths: make([]string, len(playlist)),
		CurrentIndex:  currentIndex,
		Position:      position,
		Shuffle:       shuffle,
		RepeatMode:    repeatMode,
	}
	for i, t := range playlist {
		state.PlaylistPaths[i] = t.Path
	}

	data, err := json.Marshal(state)
	if err != nil {
		return
	}

	statePath := config.GetStatePath()
	_ = os.MkdirAll(filepath.Dir(statePath), 0755)
	_ = os.WriteFile(statePath, data, 0644)
}

func LoadPlaybackState() *PlaybackState {
	statePath := config.GetStatePath()
	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil
	}

	var state PlaybackState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}

	return &state
}

func ClearPlaybackState() {
	statePath := config.GetStatePath()
	_ = os.Remove(statePath)
}

func scanLibraryCmd(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		libraryDB, err := db.NewLibraryDB()
		if err != nil {
			return scanCompleteMsg{err: err}
		}

		s := scanner.NewScanner(libraryDB)
		result, err := s.Scan(cfg.MusicDirs)
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
			return imageLoadedMsg{err: err, trackPath: trackPath}
		}

		var buf bytes.Buffer
		if err := encodeImage(&buf, img); err != nil {
			return imageLoadedMsg{err: err, trackPath: trackPath}
		}

		return imageLoadedMsg{imageData: buf.Bytes(), trackPath: trackPath}
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

func fetchArtistInfoCmd(cfg *config.Config, artist, album string, eventID int64, cache map[string]*models.ArtistInfo) tea.Cmd {
	if cache != nil {
		if cached, ok := cache[strings.ToLower(artist)]; ok {
			return func() tea.Msg {
				return artistInfoFetchedMsg{eventID: eventID, info: cached}
			}
		}
	}

	return func() tea.Msg {
		info := &models.ArtistInfo{}

		type tadbResult struct {
			artist *api.TheAudioDBArtist
			err    error
		}
		type mbResult struct {
			mbid   string
			albums []api.MBAlbum
			err    error
		}

		tadbCh := make(chan tadbResult, 1)
		mbCh := make(chan mbResult, 1)

		go func() {
			a, e := api.SearchArtistTheAudioDB(cfg.TheAudioDBApiKey, artist, album)
			tadbCh <- tadbResult{a, e}
		}()

		go func() {
			mbid, a, e := api.GetDiscographyMusicBrainz(artist, album)
			mbCh <- mbResult{mbid, a, e}
		}()

		tadb := <-tadbCh
		mb := <-mbCh

		if tadb.artist != nil {
			if tadb.artist.StrBiography != "" {
				info.Bio = tadb.artist.StrBiography
				info.BioSource = "theaudiodb"
			}
			if tadb.artist.StrArtistThumb != "" {
				info.ThumbnailURL = tadb.artist.StrArtistThumb
				info.ThumbSource = "theaudiodb"
			}
			if fanArts := tadb.artist.FanArts(); len(fanArts) > 0 {
				info.GalleryURLs = fanArts
				info.GallerySource = "theaudiodb"
			}
		}

		if len(mb.albums) > 0 {
			var b strings.Builder
			for _, a := range mb.albums {
				if a.Year != "" {
					fmt.Fprintf(&b, "%s (%s)\n", a.Title, a.Year)
				} else {
					b.WriteString(a.Title + "\n")
				}
			}
			info.Discography = strings.TrimSpace(b.String())
			info.DiscoSource = "musicbrainz"
		}

		discogsArtist, err := api.SearchArtistDiscogs(cfg.DiscogsToken, cfg.DiscogsKey, cfg.DiscogsSecret, artist)
		if err == nil && discogsArtist != nil {
			if info.Bio == "" && discogsArtist.Profile != "" {
				info.Bio = discogsArtist.Profile
				info.BioSource = "discogs"
			}
			if primaryImg := discogsArtist.PrimaryImage(); primaryImg != "" {
				info.ThumbnailURL = primaryImg
				info.ThumbSource = "discogs"
			}
			if galleryURLs := discogsArtist.GalleryURLs(); len(galleryURLs) > 0 {
				info.GalleryURLs = galleryURLs
				info.GallerySource = "discogs"
			}
		}

		if info.Bio == "" || info.ThumbnailURL == "" || info.Discography == "" {
			summary, err := api.GetArtistSummary(artist)
			if err == nil && summary != nil {
				if info.Bio == "" && summary.Extract != "" {
					info.Bio = summary.Extract
					info.BioSource = "wikipedia"
				}
				if info.ThumbnailURL == "" && summary.Thumbnail != nil && summary.Thumbnail.Source != "" {
					info.ThumbnailURL = summary.Thumbnail.Source
					info.ThumbSource = "wikipedia"
				}
				if info.PageURL == "" && summary.URL != "" {
					info.PageURL = summary.URL
				}
			}
		}

		if cfg.Lidarr.Enabled && cfg.Lidarr.URL != "" && cfg.Lidarr.APIKey != "" && mb.mbid != "" {
			info.LidarrMBID = mb.mbid
			lidarrClient := api.NewLidarrClient(cfg.Lidarr.URL, cfg.Lidarr.APIKey, cfg.Lidarr.Enabled)
			lidarrStatus, lidErr := lidarrClient.GetArtistByMBID(mb.mbid)
			if lidErr != nil {
				info.LidarrError = lidarrStatus.Error
			} else if lidarrStatus.InLidarr {
				info.LidarrInLidarr = true
				info.LidarrMonitored = lidarrStatus.Monitored
				info.LidarrArtistID = lidarrStatus.ArtistID
				info.LidarrArtistName = lidarrStatus.ArtistName

				if info.Discography != "" {
					var mbTitles []string
					for _, line := range strings.Split(info.Discography, "\n") {
						line = strings.TrimSpace(line)
						if line == "" {
							continue
						}
						if idx := strings.LastIndex(line, " ("); idx > 0 {
							mbTitles = append(mbTitles, line[:idx])
						} else {
							mbTitles = append(mbTitles, line)
						}
					}
					if len(mbTitles) > 0 {
						lidarrAlbums, albErr := lidarrClient.GetArtistAlbums(lidarrStatus.ArtistID, mbTitles)
						if albErr == nil && len(lidarrAlbums) > 0 {
							info.LidarrAlbums = make(map[string]models.LidarrAlbumInfo)
							for title, status := range lidarrAlbums {
								info.LidarrAlbums[title] = models.LidarrAlbumInfo{
									InLidarr:        status.InLidarr,
									Monitored:       status.Monitored,
									HasFiles:        status.HasFiles,
									PercentOfTracks: status.PercentOfTracks,
								}
							}
						}
					}
				}
			}
		}

		if info.Bio == "" {
			info.Bio = "No biography found."
		}

		return artistInfoFetchedMsg{eventID: eventID, info: info}
	}
}

func fetchOnlineArtCmd(cfg *config.Config, track models.Track) tea.Cmd {
	return func() tea.Msg {
		if cfg.TheAudioDBApiKey != "" {
			artURL, err := api.FetchAlbumArtURLTheAudioDB(cfg.TheAudioDBApiKey, track.Artist, track.Album)
			if err == nil && artURL != "" {
				if err := imgpkg.DownloadAndCacheArt(track.Path, artURL); err == nil {
					return onlineArtFetchedMsg{trackPath: track.Path}
				}
			}
		}
		return onlineArtFetchedMsg{err: fmt.Errorf("no online art found")}
	}
}

func sendNotificationCmd(cfg *config.Config, track models.Track, withImage bool) tea.Cmd {
	return func() tea.Msg {
		api.SendDesktopNotification(&track, cfg, withImage)
		return notificationSentMsg{}
	}
}

func copyToClipboardCmd(track models.Track) tea.Cmd {
	return func() tea.Msg {
		info := track.FormatDisplayInfo()
		tools := []string{"wl-copy", "xclip -selection clipboard", "xsel --clipboard --input", "pbcopy"}
		for _, tool := range tools {
			parts := strings.Fields(tool)
			if len(parts) == 0 {
				continue
			}
			cmd := exec.Command(parts[0], parts[1:]...)
			cmd.Stdin = strings.NewReader(info)
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
		return nil
	}
}

func loadArtistImageCmd(eventID int64, artistName string, trackPath string, thumbnailURL string) tea.Cmd {
	return func() tea.Msg {
		if trackPath != "" {
			img, err := imgpkg.GetArtistImage(artistName, trackPath)
			if err == nil && img != nil {
				var buf bytes.Buffer
				if encErr := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); encErr == nil {
					return artistImageLoadedMsg{eventID: eventID, imageData: buf.Bytes(), trackPath: trackPath}
				}
			}
		}

		if thumbnailURL != "" {
			client := &http.Client{Timeout: 10 * time.Second}
			req, err := http.NewRequest("GET", thumbnailURL, nil)
			if err != nil {
				return artistImageLoadedMsg{eventID: eventID, err: err}
			}
			req.Header.Set("User-Agent", "must/1.0")
			resp, err := client.Do(req)
			if err != nil {
				return artistImageLoadedMsg{eventID: eventID, err: err}
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				return artistImageLoadedMsg{eventID: eventID, err: fmt.Errorf("artist image HTTP %d", resp.StatusCode)}
			}

			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return artistImageLoadedMsg{eventID: eventID, err: err}
			}

			return artistImageLoadedMsg{eventID: eventID, imageData: data}
		}

		return artistImageLoadedMsg{eventID: eventID, err: fmt.Errorf("no artist image available")}
	}
}
