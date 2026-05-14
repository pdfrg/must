package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/pdfrg/must/internal/models"
)

func (m Model) handleScanComplete(msg scanCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.scanMsg = fmt.Sprintf("Scan error: %v", msg.err)
		return m, nil
	}

	m.libraryReady = true
	m.scanResult = msg.result
	m.libraryDB = msg.db

	m.loadCLIPaths()

	if m.randomMode && len(m.paths) == 0 && m.libraryDB != nil {
		return m.handleRandomPlay()
	}

	artists, err := m.libraryDB.GetAllArtists()
	if err != nil {
		m.scanMsg = fmt.Sprintf("Error loading artists: %v", err)
		return m, nil
	}
	m.artists = artists

	if msg.result != nil {
		r := msg.result
		m.scanMsg = fmt.Sprintf("Library: %d tracks (%d new, %d updated, %d removed)",
			r.TotalFiles, r.NewFiles, r.UpdatedFiles, r.RemovedFiles)
	} else {
		count, _ := m.libraryDB.TrackCount()
		m.scanMsg = fmt.Sprintf("Library: %d tracks", count)
	}

	return m, setStatus(&m, m.scanMsg, false)
}

func (m Model) openSearch() (tea.Model, tea.Cmd) {
	m.searching = true
	m.searchInput.SetValue("")
	m.searchResults = nil
	m.searchCursor = 0
	m.searchScrollOffset = 0
	cmd := m.searchInput.Focus()
	m.searchInput.SetWidth(min(m.width-4, 80))
	return m, cmd
}

func (m Model) closeSearch() (tea.Model, tea.Cmd) {
	m.searching = false
	m.searchInput.Blur()
	m.searchInput.SetValue("")
	m.searchResults = nil
	m.searchCursor = 0
	m.searchScrollOffset = 0
	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keyMap.Escape), key.Matches(msg, m.keyMap.Quit):
		return m.closeSearch()

	case key.Matches(msg, m.keyMap.Enter):
		if len(m.searchResults) > 0 && m.searchCursor < len(m.searchResults) {
			track := m.searchResults[m.searchCursor]
			m.playlist = []models.Track{track}
			for _, t := range m.searchResults {
				if t.Path != track.Path {
					m.playlist = append(m.playlist, t)
				}
			}
			m.searching = false
			m.searchInput.Blur()
			return m, m.playTrack(0)
		}
		return m, nil

	case key.Matches(msg, m.keyMap.CursorDown):
		if len(m.searchResults) > 0 && m.searchCursor < len(m.searchResults)-1 {
			m.searchCursor++
			maxVisible := m.height - 5
			if maxVisible < 1 {
				maxVisible = 1
			}
			if m.searchCursor >= m.searchScrollOffset+maxVisible {
				m.searchScrollOffset = m.searchCursor - maxVisible + 1
			}
		}
		return m, nil

	case key.Matches(msg, m.keyMap.CursorUp):
		if m.searchCursor > 0 {
			m.searchCursor--
			if m.searchCursor < m.searchScrollOffset {
				m.searchScrollOffset = m.searchCursor
			}
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)

	if m.libraryDB != nil && m.searchInput.Value() != "" {
		return m, tea.Batch(cmd, m.executeSearch(m.searchInput.Value()))
	} else {
		m.searchResults = nil
		m.searchCursor = 0
		m.searchScrollOffset = 0
	}

	return m, cmd
}

func (m Model) executeSearch(query string) tea.Cmd {
	return func() tea.Msg {
		ftsQuery := buildFTSQuery(query)
		results, err := m.libraryDB.SearchFTS(ftsQuery)
		if err != nil {
			results, err = m.libraryDB.SearchLike(query)
			if err != nil {
				return searchResultsMsg{results: nil}
			}
		}
		return searchResultsMsg{results: results}
	}
}

func buildFTSQuery(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	if strings.Contains(input, ":") || strings.Contains(input, "*") {
		return input
	}

	terms := strings.Fields(input)
	parts := make([]string, len(terms))
	for i, t := range terms {
		parts[i] = t + "*"
	}
	return strings.Join(parts, " ")
}
