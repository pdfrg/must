package modals

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/db"
	"github.com/pdfrg/must/internal/models"
)

type LibraryModalMsg struct {
	PlayTracks []models.Track
	PlayIndex  int
	Enqueue    []models.Track
	Closed     bool
}

type FocusPane int

const (
	FocusArtists FocusPane = iota
	FocusAlbums
	FocusTracks
)

type Library struct {
	styles *config.ThemeStyles
	db     *db.LibraryDB
	width  int
	height int

	artists            []string
	albums             []string
	albumTracks        []models.Track
	focusPane          FocusPane
	artistCursor       int
	artistScrollOffset int
	albumCursor        int
	albumScrollOffset  int
	trackCursor        int
	trackScrollOffset  int
}

func NewLibrary(styles *config.ThemeStyles, libraryDB *db.LibraryDB) *Library {
	return &Library{
		styles:    styles,
		db:        libraryDB,
		focusPane: FocusArtists,
	}
}

func (l *Library) SetSize(width, height int) {
	l.width = width
	l.height = height
}

func (l *Library) SetArtists(artists []string) {
	l.artists = artists
}

func (l *Library) LoadAlbumsForArtist() {
	if l.db == nil || l.artistCursor >= len(l.artists) {
		return
	}
	artist := l.artists[l.artistCursor]
	albums, err := l.db.GetAlbumsByArtist(artist)
	if err != nil || len(albums) == 0 {
		l.albums = nil
		l.albumTracks = nil
		l.albumCursor = 0
		l.albumScrollOffset = 0
		l.trackCursor = 0
		l.trackScrollOffset = 0
		return
	}
	l.albums = albums
	l.albumCursor = 0
	l.albumScrollOffset = 0
	l.albumTracks = nil
	l.trackCursor = 0
	l.trackScrollOffset = 0
}

func (l *Library) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "q":
			return func() tea.Msg { return LibraryModalMsg{Closed: true} }
		case "up", "k":
			l.moveUp()
		case "down", "j":
			l.moveDown()
		case "pgdown":
			l.pageDown()
		case "pgup":
			l.pageUp()
		case "home":
			l.jumpHome()
		case "end":
			l.jumpEnd()
		case "h", "left":
			l.focusLeft()
		case "l":
			l.focusRight()
		case "enter":
			return l.handleEnter()
		case "e":
			return l.handleEnqueue()
		}
	}
	return nil
}

func (l *Library) handleEnter() tea.Cmd {
	switch l.focusPane {
	case FocusArtists:
		if len(l.artists) > 0 && l.artistCursor < len(l.artists) {
			if len(l.albums) > 0 {
				l.focusPane = FocusAlbums
			}
		}
	case FocusAlbums:
		if len(l.albums) > 0 && l.albumCursor < len(l.albums) && l.db != nil {
			artist := l.artists[l.artistCursor]
			album := l.albums[l.albumCursor]
			tracks, err := l.db.GetTracksByArtistAndAlbum(artist, album)
			if err == nil && len(tracks) > 0 {
				l.albumTracks = tracks
				l.trackCursor = 0
				l.trackScrollOffset = 0
				l.focusPane = FocusTracks
			}
		}
	case FocusTracks:
		if len(l.albumTracks) > 0 && l.trackCursor < len(l.albumTracks) {
			return func() tea.Msg {
				return LibraryModalMsg{
					PlayTracks: l.albumTracks,
					PlayIndex:  l.trackCursor,
				}
			}
		}
	}
	return nil
}

func (l *Library) handleEnqueue() tea.Cmd {
	switch l.focusPane {
	case FocusTracks:
		if len(l.albumTracks) > 0 && l.trackCursor < len(l.albumTracks) {
			return func() tea.Msg {
				return LibraryModalMsg{
					Enqueue: []models.Track{l.albumTracks[l.trackCursor]},
				}
			}
		}
	case FocusAlbums:
		if len(l.albums) > 0 && l.albumCursor < len(l.albums) && l.db != nil {
			artist := l.artists[l.artistCursor]
			album := l.albums[l.albumCursor]
			tracks, err := l.db.GetTracksByArtistAndAlbum(artist, album)
			if err == nil && len(tracks) > 0 {
				return func() tea.Msg {
					return LibraryModalMsg{Enqueue: tracks}
				}
			}
		}
	}
	return nil
}

func (l *Library) moveDown() {
	switch l.focusPane {
	case FocusArtists:
		if len(l.artists) > 0 && l.artistCursor < len(l.artists)-1 {
			l.artistCursor++
			ensureVisible(&l.artistCursor, &l.artistScrollOffset, len(l.artists), l.paneHeight())
			l.LoadAlbumsForArtist()
		}
	case FocusAlbums:
		if len(l.albums) > 0 && l.albumCursor < len(l.albums)-1 {
			l.albumCursor++
			ensureVisible(&l.albumCursor, &l.albumScrollOffset, len(l.albums), l.paneHeight())
			l.albumTracks = nil
		}
	case FocusTracks:
		if len(l.albumTracks) > 0 && l.trackCursor < len(l.albumTracks)-1 {
			l.trackCursor++
			ensureVisible(&l.trackCursor, &l.trackScrollOffset, len(l.albumTracks), l.paneHeight())
		}
	}
}

