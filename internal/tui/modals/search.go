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
)

type SearchModalMsg struct {
	PlayTracks []models.Track
	PlayIndex  int
	Enqueue    []models.Track
	Closed     bool
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
						PlayTracks: s.results,
						PlayIndex:  s.cursor,
					}
				}
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
	case searchModalDebounceMsg:
		if s.input.Value() != msg.query {
			return nil
		}
		return s.executeSearch(msg.query)
	case searchModalResultsMsg:
		s.results = msg.results
		s.cursor = 0
		s.scrollOffset = 0
		return nil
	}
	return nil
}

func (s *Search) executeSearch(query string) tea.Cmd {
	return func() tea.Msg {
		sq := buildSearchQuery(query)
		results, err := s.db.SearchWithYearRange(sq.FTSQuery, sq.YearMin, sq.YearMax)
		if err != nil {
			results, err = s.db.SearchLike(query)
			if err != nil {
				return searchModalResultsMsg{results: nil}
			}
		}
		return searchModalResultsMsg{results: results}
	}
}

type searchModalDebounceMsg struct {
	query string
}

type searchModalResultsMsg struct {
	results []models.Track
}

func debounceSearchModalCmd(query string) tea.Cmd {
	return tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
		return searchModalDebounceMsg{query: query}
	})
}

type SearchQuery struct {
	FTSQuery string
	YearMin  int
	YearMax  int
}

func buildSearchQuery(input string) SearchQuery {
	input = strings.TrimSpace(input)
	if input == "" {
		return SearchQuery{}
	}

	var yearMin, yearMax int
	ftsParts := input

	if strings.Contains(input, "year:") {
		terms := strings.Fields(input)
		var ftsTerms []string
		for _, t := range terms {
			if strings.HasPrefix(t, "year:") {
				yearVal := strings.TrimPrefix(t, "year:")
				if dashIdx := strings.Index(yearVal, "-"); dashIdx >= 0 {
					_, _ = fmt.Sscanf(yearVal[:dashIdx], "%d", &yearMin)
					_, _ = fmt.Sscanf(yearVal[dashIdx+1:], "%d", &yearMax)
				} else {
					_, _ = fmt.Sscanf(yearVal, "%d", &yearMin)
					yearMax = yearMin
				}
			} else {
				ftsTerms = append(ftsTerms, t)
			}
		}
		ftsParts = strings.Join(ftsTerms, " ")
	}

	if !strings.Contains(ftsParts, ":") && !strings.Contains(ftsParts, "*") && ftsParts != "" {
		terms := strings.Fields(ftsParts)
		parts := make([]string, len(terms))
		for i, t := range terms {
			parts[i] = t + "*"
		}
		ftsParts = strings.Join(parts, " ")
	}

	return SearchQuery{
		FTSQuery: ftsParts,
		YearMin:  yearMin,
		YearMax:  yearMax,
	}
}

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
		b.WriteString(s.styles.MutedStyle.Render(fmt.Sprintf("%d results ↑/↓ navigate enter play ^e enqueue esc close", len(s.results))))
	} else if s.input.Value() != "" {
		b.WriteString(s.styles.MutedStyle.Render("No results found"))
	} else {
		b.WriteString(s.styles.MutedStyle.Render("Type to search (supports artist:name, album:name, genre:term, year:1997)"))
	}

	contentWidth := s.width - 4
	if contentWidth < 30 {
		contentWidth = 30
	}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.styles.AccentStyle.GetForeground()).
		Padding(0, 1).
		Width(contentWidth)

	return modalStyle.Render(b.String())
}
