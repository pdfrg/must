package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pdfrg/must/internal/api"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/ctl"
	"github.com/pdfrg/must/internal/db"
	"github.com/pdfrg/must/internal/mpv"
	"github.com/pdfrg/must/internal/scanner"
	"github.com/pdfrg/must/internal/tui"
	"github.com/pdfrg/must/internal/tui/visualizer"
)

var controlVerbs = map[string]bool{
	"enqueue": true, "e": true,
	"enqueue-next": true, "en": true,
	"play": true, "p": true,
	"pause": true,
	"clear": true, "c": true,
	"status": true, "s": true,
	"remove": true, "rm": true,
	"shuffle": true,
	"repeat":  true,
	"next":    true, "n": true,
	"previous": true, "pr": true,
	"go":     true,
	"rescan": true,
	"save":   true,
	"find":   true, "f": true,
	"library":    true,
	"list":       true,
	"move":       true,
	"playlists":  true,
	"stop":       true,
	"current":    true,
	"replaygain": true,
	"rg":         true,
}

func resolveAlias(verb string) string {
	switch verb {
	case "e":
		return "enqueue"
	case "en":
		return "enqueue-next"
	case "p":
		return "play"
	case "c":
		return "clear"
	case "s":
		return "status"
	case "rm":
		return "remove"
	case "n":
		return "next"
	case "pr":
		return "previous"
	case "f":
		return "find"
	case "rg":
		return "replaygain"
	default:
		return verb
	}
}

var Version = "dev"

func main() {
	var layoutOverride string
	sleepTimerDuration := time.Duration(0)
	var alarmTime time.Time
	var paths []string
	randomMode := false
	repeatMode := ""
	noRestore := false
	autoplay := false

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--help", "-h":
			printHelp()
			return
		case "--version", "-v":
			printVersion()
			return
		case "--lastfm-auth":
			handleLastFMAuth()
			return
		case "--random":
			randomMode = true
		case "--play":
			autoplay = true
		case "--no-restore":
			noRestore = true
		case "--repeat":
			if i+1 < len(args) {
				mode := args[i+1]
				switch mode {
				case "off", "all", "one":
					repeatMode = mode
				default:
					fmt.Fprintf(os.Stderr, "Error: --repeat requires off, all, or one (got %q)\n", mode)
					os.Exit(1)
				}
				i++
			} else {
				repeatMode = "all"
			}
		case "--layout":
			if i+1 < len(args) {
				layoutOverride = args[i+1]
				i++
			} else {
				fmt.Fprintf(os.Stderr, "Error: --layout requires an argument (large, medium, compact, narrow)\n")
				os.Exit(1)
			}
		case "--sleep":
			if i+1 < len(args) {
				d, err := time.ParseDuration(args[i+1])
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: --sleep requires a duration (e.g., 20m, 1.5h)\n")
					os.Exit(1)
				}
				sleepTimerDuration = d
				i++
			} else {
				fmt.Fprintf(os.Stderr, "Error: --sleep requires a duration argument\n")
				os.Exit(1)
			}
		case "--alarm":
			if i+1 < len(args) {
				a, err := parseAlarmTime(args[i+1])
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				alarmTime = a
				i++
			} else {
				fmt.Fprintf(os.Stderr, "Error: --alarm requires a time argument (e.g., 7:20am, 19:20)\n")
				os.Exit(1)
			}
		default:
			if strings.HasPrefix(args[i], "--") {
				fmt.Fprintf(os.Stderr, "Error: unknown flag %q\n", args[i])
				os.Exit(1)
			}
			paths = append(paths, args[i])
		}
	}

	// Check for control command (must play, must pause, must find, etc.)
	if len(paths) > 0 {
		verb := strings.ToLower(paths[0])
		if controlVerbs[verb] {
			ctlCmd := resolveAlias(verb)
			ctlArgs := paths[1:]
			for i, a := range ctlArgs {
				ctlArgs[i] = expandPath(a)
			}

			socketPath := config.GetCtlSocketPath()
			result, err := ctl.SendCommand(socketPath, ctlCmd, ctlArgs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if !result.OK {
				fmt.Fprintf(os.Stderr, "Error: %s\n", result.Error)
				os.Exit(1)
			}
			if result.Data != "" {
				fmt.Println(result.Data)
			}
			return
		}
	}

	cfg, err := config.NewConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Check for existing running instance
	socketPath := config.GetCtlSocketPath()
	if conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond); err == nil {
		_ = conn.Close()
		fmt.Fprintf(os.Stderr, "must is already running. Use 'must status' or similar commands.\n")
		os.Exit(1)
	}
	_ = os.Remove(socketPath)

	if randomMode {
		cfg.Shuffle = true
	}
	if repeatMode != "" {
		cfg.RepeatMode = repeatMode
	}

	initLogging(cfg)

	theme, err := config.LoadTheme(cfg.ColorsFile, cfg.Theme)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load theme: %v\n", err)
		theme = config.DefaultTheme()
	}

	for i, p := range paths {
		expanded := expandPath(p)
		paths[i] = expanded
	}

	if !alarmTime.IsZero() {
		handleAlarmMode(alarmTime)
	}

	m := tui.NewModel(cfg, theme, paths, layoutOverride, sleepTimerDuration, randomMode, noRestore, autoplay)

	p := tea.NewProgram(m)

	ctlServer, err := ctl.StartServer(socketPath, p)
	if err != nil {
		log.Printf("Warning: failed to start control server: %v", err)
	}

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}

	if ctlServer != nil {
		ctlServer.Stop()
	}
	_ = os.Remove(socketPath)
}

