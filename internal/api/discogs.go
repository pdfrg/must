package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const discogsBaseURL = "https://api.discogs.com"

type DiscogsArtist struct {
	ID        int            `json:"id"`
	Name      string         `json:"name"`
	Profile   string         `json:"profile"`
	ImageURLs []DiscogsImage `json:"images"`
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
	discogsArtist, err := GetDiscogsArtist(token, key, secret, artistID)
	if err != nil {
		return nil, err
	}

	discogsArtist.Profile = cleanDiscogsProfile(discogsArtist.Profile)
	return discogsArtist, nil
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
	if token != "" {
		return "Discogs token=" + token
	}
	if key != "" && secret != "" {
		return fmt.Sprintf("Discogs key=%s, secret=%s", key, secret)
	}
	return ""
}

var (
	discogsIDTagRegex         = regexp.MustCompile(`\[(a|r|l|m)(=?)(\d+)\]`)
	discogsNamedTagRegex      = regexp.MustCompile(`\[(a|r|l)=([^\]]+)\]`)
	discogsURLTagRegex        = regexp.MustCompile(`\[url=[^\]]+\](.*?)\[/url\]`)
	discogsBoldItalicTagRegex = regexp.MustCompile(`\[/?(?:b|i)\]`)
	discogsUnderlineTagRegex  = regexp.MustCompile(`\[/?(?:u)\]`)
	discogsCapitalTagRegex    = regexp.MustCompile(`\[[A-Z]=[^\]]*\]`)
)

func cleanDiscogsProfile(text string) string {
	text = discogsIDTagRegex.ReplaceAllString(text, "")
	text = discogsNamedTagRegex.ReplaceAllString(text, "$2")
	text = discogsURLTagRegex.ReplaceAllString(text, "$1")
	text = discogsBoldItalicTagRegex.ReplaceAllString(text, "")
	text = discogsUnderlineTagRegex.ReplaceAllString(text, "")
	text = discogsCapitalTagRegex.ReplaceAllString(text, "")
	text = strings.TrimSpace(text)
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`\s([,.;:!?])`).ReplaceAllString(text, "$1")
	return text
}
