package modals

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pdfrg/must/internal/api"
	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/db"
	"github.com/pdfrg/must/internal/models"
	"github.com/sahilm/fuzzy"
)

type SearchModalMsg struct {
	PlayTracks  []models.Track
	PlayIndex   int
	Enqueue     []models.Track
	EnqueueNext []models.Track
	Closed      bool
}

type SearchSource int

const (
	SearchLocal SearchSource = iota
	SearchBoth
	SearchSubsonic
)

func (s SearchSource) String() string {
	switch s {
	case SearchLocal:
		return "Local"
	case SearchBoth:
		return "Both"
	case SearchSubsonic:
		return "Subsonic"
	default:
		return "?"
	}
}

type resultKind int

const (
	resultTrack resultKind = iota
	resultArtist
	resultAlbum
)

type searchEntry struct {
	Kind        resultKind
	Track       models.Track
	ArtistName  string
	AlbumName   string
	AlbumArtist string
	SubsonicID  string
	IsSubsonic  bool
	AlbumCount  int
	TrackCount  int
}

type Search struct {
	styles *config.ThemeStyles
	db     *db.LibraryDB
	width  int
	height int

	input        textinput.Model
	entries      []searchEntry
	cursor       int
	scrollOffset int

	source        SearchSource
	subsonicBadge string

	PendingSubsonicArtistID string
	PendingSubsonicAlbumID  string
	ResolveArtistName       string
	ResolveAlbumArtist      string
	ResolveAlbumName        string
	ResolveEnqueueNext      bool
	ResolveEnqueue          bool
}

func NewSearch(styles *config.ThemeStyles, libraryDB *db.LibraryDB) *Search {
	si := textinput.New()
	si.Prompt = "/"
	si.Placeholder = "artist:radiohead year:1997"
	si.CharLimit = 200

	return &Search{
		styles:  styles,
		db:      libraryDB,
		input:   si,
		entries: nil,
	}
}

func (s *Search) SetDB(libraryDB *db.LibraryDB) {
	s.db = libraryDB
}

func (s *Search) GetResults() []models.Track {
	var tracks []models.Track
	for _, e := range s.entries {
		if e.Kind == resultTrack {
			tracks = append(tracks, e.Track)
		}
	}
	return tracks
}

func (s *Search) SetEntries(entries []searchEntry) {
	s.entries = entries
	s.cursor = 0
	s.scrollOffset = 0
}

func (s *Search) AppendEntries(entries []searchEntry) {
	s.entries = append(s.entries, entries...)
}

func (s *Search) Source() SearchSource { return s.source }

func (s *Search) SetSource(src SearchSource) { s.source = src }

func (s *Search) SourceWantsSubsonic() bool {
	return s.source == SearchBoth || s.source == SearchSubsonic
}

func (s *Search) SetSubsonicBadge(badge string) { s.subsonicBadge = badge }

func (s *Search) AddSubsonicResults(artists []api.ArtistID3, albums []api.AlbumID3, tracks []models.Track) {
	var entries []searchEntry
	for _, a := range artists {
		entries = append(entries, searchEntry{
			Kind: resultArtist, ArtistName: a.Name,
			IsSubsonic: true, SubsonicID: a.ID, AlbumCount: a.AlbumCount,
		})
	}
	for _, a := range albums {
		entries = append(entries, searchEntry{
			Kind: resultAlbum, AlbumName: a.Name, AlbumArtist: a.Artist,
			IsSubsonic: true, SubsonicID: a.ID, TrackCount: a.SongCount,
		})
	}
	for _, t := range tracks {
		entries = append(entries, searchEntry{Kind: resultTrack, Track: t, IsSubsonic: true})
	}

	if s.source == SearchSubsonic {
		s.entries = entries
	} else {
		s.entries = append(s.entries, entries...)
	}
	s.cursor = 0
	s.scrollOffset = 0
}

func (s *Search) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.input.SetWidth(min(width-4, 80))
}

func (s *Search) Focus() tea.Cmd {
	return s.input.Focus()
}

func (s *Search) Blur() {
	s.input.Blur()
}

func (s *Search) Reset() {
	s.input.SetValue("")
	s.entries = nil
	s.cursor = 0
	s.scrollOffset = 0
	s.PendingSubsonicArtistID = ""
	s.PendingSubsonicAlbumID = ""
	s.ResolveArtistName = ""
	s.ResolveAlbumArtist = ""
	s.ResolveAlbumName = ""
	s.ResolveEnqueueNext = false
	s.ResolveEnqueue = false
}

