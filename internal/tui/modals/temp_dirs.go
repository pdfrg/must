package modals

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/pdfrg/must/internal/config"
)

type TempDirsModalMsg struct {
	DirPath string
	Action  string // "play", "enqueue", "enqueue_next"
	Closed  bool
}

type tempDirEntry struct {
	isHeader   bool
	label      string
	dirPath    string
	trackCount int
}

type TempDirs struct {
	styles       *config.ThemeStyles
	entries      []tempDirEntry
	cursor       int
	scrollOffset int
	width        int
	height       int
}

func NewTempDirs(styles *config.ThemeStyles) *TempDirs {
	return &TempDirs{
		styles:  styles,
		entries: nil,
		cursor:  0,
	}
}

func (t *TempDirs) SetSize(width, height int) {
	t.width = width
	t.height = height
}

func (t *TempDirs) SetDirs(dirs []string) {
	t.entries = nil
	t.cursor = 0
	t.scrollOffset = 0
	home, _ := os.UserHomeDir()

	for _, dir := range dirs {
		expanded := os.ExpandEnv(dir)
		if strings.HasPrefix(expanded, "~/") {
			expanded = filepath.Join(home, expanded[2:])
		}

		info, err := os.Stat(expanded)
		if err != nil || !info.IsDir() {
			continue
		}

		entries, err := os.ReadDir(expanded)
		if err != nil {
			continue
		}

		var subdirs []string
		for _, e := range entries {
			if e.IsDir() {
				subdirs = append(subdirs, e.Name())
			}
		}
		if len(subdirs) == 0 {
			continue
		}

		sort.Slice(subdirs, func(i, j int) bool {
			return strings.ToLower(subdirs[i]) < strings.ToLower(subdirs[j])
		})

		headerLabel := expanded
		if home != "" {
			if rel, err := filepath.Rel(home, expanded); err == nil && !strings.HasPrefix(rel, "../") {
				headerLabel = "~/" + rel
			}
		}

		t.entries = append(t.entries, tempDirEntry{
			isHeader: true,
			label:    headerLabel,
		})

		for _, name := range subdirs {
			subPath := filepath.Join(expanded, name)
			count := countAudioFiles(subPath)
			t.entries = append(t.entries, tempDirEntry{
				label:      name,
				dirPath:    subPath,
				trackCount: count,
			})
		}
	}
}

func countAudioFiles(dir string) int {
	count := 0
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if isAudioExt(path) {
			count++
		}
		return nil
	})
	return count
}

func isAudioExt(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3", ".flac", ".ogg", ".opus", ".m4a", ".aac", ".wma", ".wav":
		return true
	}
	return false
}

func (t *TempDirs) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "q":
			return func() tea.Msg { return TempDirsModalMsg{Closed: true} }

		case "up", "k":
			t.cursorPrev()

		case "down", "j":
			t.cursorNext()

		case "pgup":
			pageSize := t.entryRowsAvailable()
			if pageSize < 1 {
				pageSize = 1
			}
			for i := 0; i < pageSize; i++ {
				t.cursorPrev()
			}

		case "pgdown":
			pageSize := t.entryRowsAvailable()
			if pageSize < 1 {
				pageSize = 1
			}
			for i := 0; i < pageSize; i++ {
				t.cursorNext()
			}

		case "home", "g":
			t.cursor = 0
			t.scrollOffset = 0
			t.skipHeadersForward()

		case "end", "G":
			t.cursor = len(t.entries) - 1
			t.scrollOffset = len(t.entries) * 3 // well past end; View() clamps
			t.skipHeadersBackward()
			t.ensureCursorVisible()

		case "enter", " ":
			if t.cursor >= 0 && t.cursor < len(t.entries) {
				entry := t.entries[t.cursor]
				if entry.isHeader {
					break
				}
				if entry.trackCount == 0 {
					break
				}
				return func() tea.Msg {
					return TempDirsModalMsg{DirPath: entry.dirPath, Action: "play"}
				}
			}

		case "e":
			if t.cursor >= 0 && t.cursor < len(t.entries) {
				entry := t.entries[t.cursor]
				if entry.isHeader || entry.trackCount == 0 {
					break
				}
				return func() tea.Msg {
					return TempDirsModalMsg{DirPath: entry.dirPath, Action: "enqueue"}
				}
			}

		case "E":
			if t.cursor >= 0 && t.cursor < len(t.entries) {
				entry := t.entries[t.cursor]
				if entry.isHeader || entry.trackCount == 0 {
					break
				}
				return func() tea.Msg {
					return TempDirsModalMsg{DirPath: entry.dirPath, Action: "enqueue_next"}
				}
			}
		}
	}
	return nil
}

