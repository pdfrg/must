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
	"github.com/pdfrg/must/internal/duration"
	imgpkg "github.com/pdfrg/must/internal/image"
	"github.com/pdfrg/must/internal/models"
	"github.com/pdfrg/must/internal/playlist"
	"github.com/pdfrg/must/internal/scanner"

	"github.com/dhowden/tag"
	"github.com/dhowden/tag/mbz"
)

type PlaybackState struct {
	PlaylistPaths     []string `json:"playlist_paths"`
	CurrentIndex      int      `json:"current_index"`
	Position          float64  `json:"position"`
	Shuffle           bool     `json:"shuffle"`
	RepeatMode        string   `json:"repeat_mode"`
	PlaylistSources   []string `json:"playlist_sources,omitempty"`
	PlaylistRemoteIDs []string `json:"playlist_remote_ids,omitempty"`
}

func SavePlaybackState(playlist []models.Track, currentIndex int, position float64, shuffle bool, repeatMode string) {
	state := PlaybackState{
		PlaylistPaths:     make([]string, len(playlist)),
		PlaylistSources:   make([]string, len(playlist)),
		PlaylistRemoteIDs: make([]string, len(playlist)),
		CurrentIndex:      currentIndex,
		Position:          position,
		Shuffle:           shuffle,
		RepeatMode:        repeatMode,
	}
	for i, t := range playlist {
		state.PlaylistPaths[i] = t.Path
		state.PlaylistSources[i] = string(t.Source)
		state.PlaylistRemoteIDs[i] = t.RemoteID
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
			for i, trackPath := range pl.Tracks {
				if playlist.IsURL(trackPath) {
					title := ""
					if i < len(pl.Titles) {
						title = pl.Titles[i]
					}
					if title == "" {
						title = urlBasename(trackPath)
					}
					tracks = append(tracks, models.Track{Path: trackPath, Title: title})
					continue
				}
				if t := findTrackByPath(trackPath, libraryDB); t != nil {
					tracks = append(tracks, *t)
				} else {
					tracks = append(tracks, readTrackFromFile(trackPath))
				}
			}
			continue
		}

		if isAudioFile(p) {
			if t := findTrackByPath(p, libraryDB); t != nil {
				tracks = append(tracks, *t)
			} else {
				tracks = append(tracks, readTrackFromFile(p))
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
			tracks = append(tracks, readTrackFromFile(path))
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

func readTrackFromFile(path string) models.Track {
	t := models.Track{Path: path, Title: filepath.Base(path)}

	f, err := os.Open(path)
	if err != nil {
		return t
	}
	defer func() { _ = f.Close() }()

	metadata, err := tag.ReadFrom(f)
	if err == nil {
		t.Title = metadata.Title()
		t.Artist = metadata.Artist()
		t.Album = metadata.Album()
		t.AlbumArtist = metadata.AlbumArtist()
		t.Year = metadata.Year()
		t.Genre = metadata.Genre()
		t.TrackNum, _ = metadata.Track()
		t.DiscNum, _ = metadata.Disc()
		t.HasCoverArt = metadata.Picture() != nil
		if mbInfo := mbz.Extract(metadata); len(mbInfo) > 0 {
			t.MBArtistID = mbInfo.Get(mbz.Artist)
			t.MBAlbumArtistID = mbInfo.Get(mbz.AlbumArtist)
			t.MBReleaseID = mbInfo.Get(mbz.Album)
			t.MBReleaseGroupID = mbInfo.Get(mbz.ReleaseGroup)
			t.MBRecordingID = mbInfo.Get(mbz.Recording)
			if t.MBRecordingID == "" {
				t.MBRecordingID = mbInfo.Get(mbz.Track)
			}
		}
	}

	if t.Title == "" {
		t.Title = filepath.Base(path)
	}

	dur, err := duration.FileDuration(path)
	if err == nil && dur > 0 {
		t.Duration = dur
	}

	return t
}

func subsonicSearchCmd(client *api.SubsonicClient, query string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return subsonicSearchResultsMsg{err: fmt.Errorf("subsonic not configured")}
		}

		field, fieldVal := parseQueryPrefix(query)
		normalized := field
		switch normalized {
		case "song", "track":
			normalized = "title"
		}

		switch normalized {
		case "title":
			result, err := client.Search3(fieldVal, 0, 0, 50)
			if err != nil {
				return subsonicSearchResultsMsg{err: err, query: query}
			}
			tracks := client.ChildrenToTracks(result.Song)
			return subsonicSearchResultsMsg{
				tracks: tracks, query: query,
			}

		case "genre":
			songs, err := client.GetSongsByGenre(fieldVal, 50)
			if err != nil {
				return subsonicSearchResultsMsg{err: err, query: query}
			}
			albums, err := client.GetAlbumList2("byGenre", 0, 0, 20, fieldVal)
			if err != nil {
				return subsonicSearchResultsMsg{
					tracks: client.ChildrenToTracks(songs),
					query:  query,
				}
			}
			return subsonicSearchResultsMsg{
				tracks: client.ChildrenToTracks(songs),
				albums: albums,
				query:  query,
			}

		case "year":
			yearMin, yearMax := parseYearRange(fieldVal)
			if yearMin == 0 {
				yearMin = yearMax
			}
			if yearMin == 0 {
				return subsonicSearchResultsMsg{
					err: fmt.Errorf("invalid year: %s", fieldVal), query: query,
				}
			}
			if yearMax < yearMin {
				yearMax = yearMin
			}
			albums, err := client.GetAlbumList2("byYear", yearMin, yearMax, 50, "")
			if err != nil {
				return subsonicSearchResultsMsg{err: err, query: query}
			}
			var allTracks []models.Track
			for _, a := range albums {
				album, err := client.GetAlbum(a.ID)
				if err != nil {
					continue
				}
				allTracks = append(allTracks, client.AlbumToTracks(album)...)
			}
			return subsonicSearchResultsMsg{
				tracks: allTracks,
				albums: albums,
				query:  query,
			}

		case "artist":
			result, err := client.Search3(fieldVal, 3, 30, 100)
			if err != nil {
				return subsonicSearchResultsMsg{err: err, query: query}
			}
			tracks := client.ChildrenToTracks(result.Song)
			return subsonicSearchResultsMsg{
				tracks: tracks, artists: result.Artist, albums: result.Album, query: query,
			}

		case "album":
			result, err := client.Search3(fieldVal, 0, 3, 50)
			if err != nil {
				return subsonicSearchResultsMsg{err: err, query: query}
			}
			tracks := client.ChildrenToTracks(result.Song)
			return subsonicSearchResultsMsg{
				tracks: tracks, artists: result.Artist, albums: result.Album, query: query,
			}

		case "playlist":
			playlists, err := client.GetPlaylists()
			if err != nil {
				return subsonicSearchResultsMsg{err: err, query: query}
			}
			var matched []api.PlaylistInfo
			for _, p := range playlists {
				if strings.Contains(strings.ToLower(p.Name), strings.ToLower(fieldVal)) {
					matched = append(matched, p)
				}
			}
			return subsonicSearchResultsMsg{playlists: matched, query: query}

		default:
			result, err := client.Search3(query, 5, 10, 50)
			if err != nil {
				return subsonicSearchResultsMsg{err: err, query: query}
			}
			tracks := client.ChildrenToTracks(result.Song)
			msg := subsonicSearchResultsMsg{
				tracks: tracks, artists: result.Artist, albums: result.Album, query: query,
			}
			if playlists, perr := client.GetPlaylists(); perr == nil {
				for _, p := range playlists {
					if strings.Contains(strings.ToLower(p.Name), strings.ToLower(query)) {
						msg.playlists = append(msg.playlists, p)
					}
				}
			}
			return msg
		}
	}
}

func subsonicArtistsCmd(client *api.SubsonicClient) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return subsonicArtistsMsg{err: fmt.Errorf("subsonic not configured")}
		}
		artists, err := client.GetArtists()
		if err != nil {
			return subsonicArtistsMsg{err: err}
		}
		var flat []api.ArtistID3
		for _, idx := range artists.Index {
			flat = append(flat, idx.Artist...)
		}
		return subsonicArtistsMsg{artists: flat}
	}
}

