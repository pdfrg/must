package modals

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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

type Search struct {
	styles *config.ThemeStyles
	db     *db.LibraryDB
	width  int
	height int

	input        textinput.Model
	results      []models.Track
	cursor       int
	scrollOffset int
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
		results: []models.Track{},
	}
}

func (s *Search) SetDB(libraryDB *db.LibraryDB) {
	s.db = libraryDB
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
	s.results = nil
	s.cursor = 0
	s.scrollOffset = 0
}

func (s *Search) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return func() tea.Msg { return SearchModalMsg{Closed: true} }
		case "enter":
			if len(s.results) > 0 && s.cursor < len(s.results) {
				return func() tea.Msg {
					return SearchModalMsg{
						PlayTracks: []models.Track{s.results[s.cursor]},
						PlayIndex:  0,
					}
				}
			}
			return nil
		case "alt+enter":
			if len(s.results) > 0 && s.cursor < len(s.results) {
				return s.playAlbumCmd(s.results[s.cursor])
			}
			return nil
		case "alt+e":
			if len(s.results) > 0 && s.cursor < len(s.results) {
				return s.enqueueAlbumCmd(s.results[s.cursor])
			}
			return nil
		case "ctrl+e":
			if len(s.results) > 0 && s.cursor < len(s.results) {
				return func() tea.Msg {
					return SearchModalMsg{
						Enqueue: []models.Track{s.results[s.cursor]},
					}
				}
			}
			return nil
		case "E":
			if len(s.results) > 0 && s.cursor < len(s.results) {
				return func() tea.Msg {
					return SearchModalMsg{
						EnqueueNext: []models.Track{s.results[s.cursor]},
					}
				}
			}
			return nil
		case "down", "ctrl+n":
			if len(s.results) > 0 && s.cursor < len(s.results)-1 {
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
		default:
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			if s.db != nil && s.input.Value() != "" {
				return tea.Batch(cmd, debounceSearchModalCmd(s.input.Value()))
			}
			s.results = nil
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
		s.results = msg.Results
		s.cursor = 0
		s.scrollOffset = 0
		return nil
	}
	return nil
}

func (s *Search) playAlbumCmd(t models.Track) tea.Cmd {
	return func() tea.Msg {
		tracks, err := s.db.GetTracksByArtistAndAlbum(t.Artist, t.Album)
		if err != nil || len(tracks) == 0 {
			return SearchModalMsg{PlayTracks: []models.Track{t}, PlayIndex: 0}
		}
		return SearchModalMsg{PlayTracks: tracks, PlayIndex: 0}
	}
}

func (s *Search) enqueueAlbumCmd(t models.Track) tea.Cmd {
	return func() tea.Msg {
		tracks, err := s.db.GetTracksByArtistAndAlbum(t.Artist, t.Album)
		if err != nil || len(tracks) == 0 {
			return SearchModalMsg{Enqueue: []models.Track{t}}
		}
		return SearchModalMsg{Enqueue: tracks}
	}
}

type SearchDebounceMsg struct {
	Query string
}

type SearchResultsMsg struct {
	Results []models.Track
}

func debounceSearchModalCmd(query string) tea.Cmd {
	return tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
		return SearchDebounceMsg{Query: query}
	})
}

// --- Parse search query ---

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

// --- Search execution ---