func (s *Search) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return func() tea.Msg { return SearchModalMsg{Closed: true} }
		case "enter":
			if len(s.entries) > 0 && s.cursor < len(s.entries) {
				return s.handleEnterEntry(s.entries[s.cursor])
			}
			return nil
		case "ctrl+e":
			if len(s.entries) > 0 && s.cursor < len(s.entries) {
				return s.handleEnqueueEntry(s.entries[s.cursor])
			}
			return nil
		case "alt+e":
			return s.handleEnqueueNext()
		case "down", "ctrl+n":
			if len(s.entries) > 0 && s.cursor < len(s.entries)-1 {
				s.cursor++
				maxVisible := s.height - 6
				if maxVisible < 1 {
					maxVisible = 1
				}
				if s.cursor >= s.scrollOffset+maxVisible {
					s.scrollOffset = s.cursor - maxVisible + 1
				}
			}
			return nil
		case "up", "ctrl+p":
			if s.cursor > 0 {
				s.cursor--
				if s.cursor < s.scrollOffset {
					s.scrollOffset = s.cursor
				}
			}
			return nil
		case "ctrl+t":
			s.source = (s.source + 1) % 3
			s.clearPending()
			if s.input.Value() != "" {
				return debounceSearchModalCmd(s.input.Value())
			}
			return nil
		case "ctrl+s":
			s.source = SearchSubsonic
			s.clearPending()
			if s.input.Value() != "" {
				return debounceSearchModalCmd(s.input.Value())
			}
			return nil
		case "ctrl+l":
			s.source = SearchLocal
			s.clearPending()
			if s.input.Value() != "" {
				return debounceSearchModalCmd(s.input.Value())
			}
			return nil
		default:
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			if s.db != nil && s.input.Value() != "" {
				return tea.Batch(cmd, debounceSearchModalCmd(s.input.Value()))
			}
			s.entries = nil
			s.cursor = 0
			s.scrollOffset = 0
			return cmd
		}
	case SearchDebounceMsg:
		if s.input.Value() != msg.Query {
			return nil
		}
		return s.executeSearch(msg.Query)
	case SearchResultsMsg:
		s.entries = msg.Entries
		s.cursor = 0
		s.scrollOffset = 0
		return nil
	}
	return nil
}

func (s *Search) clearPending() {
	s.PendingSubsonicArtistID = ""
	s.PendingSubsonicAlbumID = ""
	s.ResolveArtistName = ""
	s.ResolveAlbumArtist = ""
	s.ResolveAlbumName = ""
	s.ResolveEnqueueNext = false
	s.ResolveEnqueue = false
}

func (s *Search) handleEnterEntry(e searchEntry) tea.Cmd {
	switch e.Kind {
	case resultTrack:
		return func() tea.Msg {
			return SearchModalMsg{
				PlayTracks: []models.Track{e.Track},
				PlayIndex:  0,
			}
		}
	case resultArtist:
		if e.IsSubsonic {
			s.PendingSubsonicArtistID = e.SubsonicID
			return nil
		}
		s.ResolveArtistName = e.ArtistName
		return nil
	case resultAlbum:
		if e.IsSubsonic {
			s.PendingSubsonicAlbumID = e.SubsonicID
			return nil
		}
		s.ResolveAlbumArtist = e.AlbumArtist
		s.ResolveAlbumName = e.AlbumName
		return nil
	}
	return nil
}

func (s *Search) handleEnqueueEntry(e searchEntry) tea.Cmd {
	switch e.Kind {
	case resultTrack:
		return func() tea.Msg {
			return SearchModalMsg{
				Enqueue: []models.Track{e.Track},
			}
		}
	case resultArtist:
		if e.IsSubsonic {
			s.PendingSubsonicArtistID = e.SubsonicID
			s.ResolveEnqueue = true
			return nil
		}
		return func() tea.Msg {
			tracks, err := s.db.GetTracksByArtist(e.ArtistName)
			if err != nil || len(tracks) == 0 {
				return SearchModalMsg{Closed: true}
			}
			return SearchModalMsg{Enqueue: tracks}
		}
	case resultAlbum:
		if e.IsSubsonic {
			s.PendingSubsonicAlbumID = e.SubsonicID
			s.ResolveEnqueue = true
			return nil
		}
		return func() tea.Msg {
			tracks, err := s.db.GetTracksByArtistAndAlbum(e.AlbumArtist, e.AlbumName)
			if err != nil || len(tracks) == 0 {
				return SearchModalMsg{Closed: true}
			}
			return SearchModalMsg{Enqueue: tracks}
		}
	}
	return nil
}