func subsonicPlaylistsCmd(client *api.SubsonicClient) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return subsonicPlaylistsMsg{err: fmt.Errorf("subsonic not configured")}
		}
		playlists, err := client.GetPlaylists()
		if err != nil {
			return subsonicPlaylistsMsg{err: err}
		}
		return subsonicPlaylistsMsg{playlists: playlists}
	}
}

func subsonicPlaylistTracksCmd(client *api.SubsonicClient, playlistID string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return subsonicAlbumTracksMsg{err: fmt.Errorf("subsonic not configured")}
		}
		pl, err := client.GetPlaylist(playlistID)
		if err != nil {
			return subsonicAlbumTracksMsg{err: err}
		}
		return subsonicAlbumTracksMsg{tracks: client.ChildrenToTracks(pl.Entry)}
	}
}

func subsonicArtistPlayCmd(client *api.SubsonicClient, artistID string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return subsonicAlbumTracksMsg{err: fmt.Errorf("subsonic not configured")}
		}
		artist, err := client.GetArtist(artistID)
		if err != nil {
			return subsonicAlbumTracksMsg{err: err}
		}
		var allTracks []models.Track
		for _, album := range artist.Album {
			albumDetail, err := client.GetAlbum(album.ID)
			if err != nil {
				continue
			}
			allTracks = append(allTracks, client.AlbumToTracks(albumDetail)...)
		}
		return subsonicAlbumTracksMsg{tracks: allTracks}
	}
}

