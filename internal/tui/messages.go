package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pdfrg/must/internal/api"
	"github.com/pdfrg/must/internal/db"
	"github.com/pdfrg/must/internal/models"
	"github.com/pdfrg/must/internal/scanner"
)

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

type sleepTimerTickMsg time.Time

type artistInfoFetchedMsg struct {
	eventID int64
	info    *models.ArtistInfo
}

type restorePlaybackMsg struct {
	position float64
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

func setStatus(m *Model, msg string, isError bool) tea.Cmd {
	m.statusMsg = msg
	m.statusIsErr = isError
	m.statusSeq++
	seq := m.statusSeq
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return statusClearMsg{seq: seq}
	})
}
