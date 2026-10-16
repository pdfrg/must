package modals

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	termimg "github.com/blacktop/go-termimg"
	"github.com/charmbracelet/x/ansi"
	"github.com/pdfrg/must/internal/config"
)

const (
	exploreBorder    = 1
	explorePadH      = 2
	explorePadV      = 1
	exploreHeader    = 2 // title + source bar
	exploreChrome    = 8 // border(2) + padV(2) + header(2) + blank(1) + help(1)
	exploreMinRows   = 6
	exploreMaxRows   = 20
	exploreMaxWidth  = 96
	exploreArtWidth  = 26
	exploreMinArtW   = 72 // terminal width needed to show the art pane
	exploreMaxHeight = 22 // cap art pane height so it stays proportionate
)

// ExploreSource selects which music source(s) the batch is drawn from.
type ExploreSource int

const (
	ExploreAll ExploreSource = iota
	ExploreLocal
	ExploreSubsonic
	ExploreTemp
)

func (s ExploreSource) String() string {
	switch s {
	case ExploreLocal:
		return "local"
	case ExploreSubsonic:
		return "subsonic"
	case ExploreTemp:
		return "temp"
	default:
		return "all"
	}
}

func (s ExploreSource) Label() string {
	switch s {
	case ExploreLocal:
		return "Local"
	case ExploreSubsonic:
		return "Subsonic"
	case ExploreTemp:
		return "Temp"
	default:
		return "All"
	}
}

// ExploreAlbum is one album candidate shown in the Explore modal.
type ExploreAlbum struct {
	Name      string
	Artist    string
	Year      int
	Source    ExploreSource
	AlbumID   string // subsonic album id
	TrackPath string // local/temp representative track (art + dir)
}

// Key returns a stable identity used for dedupe and art request matching.
func (a ExploreAlbum) Key() string {
	return fmt.Sprintf("%d\x1f%s\x1f%s", a.Source, a.Artist, a.Name)
}

// ExploreAction is what the modal wants the model to do.
type ExploreAction int

const (
	ExploreClose ExploreAction = iota
	ExploreReroll
	ExploreLucky
	ExplorePlay
	ExploreEnqueue
	ExploreEnqueueNext
)

// ExploreMsg is emitted by the Explore modal for the model to act on.
type ExploreMsg struct {
	Action ExploreAction
	Source ExploreSource
	Album  *ExploreAlbum
	Limit  int
}

// ExploreAlbumsMsg delivers a freshly gathered batch back to the modal.
type ExploreAlbumsMsg struct {
	Albums    []ExploreAlbum
	Source    ExploreSource
	Counts    map[ExploreSource]int
	Available []ExploreSource
	Seq       int
	Err       error
}

// ExploreArtLoadedMsg delivers cover bytes for a highlighted album.
type ExploreArtLoadedMsg struct {
	Key       string
	Seq       int
	ImageData []byte
	Err       error
}

// ExploreRenderImageMsg asks the model to paint the rendered cover overlay.
type ExploreRenderImageMsg struct {
	ImageStr string
	Row      int
	Col      int
}

// Explore is the dedicated discovery modal: a randomized album list on the
// left with a large cover preview for the highlighted album on the right.
type Explore struct {
	styles        *config.ThemeStyles
	termWidth     int
	termHeight    int
	cellRatio     float64
	fontW         int
	fontH         int
	imageProtocol termimg.Protocol

	source    ExploreSource
	albums    []ExploreAlbum
	cursor    int
	counts    map[ExploreSource]int
	available []ExploreSource

	loading    bool
	artSeq     int
	artKey     string
	artStr     string
	artW       int
	artH       int
	artLoading bool
	artFailed  bool
	firstImage bool

	pendingArt    *ExploreAlbum
	pendingArtSeq int
}

func NewExplore(styles *config.ThemeStyles, cellRatio float64, fontW, fontH int) *Explore {
	if cellRatio < 1.0 {
		cellRatio = 2.0
	}
	return &Explore{
		styles:     styles,
		cellRatio:  cellRatio,
		fontW:      fontW,
		fontH:      fontH,
		counts:     map[ExploreSource]int{},
		firstImage: true,
	}
}

func (e *Explore) SetProtocol(p termimg.Protocol) { e.imageProtocol = p }

// SetLoading marks the batch as in-flight so the view can show progress.
func (e *Explore) SetLoading(loading bool) { e.loading = loading }

func (e *Explore) SetSize(w, h int) {
	e.termWidth = w
	e.termHeight = h
}

func (e *Explore) Source() ExploreSource { return e.source }

// VisibleLimit is how many albums fit without scrolling on this terminal.
func (e *Explore) VisibleLimit() int {
	n := e.termHeight - exploreChrome
	if n < exploreMinRows {
		n = exploreMinRows
	}
	if n > exploreMaxRows {
		n = exploreMaxRows
	}
	return n
}

