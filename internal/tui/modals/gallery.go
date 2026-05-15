package modals

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	termimg "github.com/blacktop/go-termimg"
	"github.com/pdfrg/must/internal/config"
)

const (
	galleryModalBorder = 1
	galleryModalPadH   = 2
	galleryModalPadV   = 1
	galleryLinesAbove  = 3
	galleryLinesBelow  = 4
	maxTmuxPixelArea   = 160_000
)

type GalleryMsg struct {
	Closed bool
}

type GalleryImageLoadedMsg struct {
	Index     int
	ImageData []byte
	Err       error
}

type GalleryRenderImageMsg struct {
	ImageStr string
	Row      int
	Col      int
}

type Gallery struct {
	styles        *config.ThemeStyles
	urls          []string
	source        string
	currentIdx    int
	termWidth     int
	termHeight    int
	cellRatio     float64
	fontW         int
	fontH         int
	imageProtocol termimg.Protocol

	images  []image.Image
	loading map[int]bool
	loaded  map[int]bool

	renderedStr  string
	renderedW    int
	renderedH    int
	renderFailed bool

	firstImageDisplayed bool
}

func NewGallery(styles *config.ThemeStyles, urls []string, source string, termWidth, termHeight int, cellRatio float64, fontW, fontH int) *Gallery {
	if cellRatio < 1.0 {
		cellRatio = 2.0
	}
	return &Gallery{
		styles:     styles,
		urls:       urls,
		source:     source,
		currentIdx: 0,
		termWidth:  termWidth,
		termHeight: termHeight,
		cellRatio:  cellRatio,
		fontW:      fontW,
		fontH:      fontH,
		images:     make([]image.Image, len(urls)),
		loading:    make(map[int]bool),
		loaded:     make(map[int]bool),
	}
}

func (g *Gallery) SetProtocol(p termimg.Protocol) {
	g.imageProtocol = p
}

func (g *Gallery) SetSize(w, h int) {
	g.termWidth = w
	g.termHeight = h
}

func (g *Gallery) PrefetchImages() tea.Cmd {
	indices := g.prefetchIndices()
	var cmds []tea.Cmd
	for _, idx := range indices {
		if cmd := g.loadImageCmd(idx); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (g *Gallery) prefetchIndices() []int {
	n := len(g.urls)
	if n == 0 {
		return nil
	}
	indices := []int{g.currentIdx}
	if g.currentIdx > 0 {
		indices = append(indices, g.currentIdx-1)
	} else {
		indices = append(indices, n-1)
	}
	if g.currentIdx < n-1 {
		indices = append(indices, g.currentIdx+1)
	} else {
		indices = append(indices, 0)
	}
	return indices
}

func (g *Gallery) loadImageCmd(idx int) tea.Cmd {
	if g.loaded[idx] || g.loading[idx] || idx < 0 || idx >= len(g.urls) {
		return nil
	}
	g.loading[idx] = true
	url := g.urls[idx]
	return func() tea.Msg {
		client := &http.Client{Timeout: 15 * time.Second}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return GalleryImageLoadedMsg{Index: idx, Err: err}
		}
		req.Header.Set("User-Agent", "must/1.0")
		resp, err := client.Do(req)
		if err != nil {
			return GalleryImageLoadedMsg{Index: idx, Err: err}
		}
		defer func() { _ = resp.Body.Close() }()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return GalleryImageLoadedMsg{Index: idx, Err: err}
		}
		return GalleryImageLoadedMsg{Index: idx, ImageData: data}
	}
}

func (g *Gallery) HandleImageLoaded(msg GalleryImageLoadedMsg) tea.Cmd {
	g.loading[msg.Index] = false
	if msg.Err != nil {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(msg.ImageData))
	if err != nil {
		return nil
	}
	g.images[msg.Index] = img
	g.loaded[msg.Index] = true

	if msg.Index == g.currentIdx {
		g.renderCurrentImage()
		return g.RenderImageCmd()
	}
	return nil
}

