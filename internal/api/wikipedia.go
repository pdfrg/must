package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

const wikiBaseURL = "https://en.wikipedia.org/api/rest_v1"

type WikipediaSummary struct {
	Title       string              `json:"title"`
	Extract     string              `json:"extract"`
	Thumbnail   *WikipediaThumbnail `json:"thumbnail"`
	Description string              `json:"description"`
	URL         string              `json:"content_urls,omitempty"`
}

type WikipediaThumbnail struct {
	Source string `json:"source"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type WikipediaResponse struct {
	Title       string              `json:"title"`
	Extract     string              `json:"extract"`
	Thumbnail   *WikipediaThumbnail `json:"thumbnail"`
	Description string              `json:"description"`
	ContentURLs struct {
		Desktop struct {
			Page string `json:"page"`
		} `json:"desktop"`
	} `json:"content_urls"`
}

func GetArtistSummary(artist string) (*WikipediaSummary, error) {
	pageTitle := formatWikiTitle(artist)
	url := fmt.Sprintf("%s/page/summary/%s", wikiBaseURL, pageTitle)

	headers := map[string]string{
		"Accept": "application/json; charset=utf-8",
	}

	body, err := fetchJSON(url, headers)
	if err != nil {
		if apiLogger != nil {
			apiLogger.Printf("Wikipedia: fetch failed for %q: %v", artist, err)
		}
		return nil, fmt.Errorf("wikipedia fetch failed: %w", err)
	}

	var resp WikipediaResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		if apiLogger != nil {
			apiLogger.Printf("Wikipedia: parse error for %q: %v", artist, err)
		}
		return nil, fmt.Errorf("wikipedia parse error: %w", err)
	}

	if apiLogger != nil {
		apiLogger.Printf("Wikipedia: found summary for %q (title=%q, has_thumbnail=%v)", artist, resp.Title, resp.Thumbnail != nil)
	}

	summary := &WikipediaSummary{
		Title:       resp.Title,
		Extract:     resp.Extract,
		Thumbnail:   resp.Thumbnail,
		Description: resp.Description,
		URL:         resp.ContentURLs.Desktop.Page,
	}

	return summary, nil
}

func formatWikiTitle(artist string) string {
	words := strings.Fields(artist)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, "_")
}