func subsonicArtistAlbumsCmd(client *api.SubsonicClient, artistID string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return subsonicArtistAlbumsMsg{err: fmt.Errorf("subsonic not configured")}
		}
		artist, err := client.GetArtist(artistID)
		if err != nil {
			return subsonicArtistAlbumsMsg{err: err}
		}
		return subsonicArtistAlbumsMsg{artistID: artistID, albums: artist.Album}
	}
}

func subsonicAlbumTracksCmd(client *api.SubsonicClient, albumID string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return subsonicAlbumTracksMsg{err: fmt.Errorf("subsonic not configured")}
		}
		album, err := client.GetAlbum(albumID)
		if err != nil {
			return subsonicAlbumTracksMsg{err: err}
		}
		tracks := client.AlbumToTracks(album)
		return subsonicAlbumTracksMsg{albumID: albumID, tracks: tracks}
	}
}

func subsonicGenresCmd(client *api.SubsonicClient) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return subsonicGenresMsg{err: fmt.Errorf("subsonic not configured")}
		}
		genres, err := client.GetGenres()
		if err != nil {
			return subsonicGenresMsg{err: err}
		}
		return subsonicGenresMsg{genres: genres}
	}
}

func subsonicGenreAlbumsCmd(client *api.SubsonicClient, genreName string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return subsonicGenreAlbumsMsg{err: fmt.Errorf("subsonic not configured")}
		}
		albums, err := client.GetAlbumList2("byGenre", 0, 0, 500, genreName)
		if err != nil {
			return subsonicGenreAlbumsMsg{genreName: genreName, err: err}
		}
		return subsonicGenreAlbumsMsg{genreName: genreName, albums: albums}
	}
}

func subsonicStarredCmd(client *api.SubsonicClient) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return subsonicStarredMsg{err: fmt.Errorf("subsonic not configured")}
		}
		songs, err := client.GetStarred2()
		if err != nil {
			return subsonicStarredMsg{err: err}
		}
		return subsonicStarredMsg{tracks: client.ChildrenToTracks(songs)}
	}
}

