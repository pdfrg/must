package tui

import (
	"math/rand"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pdfrg/must/internal/models"
)

// titleRevealTickMsg advances the typewriter reveal. seq drops stale ticks
// from a previous track after a skip.
type titleRevealTickMsg struct {
	seq int
}

func tickTitleRevealCmd(delay time.Duration, seq int) tea.Cmd {
	return tea.Tick(delay, func(t time.Time) tea.Msg {
		return titleRevealTickMsg{seq: seq}
	})
}

// titleAnimMode returns "off", "machine" or "human" (validated by config).
func (m *Model) titleAnimMode() string {
	if m.cfg == nil {
		return "off"
	}
	return m.cfg.TitleAnimation
}

func (m *Model) titleAnimAll() bool {
	return m.cfg != nil && m.cfg.TitleAnimationScope == "all"
}

// revealRowLens returns rune counts for the rows covered by the animation.
func revealRowLens(track *models.Track, all bool) []int {
	if track == nil {
		return nil
	}
	title := track.Title
	artist := track.Artist
	if track.ServerBadge != "" {
		artist = "[" + track.ServerBadge + "] " + artist
	}
	album := track.Album
	if track.Year > 0 {
		album = album + " (" + itoa(track.Year) + ")"
	}
	lens := []int{runeLen(title)}
	if all {
		lens = append(lens, runeLen(artist), runeLen(album))
	}
	return lens
}

func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func revealTotal(lens []int) int {
	total := 0
	for _, l := range lens {
		total += l
	}
	return total
}

// startTitleReveal resets reveal state for the current track and returns the
// first tick cmd, or nil when the animation is off / nothing to reveal.
func (m *Model) startTitleReveal() tea.Cmd {
	m.titleRevealSeq++
	if m.titleAnimMode() == "off" {
		m.titleRevealActive = false
		m.titleRevealPos = 0
		m.titleRevealLens = nil
		return nil
	}
	if m.currentIndex < 0 || m.currentIndex >= len(m.playlist) {
		m.titleRevealActive = false
		m.titleRevealPos = 0
		m.titleRevealLens = nil
		return nil
	}
	lens := revealRowLens(&m.playlist[m.currentIndex], m.titleAnimAll())
	if revealTotal(lens) == 0 {
		m.titleRevealActive = false
		m.titleRevealPos = 0
		m.titleRevealLens = nil
		return nil
	}
	m.titleRevealLens = lens
	m.titleRevealPos = 0
	m.titleRevealActive = true
	return tickTitleRevealCmd(revealDelay(m.titleAnimMode(), 0, lens, 0), m.titleRevealSeq)
}

// snapTitleReveal finishes the animation instantly (used on pause so the
// display is never stuck on a partial title).
func (m *Model) snapTitleReveal() {
	if !m.titleRevealActive {
		return
	}
	m.titleRevealPos = revealTotal(m.titleRevealLens)
	m.titleRevealActive = false
	m.titleRevealSeq++
}

// handleTitleRevealTick advances the reveal and re-arms the ticker with a
// mode-appropriate delay until complete.
func (m Model) handleTitleRevealTick(msg titleRevealTickMsg) (tea.Model, tea.Cmd) {
	if msg.seq != m.titleRevealSeq || !m.titleRevealActive {
		return m, nil
	}
	// Track changed out from under us: stale tick.
	if m.currentIndex < 0 || m.currentIndex >= len(m.playlist) {
		m.titleRevealActive = false
		return m, nil
	}
	total := revealTotal(m.titleRevealLens)
	if m.titleRevealPos >= total {
		m.titleRevealActive = false
		return m, nil
	}
	mode := m.titleAnimMode()
	chunk := 1
	if mode == "human" {
		if total >= 24 {
			chunk = 1 + rand.Intn(4) // 1-4 chars per burst
		}
		// Short strings reveal one char per tick: with 2-3 ticks total a
		// burst would finish before a pause ever lands, making the
		// animation barely visible.
	}
	// Clamp the burst at row boundaries: a 1-4 char jump would otherwise
	// skip clean over the boundary and the inter-field pause would never
	// fire. Landing exactly on it shows the cursor alone at the start of
	// the next row for the full beat.
	if b := nextRowBoundary(m.titleRevealPos, m.titleRevealLens, total); b >= 0 && m.titleRevealPos+chunk > b {
		chunk = b - m.titleRevealPos
	}
	m.titleRevealPos += chunk
	if m.titleRevealPos >= total {
		m.titleRevealPos = total
		m.titleRevealActive = false
		return m, nil
	}
	return m, tickTitleRevealCmd(revealDelay(mode, m.titleRevealPos, m.titleRevealLens, revealBoundaryRune(&m.playlist[m.currentIndex], m.titleAnimAll(), m.titleRevealPos)), m.titleRevealSeq)
}

