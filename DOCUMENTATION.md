# must - Documentation

## Usage

### Launching

```
must [FLAGS] [PATHS...]

PATHS:
  /path/to/song.mp3        Play a single file
  /path/to/album/dir/      Play all audio files in directory
  /path/to/playlist.m3u    Load and play an M3U/M3U8 playlist

FLAGS:
  -h, --help               Show help message and exit
  -v, --version            Show version and exit
  --random                 Shuffle playback order
  --play                   Auto-play on launch
  --no-restore             Don't restore last session
  --repeat [off|all|one]   Set repeat mode (default: all if flag given without arg)
  --layout LAYOUT          Set layout: large, medium, compact, narrow
  --sleep DURATION         Start sleep timer (e.g., 20m, 1.5h)
  --alarm TIME             Start app at wall-clock time (e.g., 7:20am, 19:20)
  --lastfm-auth            Run Last.fm OAuth flow
```

### Examples

```bash
must                          Launch with default settings
must --play                   Launch and auto-play
must ~/Music/Radiohead/       Play an album directory
must song.flac                Play a single track
must playlist.m3u             Play a playlist
must --random ~/Music/        Shuffle play entire library
must --repeat one track.mp3   Repeat one track
must --alarm 7:00am           Launch as an alarm (blocks until 7am, then starts playback)
```

### IPC Control Commands

While must is running, you can control it from another terminal:

```
must <COMMAND> [ARGS...]

COMMANDS:
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
```

## Keybindings

| Key | Action |
|-----|--------|
| `space` | Play/pause |
| `n` / `p` | Next / previous track |
| `←` / `→` | Seek -10s / +10s |
| `r` | Cycle repeat (off/all/one) |
| `s` | Toggle shuffle |
| `ctrl+r` | Restart current song |
| `e` | Enqueue (append to playlist) |
| `E` | Enqueue next (after current track) |
| `d` | Delete track from playlist |
| `D` | Clear playlist |
| `J` / `K` | Move track down / up |
| `g` / `G` | Move track to top / bottom |
| `S` | Save playlist to M3U |
| `R` | Rescan library |
| `c` | Copy song info to clipboard |
| `y` | Plain lyrics |
| `Y` | Synced lyrics |
| `i` | Artist bio |
| `I` | Artist gallery |
| `L` | Lidarr browser |
| `o` | Options modal |
| `z` | Sleep timer |
| `V` | Visualizer view |
| `F` | Fullscreen visualizer |
| `v` / `tab` | Cycle views (now-playing, playlist, lyrics, visualizer, etc.) |
| `u` | Update view |
| `/` | Search library |
| `l` | Library browser |
| `?` | Help |
| `esc` | Back / close modal |
| `q` / `ctrl+c` | Quit |


## Configuration

Config file: `~/.config/must/config.toml` (auto-created with defaults on first run)

### Core Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `music_dir` | `~/Music` | Root directory for library scanning (legacy, use music_dirs) |
| `music_dirs` | `["~/Music"]` | Array of root directories for scanning; overrides music_dir |
| `playlist_path_mode` | `relative` | Path format when saving playlists: relative or absolute |
| `repeat_mode` | `off` | Repeat mode: off, all, one |
| `shuffle` | `false` | Shuffle playback order |
| `replaygain_mode` | `off` | ReplayGain volume normalization: off, track, album |
| `restore_on_start` | `true` | Restore last session on startup |
| `autoplay` | `false` | Auto-play a random album on launch |
| `layout` | `large` | UI layout: large, medium, compact, narrow |

### Display & Theme Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `show_album_art` | `true` | Display album art (Kitty > iTerm2 > Sixel > halfblocks) |
| `copy_album_art` | `false` | Save album art to file (for desktop widgets) |
| `album_art_path` | `/tmp/cover.jpg` | Path for album art copy |
| `transparent_background` | `false` | Use terminal's background color |
| `disable_theme` | `false` | Disable all theming, use terminal's default colors |
| `colors_file` | `""` | Path to custom colors.toml, takes priority over theme setting |
| `theme` | `""` | Built-in theme: catppuccin-mocha, gruvbox-dark, dark-red, osaka-jade, synth, basic |
| `force_protocol` | `""` | Force image protocol: kitty, sixel, halfblocks, iterm2 |

### Terminal Palette (when disable_theme is true)

| Setting | Default | Description |
|---------|---------|-------------|
| `[terminal_palette]` | | Palette indices for cursor/accent/muted colors |
| `cursor` | `2` | Palette index for cursor color (0-15) |
| `accent` | `4` | Palette index for accent color (0-15) |
| `muted` | `8` | Palette index for muted color (0-15) |

### Visualizer Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `[visualizer]` | | Audio visualizer settings |
| `mode` | `Segmented` | Default mode: Bars, Braille, ClassicPeak, Wave, Stars, BrailleBars, Rain, Segmented, Binary |
| `show_info` | `fade` | Song info overlay: fade, on, off |
| `info_duration` | `5` | Seconds to show song info overlay |
| `real_audio` | `true` | Use real audio capture (PipeWire/PulseAudio on Linux) |

