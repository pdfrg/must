package modals

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/pdfrg/must/internal/config"
)

type HelpModalMsg struct {
	Closed bool
}

type HelpEntry struct {
	Key  string
	Desc string
}

type Help struct {
	styles  *config.ThemeStyles
	width   int
	height  int
	cursor  int
	entries []HelpEntry
}

func NewHelp(styles *config.ThemeStyles, entries []HelpEntry) *Help {
	return &Help{
		styles:  styles,
		entries: entries,
	}
}

func (h *Help) SetSize(width, height int) {
	h.width = width
	h.height = height
}

func (h *Help) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "q", "?":
			return func() tea.Msg { return HelpModalMsg{Closed: true} }
		case "down", "j":
			if h.cursor < len(h.entries)-1 {
				h.cursor++
			}
		case "up", "k":
			if h.cursor > 0 {
				h.cursor--
			}
		case "pgdown":
			ps := max(h.height-6, 1)
			h.cursor = min(h.cursor+ps, len(h.entries)-1)
		case "pgup":
			ps := max(h.height-6, 1)
			h.cursor = max(h.cursor-ps, 0)
		case "home":
			h.cursor = 0
		case "end":
			if len(h.entries) > 0 {
				h.cursor = len(h.entries) - 1
			}
		}
	}
	return nil
}

func (h Help) View() string {
	contentWidth := 60
	if h.width-8 < contentWidth {
		contentWidth = h.width - 8
	}
	if contentWidth < 30 {
		contentWidth = 30
	}

	var b strings.Builder

	title := h.styles.AccentStyle.Render("KEYBOARD SHORTCUTS")
	b.WriteString(centerStyled(title, contentWidth))
	b.WriteString("\n\n")

	maxVisible := h.height - 8
	if maxVisible < 5 {
		maxVisible = 5
	}

	start := 0
	if h.cursor >= maxVisible {
		start = h.cursor - maxVisible + 1
	}

	for i := 0; i < maxVisible && start+i < len(h.entries); i++ {
		entry := h.entries[start+i]
		line := fmt.Sprintf(" %-14s %s", entry.Key, entry.Desc)
		if start+i == h.cursor {
			b.WriteString(h.styles.CursorStyle.Render(line))
		} else {
			b.WriteString(h.styles.ForegroundStyle.Render(line))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	helpText := h.styles.MutedStyle.Render("↑/↓ scroll  esc close")
	b.WriteString(centerStyled(helpText, contentWidth))

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(h.styles.AccentStyle.GetForeground()).
		Padding(1, 2).
		Width(contentWidth + 4)

	return modalStyle.Render(b.String())
}

func centerStyled(text string, width int) string {
	visWidth := lipgloss.Width(text)
	if visWidth >= width {
		return text
	}
	pad := (width - visWidth) / 2
	return strings.Repeat(" ", pad) + text
}
