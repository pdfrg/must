package models

type LidarrAlbumInfo struct {
	InLidarr        bool
	Monitored       bool
	HasFiles        bool
	PercentOfTracks float64
}

type ArtistInfo struct {
	Bio              string
	BioSource        string
	ThumbnailURL     string
	ThumbSource      string
	GalleryURLs      []string
	GallerySource    string
	Discography      string
	DiscoSource      string
	PageURL          string
	AlbumDescription string
	AlbumSource      string
	LidarrInLidarr   bool
	LidarrMonitored  bool
	LidarrArtistID   int
	LidarrArtistName string
	LidarrError      string
	LidarrAlbums     map[string]LidarrAlbumInfo
	// LidarrAllAlbums holds every album Lidarr reports for the artist, keyed
	// by Lidarr's own title. Unlike LidarrAlbums (keyed by MusicBrainz
	// discography titles), this lets the footer resolve the playing album
	// directly even when MB omits or renames it (compilations, etc).
	LidarrAllAlbums map[string]LidarrAlbumInfo
	// Exact MusicBrainz ID indexes into Lidarr's album rows: release-group
	// ID (musicbrainz_releasegroupid tag, /album/<id> URLs) and release ID
	// (musicbrainz_albumid tag). These bypass title matching entirely.
	LidarrAlbumsByRGID      map[string]LidarrAlbumInfo
	LidarrAlbumsByReleaseID map[string]LidarrAlbumInfo
	LidarrMBID              string
}