func (e *Explore) CurrentAlbum() *ExploreAlbum {
	if e.cursor < 0 || e.cursor >= len(e.albums) {
		return nil
	}
	a := e.albums[e.cursor]
	return &a
}

// SetAlbums replaces the batch and resets selection to the top.
func (e *Explore) SetAlbums(albums []ExploreAlbum, source ExploreSource, counts map[ExploreSource]int, available []ExploreSource) {
	e.albums = albums
	e.source = source
	e.loading = false
	e.cursor = 0
	if counts != nil {
		e.counts = counts
	}
	e.available = available
	e.requestArt()
}

// TakePendingArt returns the album whose cover should be loaded next, along
// with the request sequence used to drop stale loads.
func (e *Explore) TakePendingArt() (*ExploreAlbum, int) {
	a := e.pendingArt
	seq := e.pendingArtSeq
	e.pendingArt = nil
	return a, seq
}

func (e *Explore) requestArt() {
	a := e.CurrentAlbum()
	if a == nil {
		e.artStr = ""
		e.artKey = ""
		e.artFailed = false
		e.artLoading = false
		return
	}
	e.artSeq++
	e.artKey = a.Key()
	e.artStr = ""
	e.artFailed = false
	e.artLoading = true
	e.pendingArt = a
	e.pendingArtSeq = e.artSeq
}

// HandleArtLoaded decodes the cover and, if still current, renders it.
func (e *Explore) HandleArtLoaded(msg ExploreArtLoadedMsg) tea.Cmd {
	if msg.Seq != e.artSeq || msg.Key != e.artKey {
		return nil
	}
	e.artLoading = false
	if msg.Err != nil || len(msg.ImageData) == 0 {
		e.artFailed = true
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(msg.ImageData))
	if err != nil {
		e.artFailed = true
		return nil
	}
	e.renderArt(img)
	if e.artStr == "" {
		e.artFailed = true
		return nil
	}
	return e.RenderImageCmd()
}

func (e *Explore) artPaneWidth() int {
	if !e.showArt() {
		return 0
	}
	return exploreArtWidth
}

func (e *Explore) showArt() bool {
	return e.termWidth >= exploreMinArtW && e.imageProtocol != 0
}

func (e *Explore) modalWidth() int {
	w := e.termWidth - 2
	if w > exploreMaxWidth {
		w = exploreMaxWidth
	}
	if w < 40 {
		w = 40
	}
	return w
}

func (e *Explore) contentWidth() int {
	w := e.modalWidth() - exploreBorder*2 - explorePadH*2
	if w < 20 {
		w = 20
	}
	return w
}

func (e *Explore) listWidth() int {
	w := e.contentWidth()
	if e.showArt() {
		w -= e.artPaneWidth() + 1
	}
	if w < 10 {
		w = 10
	}
	return w
}

func (e *Explore) renderArt(img image.Image) {
	maxW, maxH := e.artPaneWidth(), e.VisibleLimit()
	if maxH > exploreMaxHeight {
		maxH = exploreMaxHeight
	}
	if maxW < 4 || maxH < 2 {
		return
	}

	termimg.ClearResizeCache()

	bounds := img.Bounds()
	imgW := float64(bounds.Dx())
	imgH := float64(bounds.Dy())

	displayWidth := maxW
	displayHeight := int(float64(displayWidth) * (imgH / imgW) / e.cellRatio)
	if displayHeight > maxH {
		displayHeight = maxH
		displayWidth = int(float64(displayHeight) * (imgW / imgH) * e.cellRatio)
	}
	if displayWidth < 4 {
		displayWidth = 4
	}
	if displayHeight < 2 {
		displayHeight = 2
	}

	renderWidth, renderHeight := displayWidth, displayHeight
	if e.imageProtocol == termimg.Halfblocks {
		renderWidth = displayWidth * 2
		renderHeight = displayHeight * 2
	}

	var tiImg *termimg.Image
	if e.imageProtocol == termimg.Kitty && e.fontW > 0 && e.fontH > 0 {
		tiImg = termimg.New(img).
			SizePixels(renderWidth*e.fontW, renderHeight*e.fontH).
			Size(renderWidth, renderHeight).
			Scale(termimg.ScaleFit).
			Protocol(e.imageProtocol).
			UseUnicode(false)
	} else {
		tiImg = termimg.New(img).
			Size(renderWidth, renderHeight).
			Scale(termimg.ScaleFit).
			Protocol(e.imageProtocol).
			UseUnicode(false)
	}

	rendered, err := tiImg.Render()
	if err != nil {
		e.artFailed = true
		return
	}
	e.artStr = rendered
	e.artW = displayWidth
	e.artH = displayHeight
}

