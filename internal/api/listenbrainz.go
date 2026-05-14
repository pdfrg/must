package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const listenbrainzBaseURL = "https://api.listenbrainz.org/1"

type ListenBrainzTrack struct {
	Artist        string `json:"artist_name"`
	Title         string `json:"track_name"`
	Album         string `json:"release_name"`
	MBID          string `json:"recording_mbid"`
	AdditionalMsg string `json:"additional_info,omitempty"`
}

type ListenBrainzListen struct {
	ListenedAt int64             `json:"listened_at"`
	Track      ListenBrainzTrack `json:"track_metadata"`
}

type ListenBrainzPayload struct {
	ListenType string               `json:"listen_type"`
	Payload    []ListenBrainzListen `json:"payload"`
}

func SubmitListenBrainz(token string, track ListenBrainzTrack, timestamp int64) error {
	listen := ListenBrainzListen{
		ListenedAt: timestamp,
		Track:      track,
	}

	payload := ListenBrainzPayload{
		ListenType: "single",
		Payload:    []ListenBrainzListen{listen},
	}

	return submitListenBrainzRequest(token, payload)
}

func SubmitPlayingNowListenBrainz(token string, track ListenBrainzTrack) error {
	listen := ListenBrainzListen{
		Track: track,
	}

	payload := ListenBrainzPayload{
		ListenType: "playing_now",
		Payload:    []ListenBrainzListen{listen},
	}

	return submitListenBrainzRequest(token, payload)
}

func ValidateListenBrainzToken(token string) (string, error) {
	req, err := http.NewRequest("GET", listenbrainzBaseURL+"/validate-token", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("listenbrainz validate failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("listenbrainz read error: %w", err)
	}

	var result struct {
		Message string `json:"message"`
		Valid   bool   `json:"valid"`
		User    string `json:"user_name"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("listenbrainz parse error: %w", err)
	}

	if !result.Valid {
		return "", fmt.Errorf("invalid listenbrainz token")
	}

	return result.User, nil
}

func submitListenBrainzRequest(token string, payload ListenBrainzPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("listenbrainz marshal error: %w", err)
	}

	req, err := http.NewRequest("POST", listenbrainzBaseURL+"/submit-listens", bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("listenbrainz submit failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("listenbrainz submit error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func GetUserListensListenBrainz(token, username string, count int) ([]ListenBrainzListen, error) {
	url := fmt.Sprintf("%s/user/%s/listens?count=%d", listenbrainzBaseURL, username, count)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("listenbrainz fetch failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Payload struct {
			Listens []ListenBrainzListen `json:"listens"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("listenbrainz parse error: %w", err)
	}

	return result.Payload.Listens, nil
}
