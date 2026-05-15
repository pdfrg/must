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

type BrowseMode int

const (
	BrowseArtists BrowseMode = iota
	BrowseGenres
)

type Library struct {
	styles *config.ThemeStyles
	db     *db.LibraryDB
	width  int
	height int

	allArtists  []string
	allAlbums   []string
	artists     []string
	albums      []string
	albumTracks []models.Track
	focusPane   FocusPane
	browseMode  BrowseMode

	artistCursor       int
	artistScrollOffset int
	albumCursor        int
	albumScrollOffset  int
	trackCursor        int
	trackScrollOffset  int

	genres            []string
	allGenres         []string
	genreCursor       int
	genreScrollOffset int

	filterText      string
	filteredArtists []string
	filteredAlbums  []string
	filteredGenres  []string
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
	l.allArtists = artists
	l.artists = artists
	l.filteredArtists = nil
	l.filterText = ""
}

func (l *Library) LoadAlbumsForArtist() {
	if l.db == nil || l.artistCursor >= len(l.artists) {
		return
	}
	artist := l.artists[l.artistCursor]
	albums, err := l.db.GetAlbumsByArtist(artist)
	if err != nil || len(albums) == 0 {
		l.allAlbums = nil
		l.albums = nil
		l.albumTracks = nil
		l.albumCursor = 0
		l.albumScrollOffset = 0
		l.trackCursor = 0
		l.trackScrollOffset = 0
		return
	}
	l.allAlbums = albums
	l.albums = albums
	l.filteredAlbums = nil
	l.albumCursor = 0
	l.albumScrollOffset = 0
	l.albumTracks = nil
	l.trackCursor = 0
	l.trackScrollOffset = 0
	l.loadTracksForAlbum()
}

func (l *Library) loadTracksForAlbum() {
	if l.db == nil || len(l.albums) == 0 || l.albumCursor >= len(l.albums) {
		l.albumTracks = nil
		l.trackCursor = 0
		l.trackScrollOffset = 0
		return
	}
	if l.browseMode == BrowseGenres {
		album := l.albums[l.albumCursor]
		tracks, err := l.db.GetTracksByAlbum(album)
		if err == nil && len(tracks) > 0 {
			l.albumTracks = tracks
			l.trackCursor = 0
			l.trackScrollOffset = 0
		} else {
			l.albumTracks = nil
			l.trackCursor = 0
			l.trackScrollOffset = 0
		}
		return
	}
	artist := l.artists[l.artistCursor]
	album := l.albums[l.albumCursor]
	tracks, err := l.db.GetTracksByArtistAndAlbum(artist, album)
	if err == nil && len(tracks) > 0 {
		l.albumTracks = tracks
		l.trackCursor = 0
		l.trackScrollOffset = 0
	} else {
		l.albumTracks = nil
		l.trackCursor = 0
		l.trackScrollOffset = 0
	}
}

func (l *Library) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if l.filterText != "" {
				l.clearFilter()
				return nil
			}
			return func() tea.Msg { return LibraryModalMsg{Closed: true} }
		case "q":
			if l.filterText != "" {
				l.clearFilter()
				return nil
			}
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
		case "l", "right":
			if l.filterText != "" {
				l.appendToFilter("l")
				return nil
			}
			l.focusRight()
		case "enter":
			if l.filterText != "" {
				l.clearFilter()
			}
			return l.handleEnter()
		case "e":
			return l.handleEnqueue()
		case "g":
			l.toggleBrowseMode()
		case "backspace":
			l.backspaceFilter()
		default:
			k := msg.Key()
			if k.Text != "" {
				l.appendToFilter(k.Text)
				return nil
			}
		}
	}
	return nil
}

func (l *Library) appendToFilter(ch string) {
	l.filterText += ch
	l.applyFilter()
}

func (l *Library) backspaceFilter() {
	if len(l.filterText) > 0 {
		l.filterText = l.filterText[:len(l.filterText)-1]
		l.applyFilter()
	}
}