func (g *Gallery) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc", "q":
			return func() tea.Msg { return GalleryMsg{Closed: true} }
		case "left", "h", "p":
			return g.navigate(-1)
		case "right", "l", "n":
			return g.navigate(1)
		case "up", "k":
			return g.navigate(-1)
		case "down", "j":
			return g.navigate(1)
		}
	}
	return nil
}

func (g *Gallery) navigate(delta int) tea.Cmd {
	n := len(g.urls)
	if n == 0 {
		return nil
	}
	g.currentIdx = (g.currentIdx + delta + n) % n
	g.renderedStr = ""
	g.renderFailed = false
	g.renderCurrentImage()

	var cmds []tea.Cmd
	if g.renderedStr != "" {
		cmds = append(cmds, g.RenderImageCmd())
	}
	if prefetch := g.PrefetchImages(); prefetch != nil {
		cmds = append(cmds, prefetch)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (g *Gallery) renderCurrentImage() {
	img := g.images[g.currentIdx]
	if img == nil {
		return
	}

	termimg.ClearResizeCache()
	maxW, maxH := g.maxImageSize()
	if maxW < 4 || maxH < 2 {
		return
	}

	imgBounds := img.Bounds()
	imgW := float64(imgBounds.Dx())
	imgH := float64(imgBounds.Dy())

	displayWidth := maxW
	displayHeight := int(float64(displayWidth) * (imgH / imgW) / g.cellRatio)

	if displayHeight > maxH {
		displayHeight = maxH
		displayWidth = int(float64(displayHeight) * (imgW / imgH) * g.cellRatio)
	}
	if displayWidth < 4 {
		displayWidth = 4
	}
	if displayHeight < 2 {
		displayHeight = 2
	}

	nativeW := imgBounds.Dx()
	nativeH := imgBounds.Dy()
	fw, fh := g.fontW, g.fontH
	if fw <= 0 || fh <= 0 {
		features := termimg.QueryTerminalFeatures()
		fw = features.FontWidth
		fh = features.FontHeight
	}
	if fw > 0 && fh > 0 {
		maxNativeCols := nativeW / fw
		maxNativeRows := nativeH / fh
		if displayWidth > maxNativeCols || displayHeight > maxNativeRows {
			if displayWidth > maxNativeCols {
				scale := float64(maxNativeCols) / float64(displayWidth)
				displayWidth = maxNativeCols
				displayHeight = int(float64(displayHeight) * scale)
			}
			if displayHeight > maxNativeRows {
				scale := float64(maxNativeRows) / float64(displayHeight)
				displayHeight = maxNativeRows
				displayWidth = int(float64(displayWidth) * scale)
			}
		}
	}

	if g.imageProtocol == termimg.Kitty && g.fontW > 0 && g.fontH > 0 {
		pixelArea := displayWidth * g.fontW * displayHeight * g.fontH
		if pixelArea > maxTmuxPixelArea {
			scale := math.Sqrt(float64(maxTmuxPixelArea) / float64(pixelArea))
			displayWidth = max(4, int(float64(displayWidth)*scale))
			displayHeight = max(2, int(float64(displayHeight)*scale))
		}
	}

	renderWidth := displayWidth
	renderHeight := displayHeight
	if g.imageProtocol == termimg.Halfblocks {
		renderWidth = displayWidth * 2
		renderHeight = displayHeight * 2
	}

	var tiImg *termimg.Image
	if g.imageProtocol == termimg.Kitty && g.fontW > 0 && g.fontH > 0 {
		tiImg = termimg.New(img).
			SizePixels(renderWidth*g.fontW, renderHeight*g.fontH).
			Size(renderWidth, renderHeight).
			Scale(termimg.ScaleFit).
			Protocol(g.imageProtocol).
			UseUnicode(false)
	} else {
		tiImg = termimg.New(img).
			Size(renderWidth, renderHeight).
			Scale(termimg.ScaleFit).
			Protocol(g.imageProtocol).
			UseUnicode(false)
	}

	rendered, err := tiImg.Render()
	if err != nil {
		g.renderFailed = true
		return
	}
	g.renderedStr = rendered
	g.renderedW = displayWidth
	g.renderedH = displayHeight
}

func (g *Gallery) maxImageSize() (int, int) {
	modalWidth := g.modalWidth()
	contentWidth := modalWidth - galleryModalBorder*2 - galleryModalPadH
	w := contentWidth - 4
	h := g.termHeight - galleryModalBorder*2 - galleryModalPadV*2 - galleryLinesAbove - galleryLinesBelow
	if w < 10 {
		w = 10
	}
	if h < 4 {
		h = 4
	}
	return w, h
}

func (g *Gallery) modalWidth() int {
	return g.termWidth
}

func (g *Gallery) ImageScreenPosition() (int, int) {
	modalWidth := g.modalWidth()
	contentWidth := modalWidth - galleryModalBorder*2 - galleryModalPadH

	modalHeight := g.modalLineCount() + galleryModalBorder*2 + galleryModalPadV*2
	padTop := (g.termHeight - modalHeight) / 2
	if padTop < 0 {
		padTop = 0
	}
	padLeft := (g.termWidth - modalWidth) / 2
	if padLeft < 0 {
		padLeft = 0
	}

	row := padTop + galleryModalBorder + galleryModalPadV + galleryLinesAbove + 1

	imgPadLeft := (contentWidth - g.renderedW) / 2
	if imgPadLeft < 0 {
		imgPadLeft = 0
	}
	col := padLeft + galleryModalBorder + galleryModalPadH + imgPadLeft + 1

	return row, col
}

func (g *Gallery) modalLineCount() int {
	return galleryLinesAbove + g.imageAreaHeight() + galleryLinesBelow
}

func (g *Gallery) imageAreaHeight() int {
	_, maxH := g.maxImageSize()
	h := maxH
	if h < 3 {
		h = 3
	}
	return h
}

func (g *Gallery) RenderImageCmd() tea.Cmd {
	if g.renderedStr == "" {
		return nil
	}
	isFirst := !g.firstImageDisplayed
	g.firstImageDisplayed = true
	row, col := g.ImageScreenPosition()
	imgStr := g.renderedStr

	if isFirst {
		return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
			return GalleryRenderImageMsg{ImageStr: imgStr, Row: row, Col: col}
		})
	}
	return tea.Sequence(
		tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
			return tea.ClearScreen()
		}),
		tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
			return GalleryRenderImageMsg{ImageStr: imgStr, Row: row, Col: col}
		}),
	)
}

