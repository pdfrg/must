package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	path                  string
	MusicDir              string                `toml:"music_dir" comment:"root directory for library scanning (default: ~/Music)"`
	RepeatMode            string                `toml:"repeat_mode" comment:"repeat mode: off, all, one (default: off)"`
	Shuffle               bool                  `toml:"shuffle" comment:"shuffle playback order (default: false)"`
	RestoreOnStart        bool                  `toml:"restore_on_start" comment:"restore last session's playlist and position on startup (default: true)"`
	Autoplay              bool                  `toml:"autoplay" comment:"auto-play a random album when launched with no paths (default: false)"`
	ShowAlbumArt          bool                  `toml:"show_album_art" comment:"display album art for each song\nuses the best supported image protocol with auto fallback\nkitty > iterm2 > sixel > unicode (default: true)"`
	CopyAlbumArt          bool                  `toml:"copy_album_art" comment:"save album art to file, useful for desktop/statusbar widgets (default: false)"`
	AlbumArtPath          string                `toml:"album_art_path" comment:"file path for album art copy, needed if copy_album_art is true (default: /tmp/cover.jpg)"`
	ColorsFile            string                `toml:"colors_file" comment:"custom colors.toml file path, takes priority over theme setting\nfallback order: colors_file > theme > omarchy current theme > Catppuccin Mocha (default: '')"`
	Theme                 string                `toml:"theme" comment:"built-in theme name\ncatppuccin-mocha, gruvbox-dark, dark-red, osaka-jade, synth, basic (default: '')"`
	TransparentBackground bool                  `toml:"transparent_background" comment:"use terminal's default background color (default: false)"`
	DisableTheme          bool                  `toml:"disable_theme" comment:"disable all theming, use terminal's default colors (default: false)"`
	TerminalPalette       TerminalPaletteConfig `toml:"terminal_palette" comment:"palette indices for cursor/accent/muted when disable_theme is true"`
	DiscogsToken          string                `toml:"discogs_token" comment:"Discogs personal access token\nenables artist images + higher API rate limits\nget one at: https://www.discogs.com/settings/developers\nalternative: set discogs_key + discogs_secret, or env vars DISCOGS_TOKEN / DISCOGS_KEY + DISCOGS_SECRET (default: '')"`
	DiscogsKey            string                `toml:"discogs_key" comment:"Discogs consumer key (developer app auth)\nalternative to discogs_token, requires both key and secret (default: '')"`
	DiscogsSecret         string                `toml:"discogs_secret" comment:"Discogs consumer secret (developer app auth)\nalternative to discogs_token, requires both key and secret (default: '')"`
	TheAudioDBApiKey      string                `toml:"theaudiodb_api_key" comment:"TheAudioDB API key for artist info and album art fallback\nfree key '123' is used by default\nget a custom key at: https://www.theaudiodb.com/apikey (default: '123')"`
	LastFM                LastFMConfig          `toml:"lastfm" comment:"Last.fm scrobbling\nrun 'must --lastfm-auth' once to obtain a session key"`
	ListenBrainz          ListenBrainzConfig    `toml:"listenbrainz" comment:"ListenBrainz scrobbling\ntoken found at: https://listenbrainz.org/profile/"`
	Lidarr                LidarrConfig          `toml:"lidarr" comment:"Lidarr music collection manager\nshows artist/album monitoring status, opens Lidarr web UI\napi_key from: Lidarr Settings > General"`
	Visualizer            VisualizerConfig      `toml:"visualizer" comment:"audio visualizer settings"`
	NotificationsEnabled  bool                  `toml:"notifications_enabled" comment:"show desktop notifications on song changes (default: false)"`
	NotificationsShowArt  bool                  `toml:"notifications_show_art" comment:"include album art thumbnail in notifications (default: true)"`
	Layout                string                `toml:"layout" comment:"UI layout mode\nlarge: full layout with all elements (default)\nmedium: no bottom view (no playlist/lyrics/visualizer)\ncompact: no album art, no bottom view, mini footer\nnarrow: album art top-left, now playing below, mini footer (default: large)"`
	ForceProtocol         string                `toml:"force_protocol" comment:"force a specific image protocol instead of auto-detecting\noptions: kitty, sixel, halfblocks, iterm2, or empty for auto-detect (default: '')"`
	Audio                 AudioConfig           `toml:"audio" comment:"audio output settings"`
}

type AudioConfig struct {
	SSHAudioServer string `toml:"ssh_audio_server" comment:"audio server address for SSH audio forwarding\nused with --audio-forward flag when running over SSH\nexample: \"tcp:localhost:4713\" (default: '')"`
}

type LastFMConfig struct {
	Enabled      bool   `toml:"enabled" comment:"enable Last.fm scrobbling (default: false)"`
	APIKey       string `toml:"api_key" comment:"Last.fm API key from https://www.last.fm/api/account/create (default: '')"`
	SharedSecret string `toml:"shared_secret" comment:"Last.fm shared secret from your API account (default: '')"`
	SessionKey   string `toml:"session_key" comment:"obtained via 'must --lastfm-auth' (default: '')"`
}

type ListenBrainzConfig struct {
	Enabled bool   `toml:"enabled" comment:"enable ListenBrainz scrobbling (default: false)"`
	Token   string `toml:"token" comment:"user token from https://listenbrainz.org/profile/ (default: '')"`
}

type LidarrConfig struct {
	Enabled bool   `toml:"enabled" comment:"enable Lidarr integration (default: false)"`
	URL     string `toml:"url" comment:"Lidarr base URL (e.g., http://localhost:8686)"`
	APIKey  string `toml:"api_key" comment:"Lidarr API key from Settings > General"`
}

