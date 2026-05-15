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
		return &LidarrArtistStatus{Error: err.Error()}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return &LidarrArtistStatus{Error: "invalid API key"}, fmt.Errorf("lidarr: unauthorized")
	}
	if resp.StatusCode != http.StatusOK {
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
		return &LidarrArtistStatus{InLidarr: false}, nil
	}

	return &LidarrArtistStatus{
		InLidarr:   true,
		Monitored:  artists[0].Monitored,
		ArtistID:   artists[0].ID,
		ArtistName: artists[0].ArtistName,
	}, nil
}

func (lc *LidarrClient) GetArtistAlbums(artistID int, mbAlbumTitles []string) (map[string]*LidarrAlbumStatus, error) {
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
		Title      string `json:"title"`
		Monitored  bool   `json:"monitored"`
		Statistics struct {
			TrackFileCount  int     `json:"trackFileCount"`
			TrackCount      int     `json:"trackCount"`
			PercentOfTracks float64 `json:"percentOfTracks"`
		} `json:"statistics"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&albums); err != nil {
		return nil, err
	}

	type albumInfo struct {
		monitored       bool
		hasFiles        bool
		percentOfTracks float64
	}
	lidarrLookup := make(map[string]albumInfo)
	for _, a := range albums {
		key := strings.ToLower(strings.TrimSpace(a.Title))
		lidarrLookup[key] = albumInfo{
			monitored:       a.Monitored,
			hasFiles:        a.Statistics.TrackFileCount > 0,
			percentOfTracks: a.Statistics.PercentOfTracks,
		}
	}

	result := make(map[string]*LidarrAlbumStatus)
	for _, title := range mbAlbumTitles {
		key := strings.ToLower(strings.TrimSpace(title))
		info, inLidarr := lidarrLookup[key]
		result[title] = &LidarrAlbumStatus{
			InLidarr:        inLidarr,
			Monitored:       info.monitored,
			HasFiles:        info.hasFiles,
			PercentOfTracks: info.percentOfTracks,
		}
	}
	return result, nil
}

func (lc *LidarrClient) OpenArtistURL(mbid string) string {
	return fmt.Sprintf("%s/artist/%s", lc.baseURL, url.PathEscape(mbid))
}

func (lc *LidarrClient) OpenSearchURL(searchTerm string) string {
	return fmt.Sprintf("%s/add/search?term=%s", lc.baseURL, url.PathEscape(searchTerm))
}