// ImageScreenPosition returns the 1-based terminal row/col for the overlay.
func (e *Explore) ImageScreenPosition() (int, int) {
	modalHeight := e.modalHeight()
	padTop := (e.termHeight - modalHeight) / 2
	if padTop < 0 {
		padTop = 0
	}
	padLeft := (e.termWidth - e.modalWidth()) / 2
	if padLeft < 0 {
		padLeft = 0
	}

	row := padTop + exploreBorder + explorePadV + exploreHeader + 1
	artStart := padLeft + exploreBorder + explorePadH + e.listWidth() + 1
	imgPad := (e.artPaneWidth() - e.artW) / 2
	if imgPad < 0 {
		imgPad = 0
	}
	col := artStart + imgPad + 1
	return row, col
}

func (e *Explore) modalHeight() int {
	rows := len(e.albums)
	if rows > e.VisibleLimit() {
		rows = e.VisibleLimit()
	}
	if rows < 3 {
		rows = 3
	}
	return rows + exploreChrome
}

// RenderImageCmd paints (or repaints) the cover overlay.
func (e *Explore) RenderImageCmd() tea.Cmd {
	if e.artStr == "" {
		return nil
	}
	row, col := e.ImageScreenPosition()
	imgStr := e.artStr
	if e.firstImage {
		e.firstImage = false
		return tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg {
			return ExploreRenderImageMsg{ImageStr: imgStr, Row: row, Col: col}
		})
	}
	return tea.Sequence(
		tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg {
			return tea.ClearScreen()
		}),
		tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg {
			return ExploreRenderImageMsg{ImageStr: imgStr, Row: row, Col: col}
		}),
	)
}

// Update handles keys inside the modal.
func (e *Explore) Update(msg tea.Msg) tea.Cmd {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	switch key.String() {
	case "esc", "q":
		return emitExplore(ExploreMsg{Action: ExploreClose})

	case "up", "k":
		e.move(-1)
	case "down", "j":
		e.move(1)
	case "pgup":
		e.move(-e.VisibleLimit())
	case "pgdown":
		e.move(e.VisibleLimit())
	case "home", "g":
		e.moveTo(0)
	case "end", "G":
		e.moveTo(len(e.albums) - 1)

	case "enter":
		if a := e.CurrentAlbum(); a != nil {
			return emitExplore(ExploreMsg{Action: ExplorePlay, Album: a, Source: e.source})
		}
	case "e":
		if a := e.CurrentAlbum(); a != nil {
			return emitExplore(ExploreMsg{Action: ExploreEnqueue, Album: a, Source: e.source})
		}
	case "E":
		if a := e.CurrentAlbum(); a != nil {
			return emitExplore(ExploreMsg{Action: ExploreEnqueueNext, Album: a, Source: e.source})
		}

	case "r":
		return emitExplore(ExploreMsg{Action: ExploreReroll, Source: e.source, Limit: e.VisibleLimit()})
	case "l":
		return emitExplore(ExploreMsg{Action: ExploreLucky, Source: e.source, Limit: e.VisibleLimit()})

	case "ctrl+t", "tab", "s":
		if next, ok := e.nextSource(); ok {
			return emitExplore(ExploreMsg{Action: ExploreReroll, Source: next, Limit: e.VisibleLimit()})
		}
	case "ctrl+l":
		if e.hasSource(ExploreLocal) {
			return emitExplore(ExploreMsg{Action: ExploreReroll, Source: ExploreLocal, Limit: e.VisibleLimit()})
		}
	case "ctrl+s":
		if e.hasSource(ExploreSubsonic) {
			return emitExplore(ExploreMsg{Action: ExploreReroll, Source: ExploreSubsonic, Limit: e.VisibleLimit()})
		}
	case "ctrl+d":
		if e.hasSource(ExploreTemp) {
			return emitExplore(ExploreMsg{Action: ExploreReroll, Source: ExploreTemp, Limit: e.VisibleLimit()})
		}
	}
	return nil
}

func emitExplore(m ExploreMsg) tea.Cmd {
	return func() tea.Msg { return m }
}

func (e *Explore) hasSource(s ExploreSource) bool {
	for _, a := range e.available {
		if a == s {
			return true
		}
	}
	return false
}

func (e *Explore) nextSource() (ExploreSource, bool) {
	if len(e.available) == 0 {
		return e.source, false
	}
	cur := -1
	for i, s := range e.available {
		if s == e.source {
			cur = i
			break
		}
	}
	next := e.available[(cur+1)%len(e.available)]
	return next, next != e.source
}

func (e *Explore) move(delta int) {
	e.moveTo(e.cursor + delta)
}

