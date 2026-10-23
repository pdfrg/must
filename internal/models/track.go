package models

import "fmt"

type AlbumEntry struct {
	Name string
	Year int
}

// AlbumEntryWithArtist identifies an album together with its (album) artist.
// Used by discovery features that need to distinguish same-named albums from
// different artists.
type AlbumEntryWithArtist struct {
	Name   string
	Artist string
	Year   int
}

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
	Starred     bool
	CoverArtID  string
	AlbumID     string
	ServerName  string
	ServerBadge string
	ContentType string
	Bitrate     int

	// MusicBrainz identifiers (Picard/Lidarr tags locally, Navidrome
	// musicBrainzId over Subsonic). Empty when unknown; resolution logic
	// must always fall back to name/title matching.
	MBArtistID       string
	MBAlbumArtistID  string
	MBReleaseID      string
	MBReleaseGroupID string
	MBRecordingID    string
}

type AudioInfo struct {
	Codec      string
	Bitrate    float64
	SampleRate int
	Channels   int
	BitDepth   int
}

type ReplayGainData struct {
	TrackGain float64
	TrackPeak float64
	AlbumGain float64
	AlbumPeak float64
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

// EffectiveArtist returns the album artist when set, falling back to the
// per-track artist. Tag schemas (MusicBrainz Picard, Navidrome, Lidarr)
// treat the album artist as the canonical artist for an album, while Artist
// may vary per track (collaborations, compilations, "per-track" credits).
// Artist lookups (bio, MusicBrainz, Lidarr) must use this so a compilation
// like "Janis Joplin's Greatest Hits" resolves to Janis Joplin rather than
// e.g. "Big Brother & The Holding Company" from the playing track's tags.
func (t *Track) EffectiveArtist() string {
	if t == nil {
		return ""
	}
	if t.AlbumArtist != "" {
		return t.AlbumArtist
	}
	return t.Artist
}

func (t *Track) FormatDisplayInfo() string {
	albumYear := t.Album
	if t.Year > 0 {
		albumYear = fmt.Sprintf("%s (%d)", t.Album, t.Year)
	}
	return fmt.Sprintf("%s - %s - %s", t.Artist, albumYear, t.Title)
}
