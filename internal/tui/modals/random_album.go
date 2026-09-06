package modals

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/pdfrg/must/internal/config"
)

// RandomAlbumMsg is emitted when the random-album source picker resolves.
type RandomAlbumMsg struct {
	Source string // "all", "local", "subsonic", "temp"
	Closed bool
}

// RandomAlbumOption is one selectable source in the picker.
type RandomAlbumOption struct {
	Source   string
	Count    int
	HasCount bool
}

// RandomAlbum is a small centered popup to pick the source for random album play.
type RandomAlbum struct {
	styles  *config.ThemeStyles
	options []RandomAlbumOption
	cursor  int
	width   int
	height  int
}

func NewRandomAlbum(styles *config.ThemeStyles, options []RandomAlbumOption) *RandomAlbum {
	return &RandomAlbum{
		styles:  styles,
		options: options,
		cursor:  0,
	}
}

func (r *RandomAlbum) SetSize(width, height int) {
	r.width = width
	r.height = height
}

// SetSubsonicCount fills in the lazily-loaded subsonic album count.
func (r *RandomAlbum) SetSubsonicCount(count int) {
	for i, opt := range r.options {
		if opt.Source == "subsonic" {
			r.options[i].Count = count
			r.options[i].HasCount = true
			return
		}
	}
}

func (r *RandomAlbum) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "q":
			return func() tea.Msg { return RandomAlbumMsg{Closed: true} }

		case "up", "k":
			if r.cursor > 0 {
				r.cursor--
			}

		case "down", "j":
			if r.cursor < len(r.options)-1 {
				r.cursor++
			}

		case "home", "g":
			r.cursor = 0

		case "end", "G":
			if len(r.options) > 0 {
				r.cursor = len(r.options) - 1
			}

		case "enter", " ":
			if r.cursor >= 0 && r.cursor < len(r.options) {
				source := r.options[r.cursor].Source
				return func() tea.Msg { return RandomAlbumMsg{Source: source} }
			}
		}
	}
	return nil
}

func (r *RandomAlbum) View() string {
	contentWidth := 40
	if r.width-8 < contentWidth {
		contentWidth = r.width - 8
	}
	if contentWidth < 30 {
		contentWidth = 30
	}

	var b strings.Builder

	b.WriteString(centerStyled(r.styles.AccentStyle.Bold(true).Render("Source for random album"), contentWidth))
	b.WriteString("\n\n")

	for i, opt := range r.options {
		label := opt.Source
		if opt.HasCount {
			albumWord := "albums"
			if opt.Count == 1 {
				albumWord = "album"
			}
			label = fmt.Sprintf("%s %d %s", opt.Source, opt.Count, albumWord)
		} else if opt.Source != "all" {
			label = fmt.Sprintf("%s …", opt.Source)
		}

		style := r.styles.MutedStyle
		prefix := "○ "
		if i == r.cursor {
			style = r.styles.AccentStyle.Bold(true)
			prefix = "● "
		}

		b.WriteString(style.Render(prefix + label))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	helpText := r.styles.AccentStyle.Render("↑/↓ j/k") + r.styles.MutedStyle.Render(" select  ") +
		r.styles.AccentStyle.Render("Enter") + r.styles.MutedStyle.Render(" play  ") +
		r.styles.AccentStyle.Render("Esc") + r.styles.MutedStyle.Render(" back")
	b.WriteString(centerStyled(helpText, contentWidth))
	b.WriteString("\n")

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(r.styles.AccentStyle.GetForeground()).
		Padding(1, 2).
		Width(contentWidth + 4)

	rendered := modalStyle.Render(b.String())

	visWidth := lipgloss.Width(rendered)
	visHeight := lipgloss.Height(rendered)
	padLeft := max(0, (r.width-visWidth)/2)
	padTop := max(0, (r.height-visHeight)/2)

	if padTop > 0 || padLeft > 0 {
		padStr := strings.Repeat(" ", padLeft)
		var sb strings.Builder
		for i := 0; i < padTop; i++ {
			sb.WriteString("\n")
		}
		for i, line := range strings.Split(rendered, "\n") {
			if i > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(padStr)
			sb.WriteString(line)
		}
		return sb.String()
	}
	return rendered
}
