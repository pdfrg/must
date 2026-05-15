package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

const discogsBaseURL = "https://api.discogs.com"

type DiscogsArtist struct {
	ID          int            `json:"id"`
	Name        string         `json:"name"`
	Profile     string         `json:"profile"`
	ImageURLs   []DiscogsImage `json:"images"`
	ReleasesURL string         `json:"releases_url"`
}

func (d *DiscogsArtist) PrimaryImage() string {
	for _, img := range d.ImageURLs {
		if img.Type == "primary" && img.URI != "" {
			return img.URI
		}
	}
	for _, img := range d.ImageURLs {
		if img.URI != "" {
			return img.URI
		}
	}
	return ""
}

func (d *DiscogsArtist) GalleryURLs() []string {
	var urls []string
	for _, img := range d.ImageURLs {
		if img.URI != "" {
			urls = append(urls, img.URI)
		}
	}
	return urls
}

type DiscogsImage struct {
	Type   string `json:"type"`
	URI    string `json:"uri"`
	URI150 string `json:"uri150"`
}

type DiscogsSearchResult struct {
	ID    int    `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
	Thumb string `json:"thumb"`
}

type DiscogsSearchResponse struct {
	Results []DiscogsSearchResult `json:"results"`
}

func SearchArtistDiscogs(token, key, secret, artist string) (*DiscogsArtist, error) {
	searchURL := buildURL(discogsBaseURL+"/database/search", map[string]string{
		"q":    artist,
		"type": "artist",
	})

	headers := map[string]string{
		"Authorization": buildDiscogsAuth(token, key, secret),
	}

	body, err := fetchJSON(searchURL, headers)
	if err != nil {
		return nil, fmt.Errorf("discogs search failed: %w", err)
	}

	var resp DiscogsSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("discogs parse error: %w", err)
	}

	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("artist not found on discogs: %s", artist)
	}

	artistID := resp.Results[0].ID
	return GetDiscogsArtist(token, key, secret, artistID)
}

func GetDiscogsArtist(token, key, secret string, id int) (*DiscogsArtist, error) {
	url := fmt.Sprintf("%s/artists/%d", discogsBaseURL, id)

	headers := map[string]string{
		"Authorization": buildDiscogsAuth(token, key, secret),
	}

	body, err := fetchJSON(url, headers)
	if err != nil {
		return nil, fmt.Errorf("discogs artist fetch failed: %w", err)
	}

	var artist DiscogsArtist
	if err := json.Unmarshal(body, &artist); err != nil {
		return nil, fmt.Errorf("discogs parse error: %w", err)
	}

	return &artist, nil
}

func buildDiscogsAuth(token, key, secret string) string {
	parts := []string{"Discogs"}
	if token != "" {
		parts = append(parts, fmt.Sprintf("token=%s", token))
	}
	if key != "" && secret != "" {
		parts = append(parts, fmt.Sprintf("key=%s, secret=%s", key, secret))
	}
	return strings.Join(parts, " ")
}
