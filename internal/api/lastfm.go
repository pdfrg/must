package api

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const lastfmBaseURL = "https://ws.audioscrobbler.com/2.0/"

// LastFMAPIKey and LastFMSharedSecret are injected at build time via -ldflags
// (see Makefile). Config values take precedence; these serve as fallback defaults
// so users don't need their own Last.fm API app.
var LastFMAPIKey = ""
var LastFMSharedSecret = ""

type LastFMSession struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	Subscriber bool   `json:"subscriber"`
}

type LastFMTrack struct {
	Artist   string `json:"artist"`
	Title    string `json:"name"`
	Album    string `json:"album"`
	MBID     string `json:"mbid"`
	Duration int    `json:"duration"`
}

func GetLastFMAuthToken(apiKey, secret string) (string, error) {
	params := map[string]string{
		"method":  "auth.gettoken",
		"api_key": apiKey,
	}

	sig := generateLastFMSig(params, secret)
	params["api_sig"] = sig
	params["format"] = "json"

	body, err := lastFMRequest(params)
	if err != nil {
		return "", err
	}

	var resp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("lastfm parse error: %w", err)
	}

	return resp.Token, nil
}

func GetLastFMAuthURL(apiKey, token string) string {
	return fmt.Sprintf("https://www.last.fm/api/auth/?api_key=%s&token=%s", apiKey, token)
}

func GetLastFMSession(apiKey, secret, token string) (*LastFMSession, error) {
	params := map[string]string{
		"method":  "auth.getsession",
		"api_key": apiKey,
		"token":   token,
	}

	sig := generateLastFMSig(params, secret)
	params["api_sig"] = sig
	params["format"] = "json"

	body, err := lastFMRequest(params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Session LastFMSession `json:"session"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("lastfm parse error: %w", err)
	}

	return &resp.Session, nil
}

func ScrobbleLastFM(apiKey, secret, sessionKey string, track LastFMTrack, timestamp int64) error {
	params := map[string]string{
		"method":    "track.scrobble",
		"api_key":   apiKey,
		"sk":        sessionKey,
		"artist":    track.Artist,
		"track":     track.Title,
		"album":     track.Album,
		"timestamp": fmt.Sprintf("%d", timestamp),
	}

	if track.MBID != "" {
		params["mbid"] = track.MBID
	}

	sig := generateLastFMSig(params, secret)
	params["api_sig"] = sig
	params["format"] = "json"

	_, err := lastFMRequest(params)
	return err
}

func UpdateNowPlayingLastFM(apiKey, secret, sessionKey string, track LastFMTrack) error {
	params := map[string]string{
		"method":  "track.updatenowplaying",
		"api_key": apiKey,
		"sk":      sessionKey,
		"artist":  track.Artist,
		"track":   track.Title,
		"album":   track.Album,
	}

	if track.Duration > 0 {
		params["duration"] = fmt.Sprintf("%d", track.Duration)
	}

	sig := generateLastFMSig(params, secret)
	params["api_sig"] = sig
	params["format"] = "json"

	_, err := lastFMRequest(params)
	return err
}

func lastFMRequest(params map[string]string) ([]byte, error) {
	u, _ := url.Parse(lastfmBaseURL)
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lastfm request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("lastfm read error: %w", err)
	}

	return body, nil
}

func generateLastFMSig(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "format" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(params[k])
	}
	b.WriteString(secret)

	hash := md5.Sum([]byte(b.String()))
	return hex.EncodeToString(hash[:])
}