func initLogging(cfg *config.Config) {
	logPath := config.GetLogPath()
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return
	}

	log.SetOutput(f)
	log.SetPrefix("[must] ")
	log.SetFlags(log.Ldate | log.Ltime)

	mpv.SetLogger(log.New(f, "[MPV] ", log.Ldate|log.Ltime))
	scanner.SetLogger(log.New(f, "[Scanner] ", log.Ldate|log.Ltime))
	db.SetLogger(log.New(f, "[DB] ", log.Ldate|log.Ltime))
	tui.SetLogger(log.New(f, "[TUI] ", log.Ldate|log.Ltime))
	visualizer.SetLogger(log.New(f, "[VIS] ", log.Ldate|log.Ltime))
	visualizer.SetAudioLogger(log.New(f, "[VIS.AUDIO] ", log.Ldate|log.Ltime))
	visualizer.SetFFTLogger(log.New(f, "[VIS.FFT] ", log.Ldate|log.Ltime))
	api.SetAPILogger(log.New(f, "[API] ", log.Ldate|log.Ltime))
	api.SetScrobbleLogger(log.New(f, "[SCROBBLE] ", log.Ldate|log.Ltime))
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

func handleLastFMAuth() {
	cfg, err := config.NewConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	apiKey := cfg.LastFM.APIKey
	if apiKey == "" {
		apiKey = api.LastFMAPIKey
	}
	secret := cfg.LastFM.SharedSecret
	if secret == "" {
		secret = api.LastFMSharedSecret
	}

	sessionKey, err := api.LastFMDoAuth(apiKey, secret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg.LastFM.SessionKey = sessionKey
	cfg.LastFM.Enabled = true

	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Session key saved to config. Last.fm scrobbling is now enabled.")
}

// parseAlarmTime parses a wall-clock time string and returns the target time.
// Supported formats: 7:20am, 7:20 a.m., 7:20AM, 19:20, 07:20.
// Same-day if time is in the future, next-day if time has passed.
func parseAlarmTime(s string) (time.Time, error) {
	s = strings.ToLower(strings.TrimSpace(s))

	patterns := []struct {
		regex    *regexp.Regexp
		is24Hour bool
	}{
		{regexp.MustCompile(`^(\d{1,2}):(\d{2})\s*(a\.?m\.?|p\.?m\.?)$`), false},
		{regexp.MustCompile(`^(\d{1,2}):(\d{2})$`), true},
	}

	now := time.Now()
	var hour, minute int
	var isPM bool

	for _, p := range patterns {
		match := p.regex.FindStringSubmatch(s)
		if len(match) >= 3 {
			_, _ = fmt.Sscanf(match[1], "%d", &hour)
			_, _ = fmt.Sscanf(match[2], "%d", &minute)

			if len(match) >= 4 && match[3] != "" {
				ampm := strings.ReplaceAll(match[3], ".", "")
				ampm = strings.TrimSpace(ampm)
				isPM = strings.HasPrefix(ampm, "p")
			}

			if hour > 23 || hour < 0 || minute > 59 || minute < 0 {
				continue
			}

			if !p.is24Hour {
				if isPM && hour != 12 {
					hour += 12
				} else if !isPM && hour == 12 {
					hour = 0
				}
			}

			target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
			if !target.After(now) {
				target = target.Add(24 * time.Hour)
			}

			return target, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid alarm time: %q (valid formats: 7:20am, 7:20 a.m., 19:20)", s)
}

// handleAlarmMode blocks until alarmTime, showing a cancelable spinner countdown.
func handleAlarmMode(alarmTime time.Time) {
	duration := time.Until(alarmTime)
	if duration <= 0 {
		return
	}

	fmt.Fprintf(os.Stderr, "Alarm scheduled for %s. Press Ctrl+C to cancel.\n", alarmTime.Format("Mon 3:04 PM"))

	spinners := []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}
	spinnerIdx := 0
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			if now.After(alarmTime) || now.Equal(alarmTime) {
				fmt.Fprintln(os.Stderr, "\rAlarm time reached! Starting must...")
				return
			}

			remaining := alarmTime.Sub(now)
			hrs := int(remaining.Hours())
			mins := int(remaining.Minutes()) % 60
			secs := int(remaining.Seconds()) % 60

			var timeStr string
			if hrs > 0 {
				timeStr = fmt.Sprintf("%d:%02d:%02d", hrs, mins, secs)
			} else {
				timeStr = fmt.Sprintf("%d:%02d", mins, secs)
			}

			fmt.Fprintf(os.Stderr, "\r%c %s remaining  ", spinners[spinnerIdx%len(spinners)], timeStr)
			spinnerIdx++

		case <-sigCh:
			fmt.Fprintln(os.Stderr, "\rAlarm cancelled. Starting now...")
			return
		}
	}
}

