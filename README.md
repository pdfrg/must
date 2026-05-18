[![CI](https://github.com/pdfrg/must/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/pdfrg/must/actions/workflows/ci.yml)

**If you like must, please check out [rptui](https://github.com/pdfrg/rptui), a Radio Paradise TUI.**

# must — MUSic TUI

<img src="assets/must_bubbles_logo.png" alt="must icon" width="200" align="left" style="margin-right: 20px; margin-top: -20px; margin-bottom: 20px;">

### A fast, beautiful TUI music player for your local music library. Built with Go + Bubble Tea.

## There's a million music players, why must?

1. TUI speed.

2. Prominent album art.

3. Easy keybindings, no need to memorize, always visible in footer or hints text.

4. Lyrics and artist info view, with artist thumb, image gallery, discography, and bio.

5. Fuzzy search all or specific tags.  Super easy to find what you want.

6. Omarchy theme integration with live reloads.

7. Simple, doesn't try to do everything.

## Features

- **Local Music Library**: Scan and browse your music directory with a 3-column browser (artists/genres, albums, tracks)
- **Smart Search**: FTS5-powered full-text search with field queries (`artist:radiohead year:1997`) and fuzzy browse filtering
- **MPV Backend**: Full audio playback with seek, repeat (off/all/one), shuffle, and progress tracking
- **Lyrics**: Fetch plain and synced lyrics from [LRCLib](https://lrclib.net/)
- **Artist Info**: Bios from TheAudioDB, Discogs, and Wikipedia. Discographies from MusicBrainz. Artist images from local files or online APIs.
- **Album Art**: Smart terminal image support via go-termimg (Kitty, iTerm2, Sixel, halfblocks fallback). Local-first with online fallback.
- **Scrobbling**: Last.fm and ListenBrainz support
- **Lidarr Integration**: View artist/album monitoring status, open in Lidarr
- **Visualizer**: 9 real-time audio visualizations (bars, braille, wave, etc.)
- **Themes**: 6 built-in themes, custom colors.toml, automatic Omarchy theme detection with live-reloads
- **Playlist Management**: Reorder (J/K), save (S), delete (d), clear (D), enqueue next (E)
- **Session Restore**: Automatically restores last session on startup
- **Sleep Timer**: Fall asleep to your music
- **4 Layouts**: `large` (default), `medium`, `compact`, `narrow`

## Installation

### Prerequisites

- **mpv** — Required for audio playback
- **Go 1.23+** — To build from source
- **Any NerdFont** — For proper symbol display

### Build from Source

```bash
git clone https://github.com/pdfrg/must.git
cd must
go build -o must ./cmd/must
```

### Install with Go

```bash
go install github.com/pdfrg/must/cmd/must@latest
```

## Usage

```
must [FLAGS] [PATHS...]

PATHS:
  /path/to/song.mp3       Play a single file
  /path/to/album/dir/     Play all audio files in directory
  /path/to/playlist.m3u   Load and play an M3U/M3U8 playlist

FLAGS:
  -h, --help              Show help message
  -v, --version           Show version
  --random                Shuffle playback order
  --play                  Auto-play on launch
  --no-restore            Don't restore last session
  --repeat [off|all|one]  Set repeat mode
  --layout LAYOUT         Set layout: large, medium, compact, narrow
  --sleep DURATION        Start sleep timer (e.g., 20m, 1.5h)
  --lastfm-auth           Run Last.fm OAuth flow
```

### Examples

```bash
must                          Launch with default settings
must --play                   Launch and auto-play
must ~/Music/Albums/Radiohead/   Play an album
must song.flac                Play a single track
must playlist.m3u             Play a playlist
must --random ~/Music/        Shuffle play entire library
must --repeat one track.mp3   Repeat one track
```

## Keybindings

| Key | Action |
|-----|--------|
| `space` | Play/pause |
| `n` / `p` | Next/previous track |
| `←` / `→` | Seek -5s/+5s |
| `r` | Cycle repeat (off/all/one) |
| `s` | Toggle shuffle |
| `v` | Cycle bottom view |
| `/` | Search library |
| `l` | Library browser |
| `e` | Enqueue (append to playlist) |
| `E` | Enqueue next (after current track) |
| `d` | Delete track from playlist |
| `D` | Clear playlist |
| `J` / `K` | Move track down/up |
| `S` | Save playlist to M3U |
| `R` | Rescan library |
| `u` | Plain lyrics |
| `U` | Synced lyrics |
| `i` | Artist bio |
| `I` | Artist gallery |
| `?` | Help |
| `q` / `ctrl+c` | Quit |

## Configuration

Config file: `~/.config/must/config.toml` (auto-created with defaults on first run)

### Core Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `music_dir` | `~/Music` | Root directory for library scanning |
| `repeat_mode` | `off` | Repeat mode: off, all, one |
| `shuffle` | `false` | Shuffle playback order |
| `restore_on_start` | `true` | Restore last session on startup |
| `autoplay` | `false` | Auto-play a random album on launch |
| `layout` | `large` | UI layout: large, medium, compact, narrow |

### Display Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `show_album_art` | `true` | Display album art (Kitty > iTerm2 > Sixel > halfblocks) |
| `copy_album_art` | `false` | Save album art to file (for desktop widgets) |
| `album_art_path` | `/tmp/cover.jpg` | Path for album art copy |
| `transparent_background` | `false` | Use terminal's background color |
| `disable_theme` | `false` | Disable theming, use terminal palette |
| `force_protocol` | `""` | Force image protocol: kitty, sixel, halfblocks, iterm2 |

### API Keys

| Setting | Description |
|---------|-------------|
| `theaudiodb_api_key` | Free key `2` works for basic usage |
| `discogs_token` | Personal access token for artist images |
| `[lastfm]` | Last.fm scrobbling (run `must --lastfm-auth`) |
| `[listenbrainz]` | ListenBrainz scrobbling |
| `[lidarr]` | Lidarr music manager integration |

## Album Art Priority

1. Embedded cover art (from tags)
2. `cover.jpg` / `folder.jpg` / `front.jpg` / `artwork.jpg` in track's directory
3. Any `.jpg`/`.png` in track's directory
4. Parent directory art (multi-disc albums)
5. Online fallback (TheAudioDB) — cached after first fetch

## Artist Image Priority

1. Local `artist.jpg`/`artist.png` in the artist directory (`Artist/artist.jpg`)
2. Online fallback (TheAudioDB, Discogs, Wikipedia thumbnail)

## XDG Paths

| Component | Path |
|-----------|------|
| Config | `~/.config/must/config.toml` |
| Library DB | `~/.cache/must/library.db` |
| Scrobble cache | `~/.cache/must/scrobbles/` |
| Album art cache | `~/.cache/must/art/` |
| Playlists | `~/.cache/must/playlists/` |
| Session state | `~/.cache/must/state.json` |
| Log | `~/.local/state/must/must.log` |
| MPV socket | `$XDG_RUNTIME_DIR/mpv/must-socket` |

## Supported Formats

MP3, FLAC, OGG, Opus, M4A, AAC, WMA, WAV

## License

MIT