func (s *Search) executeSearch(query string) tea.Cmd {
	return func() tea.Msg {
		pq := parseQuery(query)

		var results []models.Track

		if pq.hasPlain && len(pq.fields) == 0 {
			results = s.fuzzySearchAll(pq.plainText)
		} else if len(pq.fields) > 0 {
			results = s.fuzzySearchFields(pq)
		}

		if pq.hasYear && len(results) > 0 {
			results = filterByYear(results, pq.yearMin, pq.yearMax)
		}

		if len(results) > 200 {
			results = results[:200]
		}

		if results == nil {
			results = []models.Track{}
		}

		return SearchResultsMsg{Results: results}
	}
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

// --- Field fuzzy: match against field vocabulary and expand to tracks ---

func (s *Search) fuzzySearchFields(pq parsedQuery) []models.Track {
	var allResults [][]models.Track

	fieldOrder := make([]string, 0, len(pq.fields))
	for f := range pq.fields {
		fieldOrder = append(fieldOrder, f)
	}

	for _, field := range fieldOrder {
		value := pq.fields[field]
		var tracks []models.Track

		switch field {
		case "title":
			tracks = s.fuzzySearchAllByField("title", value)
		case "artist":
			tracks = s.fuzzyExpandField("artist", value)
		case "album":
			tracks = s.fuzzyExpandField("album", value)
		case "genre":
			tracks = s.fuzzyExpandField("genre", value)
		default:
			continue
		}

		if len(tracks) == 0 {
			return nil
		}
		allResults = append(allResults, tracks)
	}

	if len(allResults) == 0 {
		return nil
	}
	if len(allResults) == 1 {
		return allResults[0]
	}

	return intersectTracks(allResults)
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

// --- Result helpers ---

func intersectTracks(results [][]models.Track) []models.Track {
	if len(results) == 0 {
		return nil
	}

	idSet := make(map[int64]int)
	for _, t := range results[0] {
		idSet[t.ID] = 1
	}

	for i := 1; i < len(results); i++ {
		seen := make(map[int64]bool)
		for _, t := range results[i] {
			if _, exists := idSet[t.ID]; exists {
				seen[t.ID] = true
			}
		}
		idSet = make(map[int64]int)
		for id := range seen {
			idSet[id] = 1
		}
	}

	var intersection []models.Track
	for _, t := range results[0] {
		if _, exists := idSet[t.ID]; exists {
			intersection = append(intersection, t)
		}
	}
	return intersection
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

	maxVisible := s.height - 6
	if maxVisible < 1 {
		maxVisible = 1
	}

	if len(s.results) > 0 {
		for i := 0; i < maxVisible; i++ {
			idx := s.scrollOffset + i
			if idx >= len(s.results) {
				break
			}
			t := s.results[idx]
			dur := t.GetDurationFormatted()
			label := fmt.Sprintf("%s - %s - %s", t.Artist, t.Album, t.Title)
			avail := s.width - len(dur) - 6
			if avail < 10 {
				avail = 10
			}
			label = ansi.Truncate(label, avail, "...")

			var prefix string
			if idx == s.cursor {
				prefix = s.styles.CursorStyle.Render("▸ ")
			} else {
				prefix = " "
			}

			line := fmt.Sprintf("%s%s %s", prefix, label, s.styles.MutedStyle.Render(dur))
			if idx == s.cursor {
				b.WriteString(line)
			} else {
				b.WriteString(s.styles.ForegroundStyle.Render(line))
			}
			if i < maxVisible-1 {
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(renderSearchHelp(s.styles, len(s.results)))
	} else if s.input.Value() != "" {
		b.WriteString("\n")
		b.WriteString(s.styles.MutedStyle.Render("No results found"))
	} else {
		b.WriteString("\n")
		b.WriteString(renderSearchHint(s.styles))
	}

	return lipgloss.NewStyle().Width(s.width).Render(b.String())
}

func renderSearchHelp(styles *config.ThemeStyles, numResults int) string {
	type helpItem struct{ key, desc string }
	items := []helpItem{
		{"↑↓", "nav"},
		{"↵", "play"},
		{"^e", "enq"},
		{"alt+↵", "album"},
		{"alt+e", "enq-album"},
		{"esc", "close"},
	}

	var b strings.Builder
	b.WriteString(styles.MutedStyle.Render(fmt.Sprintf("%d results ", numResults)))
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
	return styles.MutedStyle.Render("Type to search (supports artist:, album:, title:/song:/track:, genre:, year:1997)")
}
