package tui

// Rect describes a terminal-cell rectangle using zero-based coordinates.
// Every renderer uses the same rectangles so text, images, and mouse targets
// cannot drift apart as the terminal is resized.
type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

func (r Rect) Empty() bool { return r.Width <= 0 || r.Height <= 0 }
func (r Rect) Right() int  { return r.X + r.Width }
func (r Rect) Bottom() int { return r.Y + r.Height }

type LayoutPlan struct {
	Width  int
	Height int
	Mode   string

	Header     Rect
	NowPlaying Rect
	Artwork    Rect
	Bottom     Rect
	Footer     Rect

	ArtworkStacked bool
	MiniFooter     bool
}

type layoutPreferences struct {
	Requested      string
	ShowHeader     bool
	ShowFooter     bool
	ShowArtwork    bool
	ShowBottom     bool
	CompactBottom  bool
	NowPlayingRows int
	CellRatio      float64
}

const (
	minimumContentWidth = 20
	minimumBottomRows   = 3
	maximumPlayerWidth  = 140
)

func planLayout(width, height int, prefs layoutPreferences) LayoutPlan {
	width = max(width, 0)
	height = max(height, 0)
	plan := LayoutPlan{Width: width, Height: height}
	if width == 0 || height == 0 {
		plan.Mode = "compact"
		return plan
	}

	requested := prefs.Requested
	if requested == "" {
		requested = "auto"
	}
	plan.Mode = requested
	if requested == "auto" {
		plan.Mode = automaticLayoutMode(width, height)
	}
	tallStack := plan.Mode == "narrow" || (requested == "large" && height >= 40 && width < 145)

	headerRows := 0
	if prefs.ShowHeader && height >= 4 {
		headerRows = 1
		plan.Header = Rect{Width: width, Height: 1}
	}

	footerRows := 0
	if prefs.ShowFooter && height >= 4 {
		plan.MiniFooter = width < 100 || height < 36 || plan.Mode == "compact" || tallStack
		footerRows = 2
		if plan.MiniFooter {
			footerRows = 1
		}
		plan.Footer = Rect{Y: height - footerRows, Width: width, Height: footerRows}
	}

	topGap := 1
	if height < 12 {
		topGap = 0
	}
	mainY := headerRows + topGap
	footerY := height - footerRows
	if mainY >= footerY {
		mainY = min(headerRows, footerY)
	}

	margin := 2
	if width < 32 {
		margin = 1
	}
	contentWidth := max(width-margin*2, 1)
	stageWidth := min(contentWidth, maximumPlayerWidth)
	// Horizontal layouts share the playlist's left edge. Centering only the
	// player above a full-width table makes the two sections look unrelated.
	// The stacked/tall composition remains centered as a single column.
	stageX := margin
	if tallStack {
		stageX = max((width-stageWidth)/2, 0)
	}
	baseRows := min(max(prefs.NowPlayingRows, 1), max(footerY-mainY, 1))

	showArtwork := prefs.ShowArtwork && plan.Mode != "compact" && width >= 48 && footerY-mainY >= 8
	stackArtwork := showArtwork && tallStack
	plan.ArtworkStacked = stackArtwork

	mainBottom := mainY + baseRows
	if showArtwork {
		ratio := prefs.CellRatio
		if ratio <= 0 {
			ratio = 2
		}
		artHeight := desiredArtworkHeight(width, height, stackArtwork)
		if stackArtwork {
			maxHeight := max(footerY-mainY-baseRows-2, 0)
			artHeight = min(artHeight, maxHeight)
			artWidth := int(float64(artHeight) * ratio)
			if artWidth > stageWidth {
				artWidth = stageWidth
				artHeight = int(float64(artWidth) / ratio)
			}
			if artHeight >= 4 && artWidth >= 8 {
				artX := stageX + max((stageWidth-artWidth)/2, 0)
				artY := mainY + 4
				plan.Artwork = Rect{X: artX, Y: artY, Width: artWidth, Height: artHeight}
				plan.NowPlaying = Rect{X: stageX, Y: mainY, Width: stageWidth, Height: baseRows + artHeight + 1}
				mainBottom = plan.NowPlaying.Bottom()
			} else {
				showArtwork = false
			}
		} else {
			availableRows := max(footerY-mainY, 0)
			artHeight = min(artHeight, availableRows)
			artWidth := int(float64(artHeight) * ratio)
			maxArtWidth := max(stageWidth-minimumContentWidth-2, 0)
			if artWidth > maxArtWidth {
				artWidth = maxArtWidth
				artHeight = int(float64(artWidth) / ratio)
			}
			if artHeight >= 4 && artWidth >= 8 {
				artX := stageX + stageWidth - artWidth
				plan.Artwork = Rect{X: artX, Y: mainY, Width: artWidth, Height: artHeight}
				plan.NowPlaying = Rect{X: stageX, Y: mainY, Width: max(artX-stageX-2, 1), Height: baseRows}
				mainBottom = max(plan.NowPlaying.Bottom(), plan.Artwork.Bottom())
			} else {
				showArtwork = false
			}
		}
	}

	if !showArtwork {
		plan.Artwork = Rect{}
		plan.ArtworkStacked = false
		plan.NowPlaying = Rect{X: stageX, Y: mainY, Width: stageWidth, Height: baseRows}
		mainBottom = plan.NowPlaying.Bottom()
	}

	allowBottom := prefs.ShowBottom && (requested == "auto" || plan.Mode == "large")
	if allowBottom {
		bottomY := mainBottom + 1
		bottomHeight := footerY - bottomY
		if footerRows > 0 && bottomHeight > minimumBottomRows {
			bottomHeight--
		}
		if prefs.CompactBottom {
			bottomHeight = min(bottomHeight, 6)
		}
		if bottomHeight >= minimumBottomRows {
			plan.Bottom = Rect{Y: bottomY, Width: width, Height: bottomHeight}
			if prefs.CompactBottom {
				plan.Bottom.X = stageX
				plan.Bottom.Width = stageWidth
			}
		}
	}

	// Keep compact components, such as the embedded visualizer, attached to the
	// player instead of pinning them to the bottom of a very tall window.
	if prefs.CompactBottom && !plan.Bottom.Empty() {
		slack := footerY - plan.Bottom.Bottom()
		if slack >= 12 {
			shift := slack / 2
			plan.NowPlaying.Y += shift
			if !plan.Artwork.Empty() {
				plan.Artwork.Y += shift
			}
			plan.Bottom.Y += shift
		}
	}

	return plan
}

func automaticLayoutMode(width, height int) string {
	switch {
	case width < 48 || height < 15:
		return "compact"
	case height >= 40 && width >= 72 && width < 145:
		return "narrow"
	case height >= 28:
		return "large"
	default:
		return "medium"
	}
}

func desiredArtworkHeight(width, height int, stacked bool) int {
	if stacked {
		return min(max(height/3, 10), 18)
	}
	switch {
	case height <= 24 || width < 90:
		return 8
	case height < 31:
		return 12
	default:
		return 16
	}
}