func (l *Library) clearFilter() {
	l.filterText = ""
	if l.browseMode == BrowseGenres {
		l.genres = l.allGenres
		l.filteredGenres = nil
		if l.genreCursor >= len(l.genres) {
			l.genreCursor = 0
			l.genreScrollOffset = 0
		}
	} else {
		l.artists = l.allArtists
		l.filteredArtists = nil
		if l.artistCursor >= len(l.artists) {
			l.artistCursor = 0
			l.artistScrollOffset = 0
		}
	}
	l.albums = l.allAlbums
	l.filteredAlbums = nil
	if l.albumCursor >= len(l.albums) && len(l.albums) > 0 {
		l.albumCursor = 0
		l.albumScrollOffset = 0
	}
}

func (l *Library) applyFilter() {
	if l.filterText == "" {
		l.artists = l.allArtists
		l.filteredArtists = nil
		l.genres = l.allGenres
		l.filteredGenres = nil
		l.albums = l.allAlbums
		l.filteredAlbums = nil
		return
	}

	if l.browseMode == BrowseGenres {
		l.filteredGenres = filterStrings(l.allGenres, l.filterText)
		l.genres = l.filteredGenres
		if l.genreCursor >= len(l.genres) {
			l.genreCursor = 0
			l.genreScrollOffset = 0
		}
	} else {
		l.filteredArtists = filterStrings(l.allArtists, l.filterText)
		l.artists = l.filteredArtists
		if l.artistCursor >= len(l.artists) {
			l.artistCursor = 0
			l.artistScrollOffset = 0
		}
	}

	if l.focusPane >= FocusAlbums {
		l.filteredAlbums = filterStrings(l.allAlbums, l.filterText)
		l.albums = l.filteredAlbums
		if l.albumCursor >= len(l.albums) {
			l.albumCursor = 0
			l.albumScrollOffset = 0
		}
	}
}

func filterStrings(items []string, query string) []string {
	query = strings.ToLower(query)
	var result []string
	for _, item := range items {
		if strings.Contains(strings.ToLower(item), query) {
			result = append(result, item)
		}
	}
	return result
}

func (l *Library) handleEnter() tea.Cmd {
	switch l.focusPane {
	case FocusArtists:
		if l.browseMode == BrowseGenres {
			if len(l.genres) > 0 && l.genreCursor < len(l.genres) && len(l.albums) > 0 {
				l.focusPane = FocusAlbums
			}
		} else {
			if len(l.artists) > 0 && l.artistCursor < len(l.artists) && len(l.albums) > 0 {
				l.focusPane = FocusAlbums
			}
		}
	case FocusAlbums:
		if len(l.albumTracks) > 0 {
			l.focusPane = FocusTracks
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
			if len(l.albumTracks) > 0 {
				return func() tea.Msg {
					return LibraryModalMsg{Enqueue: l.albumTracks}
				}
			}
		}
	case FocusArtists:
		if l.browseMode == BrowseGenres {
			if len(l.genres) > 0 && l.genreCursor < len(l.genres) && l.db != nil && len(l.albums) > 0 {
				var allTracks []models.Track
				for _, album := range l.albums {
					tracks, err := l.db.GetTracksByAlbum(album)
					if err == nil {
						allTracks = append(allTracks, tracks...)
					}
				}
				if len(allTracks) > 0 {
					return func() tea.Msg {
						return LibraryModalMsg{Enqueue: allTracks}
					}
				}
			}
		} else {
			if len(l.artists) > 0 && l.artistCursor < len(l.artists) && l.db != nil {
				artist := l.artists[l.artistCursor]
				tracks, err := l.db.GetTracksByArtist(artist)
				if err == nil && len(tracks) > 0 {
					return func() tea.Msg {
						return LibraryModalMsg{Enqueue: tracks}
					}
				}
			}
		}
	}
	return nil
}

