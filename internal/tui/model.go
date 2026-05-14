package tui

import (
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	termimg "github.com/blacktop/go-termimg"
	"github.com/pdfrg/must/internal/api"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/db"
	"github.com/pdfrg/must/internal/image"
	"github.com/pdfrg/must/internal/models"
	"github.com/pdfrg/must/internal/mpv"
	"github.com/pdfrg/must/internal/scanner"
)

type ViewMode int

const (
	ViewLibrary ViewMode = iota
	ViewPlaylist
	ViewLyrics
	ViewSyncedLyrics
	ViewArtistBio
	ViewHelp
)

type FocusPane int

const (
	FocusArtists FocusPane = iota
	FocusAlbums
	FocusTracks
)

type Model struct {
	cfg        *config.Config
	theme      *config.ColorTheme
	styles     *config.ThemeStyles
	keyMap     KeyMap
	paths      []string
	randomMode bool

	width  int
	height int

	mpvBackend   *mpv.MPVBackend
	libraryDB    *db.LibraryDB
	themeWatcher *config.ThemeWatcher

	playlist     []models.Track
	currentIndex int
	repeatMode   string
	shuffle      bool
	shuffleOrder []int

	playing bool
	paused  bool
	volume  float64
	muted   bool

	playbackPos mpv.PlaybackPosition
	audioInfo   *models.AudioInfo

	libraryReady bool
	scanMsg      string
	scanResult   *scanner.ScanResult

	viewMode  ViewMode
	focusPane FocusPane

	artists     []string
	albums      []string
	albumTracks []models.Track

	artistCursor       int
	artistScrollOffset int
	albumCursor        int
	albumScrollOffset  int

	searching          bool
	searchInput        textinput.Model
	searchResults      []models.Track
	searchCursor       int
	searchScrollOffset int

	lyrics        string
	syncedLyrics  []api.SyncedLyric
	lyricsLoading bool

	artistBio        string
	artistBioTitle   string
	artistBioURL     string
	artistBioLoading bool

	statusMsg   string
	statusIsErr bool
	statusSeq   int

	imageRenderer  *image.Renderer
	imageProtocol  termimg.Protocol
	albumArtStr    string
	albumArtWidth  int
	albumArtHeight int
	albumArtLoaded bool

	songStartTime        time.Time
	scrobbleEligible     bool
	prevTrack            *models.Track
	prevScrobbleEligible bool
	prevSongStartTime    time.Time

	layoutOverride string
	sleepTimer     time.Duration
	sleepRemaining time.Duration
}

func NewModel(cfg *config.Config, theme *config.ColorTheme, paths []string, layoutOverride string, sleepTimer time.Duration, randomMode bool) Model {
	styles := config.NewThemeStyles(theme, cfg.TransparentBackground, cfg.DisableTheme, cfg.TerminalPalette)

	si := textinput.New()
	si.Prompt = "/"
	si.Placeholder = "artist:radiohead year:1997"
	si.CharLimit = 200

	m := Model{
		cfg:            cfg,
		theme:          theme,
		styles:         styles,
		keyMap:         DefaultKeyMap,
		paths:          paths,
		randomMode:     randomMode,
		playlist:       []models.Track{},
		currentIndex:   -1,
		repeatMode:     cfg.RepeatMode,
		shuffle:        cfg.Shuffle,
		volume:         100,
		layoutOverride: layoutOverride,
		sleepTimer:     sleepTimer,
		sleepRemaining: sleepTimer,
		viewMode:       ViewLibrary,
		focusPane:      FocusArtists,
		searchInput:    si,
		searchResults:  []models.Track{},
	}

	if cfg.ShowAlbumArt && cfg.Layout != "compact" {
		m.imageRenderer = image.NewRenderer()
		m.imageProtocol = termimg.DetectProtocol()
	}

	themeWatcher := config.NewThemeWatcher(cfg.ColorsFile)
	if err := themeWatcher.Start(); err == nil {
		m.themeWatcher = themeWatcher
	}

	m.mpvBackend = mpv.NewMPVBackend()
	if cfg.Audio.SSHAudioServer != "" {
		m.mpvBackend.SetPulseServer(cfg.Audio.SSHAudioServer)
	}

	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tickProgressCmd(),
		scanLibraryCmd(m.cfg),
	}
	if m.themeWatcher != nil {
		cmds = append(cmds, watchThemeCmd(m.themeWatcher))
	}
	return tea.Batch(cmds...)
}

func (m *Model) loadCLIPaths() {
	if len(m.paths) == 0 || m.libraryDB == nil {
		return
	}
	tracks := loadPathsIntoPlaylist(m.paths, m.libraryDB)
	if len(tracks) > 0 {
		m.playlist = tracks
		m.shuffleOrder = nil
		if m.shuffle {
			m.shuffleOrder = shuffleIndices(len(m.playlist))
		}
	}
}
