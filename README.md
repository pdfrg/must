[![CI](https://github.com/pdfrg/must/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/pdfrg/must/actions/workflows/ci.yml)

**If you like must, please check out [rptui](https://github.com/pdfrg/rptui), a Radio Paradise TUI.**

# must - MUSic TUI

<img src="assets/must_bubbles_logo.png" alt="must icon" width="200" align="left" style="margin-right: 20px; margin-top: -20px; margin-bottom: 20px;">

### A fast, beautiful TUI music player for your local music library. Built with Go + Bubble Tea.

![must default view](assets/must_default_view.png)

**See additional [SCREENSHOTS](SCREENSHOTS.md)**

## There's a million music players, why must?

1. TUI speed.

2. Prominent high-res album art _in the terminal_.

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

## Supported Formats

MP3, FLAC, OGG, Opus, M4A, AAC, WMA, WAV

## Documentation

See [DOCUMENTATION.md](DOCUMENTATION.md) for cli usage, keybindings, configuration reference, album art/artist image priority, XDG paths, search architecture, and database schema.

## Attribution

Audio visualizations: [cliamp](https://github.com/bjarneo/cliamp). Awesome music player with retro Winamp style in the terminal.

## License

MIT