func (t *TempDirs) entryLineStart(entryIdx int) int {
	if entryIdx <= 0 {
		return 0
	}
	lines := 0
	for i := 0; i < entryIdx; i++ {
		if t.entries[i].isHeader {
			lines += 3
		} else {
			lines++
		}
	}
	return lines
}

func (t *TempDirs) entryRowsAvailable() int {
	// Content area (inside border+padding): title(1) + blank(1) + help(1) + blank_before_help(1) = 4 fixed rows
	// Border adds 2 rows (top + bottom). Padding(0,1) adds 0 vertical rows.
	// Remaining rows = t.height - 2 (border) - 4 (fixed content) = t.height - 6
	available := t.height - 6
	if available < 1 {
		return 1
	}
	return available
}

func (t *TempDirs) totalEntryLines() int {
	return t.entryLineStart(len(t.entries))
}

func (t *TempDirs) visibleEntryRows(scroll int) int {
	avail := t.entryRowsAvailable()
	hasScrollUp := scroll > 0
	hasScrollDown := scroll+avail < t.totalEntryLines()
	n := avail
	if hasScrollUp {
		n--
	}
	if hasScrollDown {
		n--
	}
	if n < 1 {
		return 1
	}
	return n
}

func (t *TempDirs) cursorPrev() {
	if t.cursor <= 0 {
		return
	}
	t.cursor--
	if t.entries[t.cursor].isHeader {
		t.cursorPrev()
	}
	t.ensureCursorVisible()
}

func (t *TempDirs) cursorNext() {
	if t.cursor >= len(t.entries)-1 {
		return
	}
	t.cursor++
	if t.entries[t.cursor].isHeader {
		t.cursorNext()
	}
	t.ensureCursorVisible()
}

func (t *TempDirs) skipHeadersForward() {
	for t.cursor < len(t.entries) && t.entries[t.cursor].isHeader {
		t.cursor++
	}
}

func (t *TempDirs) skipHeadersBackward() {
	for t.cursor >= 0 && t.entries[t.cursor].isHeader {
		t.cursor--
	}
}

func (t *TempDirs) ensureCursorVisible() {
	cursorLine := t.entryLineStart(t.cursor)
	visRows := t.visibleEntryRows(t.scrollOffset)
	if cursorLine < t.scrollOffset {
		t.scrollOffset = cursorLine
		visRows = t.visibleEntryRows(t.scrollOffset) // recalc after offset change
	}
	if cursorLine >= t.scrollOffset+visRows {
		t.scrollOffset = cursorLine - visRows + 1
	}
}

