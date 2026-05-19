# must - Documentation

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
| `←` / `→` | Seek -10s/+10s |
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
- **Browse Filtering**: Fuzzy matching via bubbles list for real-time filtering
- **Porter stemmer**: "correction" matches "corrected"

### Album Art Pipeline

1. Embedded cover art (dhowden/tag at scan time, cached to XDG cache)
2. `cover.jpg` / `folder.jpg` / `front.jpg` / `artwork.jpg` in same directory
3. Any `.jpg`/`.png` in same directory
4. Parent directory art (multi-disc albums)
5. Online fallback (TheAudioDB, Discogs) — cached after first fetch
