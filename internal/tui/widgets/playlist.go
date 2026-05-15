package widgets

import (
	"fmt"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/models"
)

type Playlist struct {
	table  table.Model
	styles *config.ThemeStyles
	width  int
	height int
}

func NewPlaylist(styles *config.ThemeStyles) *Playlist {
	columns := []table.Column{
		{Title: "#", Width: 3},
		{Title: "Song", Width: 30},
		{Title: "Artist", Width: 20},
		{Title: "Duration", Width: 8},
		{Title: "Album", Width: 25},
		{Title: "Year", Width: 5},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithHeight(10),
		table.WithWidth(100),
	)

	headerBg := lightenColor(styles.Background, 0.30)
	s := table.DefaultStyles()
	s.Header = s.Header.
		Bold(false).
		Foreground(styles.MutedStyle.GetForeground()).
		Background(lipgloss.Color(headerBg))
	s.Selected = s.Selected.
		Foreground(styles.CursorStyle.GetForeground()).
		Bold(true)
	t.SetStyles(s)

	return &Playlist{
		table:  t,
		styles: styles,
	}
}

func (p *Playlist) SetSize(width, height int) {
	p.width = width
	p.height = height
	p.table.SetWidth(width)
	p.table.SetHeight(height)

	const (
		numWidth   = 3
		durWidth   = 8
		yearWidth  = 5
		fixedTotal = numWidth + durWidth + yearWidth
		numCols    = 6
		cellPad    = 2 * numCols
	)

	flexible := width - fixedTotal - cellPad
	if flexible < 30 {
		flexible = 30
	}
	songCol := flexible * 40 / 100
	artistCol := flexible * 25 / 100
	albumCol := flexible - songCol - artistCol

	p.table.SetColumns([]table.Column{
		{Title: "#", Width: numWidth},
		{Title: "Song", Width: songCol},
		{Title: "Artist", Width: artistCol},
		{Title: "Duration", Width: durWidth},
		{Title: "Album", Width: albumCol},
		{Title: "Year", Width: yearWidth},
	})
}

func (p *Playlist) UpdateStyles(styles *config.ThemeStyles) {
	p.styles = styles
	headerBg := lightenColor(styles.Background, 0.30)
	s := table.DefaultStyles()
	s.Header = s.Header.
		Bold(false).
		Foreground(styles.MutedStyle.GetForeground()).
		Background(lipgloss.Color(headerBg))
	s.Selected = s.Selected.
		Foreground(styles.CursorStyle.GetForeground()).
		Bold(true)
	p.table.SetStyles(s)
}

func (p *Playlist) SetRows(rows []table.Row) {
	p.table.SetRows(rows)
}

func (p *Playlist) SetCursor(cursor int) {
	p.table.SetCursor(cursor)
}

func (p *Playlist) GetCursor() int {
	return p.table.Cursor()
}

func (p *Playlist) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.table, cmd = p.table.Update(msg)
	return cmd
}

func (p Playlist) View() string {
	return p.table.View()
}

func BuildPlaylistRows(tracks []models.Track, currentIndex int) []table.Row {
	rows := make([]table.Row, len(tracks))
	for i, t := range tracks {
		num := fmt.Sprintf("%d", t.TrackNum)
		if t.TrackNum == 0 {
			num = fmt.Sprintf("%d", i+1)
		}
		year := ""
		if t.Year > 0 {
			year = fmt.Sprintf("%d", t.Year)
		}
		dur := formatPlaylistDuration(t.Duration)
		rows[i] = table.Row{num, t.Title, t.Artist, dur, t.Album, year}
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

func lightenColor(hex string, factor float64) string {
	if hex == "default" || len(hex) != 7 || hex[0] != '#' {
		return hex
	}
	var r, g, b int
	_, _ = fmt.Sscanf(hex[1:3], "%x", &r)
	_, _ = fmt.Sscanf(hex[3:5], "%x", &g)
	_, _ = fmt.Sscanf(hex[5:7], "%x", &b)
	r = min(255, int(float64(r)*(1+factor)))
	g = min(255, int(float64(g)*(1+factor)))
	b = min(255, int(float64(b)*(1+factor)))
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}