func (l *Library) moveUp() {
	switch l.focusPane {
	case FocusArtists:
		if l.artistCursor > 0 {
			l.artistCursor--
			ensureVisible(&l.artistCursor, &l.artistScrollOffset, len(l.artists), l.paneHeight())
			l.LoadAlbumsForArtist()
		}
	case FocusAlbums:
		if l.albumCursor > 0 {
			l.albumCursor--
			ensureVisible(&l.albumCursor, &l.albumScrollOffset, len(l.albums), l.paneHeight())
			l.albumTracks = nil
		}
	case FocusTracks:
		if l.trackCursor > 0 {
			l.trackCursor--
			ensureVisible(&l.trackCursor, &l.trackScrollOffset, len(l.albumTracks), l.paneHeight())
		}
	}
}

func (l *Library) pageDown() {
	ps := max(l.paneHeight()-1, 1)
	switch l.focusPane {
	case FocusArtists:
		if len(l.artists) > 0 {
			l.artistCursor = min(l.artistCursor+ps, len(l.artists)-1)
			ensureVisible(&l.artistCursor, &l.artistScrollOffset, len(l.artists), l.paneHeight())
			l.LoadAlbumsForArtist()
		}
	case FocusAlbums:
		if len(l.albums) > 0 {
			l.albumCursor = min(l.albumCursor+ps, len(l.albums)-1)
			ensureVisible(&l.albumCursor, &l.albumScrollOffset, len(l.albums), l.paneHeight())
			l.albumTracks = nil
		}
	case FocusTracks:
		if len(l.albumTracks) > 0 {
			l.trackCursor = min(l.trackCursor+ps, len(l.albumTracks)-1)
			ensureVisible(&l.trackCursor, &l.trackScrollOffset, len(l.albumTracks), l.paneHeight())
		}
	}
}

func (l *Library) pageUp() {
	ps := max(l.paneHeight()-1, 1)
	switch l.focusPane {
	case FocusArtists:
		l.artistCursor = max(l.artistCursor-ps, 0)
		ensureVisible(&l.artistCursor, &l.artistScrollOffset, len(l.artists), l.paneHeight())
		l.LoadAlbumsForArtist()
	case FocusAlbums:
		l.albumCursor = max(l.albumCursor-ps, 0)
		ensureVisible(&l.albumCursor, &l.albumScrollOffset, len(l.albums), l.paneHeight())
		l.albumTracks = nil
	case FocusTracks:
		l.trackCursor = max(l.trackCursor-ps, 0)
		ensureVisible(&l.trackCursor, &l.trackScrollOffset, len(l.albumTracks), l.paneHeight())
	}
}

func (l *Library) jumpHome() {
	switch l.focusPane {
	case FocusArtists:
		l.artistCursor = 0
		l.artistScrollOffset = 0
		l.LoadAlbumsForArtist()
	case FocusAlbums:
		l.albumCursor = 0
		l.albumScrollOffset = 0
		l.albumTracks = nil
	case FocusTracks:
		l.trackCursor = 0
		l.trackScrollOffset = 0
	}
}

func (l *Library) jumpEnd() {
	switch l.focusPane {
	case FocusArtists:
		if len(l.artists) > 0 {
			l.artistCursor = len(l.artists) - 1
			ensureVisible(&l.artistCursor, &l.artistScrollOffset, len(l.artists), l.paneHeight())
			l.LoadAlbumsForArtist()
		}
	case FocusAlbums:
		if len(l.albums) > 0 {
			l.albumCursor = len(l.albums) - 1
			ensureVisible(&l.albumCursor, &l.albumScrollOffset, len(l.albums), l.paneHeight())
			l.albumTracks = nil
		}
	case FocusTracks:
		if len(l.albumTracks) > 0 {
			l.trackCursor = len(l.albumTracks) - 1
			ensureVisible(&l.trackCursor, &l.trackScrollOffset, len(l.albumTracks), l.paneHeight())
		}
	}
}

func (l *Library) focusLeft() {
	if l.focusPane > FocusArtists {
		l.focusPane--
	}
}

func (l *Library) focusRight() {
	switch l.focusPane {
	case FocusArtists:
		if len(l.albums) > 0 {
			l.focusPane = FocusAlbums
		}
	case FocusAlbums:
		if len(l.albumTracks) > 0 {
			l.focusPane = FocusTracks
		}
	}
}

func (l *Library) paneHeight() int {
	return l.height - 4
}

