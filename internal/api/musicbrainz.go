package api

import (
	"encoding/json"
	"fmt"
)

const musicbrainzBaseURL = "https://musicbrainz.org/ws/2"

type MusicBrainzArtist struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	SortName       string `json:"sort-name"`
	Disambiguation string `json:"disambiguation"`
	Country        string `json:"country"`
	LifeSpan       struct {
		Begin string `json:"begin"`
		End   string `json:"end"`
	} `json:"life-span"`
	Tags []MusicBrainzTag `json:"tags"`
}

type MusicBrainzTag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type MusicBrainzArtistResponse struct {
	Artists []MusicBrainzArtist `json:"artists"`
}

type MusicBrainzLookupResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	SortName       string `json:"sort-name"`
	Country        string `json:"country"`
	Disambiguation string `json:"disambiguation"`
	LifeSpan       struct {
		Begin string `json:"begin"`
		End   string `json:"end"`
	} `json:"life-span"`
	Tags      []MusicBrainzTag      `json:"tags"`
	Relations []MusicBrainzRelation `json:"relations"`
	URLs      []MusicBrainzURL      `json:"urls"`
}

type MusicBrainzRelation struct {
	Type string `json:"type"`
	URL  struct {
		Resource string `json:"resource"`
	} `json:"url"`
}

type MusicBrainzURL struct {
	Resource string `json:"resource"`
	Type     string `json:"type"`
}

func SearchArtistMusicBrainz(artist string) (*MusicBrainzArtist, error) {
	url := buildURL(musicbrainzBaseURL+"/artist", map[string]string{
		"query": fmt.Sprintf("artist:\"%s\"", artist),
		"fmt":   "json",
		"limit": "1",
	})

	headers := map[string]string{
		"Accept": "application/json",
	}

	body, err := fetchJSON(url, headers)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz search failed: %w", err)
	}

	var resp MusicBrainzArtistResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("musicbrainz parse error: %w", err)
	}

	if len(resp.Artists) == 0 {
		return nil, fmt.Errorf("artist not found on musicbrainz: %s", artist)
	}

	return &resp.Artists[0], nil
}

func GetArtistMusicBrainz(mbid string) (*MusicBrainzLookupResponse, error) {
	url := fmt.Sprintf("%s/artist/%s?fmt=json&inc=url-rels+tags", musicbrainzBaseURL, mbid)

	headers := map[string]string{
		"Accept": "application/json",
	}

	body, err := fetchJSON(url, headers)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz lookup failed: %w", err)
	}

	var resp MusicBrainzLookupResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("musicbrainz parse error: %w", err)
	}

	return &resp, nil
}