func (s *Search) handleEnqueueNext() tea.Cmd {
	if len(s.entries) == 0 || s.cursor >= len(s.entries) {
		return nil
	}
	e := s.entries[s.cursor]
	switch e.Kind {
	case resultTrack:
		return func() tea.Msg {
			return SearchModalMsg{
				EnqueueNext: []models.Track{e.Track},
			}
		}
	case resultArtist:
		if e.IsSubsonic {
			s.PendingSubsonicArtistID = e.SubsonicID
			s.ResolveEnqueueNext = true
			return nil
		}
		s.ResolveArtistName = e.ArtistName
		s.ResolveEnqueueNext = true
		return nil
	case resultAlbum:
		if e.IsSubsonic {
			s.PendingSubsonicAlbumID = e.SubsonicID
			s.ResolveEnqueueNext = true
			return nil
		}
		s.ResolveAlbumArtist = e.AlbumArtist
		s.ResolveAlbumName = e.AlbumName
		s.ResolveEnqueueNext = true
		return nil
	}
	return nil
}

type SearchDebounceMsg struct {
	Query string
}

type SearchResultsMsg struct {
	Entries []searchEntry
}

func debounceSearchModalCmd(query string) tea.Cmd {
	return tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
		return SearchDebounceMsg{Query: query}
	})
}

type parsedQuery struct {
	fields    map[string]string
	plainText string
	hasPlain  bool
	yearMin   int
	yearMax   int
	hasYear   bool
}

func parseQuery(input string) parsedQuery {
	var pq parsedQuery
	pq.fields = make(map[string]string)

	input = strings.TrimSpace(input)
	if input == "" {
		return pq
	}

	input = strings.ReplaceAll(input, "song:", "title:")
	input = strings.ReplaceAll(input, "track:", "title:")

	terms := strings.Fields(input)
	for _, term := range terms {
		if strings.HasPrefix(term, "year:") {
			yearVal := strings.TrimPrefix(term, "year:")
			if dashIdx := strings.Index(yearVal, "-"); dashIdx >= 0 {
				_, _ = fmt.Sscanf(yearVal[:dashIdx], "%d", &pq.yearMin)
				_, _ = fmt.Sscanf(yearVal[dashIdx+1:], "%d", &pq.yearMax)
			} else {
				_, _ = fmt.Sscanf(yearVal, "%d", &pq.yearMin)
				pq.yearMax = pq.yearMin
			}
			pq.hasYear = true
		} else if colonIdx := strings.Index(term, ":"); colonIdx > 0 {
			field := strings.ToLower(term[:colonIdx])
			value := term[colonIdx+1:]
			pq.fields[field] = value
		} else {
			if pq.hasPlain {
				pq.plainText += " " + term
			} else {
				pq.plainText = term
				pq.hasPlain = true
			}
		}
	}
	return pq
}

func trackCountByArtist(db *db.LibraryDB, artist string) (int, error) {
	tracks, err := db.GetTracksByArtist(artist)
	if err != nil {
		return 0, err
	}
	return len(tracks), nil
}

// --- Search execution ---

func (s *Search) executeSearch(query string) tea.Cmd {
	return func() tea.Msg {
		var entries []searchEntry

		if s.source == SearchLocal || s.source == SearchBoth {
			entries = s.localSearch(query)
		}

		if entries == nil {
			entries = []searchEntry{}
		}

		return SearchResultsMsg{Entries: entries}
	}
}

func (s *Search) localSearch(query string) []searchEntry {
	if s.db == nil {
		return nil
	}

	pq := parseQuery(query)

	var results []models.Track
	var fieldEntries []searchEntry

	if pq.hasPlain && len(pq.fields) == 0 {
		q := pq.plainText
		seenAlbum := make(map[string]bool)

		// Artist matches + their albums
		if artists, err := s.db.SearchArtistsLike(q); err == nil {
			for i, name := range artists {
				if i >= 10 {
					break
				}
				count, _ := trackCountByArtist(s.db, name)
				fieldEntries = append(fieldEntries, searchEntry{
					Kind: resultArtist, ArtistName: name, TrackCount: count,
				})
				if albums, err := s.db.GetAlbumsByArtist(name); err == nil {
					for _, album := range albums {
						key := name + "|" + album
						if seenAlbum[key] {
							continue
						}
						seenAlbum[key] = true
						fieldEntries = append(fieldEntries, searchEntry{
							Kind: resultAlbum, AlbumName: album, AlbumArtist: name,
						})
					}
				}
			}
		}

		// Album-name matches (albums whose name directly contains the query)
		if albums, err := s.db.SearchAlbumsLike(q); err == nil {
			for _, a := range albums {
				key := a.Artist + "|" + a.Album
				if seenAlbum[key] {
					continue
				}
				seenAlbum[key] = true
				fieldEntries = append(fieldEntries, searchEntry{
					Kind: resultAlbum, AlbumName: a.Album, AlbumArtist: a.Artist,
				})
			}
		}
		results = s.fuzzySearchAll(q)
	} else if len(pq.fields) > 0 {
		results, fieldEntries = s.searchLocalFields(pq)
	}

	if pq.hasYear && len(results) > 0 {
		results = filterByYear(results, pq.yearMin, pq.yearMax)
	}

	if len(results) > 200 {
		results = results[:200]
	}

	var entries []searchEntry
	if len(fieldEntries) > 0 {
		entries = append(entries, fieldEntries...)
	}
	for _, t := range results {
		entries = append(entries, searchEntry{Kind: resultTrack, Track: t})
	}
	return entries
}

