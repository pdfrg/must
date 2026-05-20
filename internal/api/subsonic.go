package api

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pdfrg/must/internal/models"
)

const subsonicVersion = "1.16.1"
const subsonicClientName = "must"

type SubsonicClient struct {
	baseURL     string
	username    string
	password    string
	serverName  string
	serverBadge string
	httpClient  *http.Client
}

func NewSubsonicClient(serverURL, username, password, serverName, serverBadge string) (*SubsonicClient, error) {
	baseURL := strings.TrimRight(serverURL, "/")
	if !strings.HasPrefix(baseURL, "http") {
		baseURL = "http://" + baseURL
	}

	return &SubsonicClient{
		baseURL:     baseURL,
		username:    username,
		password:    password,
		serverName:  serverName,
		serverBadge: serverBadge,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func md5hex(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

func (c *SubsonicClient) authParams() url.Values {
	salt := randomHex(8)
	token := md5hex(c.password + salt)
	return url.Values{
		"u": {c.username},
		"t": {token},
		"s": {salt},
	}
}

func (c *SubsonicClient) buildURL(endpoint string, params url.Values) string {
	if params == nil {
		params = url.Values{}
	}
	for k, v := range c.authParams() {
		params[k] = v
	}
	params.Set("v", subsonicVersion)
	params.Set("c", subsonicClientName)
	params.Set("f", "json")
	return fmt.Sprintf("%s/rest/%s.view?%s", c.baseURL, endpoint, params.Encode())
}

func (c *SubsonicClient) getJSON(endpoint string, params url.Values, dst any) error {
	u := c.buildURL(endpoint, params)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return fmt.Errorf("subsonic: failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("subsonic: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("subsonic: HTTP %d %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("subsonic: failed to read response: %w", err)
	}

	var wrapper subsonicResponseWrapper
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return fmt.Errorf("subsonic: failed to parse JSON: %w", err)
	}

	if wrapper.SubsonicResponse.Status != "ok" {
		if wrapper.SubsonicResponse.Error != nil {
			return fmt.Errorf("subsonic: %s (code %d)",
				wrapper.SubsonicResponse.Error.Message,
				wrapper.SubsonicResponse.Error.Code)
		}
		return fmt.Errorf("subsonic: status %s", wrapper.SubsonicResponse.Status)
	}

	if dst != nil {
		if err := json.Unmarshal(body, dst); err != nil {
			return fmt.Errorf("subsonic: failed to decode response: %w", err)
		}
	}

	return nil
}

func (c *SubsonicClient) ServerName() string {
	return c.serverName
}

func (c *SubsonicClient) ServerBadge() string {
	return c.serverBadge
}

func (c *SubsonicClient) Ping() error {
	return c.getJSON("ping", nil, nil)
}

func (c *SubsonicClient) GetArtists() (*ArtistsID3, error) {
	var resp struct {
		SubsonicResponse struct {
			Artists *ArtistsID3 `json:"artists"`
		} `json:"subsonic-response"`
	}
	if err := c.getJSON("getArtists", nil, &resp); err != nil {
		return nil, err
	}
	if resp.SubsonicResponse.Artists == nil {
		return nil, fmt.Errorf("subsonic: no artists in response")
	}
	return resp.SubsonicResponse.Artists, nil
}

func (c *SubsonicClient) GetArtist(id string) (*ArtistWithAlbumsID3, error) {
	params := url.Values{"id": {id}}
	var resp struct {
		SubsonicResponse struct {
			Artist *ArtistWithAlbumsID3 `json:"artist"`
		} `json:"subsonic-response"`
	}
	if err := c.getJSON("getArtist", params, &resp); err != nil {
		return nil, err
	}
	if resp.SubsonicResponse.Artist == nil {
		return nil, fmt.Errorf("subsonic: artist not found")
	}
	return resp.SubsonicResponse.Artist, nil
}

func (c *SubsonicClient) GetAlbum(id string) (*AlbumID3WithSongs, error) {
	params := url.Values{"id": {id}}
	var resp struct {
		SubsonicResponse struct {
			Album *AlbumID3WithSongs `json:"album"`
		} `json:"subsonic-response"`
	}
	if err := c.getJSON("getAlbum", params, &resp); err != nil {
		return nil, err
	}
	if resp.SubsonicResponse.Album == nil {
		return nil, fmt.Errorf("subsonic: album not found")
	}
	return resp.SubsonicResponse.Album, nil
}

func (c *SubsonicClient) Search3(query string, artistCount, albumCount, songCount int) (*SearchResult3, error) {
	params := url.Values{
		"query":       {query},
		"artistCount": {fmt.Sprintf("%d", artistCount)},
		"albumCount":  {fmt.Sprintf("%d", albumCount)},
		"songCount":   {fmt.Sprintf("%d", songCount)},
	}
	var resp struct {
		SubsonicResponse struct {
			Result *SearchResult3 `json:"searchResult3"`
		} `json:"subsonic-response"`
	}
	if err := c.getJSON("search3", params, &resp); err != nil {
		return nil, err
	}
	if resp.SubsonicResponse.Result == nil {
		return &SearchResult3{}, nil
	}
	return resp.SubsonicResponse.Result, nil
}

func (c *SubsonicClient) StreamURL(id string) string {
	params := url.Values{"id": {id}, "format": {"raw"}}
	return c.buildURL("stream", params)
}

func (c *SubsonicClient) CoverArtURL(id string) string {
	params := url.Values{"id": {id}}
	return c.buildURL("getCoverArt", params)
}

func (c *SubsonicClient) GetSongsByGenre(genre string, count int) ([]Child, error) {
	params := url.Values{
		"genre": {genre},
		"count": {fmt.Sprintf("%d", count)},
	}
	var resp struct {
		SubsonicResponse struct {
			Songs *struct {
				Song []Child `json:"song"`
			} `json:"songsByGenre"`
		} `json:"subsonic-response"`
	}
	if err := c.getJSON("getSongsByGenre", params, &resp); err != nil {
		return nil, err
	}
	if resp.SubsonicResponse.Songs == nil {
		return nil, nil
	}
	return resp.SubsonicResponse.Songs.Song, nil
}

func (c *SubsonicClient) GetSong(id string) (*Child, error) {
	params := url.Values{"id": {id}}
	var resp struct {
		SubsonicResponse struct {
			Song *Child `json:"song"`
		} `json:"subsonic-response"`
	}
	if err := c.getJSON("getSong", params, &resp); err != nil {
		return nil, err
	}
	if resp.SubsonicResponse.Song == nil {
		return nil, fmt.Errorf("subsonic: song not found")
	}
	return resp.SubsonicResponse.Song, nil
}

func (c *SubsonicClient) GetAlbumList2(listType string, fromYear, toYear, size int, genre string) ([]AlbumID3, error) {
	params := url.Values{
		"type": {listType},
		"size": {fmt.Sprintf("%d", size)},
	}
	if fromYear > 0 {
		params.Set("fromYear", fmt.Sprintf("%d", fromYear))
	}
	if toYear > 0 {
		params.Set("toYear", fmt.Sprintf("%d", toYear))
	}
	if genre != "" {
		params.Set("genre", genre)
	}
	var resp struct {
		SubsonicResponse struct {
			AlbumList2 *struct {
				Album []AlbumID3 `json:"album"`
			} `json:"albumList2"`
		} `json:"subsonic-response"`
	}
	if err := c.getJSON("getAlbumList2", params, &resp); err != nil {
		return nil, err
	}
	if resp.SubsonicResponse.AlbumList2 == nil {
		return nil, nil
	}
	return resp.SubsonicResponse.AlbumList2.Album, nil
}

func (c *SubsonicClient) Scrobble(id string, submission bool, time int64) error {
	params := url.Values{
		"id":         {id},
		"submission": {fmt.Sprintf("%t", submission)},
	}
	if time > 0 {
		params.Set("time", fmt.Sprintf("%d", time))
	}
	return c.getJSON("scrobble", params, nil)
}

func (c *SubsonicClient) GetPlaylists() ([]PlaylistInfo, error) {
	var resp struct {
		SubsonicResponse struct {
			Playlists *struct {
				Playlist []PlaylistInfo `json:"playlist"`
			} `json:"playlists"`
		} `json:"subsonic-response"`
	}
	if err := c.getJSON("getPlaylists", nil, &resp); err != nil {
		return nil, err
	}
	if resp.SubsonicResponse.Playlists == nil {
		return nil, nil
	}
	return resp.SubsonicResponse.Playlists.Playlist, nil
}

func (c *SubsonicClient) GetPlaylist(id string) (*PlaylistWithSongs, error) {
	params := url.Values{"id": {id}}
	var resp struct {
		SubsonicResponse struct {
			Playlist *PlaylistWithSongs `json:"playlist"`
		} `json:"subsonic-response"`
	}
	if err := c.getJSON("getPlaylist", params, &resp); err != nil {
		return nil, err
	}
	if resp.SubsonicResponse.Playlist == nil {
		return nil, fmt.Errorf("subsonic: playlist not found")
	}
	return resp.SubsonicResponse.Playlist, nil
}

func (c *SubsonicClient) ChildToTrack(s Child) models.Track {
	return models.Track{
		Source:      models.SourceSubsonic,
		RemoteID:    s.ID,
		Path:        c.StreamURL(s.ID),
		Title:       s.Title,
		Artist:      s.Artist,
		Album:       s.Album,
		Year:        s.Year,
		Genre:       s.Genre,
		TrackNum:    s.Track,
		DiscNum:     s.DiscNumber,
		Duration:    float64(s.Duration),
		CoverArtID:  s.CoverArt,
		ServerName:  c.serverName,
		ServerBadge: c.serverBadge,
		ContentType: s.ContentType,
		Bitrate:     s.BitRate,
	}
}

func (c *SubsonicClient) ChildrenToTracks(children []Child) []models.Track {
	tracks := make([]models.Track, len(children))
	for i, s := range children {
		tracks[i] = c.ChildToTrack(s)
	}
	return tracks
}

type subsonicError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type subsonicResponseWrapper struct {
	SubsonicResponse struct {
		Status  string         `json:"status"`
		Version string         `json:"version"`
		Error   *subsonicError `json:"error,omitempty"`
		Type    string         `json:"type,omitempty"`
	} `json:"subsonic-response"`
}

type ArtistsID3 struct {
	IgnoredArticles string     `json:"ignoredArticles,omitempty"`
	Index           []IndexID3 `json:"index"`
}

type IndexID3 struct {
	Name   string      `json:"name"`
	Artist []ArtistID3 `json:"artist"`
}

type ArtistID3 struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CoverArt   string `json:"coverArt,omitempty"`
	AlbumCount int    `json:"albumCount"`
}

type ArtistWithAlbumsID3 struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CoverArt   string     `json:"coverArt,omitempty"`
	AlbumCount int        `json:"albumCount"`
	Album      []AlbumID3 `json:"album"`
}

type AlbumID3 struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Artist    string `json:"artist"`
	ArtistID  string `json:"artistId,omitempty"`
	CoverArt  string `json:"coverArt,omitempty"`
	SongCount int    `json:"songCount"`
	Duration  int    `json:"duration"`
	Year      int    `json:"year,omitempty"`
	Genre     string `json:"genre,omitempty"`
	Created   string `json:"created,omitempty"`
	PlayCount int    `json:"playCount,omitempty"`
}

