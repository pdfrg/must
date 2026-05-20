package models

import "fmt"

type TrackSource string

const (
	SourceLocal    TrackSource = "local"
	SourceSubsonic TrackSource = "subsonic"
)

type Track struct {
	ID          int64
	Path        string
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	Year        int
	Genre       string
	TrackNum    int
	DiscNum     int
	Duration    float64
	HasCoverArt bool
	FileModTime int64

	Source      TrackSource
	RemoteID    string
	CoverArtID  string
	ServerName  string
	ServerBadge string
	ContentType string
	Bitrate     int
}

type AudioInfo struct {
	Codec      string
	Bitrate    float64
	SampleRate int
	Channels   int
	BitDepth   int
}

func (t *Track) GetDurationFormatted() string {
	totalSeconds := int(t.Duration)
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func (t *Track) FormatDisplayInfo() string {
	albumYear := t.Album
	if t.Year > 0 {
		albumYear = fmt.Sprintf("%s (%d)", t.Album, t.Year)
	}
	return fmt.Sprintf("%s - %s - %s", t.Artist, albumYear, t.Title)
}
