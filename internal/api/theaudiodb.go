package api

import (
	"encoding/json"
	"fmt"
	"net/url"
)

const theaudiodbBaseURL = "https://www.theaudiodb.com/api/v1/json"
const theaudiodbFreeKey = "123"

type TheAudioDBArtist struct {
	IDArtist           string `json:"idArtist"`
	StrArtist          string `json:"strArtist"`
	StrArtistAlt       string `json:"strArtistAlternate"`
	StrLabel           string `json:"strLabel"`
	IntFormedYear      string `json:"intFormedYear"`
	StrGenre           string `json:"strGenre"`
	StrBiography       string `json:"strBiography"`
	StrArtistThumb     string `json:"strArtistThumb"`
	StrArtistLogo      string `json:"strArtistLogo"`
	StrArtistFanart    string `json:"strArtistFanart"`
	StrArtistFanart2   string `json:"strArtistFanart2"`
	StrArtistFanart3   string `json:"strArtistFanart3"`
	StrArtistFanart4   string `json:"strArtistFanart4"`
	StrArtistWideThumb string `json:"strArtistWideThumb"`
}

func (a *TheAudioDBArtist) FanArts() []string {
	var urls []string
	for _, fa := range []string{a.StrArtistFanart, a.StrArtistFanart2, a.StrArtistFanart3, a.StrArtistFanart4} {
		if fa != "" {
			urls = append(urls, fa)
		}
	}
	return urls
}

type TheAudioDBAlbum struct {
	IDAlbum          string `json:"idAlbum"`
	StrAlbum         string `json:"strAlbum"`
	StrArtist        string `json:"strArtist"`
	IntYearReleased  string `json:"intYearReleased"`
	StrAlbumThumb    string `json:"strAlbumThumb"`
	StrDescriptionEN string `json:"strDescriptionEN"`
}

type theaudiodbArtistResponse struct {
	Artists []TheAudioDBArtist `json:"artists"`
}

type theaudiodbAlbumResponse struct {
	Album []TheAudioDBAlbum `json:"album"`
}

func theaudiodbKey(apiKey string) string {
	if apiKey == "" {
		return theaudiodbFreeKey
	}
	return apiKey
}

func SearchArtistTheAudioDB(apiKey, artistName, albumName string) (*TheAudioDBArtist, error) {
	key := theaudiodbKey(apiKey)

	if albumName != "" {
		reqURL := fmt.Sprintf("%s/%s/searchalbum.php?s=%s&a=%s",
			theaudiodbBaseURL, key,
			url.QueryEscape(artistName), url.QueryEscape(albumName))

		body, err := fetchJSON(reqURL, nil)
		if err == nil {
			var albumResult struct {
				Album []struct {
					IDArtist    string `json:"idArtist"`
					StrAlbum    string `json:"strAlbum"`
					Description string `json:"strDescription"`
				} `json:"album"`
			}
			if json.Unmarshal(body, &albumResult) == nil && len(albumResult.Album) > 0 {
				idArtist := albumResult.Album[0].IDArtist
				if idArtist != "" {
					result, err := fetchTheAudioDBArtistByID(key, idArtist)
					if err == nil && result != nil {
						return result, nil
					}
				}
			}
		}
	}

	reqURL := fmt.Sprintf("%s/%s/search.php?s=%s",
		theaudiodbBaseURL, key,
		url.QueryEscape(artistName))

	body, err := fetchJSON(reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("theaudiodb search failed: %w", err)
	}

	var resp theaudiodbArtistResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("theaudiodb parse error: %w", err)
	}

	if len(resp.Artists) == 0 {
		return nil, fmt.Errorf("artist not found: %s", artistName)
	}

	return &resp.Artists[0], nil
}

func fetchTheAudioDBArtistByID(apiKey, artistID string) (*TheAudioDBArtist, error) {
	reqURL := fmt.Sprintf("%s/%s/artist.php?i=%s", theaudiodbBaseURL, apiKey, artistID)

	body, err := fetchJSON(reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("theaudiodb artist lookup failed: %w", err)
	}

	var resp theaudiodbArtistResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("theaudiodb parse error: %w", err)
	}

	if len(resp.Artists) == 0 {
		return nil, fmt.Errorf("artist ID not found: %s", artistID)
	}

	return &resp.Artists[0], nil
}

func GetAlbumsByArtistIDTheAudioDB(apiKey, artistID string) ([]TheAudioDBAlbum, error) {
	key := theaudiodbKey(apiKey)
	reqURL := fmt.Sprintf("%s/%s/album.php?i=%s", theaudiodbBaseURL, key, artistID)

	body, err := fetchJSON(reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("theaudiodb albums failed: %w", err)
	}

	var resp theaudiodbAlbumResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("theaudiodb parse error: %w", err)
	}

	return resp.Album, nil
}

func FetchAlbumArtURLTheAudioDB(apiKey, artist, album string) (string, error) {
	key := theaudiodbKey(apiKey)
	a, err := SearchArtistTheAudioDB(key, artist, album)
	if err != nil {
		return "", err
	}

	albums, err := GetAlbumsByArtistIDTheAudioDB(key, a.IDArtist)
	if err != nil {
		return "", err
	}

	for _, al := range albums {
		if al.StrAlbum == album && al.StrAlbumThumb != "" {
			return al.StrAlbumThumb, nil
		}
	}

	for _, al := range albums {
		if al.StrAlbumThumb != "" {
			return al.StrAlbumThumb, nil
		}
	}

	return "", fmt.Errorf("no album art found on TheAudioDB for %s - %s", artist, album)
}