type AlbumID3WithSongs struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Artist    string  `json:"artist"`
	ArtistID  string  `json:"artistId,omitempty"`
	CoverArt  string  `json:"coverArt,omitempty"`
	SongCount int     `json:"songCount"`
	Duration  int     `json:"duration"`
	Year      int     `json:"year,omitempty"`
	Genre     string  `json:"genre,omitempty"`
	Created   string  `json:"created,omitempty"`
	Song      []Child `json:"song"`
}

type Child struct {
	ID           string `json:"id"`
	Parent       string `json:"parent,omitempty"`
	IsDir        bool   `json:"isDir"`
	Title        string `json:"title"`
	Album        string `json:"album"`
	Artist       string `json:"artist"`
	Track        int    `json:"track,omitempty"`
	Year         int    `json:"year,omitempty"`
	CoverArt     string `json:"coverArt,omitempty"`
	Size         int64  `json:"size,omitempty"`
	ContentType  string `json:"contentType,omitempty"`
	Suffix       string `json:"suffix,omitempty"`
	Duration     int    `json:"duration,omitempty"`
	BitRate      int    `json:"bitRate,omitempty"`
	BitDepth     int    `json:"bitDepth,omitempty"`
	SamplingRate int    `json:"samplingRate,omitempty"`
	ChannelCount int    `json:"channelCount,omitempty"`
	Path         string `json:"path,omitempty"`
	DiscNumber   int    `json:"discNumber,omitempty"`
	Genre        string `json:"genre,omitempty"`
	PlayCount    int    `json:"playCount,omitempty"`
	Type         string `json:"type,omitempty"`
	IsVideo      bool   `json:"isVideo"`
}

type SearchResult3 struct {
	Artist []ArtistID3 `json:"artist"`
	Album  []AlbumID3  `json:"album"`
	Song   []Child     `json:"song"`
}

type PlaylistInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SongCount int    `json:"songCount"`
	Duration  int    `json:"duration"`
	Owner     string `json:"owner"`
	Public    bool   `json:"public"`
	Created   string `json:"created"`
}

type PlaylistWithSongs struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	SongCount int     `json:"songCount"`
	Duration  int     `json:"duration"`
	Owner     string  `json:"owner"`
	Public    bool    `json:"public"`
	Created   string  `json:"created"`
	Entry     []Child `json:"entry"`
}