func (l Library) View() string {
	if len(l.artists) == 0 {
		return l.styles.MutedStyle.Render("Library empty - press R to rescan")
	}

	leftWidth := l.width / 3
	if leftWidth < 20 {
		leftWidth = 20
	}
	rightWidth := l.width - leftWidth - 3
	height := l.paneHeight()
	if height < 3 {
		height = 3
	}

	artistList := l.renderArtistList(leftWidth, height)
	rightPane := l.renderRightPane(rightWidth, height)

	sep := l.styles.MutedStyle.Render("|")
	if l.focusPane == FocusArtists {
		sep = l.styles.AccentStyle.Render("|")
	}

	content := artistList + " " + sep + " " + rightPane

	focusStr := "[artists]"
	switch l.focusPane {
	case FocusAlbums:
		focusStr = "[albums]"
	case FocusTracks:
		focusStr = "[tracks]"
	}
	helpLine := l.styles.MutedStyle.Render(fmt.Sprintf("up/dn navigate  h/l focus  enter play  e enqueue  esc close  %s", focusStr))

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(l.styles.AccentStyle.GetForeground()).
		Padding(0, 1).
		Width(l.width - 4)

	inner := content + "\n" + helpLine
	return modalStyle.Render(inner)
}

func (l Library) renderArtistList(width, height int) string {
	var b strings.Builder
	maxRows := min(height, len(l.artists)-l.artistScrollOffset)
	if maxRows < 1 {
		maxRows = 1
	}
	focused := l.focusPane == FocusArtists
	for i := 0; i < maxRows; i++ {
		idx := l.artistScrollOffset + i
		if idx >= len(l.artists) {
			break
		}
		name := ansi.Truncate(l.artists[idx], width-2, "...")
		if idx == l.artistCursor && focused {
			name = l.styles.CursorStyle.Render("> " + name)
		} else if idx == l.artistCursor {
			name = l.styles.AccentStyle.Render("  " + name)
		} else {
			name = l.styles.MutedStyle.Render("  " + name)
		}
		b.WriteString(name)
		if i < maxRows-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (l Library) renderRightPane(width, height int) string {
	if len(l.albumTracks) > 0 && l.focusPane == FocusTracks {
		return l.renderTrackList(l.albumTracks, width, height, true)
	}
	if len(l.albums) > 0 {
		return l.renderAlbumList(width, height)
	}
	if len(l.artists) > 0 && l.artistCursor < len(l.artists) && l.db != nil {
		artist := l.artists[l.artistCursor]
		tracks, err := l.db.GetTracksByArtist(artist)
		if err == nil && len(tracks) > 0 {
			return l.renderTrackList(tracks, width, height, false)
		}
	}
	return l.styles.MutedStyle.Render("Select an artist")
}

func (l Library) renderAlbumList(width, height int) string {
	var b strings.Builder
	maxRows := min(height, len(l.albums)-l.albumScrollOffset)
	if maxRows < 1 {
		maxRows = 1
	}
	focused := l.focusPane == FocusAlbums
	for i := 0; i < maxRows; i++ {
		idx := l.albumScrollOffset + i
		if idx >= len(l.albums) {
			break
		}
		name := ansi.Truncate(l.albums[idx], width-2, "...")
		if idx == l.albumCursor && focused {
			name = l.styles.CursorStyle.Render("> " + name)
		} else if idx == l.albumCursor {
			name = l.styles.AccentStyle.Render("  " + name)
		} else {
			name = l.styles.ForegroundStyle.Render("  " + name)
		}
		b.WriteString(name)
		if i < maxRows-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (l Library) renderTrackList(tracks []models.Track, width, height int, focused bool) string {
	var b strings.Builder
	start := l.trackScrollOffset
	maxRows := min(height, len(tracks)-start)
	if maxRows < 1 {
		maxRows = 1
	}
	for i := 0; i < maxRows; i++ {
		idx := start + i
		if idx >= len(tracks) {
			break
		}
		t := tracks[idx]
		dur := t.GetDurationFormatted()
		label := fmt.Sprintf("%2d. %s", t.TrackNum, t.Title)
		avail := width - len(dur) - 4
		if avail < 10 {
			avail = 10
		}
		label = ansi.Truncate(label, avail, "...")
		var prefix string
		if idx == l.trackCursor && focused {
			prefix = l.styles.CursorStyle.Render("> ")
		} else if idx == l.trackCursor {
			prefix = l.styles.AccentStyle.Render("  ")
		} else {
			prefix = "  "
		}
		line := fmt.Sprintf("%s%s %s", prefix, label, l.styles.MutedStyle.Render(dur))
		if idx == l.trackCursor && focused {
			b.WriteString(line)
		} else {
			b.WriteString(l.styles.ForegroundStyle.Render(line))
		}
		if i < maxRows-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func ensureVisible(cursor, offset *int, total, visibleHeight int) {
	if visibleHeight < 1 {
		visibleHeight = 1
	}
	if *cursor < *offset {
		*offset = *cursor
	}
	if *cursor >= *offset+visibleHeight {
		*offset = *cursor - visibleHeight + 1
	}
	if *offset < 0 {
		*offset = 0
	}
}
