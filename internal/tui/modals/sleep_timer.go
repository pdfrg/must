package modals

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/pdfrg/must/internal/config"
)

type SleepTimerMsg struct {
	Duration  time.Duration
	Cancelled bool
	Closed    bool
}

type SleepTimer struct {
	styles   *config.ThemeStyles
	width    int
	height   int
	cursor   int
	active   bool
	duration time.Duration
}

func NewSleepTimer(styles *config.ThemeStyles, active bool, dur time.Duration) *SleepTimer {
	return &SleepTimer{
		styles:   styles,
		active:   active,
		duration: dur,
		cursor:   0,
	}
}

func (s *SleepTimer) SetSize(width, height int) {
	s.width = width
	s.height = height
}

func (s *SleepTimer) numOptions() int {
	return 9
}

func (s *SleepTimer) getSelectedDuration() time.Duration {
	switch s.cursor {
	case 0:
		return 5 * time.Minute
	case 1:
		return 10 * time.Minute
	case 2:
		return 15 * time.Minute
	case 3:
		return 30 * time.Minute
	case 4:
		return 45 * time.Minute
	case 5:
		return 60 * time.Minute
	case 6:
		return 90 * time.Minute
	case 7:
		return 2 * time.Hour
	default:
		return 0
	}
}

func (s *SleepTimer) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "q":
			return func() tea.Msg { return SleepTimerMsg{Closed: true} }

		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}

		case "down", "j":
			if s.cursor < s.numOptions()-1 {
				s.cursor++
			}

		case "enter", " ":
			if s.active && s.cursor == 8 {
				return func() tea.Msg { return SleepTimerMsg{Cancelled: true} }
			}
			if !s.active {
				dur := s.getSelectedDuration()
				if dur > 0 {
					return func() tea.Msg { return SleepTimerMsg{Duration: dur} }
				}
			}
		}
	}
	return nil
}

func (s *SleepTimer) View() string {
	contentWidth := 40
	if s.width-8 < contentWidth {
		contentWidth = s.width - 8
	}
	if contentWidth < 30 {
		contentWidth = 30
	}

	var b strings.Builder

	if s.active {
		remainingMins := int(s.duration.Minutes())
		headerText := fmt.Sprintf("Sleep Timer — %d min remaining", remainingMins)
		b.WriteString(centerStyled(s.styles.AccentStyle.Bold(true).Render(headerText), contentWidth))
	} else {
		b.WriteString(centerStyled(s.styles.AccentStyle.Bold(true).Render("Sleep Timer"), contentWidth))
	}
	b.WriteString("\n\n")

	options := []string{
		"5 min", "10 min", "15 min", "30 min",
		"45 min", "60 min", "90 min", "2 hours",
		"Cancel",
	}

	for i, label := range options {
		style := s.styles.MutedStyle
		prefix := "  "
		if i == s.cursor {
			style = s.styles.AccentStyle.Bold(true)
			prefix = "» "
		}

		b.WriteString(style.Render(prefix + label))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	helpText := s.styles.AccentStyle.Render("Enter") + s.styles.MutedStyle.Render(" set ") +
		s.styles.AccentStyle.Render("Esc/q") + s.styles.MutedStyle.Render(" close")
	b.WriteString(centerStyled(helpText, contentWidth))
	b.WriteString("\n")

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.styles.AccentStyle.GetForeground()).
		Padding(1, 2).
		Width(contentWidth + 4)

	rendered := modalStyle.Render(b.String())

	visWidth := lipgloss.Width(rendered)
	visHeight := lipgloss.Height(rendered)
	padLeft := max(0, (s.width-visWidth)/2)
	padTop := max(0, (s.height-visHeight)/2)

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