func loadSubsonicAlbumArtCmd(renderer *imgpkg.Renderer, client *api.SubsonicClient, track models.Track) tea.Cmd {
	return func() tea.Msg {
		// Prefer album-level art: per-song (disc-level, dc-*) artwork rows
		// can go stale server-side while the album row stays correct.
		// getCoverArt resolves a raw album ID to the album image.
		ids := []string{}
		if track.AlbumID != "" {
			ids = append(ids, track.AlbumID)
		}
		if track.CoverArtID != "" && track.CoverArtID != track.AlbumID {
			ids = append(ids, track.CoverArtID)
		}
		if len(ids) == 0 {
			return imageLoadedMsg{err: fmt.Errorf("no cover art ID"), trackPath: track.Path}
		}
		for _, id := range ids {
			if msg := trySubsonicArt(client, id, track.Path); msg != nil {
				return *msg
			}
		}
		return imageLoadedMsg{err: fmt.Errorf("no usable subsonic art"), trackPath: track.Path}
	}
}

func trySubsonicArt(client *api.SubsonicClient, artID, trackPath string) *imageLoadedMsg {
	// Try local cache first
	cacheKey := "subsonic_" + artID
	cachedPath := filepath.Join(config.GetArtCacheDir(), cacheKey+".jpg")
	if img, err := imgpkg.LoadImageFromPath(cachedPath); err == nil {
		var buf bytes.Buffer
		if encErr := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); encErr == nil {
			return &imageLoadedMsg{imageData: buf.Bytes(), trackPath: trackPath}
		}
	}
	// Download from Subsonic server
	artURL := client.CoverArtURL(artID)
	resp, err := http.Get(artURL)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	// Navidrome 0.64+ resolves artwork in the background and serves a generic
	// placeholder with `Cache-Control: no-store` until the real image is ready.
	// Never persist those: the cache is keyed by album ID and never revalidated,
	// so a stored placeholder would stick forever. Treat it as "no art yet" and
	// let a later track change retry once resolution has completed.
	if isNoStore(resp.Header.Get("Cache-Control")) {
		return nil
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	// Encode and return for rendering
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	// Cache it
	if err := os.MkdirAll(filepath.Dir(cachedPath), 0755); err == nil {
		_ = os.WriteFile(cachedPath, data, 0644)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil
	}
	return &imageLoadedMsg{imageData: buf.Bytes(), trackPath: trackPath}
}