func printHelp() {
	help := `must - MUSic TUI: A terminal UI for local music

USAGE:
  must [FLAGS] [PATHS...]
  must <COMMAND> [ARGS...]

PATHS:
  /path/to/song.mp3        Play a single file
  /path/to/album/dir/      Play all audio files in directory
  /path/to/playlist.m3u    Load and play an M3U/M3U8 playlist

FLAGS:
  -h, --help               Show this help message and exit
  -v, --version            Show version information and exit
  --random                 Shuffle playback order
  --play                   Auto-play on launch
  --no-restore             Don't restore last session
  --repeat [off|all|one]   Set repeat mode (default: all if flag given without arg)
  --layout LAYOUT          Set UI layout: large, medium, compact, narrow
  --sleep DURATION         Start sleep timer (e.g., 20m, 1.5h)
  --alarm TIME             Start app at wall-clock time (e.g., 7:20am, 19:20)
  --lastfm-auth            Run Last.fm OAuth authentication flow

CONTROL COMMANDS (when must is already running):
  play [arg] / p [arg]        Replace playlist and play (resume if no arg)
  enqueue <arg> / e <arg>     Add to end of playlist
  enqueue-next <arg> / en <arg>  Insert after current track
  pause                       Toggle play/pause
  next / n                    Skip forward
  previous / pr               Skip backward
  stop                        Stop playback
  clear / c                   Clear playlist (current song finishes)
  remove <pos> / rm <pos>     Remove track at playlist position
  go [pos]                    Jump to playlist position (or show current)
  move <from> <to>            Move track in playlist
  shuffle                     Toggle shuffle
  repeat [all|one|off]        Set repeat mode (or show current)
  replaygain [off|track|album] / rg  Set ReplayGain normalization (or show current)
  status / s                  Show full playback state
  current                     Show now-playing (one line)
  list                        Show full playlist
  find <query> / f <query>    Search library, returns numbered results
                              Prefix: artist:<q>, album:<q>, genre:<q>, year:<y>
                              Subsonic: subsonic:artist:<q>, subsonic:album:<q>,
                                subsonic:song:<q>, subsonic:genre:<q>, subsonic:year:<y>
                              (config server_name prefix also works, e.g. navidrome:<q>)
  library                     Show music directory, library stats, and Subsonic status
  playlists                   List saved and Subsonic playlists
  save <name>                 Save current playlist as .m3u
  rescan                      Rescan music library

ARG resolution for play / enqueue / enqueue-next:
  <n>           Result number from last 'must find'
  /path         File, album directory, or .m3u playlist
  playlist:<n>  Saved playlist from playlists directory
  subsonic:<q>  Search Subsonic server and play (or server_name:<q>)
  artist:<q>    Search and play artist
  album:<q>     Search and play album
  genre:<q>     Search and play genre
  year:<y>      Search and play year or range (1997 or 1995-2000)
  <text>        Free-text FTS5 search

EXAMPLES:
  must                          Launch TUI
  must ~/Music/Radiohead/       Play an album directory (new instance)
  must play artist:radiohead    Search and play artist (existing instance)
  must find radiohead           Search library
  must play 1                   Play first result from last find
  must enqueue ~/Music/song.mp3 Add a file to playlist
  must next                     Skip to next track
  must status                   Show what's playing

CONFIGURATION:
  Config file:     ~/.config/must/config.toml
  Library DB:      ~/.cache/must/library.db
  Log file:        ~/.local/state/must/must.log
`
	fmt.Print(help)
}

func printVersion() {
	goVersion := runtime.Version()
	osArch := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

	version := Version
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			if info.Main.Version != "" && info.Main.Version != "(devel)" {
				version = info.Main.Version
			}
		}
	}

	fmt.Printf("must %s (%s, %s)\n", version, goVersion, osArch)
}
