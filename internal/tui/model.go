package tui

import (
	"image"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	termimg "github.com/blacktop/go-termimg"
	"github.com/pdfrg/must/assets"
	"github.com/pdfrg/must/internal/api"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/db"
	pkgimage "github.com/pdfrg/must/internal/image"
	"github.com/pdfrg/must/internal/models"
	"github.com/pdfrg/must/internal/mpv"
	"github.com/pdfrg/must/internal/scanner"
	"github.com/pdfrg/must/internal/tui/modals"
	"github.com/pdfrg/must/internal/tui/widgets"
)

type BottomViewMode int

const (
	BottomPlaylist BottomViewMode = iota
	BottomLyrics
	BottomSyncedLyrics
	BottomArtistBio
	BottomOff
	BottomViewModeCount
)

type ActiveModal int

const (
	ModalNone ActiveModal = iota
	ModalLibrary
	ModalSearch
	ModalHelp
	ModalGallery
	ModalOptions
	ModalSleepTimer
)

type Model struct {
	cfg        *config.Config
	theme      *config.ColorTheme
	styles     *config.ThemeStyles
	keyMap     KeyMap
	paths      []string
	randomMode bool
	noRestore  bool
	autoplay   bool

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

	playbackPos mpv.PlaybackPosition
	audioInfo   *models.AudioInfo

	libraryReady bool
	scanMsg      string
	scanResult   *scanner.ScanResult

	bottomViewMode BottomViewMode
	activeModal    ActiveModal

	artists []string

	lyrics        string
	syncedLyrics  []api.SyncedLyric
	lyricsLoading bool

	artistInfo        *models.ArtistInfo
	artistInfoLoading bool
	artistInfoEventID int64
	artistCache       map[string]*models.ArtistInfo

	pendingLyrics     string
	pendingArtistInfo *models.ArtistInfo
	hasPendingUpdate  bool

	statusMsg   string
	statusIsErr bool
	statusSeq   int

	imageRenderer  *pkgimage.Renderer
	logoImage      image.Image
	imageProtocol  termimg.Protocol
	cellRatio      float64
	fontW          int
	fontH          int
	albumArtStr    string
	albumArtWidth  int
	albumArtHeight int
	albumArtLoaded bool
	logoArtStr     string
	logoArtWidth   int
	logoArtHeight  int
	logoArtLoaded  bool

	artistArtStr     string
	artistArtWidth   int
	artistArtHeight  int
	artistArtLoaded  bool
	artistArtEventID int64

	bottomSectionStartRow int

	notifSentForSong bool

	songStartTime        time.Time
	scrobbleEligible     bool
	prevTrack            *models.Track
	prevScrobbleEligible bool
	prevSongStartTime    time.Time

	layoutOverride      string
	sleepTimer          time.Duration
	sleepRemaining      time.Duration
	sleepTimerActive    bool
	sleepTimerExpiresAt time.Time
	quittingActive      bool
	quittingStartedAt   time.Time

	header         *widgets.Header
	nowPlaying     *widgets.NowPlaying
	playlistWidget *widgets.Playlist
	footer         *widgets.Footer

	libraryModal    *modals.Library
	searchModal     *modals.Search
	helpModal       *modals.Help
	galleryModal    *modals.Gallery
	optionsModal    *modals.Options
	sleepTimerModal *modals.SleepTimer
	viewport        viewport.Model
	viewportReady   bool

	saveInput         textinput.Model
	savingPlaylist    bool
	saveAsRelative    bool
	restoringPlayback bool

	scrobbleStates  map[string]int
	scrobbleFlashAt time.Time
}