func (s *Search) searchLocalFields(pq parsedQuery) ([]models.Track, []searchEntry) {
	fieldOrder := make([]string, 0, len(pq.fields))
	for f := range pq.fields {
		fieldOrder = append(fieldOrder, f)
	}

	var allTracks []models.Track
	var entryResults []searchEntry

	for _, field := range fieldOrder {
		value := pq.fields[field]

		switch field {
		case "artist":
			matches, _ := s.db.SearchArtistsLike(value)
			for _, name := range matches {
				count, _ := trackCountByArtist(s.db, name)
				entryResults = append(entryResults, searchEntry{
					Kind: resultArtist, ArtistName: name, TrackCount: count,
				})
			}
			tracks := s.fuzzyExpandField("artist", value)
			allTracks = append(allTracks, tracks...)

		case "album":
			albums, _ := s.db.SearchAlbumsLike(value)
			for _, a := range albums {
				entryResults = append(entryResults, searchEntry{
					Kind: resultAlbum, AlbumName: a.Album, AlbumArtist: a.Artist,
				})
			}
			tracks := s.fuzzyExpandField("album", value)
			allTracks = append(allTracks, tracks...)

		case "title":
			tracks := s.fuzzySearchAllByField("title", value)
			allTracks = append(allTracks, tracks...)

		case "genre":
			tracks := s.fuzzyExpandField("genre", value)
			allTracks = append(allTracks, tracks...)
		}
	}

	return allTracks, entryResults
}

// --- Plain fuzzy: match against combined "artist album title" strings ---

func (s *Search) fuzzySearchAll(query string) []models.Track {
	all, err := s.db.GetAllTracks()
	if err != nil {
		return nil
	}

	targets := make([]string, len(all))
	for i, t := range all {
		targets[i] = strings.ToLower(t.Artist + " " + t.Album + " " + t.Title)
	}

	matches := fuzzy.Find(strings.ToLower(query), targets)
	if len(matches) == 0 {
		return nil
	}

	results := make([]models.Track, len(matches))
	for i, m := range matches {
		results[i] = all[m.Index]
	}
	return results
}

// fuzzySearchAllByField matches against a single field for all tracks
// (used for title searches where we want direct track matches, not expansion)
func (s *Search) fuzzySearchAllByField(field, value string) []models.Track {
	all, err := s.db.GetAllTracks()
	if err != nil {
		return nil
	}

	targets := make([]string, len(all))
	switch field {
	case "title":
		for i, t := range all {
			targets[i] = strings.ToLower(t.Title)
		}
	default:
		return nil
	}

	matches := fuzzy.Find(strings.ToLower(value), targets)
	if len(matches) == 0 {
		return nil
	}

	results := make([]models.Track, len(matches))
	for i, m := range matches {
		results[i] = all[m.Index]
	}
	return results
}

// fuzzyExpandField fuzzy-matches query against field vocabulary (e.g. artist names),
// then expands the top matches to their tracks via DB lookup.
func (s *Search) fuzzyExpandField(field, value string) []models.Track {
	var vocab []string
	var err error

	switch field {
	case "artist":
		vocab, err = s.db.GetAllArtists()
	case "album":
		vocab, err = s.db.GetAllAlbums()
	case "genre":
		vocab, err = s.db.GetGenres()
	}
	if err != nil || len(vocab) == 0 {
		return nil
	}

	lowerVocab := make([]string, len(vocab))
	for i, v := range vocab {
		lowerVocab[i] = strings.ToLower(v)
	}

	matches := fuzzy.Find(strings.ToLower(value), lowerVocab)
	if len(matches) == 0 {
		return nil
	}

	n := min(len(matches), 5)
	matched := make([]string, n)
	for i := 0; i < n; i++ {
		matched[i] = vocab[matches[i].Index]
	}

	tracks, err := s.db.GetTracksByField(field, matched)
	if err != nil {
		return nil
	}
	return tracks
}

