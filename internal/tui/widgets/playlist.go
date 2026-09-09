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
	columns    []string
}

func NewPlaylist(styles *config.ThemeStyles) *Playlist {
	return &Playlist{
		styles:     styles,
		currentIdx: -1,
		cursor:     0,
		columns:    append([]string(nil), config.DefaultPlaylistColumns...),
	}
}

func (p *Playlist) SetColumns(columns []string) {
	valid := map[string]bool{
		"position": true,
		"title":    true,
		"artist":   true,
		"track":    true,
		"album":    true,
		"year":     true,
		"duration": true,
	}
	seen := make(map[string]bool)
	p.columns = p.columns[:0]
	for _, column := range columns {
		if valid[column] && !seen[column] {
			p.columns = append(p.columns, column)
			seen[column] = true
		}
	}
	if len(p.columns) == 0 {
		p.columns = append([]string(nil), config.DefaultPlaylistColumns...)
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

	albumMultiDisc := p.albumIsMultiDisc()
	columns := p.layoutColumns(albumMultiDisc)
	widths := []int{2}
	headings := []string{""}
	for _, column := range columns {
		widths = append(widths, column.width)
		headings = append(headings, column.heading)
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render(cellRow(headings, widths)))
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

		cells := []string{playIcon}
		for _, column := range columns {
			cells = append(cells, playlistCell(column.id, t, idx, albumMultiDisc[albumKey(t)], column.width))
		}
		row := cellRow(cells, widths)

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

type playlistColumn struct {
	id         string
	heading    string
	width      int
	flexWeight int
}

func (p Playlist) layoutColumns(albumMultiDisc map[string]bool) []playlistColumn {
	trackWidth := 4
	for _, t := range p.tracks {
		multi := albumMultiDisc[albumKey(t)]
		if multi && t.DiscNum > 0 {
			n := t.TrackNum
			if n == 0 {
				n = 99
			}
			trackWidth = max(trackWidth, len(fmt.Sprintf("%d/%d", t.DiscNum, n)))
		} else if t.TrackNum > 0 {
			trackWidth = max(trackWidth, len(fmt.Sprintf("%d", t.TrackNum)))
		}
	}

	definitions := map[string]playlistColumn{
		"position": {id: "position", heading: "#", width: 4},
		"title":    {id: "title", heading: "Song", flexWeight: 30},
		"artist":   {id: "artist", heading: "Artist", flexWeight: 25},
		"track":    {id: "track", heading: "Trk#", width: trackWidth},
		"album":    {id: "album", heading: "Album", flexWeight: 45},
		"year":     {id: "year", heading: "Year", width: 5},
		"duration": {id: "duration", heading: "Time", width: 8},
	}

	columns := make([]playlistColumn, 0, len(p.columns))
	fixedWidth := 2 + len(p.columns) // selector plus one space between every cell
	flexCount := 0
	totalWeight := 0
	for _, id := range p.columns {
		column, ok := definitions[id]
		if !ok {
			continue
		}
		columns = append(columns, column)
		if column.flexWeight > 0 {
			flexCount++
			totalWeight += column.flexWeight
		} else {
			fixedWidth += column.width
		}
	}

	if flexCount == 0 {
		return columns
	}
	flexibleWidth := max(p.width-fixedWidth, flexCount*8)
	remainingExtra := flexibleWidth - flexCount*8
	remainingWidth := flexibleWidth
	remainingFlex := flexCount
	remainingWeight := totalWeight
	for i := range columns {
		if columns[i].flexWeight == 0 {
			continue
		}
		width := 8
		remainingFlex--
		if remainingFlex == 0 {
			width = remainingWidth
		} else if remainingWeight > 0 {
			share := remainingExtra * columns[i].flexWeight / remainingWeight
			width += share
			remainingExtra -= share
			remainingWeight -= columns[i].flexWeight
		}
		columns[i].width = width
		remainingWidth -= width
	}
	return columns
}

func playlistCell(id string, track models.Track, idx int, multiDisc bool, width int) string {
	var value string
	switch id {
	case "position":
		value = fmt.Sprintf("%d", idx+1)
	case "title":
		value = track.Title
		if track.ServerBadge != "" {
			value = "[" + track.ServerBadge + "] " + value
		}
	case "artist":
		value = track.Artist
	case "track":
		value = formatTrackNum(track, idx, multiDisc)
	case "album":
		value = track.Album
	case "year":
		if track.Year != 0 {
			value = fmt.Sprintf("%d", track.Year)
		}
	case "duration":
		value = formatPlaylistDuration(track.Duration)
	}
	if id == "title" || id == "artist" || id == "album" {
		return ansi.Truncate(value, max(width-1, 1), "…")
	}
	return value
}

func cellRow(cells []string, widths []int) string {
	padded := make([]string, len(cells))
	for i, c := range cells {
		padded[i] = padCell(c, widths[i])
	}
	return strings.Join(padded, " ")
}

func padCell(s string, w int) string {
	sw := ansi.StringWidth(s)
	if sw < w {
		return s + strings.Repeat(" ", w-sw)
	}
	return s
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
