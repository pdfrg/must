package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const musicbrainzBaseURL = "https://musicbrainz.org/ws/2"

type MBAlbum struct {
	Title string
	Year  string
}

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
	result, err := searchArtistMB(artist)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		if apiLogger != nil {
			apiLogger.Printf("MusicBrainz: artist not found: %q", artist)
		}
		return nil, fmt.Errorf("artist not found on musicbrainz: %s", artist)
	}
	if apiLogger != nil {
		apiLogger.Printf("MusicBrainz: found artist %q (ID=%s, country=%s)", result[0].Name, result[0].ID, result[0].Country)
	}
	return &MusicBrainzArtist{
		ID:             result[0].ID,
		Name:           result[0].Name,
		Disambiguation: result[0].Disambiguation,
		Country:        result[0].Country,
	}, nil
}

func searchArtistMB(artistName string) ([]mbArtistEntry, error) {
	doSearch := func(query string) ([]mbArtistEntry, error) {
		reqURL := fmt.Sprintf("%s/artist/?query=%s&fmt=json&limit=10",
			musicbrainzBaseURL, url.QueryEscape(query))

		headers := map[string]string{"Accept": "application/json"}
		body, err := fetchJSON(reqURL, headers)
		if err != nil {
			return nil, err
		}

		var result struct {
			Artists []struct {
				ID             string `json:"id"`
				Name           string `json:"name"`
				Score          int    `json:"score"`
				Type           string `json:"type"`
				Disambiguation string `json:"disambiguation"`
				Country        string `json:"country"`
			} `json:"artists"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}

		var entries []mbArtistEntry
		for _, a := range result.Artists {
			entries = append(entries, mbArtistEntry{
				ID: a.ID, Name: a.Name, Score: a.Score,
				Type: a.Type, Disambiguation: a.Disambiguation, Country: a.Country,
			})
		}
		return entries, nil
	}

	artists, err := doSearch(fmt.Sprintf("artist:\"%s\"", artistName))
	if err != nil {
		if apiLogger != nil {
			apiLogger.Printf("MusicBrainz: search failed for %q: %v", artistName, err)
		}
		return nil, err
	}

	if len(artists) == 0 {
		if apiLogger != nil {
			apiLogger.Printf("MusicBrainz: no results for %q", artistName)
		}
		return nil, nil
	}

	if apiLogger != nil {
		apiLogger.Printf("MusicBrainz: search for %q returned %d results", artistName, len(artists))
	}

	artistLower := strings.ToLower(artistName)
	artistNorm := normalizeForCompare(artistLower)

	for _, a := range artists {
		if strings.ToLower(a.Name) == artistLower {
			return []mbArtistEntry{a}, nil
		}
	}
	for _, a := range artists {
		if normalizeForCompare(strings.ToLower(a.Name)) == artistNorm {
			return []mbArtistEntry{a}, nil
		}
	}
	for _, a := range artists {
		if strings.Contains(strings.ToLower(a.Name), artistLower) {
			return []mbArtistEntry{a}, nil
		}
	}
	for _, a := range artists {
		if a.Type == "Person" || a.Type == "Group" {
			return []mbArtistEntry{a}, nil
		}
	}

	return artists, nil
}

type mbArtistEntry struct {
	ID             string
	Name           string
	Score          int
	Type           string
	Disambiguation string
	Country        string
}

func GetDiscographyMusicBrainz(artistName string, album string) (string, []MBAlbum, error) {
	entries, err := searchArtistMB(artistName)
	if err != nil {
		return "", nil, err
	}
	if len(entries) == 0 {
		if apiLogger != nil {
			apiLogger.Printf("MusicBrainz: discography search returned no results for %q", artistName)
		}
		return "", nil, nil
	}

	mbID := entries[0].ID
	matchedName := entries[0].Name

	time.Sleep(1 * time.Second)

	mbQuery := fmt.Sprintf("arid:%s AND primarytype:Album NOT secondarytype:* AND status:official", mbID)
	reqURL := fmt.Sprintf("%s/release-group/?query=%s&fmt=json&limit=100",
		musicbrainzBaseURL, strings.ReplaceAll(mbQuery, " ", "%20"))

	headers := map[string]string{"Accept": "application/json"}
	body, err := fetchJSON(reqURL, headers)
	if err != nil {
		return "", nil, err
	}

	var rgResult struct {
		ReleaseGroups []struct {
			Title            string `json:"title"`
			FirstReleaseDate string `json:"first-release-date"`
		} `json:"release-groups"`
	}
	if err := json.Unmarshal(body, &rgResult); err != nil {
		return "", nil, err
	}

	type entry struct {
		title string
		year  string
	}
	seen := make(map[string]bool)
	var entries2 []entry
	for _, rg := range rgResult.ReleaseGroups {
		key := strings.ToLower(rg.Title)
		if seen[key] {
			continue
		}
		seen[key] = true

		year := ""
		if len(rg.FirstReleaseDate) >= 4 {
			year = rg.FirstReleaseDate[:4]
		}
		entries2 = append(entries2, entry{rg.Title, year})
	}

	for i := 1; i < len(entries2); i++ {
		for j := i; j > 0; j-- {
			if entries2[j].year == "" {
				continue
			}
			if entries2[j-1].year == "" || entries2[j].year < entries2[j-1].year {
				entries2[j], entries2[j-1] = entries2[j-1], entries2[j]
			}
		}
	}

	if !strings.EqualFold(matchedName, artistName) && album != "" {
		hasMatch := false
		for _, rg := range rgResult.ReleaseGroups {
			if AlbumNamesMatch(album, rg.Title) {
				hasMatch = true
				break
			}
		}
		if !hasMatch {
			return "", nil, nil
		}
	}

	var albums []MBAlbum
	for _, e := range entries2 {
		albums = append(albums, MBAlbum{Title: e.title, Year: e.year})
	}

	if apiLogger != nil {
		apiLogger.Printf("MusicBrainz: discography for %q (ID=%s) = %d albums", matchedName, mbID, len(albums))
	}
	return mbID, albums, nil
}

func GetArtistMusicBrainz(mbid string) (*MusicBrainzLookupResponse, error) {
	apiURL := fmt.Sprintf("%s/artist/%s?fmt=json&inc=url-rels+tags", musicbrainzBaseURL, mbid)

	headers := map[string]string{
		"Accept": "application/json",
	}

	body, err := fetchJSON(apiURL, headers)
	if err != nil {
		if apiLogger != nil {
			apiLogger.Printf("MusicBrainz: lookup for MBID %s failed: %v", mbid, err)
		}
		return nil, fmt.Errorf("musicbrainz lookup failed: %w", err)
	}

	var resp MusicBrainzLookupResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		if apiLogger != nil {
			apiLogger.Printf("MusicBrainz: parse error for MBID %s: %v", mbid, err)
		}
		return nil, fmt.Errorf("musicbrainz parse error: %w", err)
	}

	if apiLogger != nil {
		apiLogger.Printf("MusicBrainz: lookup %q (MBID=%s, country=%s, relations=%d)", resp.Name, mbid, resp.Country, len(resp.Relations))
	}
	return &resp, nil
}