### API Keys & Integrations

| Setting | Description |
|---------|-------------|
| `theaudiodb_api_key` | Free key `123` works for basic usage |
| `discogs_token` | Discogs personal access token (artist images + higher rate limits) |
| `discogs_key` / `discogs_secret` | Alternative OAuth app auth (requires both) |
| `[lastfm]` | Last.fm scrobbling (run `must --lastfm-auth`) |
| `[listenbrainz]` | ListenBrainz scrobbling |
| `[lidarr]` | Lidarr music manager integration |
| `[subsonic]` | Subsonic-compatible server client (Navidrome, Jellyfin, etc.) |
| `notifications_enabled` | Desktop notifications on song change |
| `notifications_show_art` | Include album art thumbnail in notifications |

### Subsonic Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `[subsonic]` | | Subsonic-compatible server client (Navidrome, Jellyfin, etc.) |
| `enabled` | `false` | Enable Subsonic server integration |
| `url` | `""` | Server base URL (e.g., `http://navidrome.local:4533`) |
| `username` | `""` | Subsonic username |
| `password` | `""` | Subsonic password or hex-encoded token |
| `server_name` | `"Subsonic"` | Display name for the server; used as a prefix in IPC commands (e.g., `navidrome:radiohead`) |
| `server_badge` | `"S"` | 1-2 char badge shown next to remote search results and tracks |

### Audio Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `[audio]` | | Audio output settings |
| `ssh_audio_server` | `""` | Audio server address for SSH forwarding (e.g., `tcp:localhost:4713`) |

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
| Saved playlists | `~/.cache/must/playlists/` |
| Session state | `~/.cache/must/state.json` |
| Log | `~/.local/state/must/must.log` |
| MPV socket | `$XDG_RUNTIME_DIR/mpv/must-socket` |
| IPC control socket | `$XDG_RUNTIME_DIR/must/ctl.sock` |

## Architecture

### SQLite Schema

```sql
CREATE TABLE tracks (
    id INTEGER PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    artist TEXT NOT NULL,
    album TEXT NOT NULL,
    album_artist TEXT NOT NULL DEFAULT '',
    year INTEGER NOT NULL DEFAULT 0,
    genre TEXT NOT NULL DEFAULT '',
    track_num INTEGER NOT NULL DEFAULT 0,
    disc_num INTEGER NOT NULL DEFAULT 0,
    duration REAL NOT NULL DEFAULT 0,
    has_cover_art INTEGER NOT NULL DEFAULT 0,
    file_mod_time INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_tracks_artist ON tracks(artist);
CREATE INDEX idx_tracks_album_artist ON tracks(album_artist);
CREATE INDEX idx_tracks_album ON tracks(album);
CREATE INDEX idx_tracks_path ON tracks(path);

CREATE VIRTUAL TABLE tracks_fts USING fts5(
    title, artist, album, genre,
    content='tracks',
    content_rowid='id',
    tokenize='porter unicode61',
    prefix='2 3'
);
```

### Search Architecture

- **Advanced Search**: SQLite FTS5 with field-specific queries (`artist:radiohead year:1997`) and BM25 weighted ranking (artist=20, title=10, album=5, genre=1)
- **Year Range**: `year:1995-2000` or `year:1997` — combined with FTS query for date-filtered search
- **Browse Filtering**: Fuzzy matching via bubbles list for real-time filtering
- **Porter stemmer**: "correction" matches "corrected"
- **Prefix queries**: `rad*` matches "radio", "radiohead"
- **Fallback**: SQL LIKE for patterns FTS5 can't handle

### Subsonic Search

When configured, Subsonic search is available through IPC commands and the search modal:

- **IPC `find`**: Use `subsonic:artist:<q>`, `subsonic:album:<q>`, `subsonic:song:<q>` (alias `track`/`title`), `subsonic:genre:<q>`, or `subsonic:year:<y>` for field-specific remote search. Plain `subsonic:<q>` does a combined artist/album/song search.
- **Server name alias**: The config `server_name` value can be used as a prefix instead of `subsonic`. Example: with `server_name = "Navidrome"`, use `navidrome:radiohead` — it normalizes to `subsonic:radiohead`.
- **Search modal**: Three source modes cycled via `Ctrl+t`: Local only (`Ctrl+l`), Subsonic only (`Ctrl+s`), or Both combined.
- **Result display**: Remote results are tagged with the configured `server_badge` (e.g., `[S] Artist: Radiohead (9 albums)`). Use result numbers from `must find` to play remote tracks.
- **Streaming**: Playing a Subsonic result streams the audio from the remote server via MPV.

### Audio Info Properties

Queried from MPV at playback time (shown in now-playing view):

| Property | Example |
|----------|---------|
| Codec | flac, mp3, aac |
| Bitrate | 1411 kbps |
| Sample Rate | 44100 Hz |
| Channels | 2 |
| Bit Depth | 16 (lossless only) |

