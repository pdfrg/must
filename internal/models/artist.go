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
	LidarrMBID       string
}