type TerminalPaletteConfig struct {
	Cursor int `toml:"cursor" comment:"palette index for cursor color (0-15, default: 2 = green)"`
	Accent int `toml:"accent" comment:"palette index for accent color (0-15, default: 4 = blue)"`
	Muted  int `toml:"muted" comment:"palette index for muted color (0-15, default: 8 = gray)"`
}

type VisualizerConfig struct {
	Mode         string `toml:"mode" comment:"default visualizer mode\nBars, Braille, ClassicPeak, Wave, Stars, BrailleBars, Rain, Segmented, Binary (default: Bars)"`
	ShowInfo     string `toml:"show_info" comment:"song info overlay in fullscreen visualizer\nfade, on, off (default: fade)"`
	InfoDuration int    `toml:"info_duration" comment:"seconds to show song info overlay (default: 5)"`
	RealAudio    bool   `toml:"real_audio" comment:"use real audio capture\nLinux: PipeWire (pw-record) or PulseAudio (parecord)\nWindows: WASAPI loopback\nmacOS: not supported (default: true)"`
}

func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		MusicDir:       filepath.Join(homeDir, "Music"),
		RepeatMode:     "off",
		Shuffle:        false,
		RestoreOnStart: true,
		Autoplay:       false,
		ShowAlbumArt:   true,
		AlbumArtPath:   filepath.Join(os.TempDir(), "cover.jpg"),
		CopyAlbumArt:   false,
		Visualizer: VisualizerConfig{
			Mode:         "Segmented",
			ShowInfo:     "fade",
			InfoDuration: 5,
			RealAudio:    true,
		},
		NotificationsEnabled:  false,
		NotificationsShowArt:  true,
		Layout:                "large",
		ForceProtocol:         "",
		TransparentBackground: false,
		DisableTheme:          false,
		TerminalPalette: TerminalPaletteConfig{
			Cursor: 2,
			Accent: 4,
			Muted:  8,
		},
	}
}

func NewConfig() (*Config, error) {
	cfg := DefaultConfig()

	configDir := filepath.Join(xdg.ConfigHome, "must")
	cfg.path = filepath.Join(configDir, "config.toml")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := cfg.Load(); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := cfg.Save(); err != nil {
			return nil, fmt.Errorf("failed to save default config: %w", err)
		}
	}

	return cfg, nil
}

func (c *Config) Load() error {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return err
	}

	var temp map[string]any
	if err := toml.Unmarshal(data, &temp); err != nil {
		return fmt.Errorf("failed to parse config TOML: %w", err)
	}

	if err := toml.Unmarshal(data, c); err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}

	c.applyDefaults()
	return nil
}

func (c *Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	header := []byte("# must configuration file\n\n")
	output := make([]byte, len(header)+len(data))
	copy(output, header)
	copy(output[len(header):], data)

	if err := os.WriteFile(c.path, output, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (c *Config) applyDefaults() {
	defaults := DefaultConfig()

	if c.MusicDir == "" {
		c.MusicDir = defaults.MusicDir
	}
	if c.RepeatMode != "off" && c.RepeatMode != "all" && c.RepeatMode != "one" {
		c.RepeatMode = defaults.RepeatMode
	}
	if c.AlbumArtPath == "" {
		c.AlbumArtPath = defaults.AlbumArtPath
	}
	if c.Visualizer.Mode == "" {
		c.Visualizer.Mode = defaults.Visualizer.Mode
	}
	if c.Visualizer.ShowInfo == "" {
		c.Visualizer.ShowInfo = defaults.Visualizer.ShowInfo
	} else if c.Visualizer.ShowInfo != "fade" && c.Visualizer.ShowInfo != "on" && c.Visualizer.ShowInfo != "off" {
		c.Visualizer.ShowInfo = defaults.Visualizer.ShowInfo
	}
	if c.Visualizer.InfoDuration <= 0 {
		c.Visualizer.InfoDuration = defaults.Visualizer.InfoDuration
	}

	validLayouts := map[string]bool{"large": true, "medium": true, "compact": true, "narrow": true}
	if c.Layout == "" || !validLayouts[c.Layout] {
		c.Layout = defaults.Layout
	}

	if c.ForceProtocol != "" {
		validProtocols := map[string]bool{"kitty": true, "sixel": true, "halfblocks": true, "iterm2": true}
		if !validProtocols[c.ForceProtocol] {
			c.ForceProtocol = ""
		}
	}
}

func GetScrobbleCacheDir() string {
	return filepath.Join(xdg.CacheHome, "must", "scrobbles")
}

func GetLibraryDBPath() string {
	return filepath.Join(xdg.CacheHome, "must", "library.db")
}

func GetArtCacheDir() string {
	return filepath.Join(xdg.CacheHome, "must", "art")
}

func GetStatePath() string {
	return filepath.Join(xdg.CacheHome, "must", "state.json")
}

func GetLogPath() string {
	return filepath.Join(xdg.StateHome, "must", "must.log")
}

func GetMPVSocketPath() string {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = os.TempDir()
	}
	return filepath.Join(runtimeDir, "mpv", "must-socket")
}

func GetPlaylistSaveDir() string {
	return filepath.Join(xdg.CacheHome, "must", "playlists")
}

func GetPlaylistSavePath(name string) string {
	return filepath.Join(GetPlaylistSaveDir(), name+".m3u")
}

func InSSHSession() bool {
	return os.Getenv("SSH_CONNECTION") != ""
}