func (g Gallery) View() string {
	modalWidth := g.modalWidth()
	contentWidth := modalWidth - galleryModalBorder*2 - galleryModalPadH
	imageAreaH := g.imageAreaHeight()

	accentStyle := g.styles.AccentStyle
	mutedStyle := g.styles.MutedStyle

	var b strings.Builder

	indicator := fmt.Sprintf("%d/%d", g.currentIdx+1, len(g.urls))
	titleLine := accentStyle.Render("ARTIST IMAGES") + " " + accentStyle.Render(indicator)
	b.WriteString(centerStyled(titleLine, contentWidth))
	b.WriteString("\n\n")

	if g.loading[g.currentIdx] {
		b.WriteString(centerStyled(mutedStyle.Render("Loading image..."), contentWidth))
	} else if g.renderFailed {
		b.WriteString(centerStyled(mutedStyle.Render("Failed to render image"), contentWidth))
	} else if g.renderedStr == "" {
		b.WriteString(centerStyled(mutedStyle.Render("Loading image..."), contentWidth))
	}

	for i := 1; i < imageAreaH; i++ {
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if g.source != "" {
		sourceText := mutedStyle.Render("Source: " + g.source)
		b.WriteString(centerStyled(sourceText, contentWidth))
	}
	b.WriteString("\n")

	helpText := accentStyle.Render("←/→") + mutedStyle.Render(" navigate ") + accentStyle.Render("esc") + mutedStyle.Render(" close")
	b.WriteString(centerStyled(helpText, contentWidth))

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(g.styles.AccentStyle.GetForeground()).
		Padding(1, 2).
		Width(modalWidth)

	return modalStyle.Render(b.String())
}

func (g *Gallery) HasImages() bool {
	return len(g.urls) > 0
}

func (g *Gallery) ImageCount() int {
	return len(g.urls)
}