func (e *Explore) moveTo(idx int) {
	if len(e.albums) == 0 {
		return
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(e.albums) {
		idx = len(e.albums) - 1
	}
	if idx == e.cursor {
		return
	}
	e.cursor = idx
	e.requestArt()
}

func (e *Explore) renderSourceBar() string {
	order := []ExploreSource{ExploreAll, ExploreLocal, ExploreSubsonic, ExploreTemp}
	var parts []string
	for _, s := range order {
		label := s.Label()
		if c, ok := e.counts[s]; ok && s != ExploreAll {
			label = fmt.Sprintf("%s %d", label, c)
		}
		if s == e.source {
			parts = append(parts, e.styles.AccentStyle.Bold(true).Render("["+label+"]"))
		} else if e.hasSource(s) || s == ExploreAll {
			parts = append(parts, e.styles.MutedStyle.Render(label))
		}
	}
	return strings.Join(parts, "  ")
}

func (e *Explore) View() string {
	contentWidth := e.contentWidth()
	listWidth := e.listWidth()

	var b strings.Builder

	count := e.counts[ExploreAll]
	title := e.styles.AccentStyle.Bold(true).Render("Explore your library")
	if count > 0 {
		title += e.styles.MutedStyle.Render(fmt.Sprintf("  %d albums", count))
	}
	b.WriteString(ansi.Truncate(title, contentWidth, "…"))
	b.WriteString("\n")
	b.WriteString(ansi.Truncate(e.renderSourceBar(), contentWidth, "…"))
	b.WriteString("\n")

	rows := e.VisibleLimit()
	if len(e.albums) == 0 {
		empty := "  No albums found"
		if e.loading {
			empty = "  Loading…"
		}
		b.WriteString(e.styles.MutedStyle.Render(empty))
		b.WriteString("\n")
		for i := 1; i < rows; i++ {
			b.WriteString("\n")
		}
	} else {
		for i := 0; i < rows; i++ {
			var line string
			if i < len(e.albums) {
				line = e.renderAlbumLine(i, listWidth)
			} else {
				line = strings.Repeat(" ", listWidth)
			}
			if e.showArt() {
				line = padRight(line, listWidth) + " " + e.renderArtCell(i)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(ansi.Truncate(e.helpText(), contentWidth, "…"))

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(e.styles.AccentStyle.GetForeground()).
		Padding(explorePadV, explorePadH).
		Width(e.modalWidth())

	rendered := modalStyle.Render(b.String())

	visWidth := lipgloss.Width(rendered)
	padLeft := max(0, (e.termWidth-visWidth)/2)
	padTop := max(0, (e.termHeight-lipgloss.Height(rendered))/2)
	return centerBlock(rendered, padTop, padLeft)
}

// renderArtCell occupies the art pane so the layout is fixed-width and the
// raw image overlay has blank cells to paint over.
func (e *Explore) renderArtCell(row int) string {
	w := e.artPaneWidth()
	if w <= 0 {
		return ""
	}
	if e.artStr != "" {
		return strings.Repeat(" ", w)
	}
	if row == 0 {
		text := "♪"
		if e.artLoading {
			text = "loading…"
		} else if e.artFailed {
			text = "no art"
		}
		return padRight(centerStyled(e.styles.MutedStyle.Render(text), w), w)
	}
	return strings.Repeat(" ", w)
}

func (e *Explore) renderAlbumLine(idx, width int) string {
	a := e.albums[idx]
	year := ""
	if a.Year > 0 {
		year = fmt.Sprintf(" (%d)", a.Year)
	}
	yearW := ansi.StringWidth(year)
	name := a.Name
	if a.Artist != "" {
		name = a.Name + " — " + a.Artist
	}
	name = ansi.Truncate(name, max(width-2-yearW, 3), "…")

	var line string
	if idx == e.cursor {
		line = e.styles.CursorStyle.Render("> "+name) + e.styles.MutedStyle.Render(year)
	} else {
		line = e.styles.ForegroundStyle.Render(" "+name) + e.styles.MutedStyle.Render(year)
	}
	return line
}

func (e *Explore) helpText() string {
	key := func(s string) string { return e.styles.AccentStyle.Render(s) }
	desc := func(s string) string { return e.styles.MutedStyle.Render(s) }
	return key("↑/↓") + desc(" nav  ") +
		key("enter") + desc(" play  ") +
		key("e") + desc(" enqueue  ") +
		key("l") + desc(" lucky  ") +
		key("r") + desc(" reroll  ") +
		key("^t") + desc(" source  ") +
		key("esc") + desc(" close")
}

func padRight(s string, width int) string {
	w := ansi.StringWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func centerBlock(s string, padTop, padLeft int) string {
	if padTop <= 0 && padLeft <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	pad := strings.Repeat(" ", padLeft)
	var b strings.Builder
	for i := 0; i < padTop; i++ {
		b.WriteString("\n")
	}
	for i, line := range lines {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(pad)
		b.WriteString(line)
	}
	return b.String()
}
