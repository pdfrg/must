package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const lrclibBaseURL = "https://lrclib.net/api"

type LRCLibLyric struct {
	ID           int     `json:"id"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

type SyncedLyric struct {
	Time    float64
	Content string
}

func SearchLyrics(artist, title string) ([]LRCLibLyric, error) {
	url := buildURL(lrclibBaseURL+"/search", map[string]string{
		"artist_name": artist,
		"track_name":  title,
	})

	body, err := fetchJSON(url, nil)
	if err != nil {
		return nil, fmt.Errorf("lrclib search failed: %w", err)
	}

	var lyrics []LRCLibLyric
	if err := json.Unmarshal(body, &lyrics); err != nil {
		return nil, fmt.Errorf("lrclib parse error: %w", err)
	}

	return lyrics, nil
}

func GetLyricsByID(id int) (*LRCLibLyric, error) {
	url := fmt.Sprintf("%s/get/%d", lrclibBaseURL, id)

	body, err := fetchJSON(url, nil)
	if err != nil {
		return nil, fmt.Errorf("lrclib get failed: %w", err)
	}

	var lyric LRCLibLyric
	if err := json.Unmarshal(body, &lyric); err != nil {
		return nil, fmt.Errorf("lrclib parse error: %w", err)
	}

	return &lyric, nil
}

func GetBestLyrics(artist, title, album string, duration float64) (*LRCLibLyric, error) {
	results, err := SearchLyrics(artist, title)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no lyrics found")
	}

	durationSec := int(duration)

	for _, r := range results {
		if r.SyncedLyrics != "" && abs(int(r.Duration)-durationSec) <= 2 {
			return &r, nil
		}
	}

	for _, r := range results {
		if r.PlainLyrics != "" && albumMatch(r.AlbumName, album) {
			return &r, nil
		}
	}

	for _, r := range results {
		if r.PlainLyrics != "" {
			return &r, nil
		}
	}

	return &results[0], nil
}

func ParseSyncedLyrics(syncedLyrics string) []SyncedLyric {
	if syncedLyrics == "" {
		return nil
	}

	var result []SyncedLyric
	lrcRegex := regexp.MustCompile(`^\[(\d{2}):(\d{2})(?:\.(\d{2,3}))?\](.*)$`)

	for _, line := range strings.Split(syncedLyrics, "\n") {
		line = strings.TrimSpace(line)
		matches := lrcRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		mins, _ := strconv.Atoi(matches[1])
		secs, _ := strconv.Atoi(matches[2])
		ms := 0
		if matches[3] != "" {
			ms, _ = strconv.Atoi(matches[3])
			if len(matches[3]) == 2 {
				ms *= 10
			}
		}

		timeVal := float64(mins*60+secs) + float64(ms)/1000.0
		content := strings.TrimSpace(matches[4])

		result = append(result, SyncedLyric{
			Time:    timeVal,
			Content: content,
		})
	}

	return result
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func albumMatch(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	return a != "" && b != "" && (a == b || strings.Contains(a, b) || strings.Contains(b, a))
}
