package api

import (
	"encoding/json"
	"fmt"
)

const theaudiodbBaseURL = "https://www.theaudiodb.com/api/v1/json"

type TheAudioDBArtist struct {
	IDArtist           string `json:"idArtist"`
	StrArtist          string `json:"strArtist"`
	StrArtistAlt       string `json:"strArtistAlternate"`
	StrLabel           string `json:"strLabel"`
	IntFormedYear      string `json:"intFormedYear"`
	StrGenre           string `json:"strGenre"`
	StrBiographyEN     string `json:"strBiographyEN"`
	StrArtistThumb     string `json:"strArtistThumb"`
	StrArtistLogo      string `json:"strArtistLogo"`
	StrArtistFanart    string `json:"strArtistFanart"`
	StrArtistWideThumb string `json:"strArtistWideThumb"`
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

func SearchArtistTheAudioDB(apiKey, artist string) (*TheAudioDBArtist, error) {
	url := fmt.Sprintf("%s/%s/search.php?s=%s", theaudiodbBaseURL, apiKey, artist)

	body, err := fetchJSON(url, nil)
	if err != nil {
		return nil, fmt.Errorf("theaudiodb search failed: %w", err)
	}

	var resp theaudiodbArtistResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("theaudiodb parse error: %w", err)
	}

	if len(resp.Artists) == 0 {
		return nil, fmt.Errorf("artist not found: %s", artist)
	}

	return &resp.Artists[0], nil
}

func GetAlbumsByArtistIDTheAudioDB(apiKey, artistID string) ([]TheAudioDBAlbum, error) {
	url := fmt.Sprintf("%s/%s/album.php?i=%s", theaudiodbBaseURL, apiKey, artistID)

	body, err := fetchJSON(url, nil)
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
	a, err := SearchArtistTheAudioDB(apiKey, artist)
	if err != nil {
		return "", err
	}

	albums, err := GetAlbumsByArtistIDTheAudioDB(apiKey, a.IDArtist)
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
