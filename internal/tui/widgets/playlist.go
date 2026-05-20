package widgets

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/models"
)

type Playlist struct {
	styles     *config.ThemeStyles
	width      int
	height     int
	tracks     []models.Track
	currentIdx int
	cursor     int
	scrollOff  int
}

func NewPlaylist(styles *config.ThemeStyles) *Playlist {
	return &Playlist{
		styles:     styles,
		currentIdx: -1,
		cursor:     0,
	}
}

func (p *Playlist) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p *Playlist) UpdateStyles(styles *config.ThemeStyles) {
	p.styles = styles
}

func (p *Playlist) SetRows(rows []TrackRow) {
	if len(rows) == 0 {
		p.tracks = nil
		return
	}
	p.tracks = make([]models.Track, len(rows))
	for i, r := range rows {
		p.tracks[i] = r.Track
	}
}

func (p *Playlist) SetCurrentIndex(idx int) {
	p.currentIdx = idx
}

func (p *Playlist) SetCursor(cursor int) {
	if cursor < 0 {
		cursor = 0
	}
	if p.tracks != nil && cursor >= len(p.tracks) {
		cursor = len(p.tracks) - 1
	}
	p.cursor = cursor
	p.ensureVisible()
}

func (p *Playlist) GetCursor() int {
	return p.cursor
}

func (p *Playlist) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if p.cursor > 0 {
				p.cursor--
				p.ensureVisible()
			}
		case "down", "j":
			if p.tracks != nil && p.cursor < len(p.tracks)-1 {
				p.cursor++
				p.ensureVisible()
			}
		case "pgup":
			ps := max(p.visibleHeight()-1, 1)
			p.cursor = max(p.cursor-ps, 0)
			p.ensureVisible()
		case "pgdown":
			if p.tracks == nil {
				return nil
			}
			ps := max(p.visibleHeight()-1, 1)
			p.cursor = min(p.cursor+ps, len(p.tracks)-1)
			p.ensureVisible()
		case "home":
			p.cursor = 0
			p.scrollOff = 0
		case "end":
			if p.tracks != nil {
				p.cursor = len(p.tracks) - 1
				p.ensureVisible()
			}
		}
	}
	return nil
}

func (p Playlist) View() string {
	if len(p.tracks) == 0 {
		return p.styles.MutedStyle.Render(" No tracks in playlist")
	}

	headerBg := lightenColor(p.styles.Background, 0.30)
	headerStyle := p.styles.MutedStyle.Background(lipgloss.Color(headerBg))

	const (
		posW      = 4
		playingW  = 2
		durWidth  = 8
		yearWidth = 5
	)

	albumMultiDisc := p.albumIsMultiDisc()

	trkW := 4
	for _, t := range p.tracks {
		multi := albumMultiDisc[albumKey(t)]
		if multi && t.DiscNum > 0 {
			n := t.TrackNum
			if n == 0 {
				n = 99
			}
			w := len(fmt.Sprintf("%d/%d", t.DiscNum, n))
			if w > trkW {
				trkW = w
			}
		} else if t.TrackNum > 0 {
			w := len(fmt.Sprintf("%d", t.TrackNum))
			if w > trkW {
				trkW = w
			}
		}
	}
	trkHeader := "Trk#"

	fixed := posW + playingW + trkW + durWidth + yearWidth + 8
	flexible := p.width - fixed
	if flexible < 30 {
		flexible = 30
	}
	songW := flexible * 30 / 100
	artistW := flexible * 25 / 100
	albumW := flexible - songW - artistW

	var b strings.Builder
	b.WriteString(headerStyle.Render(fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s",
		playingW, "", posW, "#", songW, "Song", artistW, "Artist", trkW, trkHeader, albumW, "Album", yearWidth, "Year", durWidth, "Time")))
	b.WriteString("\n")

	vh := p.visibleHeight()
	for i := 0; i < vh; i++ {
		idx := p.scrollOff + i
		if idx >= len(p.tracks) {
			break
		}

		t := p.tracks[idx]
		isPlaying := idx == p.currentIdx
		isCursor := idx == p.cursor

		var playIcon string
		if isPlaying {
			playIcon = "▶"
		} else {
			playIcon = " "
		}

		songLabel := t.Title
		if t.ServerBadge != "" {
			songLabel = "[" + t.ServerBadge + "] " + t.Title
		}

		pos := fmt.Sprintf("%d", idx+1)
		num := formatTrackNum(t, idx, albumMultiDisc[albumKey(t)])
		dur := formatPlaylistDuration(t.Duration)
		year := ""
		if t.Year != 0 {
			year = fmt.Sprintf("%d", t.Year)
		}

		song := ansi.Truncate(songLabel, songW-1, "…")
		artist := ansi.Truncate(t.Artist, artistW-1, "…")
		album := ansi.Truncate(t.Album, albumW-1, "…")

		row := fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s",
			playingW, playIcon, posW, pos, songW, song, artistW, artist, trkW, num, albumW, album, yearWidth, year, durWidth, dur)

		switch {
		case isCursor && isPlaying:
			b.WriteString(p.styles.CursorStyle.Bold(true).Render(row))
		case isCursor:
			b.WriteString(p.styles.CursorStyle.Render(row))
		case isPlaying:
			b.WriteString(p.styles.AccentStyle.Render(row))
		default:
			b.WriteString(p.styles.ForegroundStyle.Render(row))
		}

		if i < vh-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (p Playlist) albumIsMultiDisc() map[string]bool {
	result := make(map[string]bool)
	for _, t := range p.tracks {
		if t.DiscNum > 1 {
			result[albumKey(t)] = true
		}
	}
	return result
}

func albumKey(t models.Track) string {
	return t.Artist + " - " + t.Album
}

func formatTrackNum(t models.Track, idx int, multiDisc bool) string {
	if multiDisc && t.DiscNum > 0 {
		if t.TrackNum > 0 {
			return fmt.Sprintf("%d/%d", t.DiscNum, t.TrackNum)
		}
		return fmt.Sprintf("%d/%d", t.DiscNum, idx+1)
	}
	if t.TrackNum > 0 {
		return fmt.Sprintf("%d", t.TrackNum)
	}
	return "-"
}

func (p *Playlist) visibleHeight() int {
	return max(p.height-1, 1)
}

func (p *Playlist) ensureVisible() {
	vh := p.visibleHeight()
	if p.cursor < p.scrollOff {
		p.scrollOff = p.cursor
	}
	if p.cursor >= p.scrollOff+vh {
		p.scrollOff = p.cursor - vh + 1
	}
	if p.scrollOff < 0 {
		p.scrollOff = 0
	}
}

type TrackRow struct {
	Track models.Track
}

func BuildPlaylistRows(tracks []models.Track, currentIndex int) []TrackRow {
	rows := make([]TrackRow, len(tracks))
	for i, t := range tracks {
		rows[i] = TrackRow{Track: t}
	}
	return rows
}

func formatPlaylistDuration(seconds float64) string {
	if seconds <= 0 {
		return "--:--"
	}
	total := int(seconds)
	m := total / 60
	s := total % 60
	h := total / 3600
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m%60, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