func (l *Library) moveDown() {
	switch l.focusPane {
	case FocusArtists:
		if l.browseMode == BrowseGenres {
			if len(l.genres) > 0 && l.genreCursor < len(l.genres)-1 {
				l.genreCursor++
				ensureVisible(&l.genreCursor, &l.genreScrollOffset, len(l.genres), l.paneHeight())
				l.loadAlbumsForGenre()
			}
		} else {
			if len(l.artists) > 0 && l.artistCursor < len(l.artists)-1 {
				l.artistCursor++
				ensureVisible(&l.artistCursor, &l.artistScrollOffset, len(l.artists), l.paneHeight())
				l.LoadAlbumsForArtist()
			}
		}
	case FocusAlbums:
		if len(l.albums) > 0 && l.albumCursor < len(l.albums)-1 {
			l.albumCursor++
			ensureVisible(&l.albumCursor, &l.albumScrollOffset, len(l.albums), l.paneHeight())
			l.loadTracksForAlbum()
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
		if l.browseMode == BrowseGenres {
			if l.genreCursor > 0 {
				l.genreCursor--
				ensureVisible(&l.genreCursor, &l.genreScrollOffset, len(l.genres), l.paneHeight())
				l.loadAlbumsForGenre()
			}
		} else {
			if l.artistCursor > 0 {
				l.artistCursor--
				ensureVisible(&l.artistCursor, &l.artistScrollOffset, len(l.artists), l.paneHeight())
				l.LoadAlbumsForArtist()
			}
		}
	case FocusAlbums:
		if l.albumCursor > 0 {
			l.albumCursor--
			ensureVisible(&l.albumCursor, &l.albumScrollOffset, len(l.albums), l.paneHeight())
			l.loadTracksForAlbum()
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
		if l.browseMode == BrowseGenres {
			if len(l.genres) > 0 {
				l.genreCursor = min(l.genreCursor+ps, len(l.genres)-1)
				ensureVisible(&l.genreCursor, &l.genreScrollOffset, len(l.genres), l.paneHeight())
				l.loadAlbumsForGenre()
			}
		} else {
			if len(l.artists) > 0 {
				l.artistCursor = min(l.artistCursor+ps, len(l.artists)-1)
				ensureVisible(&l.artistCursor, &l.artistScrollOffset, len(l.artists), l.paneHeight())
				l.LoadAlbumsForArtist()
			}
		}
	case FocusAlbums:
		if len(l.albums) > 0 {
			l.albumCursor = min(l.albumCursor+ps, len(l.albums)-1)
			ensureVisible(&l.albumCursor, &l.albumScrollOffset, len(l.albums), l.paneHeight())
			l.loadTracksForAlbum()
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
		if l.browseMode == BrowseGenres {
			l.genreCursor = max(l.genreCursor-ps, 0)
			ensureVisible(&l.genreCursor, &l.genreScrollOffset, len(l.genres), l.paneHeight())
			l.loadAlbumsForGenre()
		} else {
			l.artistCursor = max(l.artistCursor-ps, 0)
			ensureVisible(&l.artistCursor, &l.artistScrollOffset, len(l.artists), l.paneHeight())
			l.LoadAlbumsForArtist()
		}
	case FocusAlbums:
		l.albumCursor = max(l.albumCursor-ps, 0)
		ensureVisible(&l.albumCursor, &l.albumScrollOffset, len(l.albums), l.paneHeight())
		l.loadTracksForAlbum()
	case FocusTracks:
		l.trackCursor = max(l.trackCursor-ps, 0)
		ensureVisible(&l.trackCursor, &l.trackScrollOffset, len(l.albumTracks), l.paneHeight())
	}
}

func (l *Library) jumpHome() {
	switch l.focusPane {
	case FocusArtists:
		if l.browseMode == BrowseGenres {
			l.genreCursor = 0
			l.genreScrollOffset = 0
			l.loadAlbumsForGenre()
		} else {
			l.artistCursor = 0
			l.artistScrollOffset = 0
			l.LoadAlbumsForArtist()
		}
	case FocusAlbums:
		l.albumCursor = 0
		l.albumScrollOffset = 0
		l.loadTracksForAlbum()
	case FocusTracks:
		l.trackCursor = 0
		l.trackScrollOffset = 0
	}
}

func (l *Library) jumpEnd() {
	switch l.focusPane {
	case FocusArtists:
		if l.browseMode == BrowseGenres {
			if len(l.genres) > 0 {
				l.genreCursor = len(l.genres) - 1
				ensureVisible(&l.genreCursor, &l.genreScrollOffset, len(l.genres), l.paneHeight())
				l.loadAlbumsForGenre()
			}
		} else {
			if len(l.artists) > 0 {
				l.artistCursor = len(l.artists) - 1
				ensureVisible(&l.artistCursor, &l.artistScrollOffset, len(l.artists), l.paneHeight())
				l.LoadAlbumsForArtist()
			}
		}
	case FocusAlbums:
		if len(l.albums) > 0 {
			l.albumCursor = len(l.albums) - 1
			ensureVisible(&l.albumCursor, &l.albumScrollOffset, len(l.albums), l.paneHeight())
			l.loadTracksForAlbum()
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
	if l.focusPane < FocusTracks {
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
}

func (l *Library) toggleBrowseMode() {
	l.filterText = ""
	if l.browseMode == BrowseArtists {
		l.browseMode = BrowseGenres
		l.focusPane = FocusArtists
		l.loadGenres()
	} else {
		l.browseMode = BrowseArtists
		l.focusPane = FocusArtists
		l.artists = l.allArtists
		l.filteredArtists = nil
		l.albums = nil
		l.allAlbums = nil
		l.albumTracks = nil
		l.albumCursor = 0
		l.albumScrollOffset = 0
		l.trackCursor = 0
		l.trackScrollOffset = 0
		if l.artistCursor >= len(l.artists) {
			l.artistCursor = 0
			l.artistScrollOffset = 0
		}
		l.LoadAlbumsForArtist()
	}
}

func (l *Library) loadGenres() {
	if l.db == nil {
		return
	}
	genres, err := l.db.GetGenres()
	if err != nil || len(genres) == 0 {
		l.genres = nil
		l.allGenres = nil
		l.genreCursor = 0
		l.genreScrollOffset = 0
		l.albums = nil
		l.albumTracks = nil
		return
	}
	l.genres = genres
	l.allGenres = genres
	l.filteredGenres = nil
	if l.genreCursor >= len(l.genres) {
		l.genreCursor = 0
		l.genreScrollOffset = 0
	}
	l.loadAlbumsForGenre()
}

func (l *Library) loadAlbumsForGenre() {
	if l.db == nil || l.genreCursor >= len(l.genres) {
		return
	}
	genre := l.genres[l.genreCursor]
	albums, err := l.db.GetAlbumsByGenre(genre)
	if err != nil || len(albums) == 0 {
		l.albums = nil
		l.allAlbums = nil
		l.albumTracks = nil
		l.albumCursor = 0
		l.albumScrollOffset = 0
		l.trackCursor = 0
		l.trackScrollOffset = 0
		return
	}
	l.allAlbums = albums
	l.albums = albums
	l.filteredAlbums = nil
	l.albumCursor = 0
	l.albumScrollOffset = 0
	l.albumTracks = nil
	l.trackCursor = 0
	l.trackScrollOffset = 0
	l.loadTracksForAlbum()
}

func (l *Library) paneHeight() int {
	return l.height - 3
}

func (l Library) View() string {
	if l.browseMode == BrowseArtists && len(l.allArtists) == 0 {
		return l.styles.MutedStyle.Render("Library empty - press R to rescan")
	}
	if l.browseMode == BrowseGenres && len(l.allGenres) == 0 {
		return l.styles.MutedStyle.Render("Library empty - press R to rescan")
	}

	colWidth := (l.width - 4) / 3
	if colWidth < 16 {
		colWidth = 16
	}
	height := l.paneHeight()
	if height < 3 {
		height = 3
	}

	var col1, col2, col3 string
	if l.browseMode == BrowseGenres {
		col1 = l.renderGenreList(colWidth, height)
	} else {
		col1 = l.renderArtistList(colWidth, height)
	}
	col2 = l.renderAlbumColumn(colWidth, height)
	col3 = l.renderTrackColumn(colWidth, height)

	sep1 := l.styles.MutedStyle.Render("│")
	if l.focusPane == FocusArtists {
		sep1 = l.styles.AccentStyle.Render("│")
	}
	sep2 := l.styles.MutedStyle.Render("│")
	if l.focusPane == FocusAlbums || l.focusPane == FocusTracks {
		sep2 = l.styles.AccentStyle.Render("│")
	}

	col1Lines := strings.Split(col1, "\n")
	col2Lines := strings.Split(col2, "\n")
	col3Lines := strings.Split(col3, "\n")
	maxLines := max(len(col1Lines), len(col2Lines), len(col3Lines), height)

	var b strings.Builder
	for i := 0; i < maxLines; i++ {
		var c1, c2, c3 string
		if i < len(col1Lines) {
			c1 = col1Lines[i]
		}
		if i < len(col2Lines) {
			c2 = col2Lines[i]
		}
		if i < len(col3Lines) {
			c3 = col3Lines[i]
		}
		c1 = l.padOrTruncateLine(c1, colWidth)
		c2 = l.padOrTruncateLine(c2, colWidth)
		c3 = l.padOrTruncateLine(c3, colWidth)
		b.WriteString(c1)
		b.WriteString(" ")
		b.WriteString(sep1)
		b.WriteString(" ")
		b.WriteString(c2)
		b.WriteString(" ")
		b.WriteString(sep2)
		b.WriteString(" ")
		b.WriteString(c3)
		if i < maxLines-1 {
			b.WriteString("\n")
		}
	}

	topBar := l.renderTopBar()
	helpLine := l.renderHelpLine()
	inner := topBar + b.String() + "\n" + helpLine
	return lipgloss.NewStyle().Width(l.width).Render(inner)
}

func (l Library) padOrTruncateLine(line string, width int) string {
	visualWidth := lipgloss.Width(line)
	if visualWidth > width {
		return ansi.Truncate(line, width, "")
	}
	if visualWidth < width {
		return line + strings.Repeat(" ", width-visualWidth)
	}
	return line
}

func (l Library) renderTopBar() string {
	modeLabel := "artists"
	if l.browseMode == BrowseGenres {
		modeLabel = "genres"
	}
	var topBar string
	if l.filterText != "" {
		filterDisplay := l.filterText
		if len(filterDisplay) > 30 {
			filterDisplay = filterDisplay[:30] + "…"
		}
		topBar = l.styles.AccentStyle.Render(fmt.Sprintf("filter: %s", filterDisplay)) +
			l.styles.MutedStyle.Render(fmt.Sprintf(" [%s]", modeLabel)) + "\n"
	} else {
		topBar = l.styles.MutedStyle.Render(fmt.Sprintf("[%s]", modeLabel)) + "\n"
	}
	return topBar
}

func (l Library) renderHelpLine() string {
	helpPairs := []struct {
		key  string
		desc string
	}{
		{"↑/↓", "nav"},
		{"h/l", "focus"},
		{"enter", "play"},
		{"e", "enqueue"},
		{"g", "genre"},
		{"esc", "close"},
	}
	if l.filterText != "" {
		helpPairs = []struct {
			key  string
			desc string
		}{
			{"type", "filter"},
			{"bksp", "del"},
			{"esc", "clear"},
		}
	}
	focusName := "artists"
	if l.browseMode == BrowseGenres {
		focusName = "genres"
	}
	switch l.focusPane {
	case FocusAlbums:
		focusName = "albums"
	case FocusTracks:
		focusName = "tracks"
	}

	var b strings.Builder
	for i, p := range helpPairs {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(l.styles.AccentStyle.Render(p.key))
		b.WriteString(l.styles.MutedStyle.Render(p.desc))
	}
	b.WriteString(" ")
	b.WriteString(l.styles.MutedStyle.Render(fmt.Sprintf("[%s]", focusName)))
	return b.String()
}

func (l Library) renderArtistList(width, height int) string {
	var b strings.Builder
	items := l.artists
	maxRows := min(height, len(items)-l.artistScrollOffset)
	if maxRows < 1 {
		if len(items) == 0 {
			return l.styles.MutedStyle.Render("  No artists")
		}
		maxRows = 1
	}
	focused := l.focusPane == FocusArtists
	for i := 0; i < maxRows; i++ {
		idx := l.artistScrollOffset + i
		if idx >= len(items) {
			break
		}
		name := ansi.Truncate(items[idx], width-2, "…")
		if idx == l.artistCursor && focused {
			name = l.styles.CursorStyle.Render("> " + name)
		} else if idx == l.artistCursor {
			name = l.styles.AccentStyle.Render(" " + name)
		} else {
			name = l.styles.MutedStyle.Render(" " + name)
		}
		b.WriteString(name)
		if i < maxRows-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (l Library) renderGenreList(width, height int) string {
	var b strings.Builder
	items := l.genres
	maxRows := min(height, len(items)-l.genreScrollOffset)
	if maxRows < 1 {
		if len(items) == 0 {
			return l.styles.MutedStyle.Render("  No genres")
		}
		maxRows = 1
	}
	focused := l.focusPane == FocusArtists
	for i := 0; i < maxRows; i++ {
		idx := l.genreScrollOffset + i
		if idx >= len(items) {
			break
		}
		name := ansi.Truncate(items[idx], width-2, "…")
		if idx == l.genreCursor && focused {
			name = l.styles.CursorStyle.Render("> " + name)
		} else if idx == l.genreCursor {
			name = l.styles.AccentStyle.Render(" " + name)
		} else {
			name = l.styles.MutedStyle.Render(" " + name)
		}
		b.WriteString(name)
		if i < maxRows-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (l Library) renderAlbumColumn(width, height int) string {
	if len(l.albums) == 0 {
		if l.browseMode == BrowseGenres {
			return l.styles.MutedStyle.Render("  Select a genre")
		}
		return l.styles.MutedStyle.Render("  Select an artist")
	}

	var b strings.Builder
	items := l.albums
	maxRows := min(height, len(items)-l.albumScrollOffset)
	if maxRows < 1 {
		maxRows = 1
	}
	focused := l.focusPane == FocusAlbums
	for i := 0; i < maxRows; i++ {
		idx := l.albumScrollOffset + i
		if idx >= len(items) {
			break
		}
		name := ansi.Truncate(items[idx], width-2, "…")
		if idx == l.albumCursor && focused {
			name = l.styles.CursorStyle.Render("> " + name)
		} else if idx == l.albumCursor {
			name = l.styles.AccentStyle.Render(" " + name)
		} else {
			name = l.styles.ForegroundStyle.Render(" " + name)
		}
		b.WriteString(name)
		if i < maxRows-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (l Library) renderTrackColumn(width, height int) string {
	if len(l.albumTracks) == 0 {
		if len(l.albums) > 0 {
			return l.styles.MutedStyle.Render("  Select an album")
		}
		return ""
	}

	focused := l.focusPane == FocusTracks
	tracks := l.albumTracks
	start := l.trackScrollOffset
	maxRows := min(height, len(tracks)-start)
	if maxRows < 1 {
		maxRows = 1
	}
	var b strings.Builder
	for i := 0; i < maxRows; i++ {
		idx := start + i
		if idx >= len(tracks) {
			break
		}
		t := tracks[idx]
		dur := t.GetDurationFormatted()
		label := fmt.Sprintf("%2d. %s", t.TrackNum, t.Title)
		avail := width - len(dur) - 5
		if avail < 10 {
			avail = 10
		}
		label = ansi.Truncate(label, avail, "…")
		var prefix string
		if idx == l.trackCursor && focused {
			prefix = l.styles.CursorStyle.Render("> ")
		} else if idx == l.trackCursor {
			prefix = l.styles.AccentStyle.Render(" ")
		} else {
			prefix = " "
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