func filterByYear(tracks []models.Track, yearMin, yearMax int) []models.Track {
	var filtered []models.Track
	for _, t := range tracks {
		if t.Year >= yearMin && t.Year <= yearMax {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// --- View ---

func (s Search) View() string {
	var b strings.Builder

	b.WriteString(s.styles.AccentStyle.Render("Search: "))
	b.WriteString(s.input.View())
	b.WriteString("\n")

	b.WriteString(s.renderSourceBar())
	b.WriteString("\n")

	maxVisible := s.height - 7
	if maxVisible < 1 {
		maxVisible = 1
	}

	if len(s.entries) > 0 {
		for i := 0; i < maxVisible; i++ {
			idx := s.scrollOffset + i
			if idx >= len(s.entries) {
				break
			}
			e := s.entries[idx]
			badge := ""
			if e.IsSubsonic && s.subsonicBadge != "" {
				badge = s.styles.MutedStyle.Render("[" + s.subsonicBadge + "] ")
			}

			var line string
			switch e.Kind {
			case resultArtist:
				if e.IsSubsonic {
					label := fmt.Sprintf("Artist: %s (%d albums)", e.ArtistName, e.AlbumCount)
					label = ansi.Truncate(label, s.width-10, "...")
					line = badge + label
				} else {
					label := fmt.Sprintf("Artist: %s (%d tracks)", e.ArtistName, e.TrackCount)
					label = ansi.Truncate(label, s.width-10, "...")
					line = label
				}
			case resultAlbum:
				label := fmt.Sprintf("Album: %s — %s", e.AlbumArtist, e.AlbumName)
				label = ansi.Truncate(label, s.width-10, "...")
				line = badge + label
			case resultTrack:
				dur := e.Track.GetDurationFormatted()
				label := fmt.Sprintf("%s - %s - %s", e.Track.Artist, e.Track.Album, e.Track.Title)
				avail := s.width - len(badge) - len(dur) - 8
				if avail < 10 {
					avail = 10
				}
				label = ansi.Truncate(label, avail, "...")
				line = fmt.Sprintf("%s%s %s", badge, label, s.styles.MutedStyle.Render(dur))
			}

			var prefix string
			if idx == s.cursor {
				prefix = s.styles.CursorStyle.Render("▸ ")
			} else {
				prefix = " "
			}

			row := prefix + line
			if idx == s.cursor {
				b.WriteString(row)
			} else {
				b.WriteString(s.styles.ForegroundStyle.Render(row))
			}
			if i < maxVisible-1 {
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(renderSearchHelp(s.styles, len(s.entries), s.source.String()))
	} else if s.input.Value() != "" {
		b.WriteString("\n")
		b.WriteString(s.styles.MutedStyle.Render("No results found"))
	} else {
		b.WriteString("\n")
		b.WriteString(renderSearchHint(s.styles))
	}

	return lipgloss.NewStyle().Width(s.width).Render(b.String())
}

func (s Search) renderSourceBar() string {
	labels := []string{"Local", "Both", "Subsonic"}
	var parts []string
	for i, l := range labels {
		if SearchSource(i) == s.source {
			parts = append(parts, s.styles.AccentStyle.Render("["+l+"]"))
		} else {
			parts = append(parts, s.styles.MutedStyle.Render(l))
		}
	}
	return strings.Join(parts, "  ")
}

func renderSearchHelp(styles *config.ThemeStyles, numResults int, source string) string {
	type helpItem struct{ key, desc string }
	items := []helpItem{
		{"↑↓", "nav"},
		{"↵", "play"},
		{"^e", "enq"},
		{"alt+e", "next"},
		{"^t", "source"},
		{"esc", "close"},
	}

	var b strings.Builder
	b.WriteString(styles.MutedStyle.Render(fmt.Sprintf("%d results [%s] ", numResults, source)))
	for i, h := range items {
		if i > 0 {
			b.WriteString(styles.MutedStyle.Render("  "))
		}
		b.WriteString(styles.AccentStyle.Render(h.key))
		b.WriteString(styles.MutedStyle.Render(" " + h.desc))
	}
	return b.String()
}

func renderSearchHint(styles *config.ThemeStyles) string {
	return styles.MutedStyle.Render("Type to search (supports artist:, album:, title:/song:/track:, genre:, year:1997)  ^t source")
}