func NewModel(cfg *config.Config, theme *config.ColorTheme, paths []string, layoutOverride string, sleepTimer time.Duration, randomMode bool, noRestore bool, autoplay bool) Model {
	styles := config.NewThemeStyles(theme, cfg.TransparentBackground, cfg.DisableTheme, cfg.TerminalPalette)

	m := Model{
		cfg:                 cfg,
		theme:               theme,
		styles:              styles,
		keyMap:              DefaultKeyMap,
		paths:               paths,
		randomMode:          randomMode,
		noRestore:           noRestore,
		autoplay:            autoplay,
		playlist:            []models.Track{},
		currentIndex:        -1,
		repeatMode:          cfg.RepeatMode,
		shuffle:             cfg.Shuffle,
		layoutOverride:      layoutOverride,
		sleepTimer:          sleepTimer,
		sleepRemaining:      sleepTimer,
		sleepTimerActive:    sleepTimer > 0,
		sleepTimerExpiresAt: time.Now().Add(sleepTimer),
		bottomViewMode:      BottomPlaylist,
		activeModal:         ModalNone,
		artistCache:         make(map[string]*models.ArtistInfo),
	}

	m.header = widgets.NewHeader(styles.Header, "must - MUSic TUI")
	m.nowPlaying = widgets.NewNowPlaying(styles, styles.Accent, styles.Cursor, styles.Background)
	m.playlistWidget = widgets.NewPlaylist(styles)
	m.footer = widgets.NewFooter(styles.AccentStyle, styles.MutedStyle)

	m.searchModal = modals.NewSearch(styles, nil)
	m.helpModal = modals.NewHelp(styles, defaultHelpEntries())
	m.viewport = viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))

	m.saveInput = textinput.New()
	m.saveInput.Placeholder = "playlist name"
	m.saveInput.SetWidth(30)

	if cfg.ShowAlbumArt && cfg.Layout != "compact" {
		m.imageRenderer = pkgimage.NewRendererWithProtocol(cfg.ForceProtocol)

		switch cfg.ForceProtocol {
		case "kitty":
			m.imageProtocol = termimg.Kitty
		case "sixel":
			m.imageProtocol = termimg.Sixel
		case "halfblocks":
			m.imageProtocol = termimg.Halfblocks
		case "iterm2":
			m.imageProtocol = termimg.ITerm2
		default:
			m.imageProtocol = termimg.DetectProtocol()
		}

		features := termimg.QueryTerminalFeatures()
		fontW, fontH := features.FontWidth, features.FontHeight
		cellRatio := float64(fontH) / float64(fontW)
		if cellRatio < 1.0 && fontW > 0 && fontH > 0 {
			fontW, fontH = fontH, fontW
			cellRatio = float64(fontH) / float64(fontW)
		}
		if cellRatio <= 0 {
			cellRatio = 2.0
		}
		m.cellRatio = cellRatio
		m.fontW = fontW
		m.fontH = fontH
	} else {
		m.cellRatio = 2.0
	}

	if logoImg, err := pkgimage.LoadImageFromBytes(assets.BubblesLogoPNG); err == nil {
		m.logoImage = logoImg
		if m.imageRenderer != nil {
			m.renderLogoArt(logoImg)
		}
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
	if m.logoArtLoaded {
		cmds = append(cmds, renderAlbumArtAfterDelay())
	}
	if m.sleepTimerActive {
		cmds = append(cmds, tickSleepTimerCmd())
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
		m.updatePlaylist()
	}
}

func (m *Model) updatePlaylist() {
	if len(m.playlist) == 0 {
		m.playlistWidget.SetRows(nil)
		return
	}
	rows := widgets.BuildPlaylistRows(m.playlist, m.currentIndex)
	m.playlistWidget.SetRows(rows)
	m.playlistWidget.SetCurrentIndex(m.currentIndex)
}

func defaultHelpEntries() []modals.HelpEntry {
	return []modals.HelpEntry{
		{Key: "space", Desc: "play/pause"},
		{Key: "n", Desc: "next track"},
		{Key: "p", Desc: "previous track"},
		{Key: "←/→", Desc: "seek -5s/+5s"},
		{Key: "ctrl+r", Desc: "restart song"},
		{Key: "r", Desc: "cycle repeat (off/all/one)"},
		{Key: "s", Desc: "toggle shuffle"},
		{Key: "v/tab", Desc: "cycle bottom view"},
		{Key: "/", Desc: "search library"},
		{Key: "l", Desc: "library browser"},
		{Key: "e", Desc: "enqueue track/album"},
		{Key: "E", Desc: "enqueue highlighted next"},
		{Key: "d", Desc: "delete track from playlist"},
		{Key: "D", Desc: "clear playlist"},
		{Key: "J/K", Desc: "move track down/up"},
		{Key: "g/G", Desc: "move track to top/bottom"},
		{Key: "S", Desc: "save playlist"},
		{Key: "R", Desc: "rescan library"},
		{Key: "y", Desc: "plain lyrics"},
		{Key: "Y", Desc: "synced lyrics"},
		{Key: "u", Desc: "update lyrics/bio"},
		{Key: "i", Desc: "artist bio"},
		{Key: "I", Desc: "artist gallery"},
		{Key: "z", Desc: "sleep timer"},
		{Key: "o", Desc: "options"},
		{Key: "?", Desc: "help"},
		{Key: "q/ctrl+c", Desc: "quit"},
	}
}

func (m Model) layoutMode() string {
	if m.layoutOverride != "" {
		return m.layoutOverride
	}
	return m.cfg.Layout
}