// revealDelay returns the next tick delay. Machine is a fixed cadence;
// human alternates fast intra-burst ticks with longer pauses (longer on
// spaces/punctuation), scaled down for very long metadata so the whole
// reveal still completes within ~3s.
func revealDelay(mode string, pos int, lens []int, boundaryRune rune) time.Duration {
	total := revealTotal(lens)
	scale := 1.0
	if total > 100 {
		scale = 100.0 / float64(total)
	}
	if mode != "human" {
		return time.Duration(float64(28*time.Millisecond) * scale)
	}
	// Inter-field beat: pausing as the typist tabs to the next row keeps
	// short artist/album names in "all" scope from flashing by.
	if isRowBoundary(pos, lens, total) {
		return time.Duration(float64(time.Duration(350+rand.Intn(301)) * time.Millisecond))
	}
	// Stretch short strings so one-word titles stay visible (~0.8s floor).
	if total < 40 {
		scale *= 1 + float64(40-total)/40*2
	}
	// 70% fast burst tick, 30% pause.
	if rand.Intn(100) < 70 {
		return time.Duration(float64(time.Duration(15+rand.Intn(21))*time.Millisecond) * scale)
	}
	d := 80 + rand.Intn(171) // 80-250ms
	// Longer beat after word boundaries.
	switch boundaryRune {
	case ' ', ',', '.', '-', '(', '/', '&', ':':
		d += 60 + rand.Intn(120)
	}
	return time.Duration(float64(time.Duration(d)*time.Millisecond) * scale)
}

// nextRowBoundary returns the first row boundary strictly after pos,
// or -1 if none (single row, or pos already past the last boundary).
func nextRowBoundary(pos int, lens []int, total int) int {
	if len(lens) < 2 {
		return -1
	}
	acc := 0
	for _, l := range lens[:len(lens)-1] {
		acc += l
		if acc > pos && acc < total {
			return acc
		}
	}
	return -1
}

// isRowBoundary reports whether pos sits exactly at a row boundary
// (title|artist|album) in "all" scope, excluding the final position.
func isRowBoundary(pos int, lens []int, total int) bool {
	if len(lens) < 2 || pos <= 0 || pos >= total {
		return false
	}
	acc := 0
	for _, l := range lens[:len(lens)-1] {
		acc += l
		if pos == acc {
			return true
		}
	}
	return false
}

// across the concatenated display rows, for word-boundary pause detection.
func revealBoundaryRune(track *models.Track, all bool, pos int) rune {
	if track == nil || pos <= 0 {
		return 'x'
	}
	texts := []string{track.Title}
	if all {
		artist := track.Artist
		if track.ServerBadge != "" {
			artist = "[" + track.ServerBadge + "] " + artist
		}
		album := track.Album
		if track.Year > 0 {
			album = album + " (" + itoa(track.Year) + ")"
		}
		texts = append(texts, artist, album)
	}
	i := pos - 1 // the rune just revealed
	for _, t := range texts {
		n := 0
		for _, r := range t {
			if n == i {
				return r
			}
			n++
		}
		i -= n
	}
	return 'x'
}
