package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	termimg "github.com/blacktop/go-termimg"
	"github.com/pdfrg/must/internal/api"
	"github.com/pdfrg/must/internal/db"
	"github.com/pdfrg/must/internal/models"
	"github.com/pdfrg/must/internal/scanner"
)

type subsonicSearchResultsMsg struct {
	tracks  []models.Track
	artists []api.ArtistID3
	albums  []api.AlbumID3
	query   string
	err     error
}

type subsonicArtistsMsg struct {
	artists []api.ArtistID3
	err     error
}

type subsonicArtistAlbumsMsg struct {
	albums []api.AlbumID3
	err    error
}

type subsonicAlbumTracksMsg struct {
	tracks []models.Track
	err    error
}

type subsonicGenresMsg struct {
	genres []api.GenreID3
	err    error
}

type subsonicGenreAlbumsMsg struct {
	genreName string
	albums    []api.AlbumID3
	err       error
}

type progressTickMsg time.Time

type scanCompleteMsg struct {
	result *scanner.ScanResult
	err    error
	db     *db.LibraryDB
}

type trackChangedMsg struct{}

type statusClearMsg struct {
	seq int
}

type audioInfoMsg struct {
	info *models.AudioInfo
}

type imageLoadedMsg struct {
	imageData []byte
	trackPath string
	err       error
}

type renderAlbumArtMsg struct{}

type artistImageLoadedMsg struct {
	eventID   int64
	imageData []byte
	trackPath string
	err       error
}

type renderArtistArtMsg struct{}

type onlineArtFetchedMsg struct {
	trackPath string
	err       error
}

type notificationSentMsg struct{}

type themeChangedMsg struct {
	path string
}

type lyricsFetchedMsg struct {
	plain  string
	synced []api.SyncedLyric
	err    error
}

type visTickMsg time.Time

type sleepTimerTickMsg time.Time
type quitTickMsg time.Time

type scrobbleResult struct {
	Service string
	Success bool
}

type scrobbleResultMsg struct {
	results []scrobbleResult
}

type artistInfoFetchedMsg struct {
	eventID int64
	info    *models.ArtistInfo
}

type restorePlaybackMsg struct {
	position float64
}

func tickVisCmd() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return visTickMsg(t)
	})
}

func tickProgressCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return progressTickMsg(t)
	})
}

func tickSleepTimerCmd() tea.Cmd {
	return tea.Tick(time.Minute, func(t time.Time) tea.Msg {
		return sleepTimerTickMsg(t)
	})
}

func tickQuitCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return quitTickMsg(t)
	})
}

func renderArtistArtAfterDelay() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return renderArtistArtMsg{}
	})
}

func clearKittyImagesCmd() tea.Cmd {
	return tea.Raw(termimg.ClearAllString())
}

func clearKittyImagesCmdIf(protocol termimg.Protocol) tea.Cmd {
	if protocol != termimg.Kitty {
		return nil
	}
	return clearKittyImagesCmd()
}

func setStatus(m *Model, msg string, isError bool) tea.Cmd {
	m.statusMsg = msg
	m.statusIsErr = isError
	m.statusSeq++
	seq := m.statusSeq
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return statusClearMsg{seq: seq}
	})
}