func (t *TempDirs) View() string {
	if len(t.entries) == 0 {
		return t.renderEmpty()
	}

	contentWidth := t.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	entryLines := t.buildEntryLines(contentWidth)

	hasScrollUp := t.scrollOffset > 0
	hasScrollDown := len(entryLines) > t.scrollOffset+t.entryRowsAvailable()

	visibleRows := t.visibleEntryRows(t.scrollOffset)

	// Clamp scrollOffset to valid range in line space
	maxOffset := len(entryLines) - visibleRows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if t.scrollOffset > maxOffset {
		t.scrollOffset = maxOffset
	}

	var visibleLines []string
	if len(entryLines) > t.scrollOffset+visibleRows {
		visibleLines = entryLines[t.scrollOffset : t.scrollOffset+visibleRows]
	} else if t.scrollOffset < len(entryLines) {
		visibleLines = entryLines[t.scrollOffset:]
	}

	var b strings.Builder

	title := t.styles.AccentStyle.Bold(true).Render("Temp Directories")
	b.WriteString(centerStyled(title, contentWidth))
	b.WriteString("\n\n")

	if hasScrollUp {
		b.WriteString(t.styles.MutedStyle.Render(centerStyled("↑", contentWidth)))
		b.WriteString("\n")
	}

	for _, line := range visibleLines {
		b.WriteString(line)
		b.WriteString("\n")
	}

	if hasScrollDown {
		b.WriteString(t.styles.MutedStyle.Render(centerStyled("↓", contentWidth)))
		b.WriteString("\n")
	}

	helpText := t.styles.AccentStyle.Render("Enter") + t.styles.MutedStyle.Render(" play  ") +
		t.styles.AccentStyle.Render("e") + t.styles.MutedStyle.Render(" enqueue  ") +
		t.styles.AccentStyle.Render("E") + t.styles.MutedStyle.Render(" enqueue next  ") +
		t.styles.AccentStyle.Render("Esc/q") + t.styles.MutedStyle.Render(" close")
	b.WriteString("  ")
	b.WriteString(helpText)

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.styles.AccentStyle.GetForeground()).
		Padding(0, 1)

	rendered := modalStyle.Render(b.String())

	visHeight := lipgloss.Height(rendered)
	padTop := max(0, (t.height-visHeight)/2)

	if padTop > 0 {
		return strings.Repeat("\n", padTop) + rendered
	}

	return rendered
}

func (t *TempDirs) buildEntryLines(contentWidth int) []string {
	var lines []string

	for i, entry := range t.entries {
		if entry.isHeader {
			sep := strings.Repeat("─", contentWidth)
			lines = append(lines, t.styles.MutedStyle.Render(sep))
			lines = append(lines, t.styles.MutedStyle.Render("  "+entry.label))
			lines = append(lines, t.styles.MutedStyle.Render(sep))
			continue
		}

		prefix := "  "
		style := t.styles.ForegroundStyle
		if i == t.cursor {
			prefix = "» "
			style = t.styles.AccentStyle.Bold(true)
		}

		trackStr := fmt.Sprintf("%d tracks", entry.trackCount)
		if entry.trackCount == 0 {
			trackStr = "(empty)"
		}
		countStr := t.styles.MutedStyle.Render(trackStr)
		trackW := lipgloss.Width(countStr)

		maxNameW := contentWidth - trackW - 4
		if maxNameW < 3 {
			maxNameW = 3
		}

		name := entry.label
		nameW := lipgloss.Width(name)
		if nameW > maxNameW {
			truncW := maxNameW - 3
			if truncW < 1 {
				truncW = 1
			}
			truncated := ""
			runeW := 0
			for _, r := range name {
				rw := lipgloss.Width(string(r))
				if runeW+rw > truncW {
					break
				}
				truncated += string(r)
				runeW += rw
			}
			name = truncated + "..."
		}

		line := style.Render(prefix + name)
		padding := contentWidth - lipgloss.Width(line) - trackW - 2
		if padding < 1 {
			padding = 1
		}
		lines = append(lines, line+strings.Repeat(" ", padding)+countStr)
	}

	return lines
}

func (t *TempDirs) renderEmpty() string {
	contentWidth := t.width - 4
	if contentWidth < 30 {
		contentWidth = 30
	}

	var b strings.Builder
	b.WriteString(centerStyled(t.styles.AccentStyle.Bold(true).Render("Temp Directories"), contentWidth))
	b.WriteString("\n\n")
	b.WriteString(centerStyled(t.styles.MutedStyle.Render("No temp directories configured"), contentWidth))
	b.WriteString("\n")
	b.WriteString(centerStyled(t.styles.MutedStyle.Render("Set `temp_dirs` in config"), contentWidth))
	b.WriteString("\n\n")
	b.WriteString(centerStyled(
		t.styles.AccentStyle.Render("Esc/q")+t.styles.MutedStyle.Render(" close"),
		contentWidth,
	))

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.styles.AccentStyle.GetForeground()).
		Padding(0, 1)

	rendered := modalStyle.Render(b.String())

	visHeight := lipgloss.Height(rendered)
	padTop := max(0, (t.height-visHeight)/2)

	if padTop > 0 {
		return strings.Repeat("\n", padTop) + rendered
	}

	return rendered
}
