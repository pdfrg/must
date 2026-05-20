package ctl

import "github.com/pdfrg/must/internal/models"

type CtlMessage struct {
	Cmd      string
	Args     []string
	ResultCh chan<- CtlResult
}

func (CtlMessage) Msg() {}

type CtlResult struct {
	OK    bool
	Data  string
	Error string
}

type SearchResultType string

const (
	ResultTrack         SearchResultType = "track"
	ResultArtist        SearchResultType = "artist"
	ResultAlbum         SearchResultType = "album"
	ResultGenre         SearchResultType = "genre"
	ResultYear          SearchResultType = "year"
	ResultSubsonicTrack SearchResultType = "subsonic_track"
)

type SearchResult struct {
	Index   int
	Type    SearchResultType
	Display string

	TrackID    int64
	TrackPath  string
	Title      string
	ArtistName string
	AlbumName  string
	GenreName  string
	Year       int
	YearEnd    int

	TrackCount int

	SubsonicTrack *TrackRef
}

type TrackRef struct {
	models.Track
}
