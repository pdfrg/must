package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type LidarrClient struct {
	baseURL    string
	apiKey     string
	enabled    bool
	httpClient *http.Client
}

type LidarrArtistStatus struct {
	InLidarr   bool
	Monitored  bool
	ArtistID   int
	ArtistName string
	Error      string
}

type LidarrAlbumStatus struct {
	InLidarr        bool
	Monitored       bool
	HasFiles        bool
	PercentOfTracks float64
}

func NewLidarrClient(baseURL, apiKey string, enabled bool) *LidarrClient {
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &LidarrClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		enabled:    enabled,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (lc *LidarrClient) IsConfigured() bool {
	return lc.enabled && lc.baseURL != "" && lc.apiKey != ""
}

func (lc *LidarrClient) makeRequest(method, path string) (*http.Response, error) {
	req, err := http.NewRequest(method, lc.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", lc.apiKey)
	req.Header.Set("Accept", "application/json")
	return lc.httpClient.Do(req)
}

func (lc *LidarrClient) GetArtistByMBID(mbid string) (*LidarrArtistStatus, error) {
	if !lc.IsConfigured() {
		return &LidarrArtistStatus{InLidarr: false}, nil
	}

	reqURL := fmt.Sprintf("/api/v1/artist?mbId=%s", url.PathEscape(mbid))
	resp, err := lc.makeRequest("GET", reqURL)
	if err != nil {
		if apiLogger != nil {
			apiLogger.Printf("Lidarr: artist lookup for MBID %s failed: %v", mbid, err)
		}
		return &LidarrArtistStatus{Error: err.Error()}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		if apiLogger != nil {
			apiLogger.Printf("Lidarr: unauthorized for MBID %s", mbid)
		}
		return &LidarrArtistStatus{Error: "invalid API key"}, fmt.Errorf("lidarr: unauthorized")
	}
	if resp.StatusCode != http.StatusOK {
		if apiLogger != nil {
			apiLogger.Printf("Lidarr: status %d for MBID %s", resp.StatusCode, mbid)
		}
		return &LidarrArtistStatus{Error: fmt.Sprintf("status %d", resp.StatusCode)}, fmt.Errorf("lidarr: status %d", resp.StatusCode)
	}

	var artists []struct {
		ID         int    `json:"id"`
		ArtistName string `json:"artistName"`
		Monitored  bool   `json:"monitored"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&artists); err != nil {
		return &LidarrArtistStatus{Error: err.Error()}, err
	}

	if len(artists) == 0 {
		if apiLogger != nil {
			apiLogger.Printf("Lidarr: artist MBID %s not found in Lidarr", mbid)
		}
		return &LidarrArtistStatus{InLidarr: false}, nil
	}

	if apiLogger != nil {
		apiLogger.Printf("Lidarr: found artist %q (ID=%d, monitored=%v) for MBID %s", artists[0].ArtistName, artists[0].ID, artists[0].Monitored, mbid)
	}
	return &LidarrArtistStatus{
		InLidarr:   true,
		Monitored:  artists[0].Monitored,
		ArtistID:   artists[0].ID,
		ArtistName: artists[0].ArtistName,
	}, nil
}

// LidarrAlbumEntry is one album row from Lidarr's /api/v1/album endpoint,
// keyed by Lidarr's own title (not MusicBrainz's). The foreign IDs are
// MusicBrainz identifiers: ForeignReleaseGroupID matches the
// musicbrainz_releasegroupid tag (and /album/<id> URLs), ForeignReleaseID
// matches musicbrainz_albumid. Either enables exact matching without any
// title fuzz.
type LidarrAlbumEntry struct {
	Title                 string
	Status                LidarrAlbumStatus
	ForeignReleaseGroupID string
	ForeignReleaseID      string
}

func (lc *LidarrClient) fetchArtistAlbums(artistID int) ([]LidarrAlbumEntry, error) {
	if !lc.IsConfigured() || artistID == 0 {
		return nil, nil
	}

	reqURL := fmt.Sprintf("/api/v1/album?artistId=%d&includeAllArtistAlbums=true", artistID)
	resp, err := lc.makeRequest("GET", reqURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lidarr: album status %d", resp.StatusCode)
	}

	var albums []struct {
		Title            string `json:"title"`
		ForeignAlbumID   string `json:"foreignAlbumId"`
		ForeignReleaseID string `json:"foreignReleaseId"`
		Monitored        bool   `json:"monitored"`
		Statistics       struct {
			TrackFileCount  int     `json:"trackFileCount"`
			TrackCount      int     `json:"trackCount"`
			PercentOfTracks float64 `json:"percentOfTracks"`
		} `json:"statistics"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&albums); err != nil {
		return nil, err
	}
	entries := make([]LidarrAlbumEntry, 0, len(albums))
	for _, a := range albums {
		entries = append(entries, LidarrAlbumEntry{
			Title:                 a.Title,
			ForeignReleaseGroupID: a.ForeignAlbumID,
			ForeignReleaseID:      a.ForeignReleaseID,
			Status: LidarrAlbumStatus{
				InLidarr:        true,
				Monitored:       a.Monitored,
				HasFiles:        a.Statistics.TrackFileCount > 0,
				PercentOfTracks: a.Statistics.PercentOfTracks,
			},
		})
	}
	if apiLogger != nil {
		apiLogger.Printf("Lidarr: fetched %d albums for artist ID %d", len(entries), artistID)
	}
	return entries, nil
}

// GetArtistAllAlbums returns every album Lidarr knows for the artist,
// keyed by Lidarr's own title. Use this to resolve the *playing* album
// directly instead of going through the MusicBrainz discography titles,
// which may omit compilations or spell titles differently.
func (lc *LidarrClient) GetArtistAllAlbums(artistID int) ([]LidarrAlbumEntry, error) {
	return lc.fetchArtistAlbums(artistID)
}

func (lc *LidarrClient) GetArtistAlbums(artistID int, mbAlbumTitles []string) (map[string]*LidarrAlbumStatus, error) {
	entries, err := lc.fetchArtistAlbums(artistID)
	if err != nil || len(entries) == 0 {
		if len(mbAlbumTitles) == 0 {
			return nil, err
		}
		result := make(map[string]*LidarrAlbumStatus)
		for _, title := range mbAlbumTitles {
			result[title] = &LidarrAlbumStatus{}
		}
		return result, err
	}

	// Match on normalized titles (see NormalizeAlbumName): Lidarr, MusicBrainz
	// and local file tags often disagree on punctuation, case, subtitles and
	// edition suffixes ("Greatest Hits" vs "Janis Joplin's Greatest Hits",
	// "Remastered", etc). Exact string equality misses those.
	result := make(map[string]*LidarrAlbumStatus)
	for _, title := range mbAlbumTitles {
		matched := false
		var st LidarrAlbumStatus
		for _, e := range entries {
			if AlbumNamesMatch(title, e.Title) {
				matched = true
				st = e.Status
				break
			}
		}
		if !matched {
			result[title] = &LidarrAlbumStatus{}
			continue
		}
		s := st
		result[title] = &s
	}
	return result, nil
}

// FindAlbumKey returns the key in titles that matches target under
// AlbumNamesMatch normalization, so local file tags can be resolved against
// MusicBrainz/Lidarr titles that differ in punctuation or edition suffixes.
func FindAlbumKey(titles []string, target string) (string, bool) {
	for _, t := range titles {
		if AlbumNamesMatch(t, target) {
			return t, true
		}
	}
	return "", false
}

func (lc *LidarrClient) OpenArtistURL(mbid string) string {
	return fmt.Sprintf("%s/artist/%s", lc.baseURL, url.PathEscape(mbid))
}

func (lc *LidarrClient) OpenSearchURL(searchTerm string) string {
	return fmt.Sprintf("%s/add/search?term=%s", lc.baseURL, url.PathEscape(searchTerm))
}

func (lc *LidarrClient) OpenSearchByMBID(mbid string) string {
	return lc.OpenSearchURL("mbid:" + mbid)
}