// isNoStore reports whether a Cache-Control header forbids storing the response.
func isNoStore(cacheControl string) bool {
	return strings.Contains(strings.ToLower(cacheControl), "no-store")
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

// artistInfoDebounce is the settle delay before a track change triggers the
// artist-info network lookup (bio, MusicBrainz, Lidarr).
const artistInfoDebounce = 800 * time.Millisecond

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

// ArtistLookup carries everything known about the playing track's artist
// and album into the artist-info fetch. Name strings are always present;
// MusicBrainz IDs (from file tags or Navidrome) take precedence wherever
// accepted, since they are immune to per-track credit variations and
// title spelling differences.
type ArtistLookup struct {
	Artist           string
	Album            string
	MBArtistID       string
	MBAlbumArtistID  string
	MBReleaseGroupID string
	MBReleaseID      string
}

// LookupFromTrack builds an ArtistLookup for playlist[idx]: the display
// artist via album-level consensus, plus the track's own MBIDs.
func LookupFromTrack(playlist []models.Track, idx int) ArtistLookup {
	if idx < 0 || idx >= len(playlist) {
		return ArtistLookup{}
	}
	t := playlist[idx]
	// NOTE: album-level consensus lives on Model (needs playlist + DB);
	// callers that have it should prefer m.lookupArtistFor(idx). This helper
	// covers the plain track-level fields.
	return ArtistLookup{
		Artist:           t.EffectiveArtist(),
		Album:            t.Album,
		MBArtistID:       t.MBArtistID,
		MBAlbumArtistID:  t.MBAlbumArtistID,
		MBReleaseGroupID: t.MBReleaseGroupID,
		MBReleaseID:      t.MBReleaseID,
	}
}

// artistCacheKey keys the artist-info cache by MB album-artist ID when
// known (stable across compilations), else by lowercased artist name.
func artistCacheKey(l ArtistLookup) string {
	if l.MBAlbumArtistID != "" {
		return "mbid:" + strings.ToLower(l.MBAlbumArtistID)
	}
	if l.MBArtistID != "" {
		return "mbid:" + strings.ToLower(l.MBArtistID)
	}
	return strings.ToLower(l.Artist)
}

func fetchArtistInfoCmd(cfg *config.Config, lookup ArtistLookup, eventID int64, cache map[string]*models.ArtistInfo) tea.Cmd {
	artist, album := lookup.Artist, lookup.Album
	if cache != nil {
		if cached, ok := cache[artistCacheKey(lookup)]; ok {
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

		// Prefer tag/server MusicBrainz IDs for the Lidarr artist lookup: the
		// MB album-artist ID identifies the canonical artist even when
		// per-track credits (or a name search) would resolve to a backing
		// band. Fall back to the MusicBrainz name-search result.
		lidarrArtistMBID := lookup.MBAlbumArtistID
		if lidarrArtistMBID == "" {
			lidarrArtistMBID = lookup.MBArtistID
		}
		if lidarrArtistMBID == "" {
			lidarrArtistMBID = mb.mbid
		}
		if cfg.Lidarr.Enabled && cfg.Lidarr.URL != "" && cfg.Lidarr.APIKey != "" && lidarrArtistMBID != "" {
			info.LidarrMBID = lidarrArtistMBID
			lidarrClient := api.NewLidarrClient(cfg.Lidarr.URL, cfg.Lidarr.APIKey, cfg.Lidarr.Enabled)
			lidarrStatus, lidErr := lidarrClient.GetArtistByMBID(lidarrArtistMBID)
			if lidErr != nil {
				info.LidarrError = lidarrStatus.Error
			} else if lidarrStatus.InLidarr {
				info.LidarrInLidarr = true
				info.LidarrMonitored = lidarrStatus.Monitored
				info.LidarrArtistID = lidarrStatus.ArtistID
				info.LidarrArtistName = lidarrStatus.ArtistName

				// Single fetch: build both the full Lidarr-title map (for the
				// footer/playing-album lookup) and the MB-title map (for the
				// discography view) from the same response. This runs even
				// when the MB discography is empty so the playing album can
				// still resolve directly against Lidarr's titles.
				allEntries, allErr := lidarrClient.GetArtistAllAlbums(lidarrStatus.ArtistID)
				if allErr == nil && len(allEntries) > 0 {
					info.LidarrAllAlbums = make(map[string]models.LidarrAlbumInfo, len(allEntries))
					lidarrTitles := make([]string, 0, len(allEntries))
					for _, e := range allEntries {
						st := models.LidarrAlbumInfo{
							InLidarr:        e.Status.InLidarr,
							Monitored:       e.Status.Monitored,
							HasFiles:        e.Status.HasFiles,
							PercentOfTracks: e.Status.PercentOfTracks,
						}
						info.LidarrAllAlbums[e.Title] = st
						// Exact MBID indexes: release-group ID matches the
						// musicbrainz_releasegroupid tag, release ID the
						// musicbrainz_albumid tag. Either bypasses title fuzz.
						if e.ForeignReleaseGroupID != "" {
							if info.LidarrAlbumsByRGID == nil {
								info.LidarrAlbumsByRGID = make(map[string]models.LidarrAlbumInfo)
							}
							info.LidarrAlbumsByRGID[strings.ToLower(e.ForeignReleaseGroupID)] = st
						}
						if e.ForeignReleaseID != "" {
							if info.LidarrAlbumsByReleaseID == nil {
								info.LidarrAlbumsByReleaseID = make(map[string]models.LidarrAlbumInfo)
							}
							info.LidarrAlbumsByReleaseID[strings.ToLower(e.ForeignReleaseID)] = st
						}
						lidarrTitles = append(lidarrTitles, e.Title)
					}
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
							info.LidarrAlbums = make(map[string]models.LidarrAlbumInfo, len(mbTitles))
							for _, t := range mbTitles {
								if key, ok := api.FindAlbumKey(lidarrTitles, t); ok {
									info.LidarrAlbums[t] = info.LidarrAllAlbums[key]
								} else {
									info.LidarrAlbums[t] = models.LidarrAlbumInfo{}
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

		// Only cache when artist identity resolved: a transient MusicBrainz
		// failure (mbid empty, no tag IDs) must not stick as a permanent
		// "not in Lidarr". Uncached misses simply retry on the next track.
		mbResolved := mb.err == nil && (mb.mbid != "" || lookup.MBAlbumArtistID != "" || lookup.MBArtistID != "")
		return artistInfoFetchedMsg{eventID: eventID, info: info, cacheable: mbResolved}
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

type randomAlbumPlayMsg struct {
	tracks []models.Track
	err    error
}

func (m *Model) randomAlbumCmd(source string) tea.Cmd {
	return func() tea.Msg {
		if source != "" {
			tracks, err := m.randomAlbumFromSource(source)
			if err == nil && len(tracks) > 0 {
				return randomAlbumPlayMsg{tracks: tracks}
			}
			return randomAlbumPlayMsg{err: fmt.Errorf("no albums available from %s", source)}
		}

		type sourceEntry struct {
			name string
			fn   func() ([]models.Track, error)
		}

		var sources []sourceEntry
		if m.libraryDB != nil && m.libraryReady {
			sources = append(sources, sourceEntry{"local", m.randomLocalAlbumTracks})
		}
		if m.subsonicClient != nil {
			sources = append(sources, sourceEntry{"subsonic", m.randomSubsonicAlbumTracks})
		}
		if len(m.cfg.TempDirs) > 0 {
			sources = append(sources, sourceEntry{"temp", m.randomTempAlbumTracks})
		}

		if len(sources) == 0 {
			return randomAlbumPlayMsg{err: fmt.Errorf("no music sources available")}
		}

		// Try sources in random order until one succeeds
		startIdx := randInt(len(sources))
		for i := 0; i < len(sources); i++ {
			idx := (startIdx + i) % len(sources)
			tracks, err := sources[idx].fn()
			if err == nil && len(tracks) > 0 {
				return randomAlbumPlayMsg{tracks: tracks}
			}
		}

		return randomAlbumPlayMsg{err: fmt.Errorf("no albums found in any source")}
	}
}

func (m *Model) randomAlbumFromSource(source string) ([]models.Track, error) {
	switch source {
	case "local":
		return m.randomLocalAlbumTracks()
	case "subsonic":
		return m.randomSubsonicAlbumTracks()
	case "temp":
		return m.randomTempAlbumTracks()
	default:
		return nil, fmt.Errorf("unknown source: %s", source)
	}
}

func (m *Model) randomLocalAlbumTracks() ([]models.Track, error) {
	return m.libraryDB.RandomAlbumTracks()
}

func (m *Model) randomSubsonicAlbumTracks() ([]models.Track, error) {
	albums, err := m.subsonicClient.GetAlbumList2("random", 0, 0, 1, "")
	if err != nil {
		return nil, err
	}
	if len(albums) == 0 {
		return nil, fmt.Errorf("no random subsonic albums returned")
	}
	album, err := m.subsonicClient.GetAlbum(albums[0].ID)
	if err != nil {
		return nil, err
	}
	return m.subsonicClient.AlbumToTracks(album), nil
}

func (m *Model) randomTempAlbumTracks() ([]models.Track, error) {
	var candidates []string
	for _, dir := range m.cfg.TempDirs {
		expanded := os.ExpandEnv(dir)
		if strings.HasPrefix(expanded, "~/") {
			home, _ := os.UserHomeDir()
			expanded = filepath.Join(home, expanded[2:])
		}
		entries, err := os.ReadDir(expanded)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				subPath := filepath.Join(expanded, e.Name())
				if countAudioFilesInDir(subPath) > 0 {
					candidates = append(candidates, subPath)
				}
			}
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no temp directories with audio found")
	}
	dir := candidates[randInt(len(candidates))]
	return loadDirTracks(dir, m.libraryDB), nil
}

func countAudioFilesInDir(dir string) int {
	count := 0
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if isAudioFile(path) {
			count++
		}
		return nil
	})
	return count
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
