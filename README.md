[![CI](https://github.com/pdfrg/must/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/pdfrg/must/actions/workflows/ci.yml)

**If you like must, please check out [rptui](https://github.com/pdfrg/rptui), a Radio Paradise TUI.**

# must - MUSic TUI

<img src="assets/must_bubbles_logo.png" alt="must icon" width="200" align="left" style="margin-right: 20px; margin-top: -20px; margin-bottom: 20px;">

### A fast, beautiful TUI music player for your local music library. Built with Go + Bubble Tea.

![must default view](assets/must_default_view.png)

**See additional [SCREENSHOTS](SCREENSHOTS.md)**

## There's a million music players, why must?

1. TUI speed — incredibly fast, responsive, keyboard-driven.

2. Prominent high-res album art _in the terminal_.

3. Easy keybindings, always visible in footer. No memorization needed.

4. Lyrics and artist info view, with artist thumb, image gallery, discography, and bio.

5. Fuzzy search all or specific tags (artist, album, year, genre).

6. Omarchy theme integration with live reloads.

7. IPC control — control must from the command line while it's running.

8. Simple, doesn't try to do everything.

## Features

- **Local Music Library**: Scan and browse your music collection with a 3-column browser (artists, albums, tracks), genre browsing, and field-specific search
- **Smart Search**: FTS5-powered full-text search with field queries (`artist:radiohead year:1997`) and year range filtering
- **MPV Backend**: Full gapless audio playback with seek, repeat (off/all/one), shuffle, progress tracking, and ReplayGain normalization
- **Lyrics**: Fetch plain and synced lyrics from [LRCLib](https://lrclib.net/)
- **Artist Info**: Bios from TheAudioDB, Discogs, and Wikipedia. Discographies from MusicBrainz. Artist images from local files or online APIs.
- **Album Art**: Smart terminal image support via go-termimg (Kitty, iTerm2, Sixel, halfblocks fallback). Local-first with online fallback.
- **Scrobbling**: Last.fm and ListenBrainz support
- **Lidarr Integration**: View artist/album monitoring status, open in Lidarr
- **Visualizer**: 9 real-time audio visualizations (bars, braille, wave, stars, rain, etc.)
- **Themes**: 6 built-in themes, custom colors.toml, automatic Omarchy theme detection with live-reloads
- **Playlist Management**: Reorder (J/K/g/G), save (S), delete (d), clear (D), enqueue next (E), move to top/bottom
- **Session Restore**: Automatically restores last session on startup
- **Sleep Timer & Alarm Clock**: Fall asleep or wake up to your music
- **4 Layouts**: `large` (default), `medium`, `compact`, `narrow`
- **IPC Control**: Control a running must instance from the terminal (`must play`, `must next`, `must find radiohead`, etc.)
- **Options Modal**: Adjust replaygain, view, and visualizer settings on the fly

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

## Supported Formats

MP3, FLAC, OGG, Opus, M4A, AAC, WMA, WAV

## Documentation

See [DOCUMENTATION.md](DOCUMENTATION.md) for cli usage, IPC control commands, keybindings, configuration reference, album art/artist image priority, XDG paths, search architecture, and database schema.

## Attribution

Audio visualizations: [cliamp](https://github.com/bjarneo/cliamp). Awesome music player with retro Winamp style in the terminal.

## License

MIT
