package tui

import (
	"path/filepath"
	"time"

	"github.com/pdfrg/must/internal/models"
	"github.com/pdfrg/must/internal/playlist"
)

// The UI playlist is the single source of truth for what is queued and in what
// order it should play. mpv holds a mirror of it (in playback order, i.e. after
// the shuffle permutation) and is reconciled whenever the queue is mutated, so
// the index the UI shows can never drift from what mpv is actually playing.
//
// Reconciliation is deliberately move/insert/remove only: existing entries are
// relocated with playlist-move, which never interrupts the entry that is
// currently playing.

// mpvPathKey normalises a path so a UI path and the string mpv echoes back in
// its playlist can be compared. mpv hands back exactly what it was given, so
// relative paths must be resolved before they compare equal.
func mpvPathKey(path string) string {
	if playlist.IsURL(path) {
		return path
	}
	if abs, err := filepath.Abs(path); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(path)
}

type syncKind int

const (
	syncMove syncKind = iota
	syncInsert
	syncRemove
)

// syncOp is one step of a playlist reconciliation.
type syncOp struct {
	kind syncKind
	from int // syncMove/syncRemove: index to take from
	to   int // syncMove/syncInsert: index to place at
	path string
}

// planPlaylistSync turns mpv's playlist `current` (normalised keys, in playback
// order) into `want` with as few operations as possible. It is pure so it can
// be tested without an mpv process.
//
// The prefix that already matches is left alone; each remaining position is
// filled either by moving the matching existing entry up to it (mpv's
// playlist-move lands a from>to move exactly at `to`) or by loading a missing
// entry. Whatever is left over at the end is removed.
func planPlaylistSync(current, want []string) []syncOp {
	var ops []syncOp
	got := append([]string(nil), current...)

	for i, w := range want {
		if i < len(got) && got[i] == w {
			continue
		}
		j := -1
		for k := i; k < len(got); k++ {
			if got[k] == w {
				j = k
				break
			}
		}
		if j >= 0 {
			ops = append(ops, syncOp{kind: syncMove, from: j, to: i})
			e := got[j]
			got = append(got[:j], got[j+1:]...)
			got = append(got[:i], append([]string{e}, got[i:]...)...)
			continue
		}
		ops = append(ops, syncOp{kind: syncInsert, to: i, path: w})
		got = append(got[:i], append([]string{w}, got[i:]...)...)
	}

	for k := len(got) - 1; k >= len(want); k-- {
		ops = append(ops, syncOp{kind: syncRemove, from: k})
	}
	return ops
}

// mpvPlaylistKeys returns mpv's playlist as normalised keys, in playback order.
func (m *Model) mpvPlaylistKeys() ([]string, error) {
	entries, err := m.mpvBackend.GetPlaylist()
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(entries))
	for i, e := range entries {
		keys[i] = mpvPathKey(e.Filename)
	}
	return keys, nil
}

// syncMPVPlaylist makes mpv's playlist identical to the UI playlist: same
// entries, same order. It is safe to call after any mutation and is a no-op
// while nothing is playing.
func (m *Model) syncMPVPlaylist() {
	if m.mpvBackend == nil || !m.playing || !m.mpvBackend.IsRunning() {
		return
	}

	// A shuffle permutation that no longer covers the playlist would make the
	// playback order meaningless; fall back to the displayed order rather than
	// mirroring garbage.
	if m.shuffle && len(m.shuffleOrder) != len(m.playlist) {
		m.shuffleOrder = nil
	}

	paths := m.buildMPVPlaylistPaths()
	if len(paths) == 0 {
		return
	}
	current, err := m.mpvPlaylistKeys()
	if err != nil {
		logf("Playlist sync: cannot read mpv playlist: %v", err)
		return
	}
	// mpv briefly reports an empty playlist while it switches entries. An empty
	// playlist cannot be mirrored into, so wait for it to settle rather than
	// reloading the whole queue (which would restart playback).
	for attempt := 0; len(current) == 0 && attempt < 3; attempt++ {
		time.Sleep(60 * time.Millisecond)
		current, err = m.mpvPlaylistKeys()
		if err != nil {
			logf("Playlist sync: cannot read mpv playlist: %v", err)
			return
		}
	}
	if len(current) == 0 {
		logf("Playlist sync: mpv playlist still empty, skipping")
		return
	}
	want := make([]string, len(paths))
	for i, p := range paths {
		want[i] = mpvPathKey(p)
	}

	for _, op := range planPlaylistSync(current, want) {
		var err error
		switch op.kind {
		case syncMove:
			err = m.mpvBackend.PlaylistMove(op.from, op.to)
		case syncInsert:
			err = m.mpvBackend.InsertPlaylistEntry(paths[op.to], op.to)
		case syncRemove:
			err = m.mpvBackend.RemoveFromPlaylist(op.from)
		}
		if err != nil {
			logf("Playlist sync: op %+v failed: %v", op, err)
			return
		}
	}
}

// ensureMPVCurrent makes mpv play the entry the UI marks as current, without
// touching it when mpv already plays that entry (setting playlist-pos to the
// entry that is already playing is harmless, but skipping it keeps the audio
// untouched on ordinary edits).
func (m *Model) ensureMPVCurrent() {
	if m.mpvBackend == nil || !m.playing || !m.mpvBackend.IsRunning() {
		return
	}
	if m.currentIndex < 0 || m.currentIndex >= len(m.playlist) {
		return
	}
	mpvIdx := m.playlistIndexToMPVIndex(m.currentIndex)
	if mpvIdx < 0 {
		return
	}
	pos, err := m.mpvBackend.GetPlaylistPosition()
	if err != nil || pos == mpvIdx {
		return
	}
	if err := m.mpvBackend.PlaylistPlayIndex(mpvIdx); err != nil {
		logf("Playlist sync: could not select entry %d: %v", mpvIdx, err)
	}
}

// commitPlaylistChange reconciles mpv with the UI playlist after a mutation.
// Every queue mutation must end with this so the two can never diverge.
func (m *Model) commitPlaylistChange() {
	m.syncMPVPlaylist()
	m.ensureMPVCurrent()
}

// playbackPosOf returns the playback position of a display index, i.e. its
// slot in the order mpv plays (the shuffle permutation when shuffling).
func (m *Model) playbackPosOf(displayIdx int) int {
	if m.shuffle && len(m.shuffleOrder) == len(m.playlist) {
		for i, idx := range m.shuffleOrder {
			if idx == displayIdx {
				return i
			}
		}
		return -1
	}
	if displayIdx < 0 || displayIdx >= len(m.playlist) {
		return -1
	}
	return displayIdx
}

// insertPlaylistTracks inserts tracks at display index at. plPos is the
// playback position the new entries take (only meaningful while shuffling);
// pass len(shuffleOrder) to enqueue at the end of the playback order.
func (m *Model) insertPlaylistTracks(at, plPos int, tracks []models.Track) {
	if len(tracks) == 0 {
		return
	}
	n := len(tracks)
	oldLen := len(m.playlist)
	if at < 0 {
		at = 0
	}
	if at > oldLen {
		at = oldLen
	}

	inserted := append([]models.Track(nil), tracks...)
	m.playlist = append(m.playlist[:at], append(inserted, m.playlist[at:]...)...)

	if m.shuffle && len(m.shuffleOrder) == oldLen {
		// Display indices at or past the insertion point shift by n.
		order := make([]int, 0, oldLen+n)
		for _, idx := range m.shuffleOrder {
			if idx >= at {
				idx += n
			}
			order = append(order, idx)
		}
		if plPos < 0 {
			plPos = 0
		}
		if plPos > len(order) {
			plPos = len(order)
		}
		fresh := make([]int, 0, n)
		for i := 0; i < n; i++ {
			fresh = append(fresh, at+i)
		}
		order = append(order[:plPos], append(fresh, order[plPos:]...)...)
		m.shuffleOrder = order
	} else {
		// Not shuffling, or the permutation no longer matches: the playback
		// order is the displayed order.
		m.shuffleOrder = nil
	}

	if m.currentIndex >= at {
		m.currentIndex += n
	}
	m.updatePlaylist()
}

// removePlaylistTrack removes the track at display index idx. currentIndex
// follows the neighbours: it shifts down when the removal happened before it,
// and otherwise stays put so it now refers to the entry that took the slot.
func (m *Model) removePlaylistTrack(idx int) {
	if idx < 0 || idx >= len(m.playlist) {
		return
	}
	oldLen := len(m.playlist)
	m.playlist = append(m.playlist[:idx], m.playlist[idx+1:]...)

	if m.shuffle && len(m.shuffleOrder) == oldLen {
		order := make([]int, 0, oldLen-1)
		for _, i := range m.shuffleOrder {
			switch {
			case i == idx:
				continue
			case i > idx:
				order = append(order, i-1)
			default:
				order = append(order, i)
			}
		}
		m.shuffleOrder = order
	} else {
		m.shuffleOrder = nil
	}

	if m.currentIndex > idx {
		m.currentIndex--
	}
	if m.currentIndex >= len(m.playlist) {
		m.currentIndex = len(m.playlist) - 1
	}
	m.updatePlaylist()
}

// movePlaylistTrack moves the track at display index from to display index to,
// remapping the shuffle permutation so the playback order stays a permutation
// of the new displayed order.
func (m *Model) movePlaylistTrack(from, to int) {
	n := len(m.playlist)
	if n == 0 || from < 0 || from >= n || to < 0 || to >= n || from == to {
		return
	}

	track := m.playlist[from]
	m.playlist = append(m.playlist[:from], m.playlist[from+1:]...)
	m.playlist = append(m.playlist[:to], append([]models.Track{track}, m.playlist[to:]...)...)

	if m.shuffle && len(m.shuffleOrder) == n {
		perm := displayIndexPerm(from, to)
		for i, idx := range m.shuffleOrder {
			m.shuffleOrder[i] = perm(idx)
		}
	} else {
		m.shuffleOrder = nil
	}

	switch {
	case m.currentIndex == from:
		m.currentIndex = to
	case from < m.currentIndex && to >= m.currentIndex:
		m.currentIndex--
	case from > m.currentIndex && to <= m.currentIndex:
		m.currentIndex++
	}
	m.updatePlaylist()
}

// moveToPlaybackPos moves the entry at display index from so that it plays at
// playback position plPos. While shuffling the displayed order is a different
// order and is left alone (only the queue moves); otherwise the two coincide,
// so this is an ordinary display move.
func (m *Model) moveToPlaybackPos(from, plPos int) {
	if m.shuffle && len(m.shuffleOrder) == len(m.playlist) {
		at := -1
		for i, idx := range m.shuffleOrder {
			if idx == from {
				at = i
				break
			}
		}
		if at < 0 {
			return
		}
		entry := m.shuffleOrder[at]
		m.shuffleOrder = append(m.shuffleOrder[:at], m.shuffleOrder[at+1:]...)
		if plPos < 0 {
			plPos = 0
		}
		if plPos > len(m.shuffleOrder) {
			plPos = len(m.shuffleOrder)
		}
		m.shuffleOrder = append(m.shuffleOrder[:plPos], append([]int{entry}, m.shuffleOrder[plPos:]...)...)
		return
	}
	m.movePlaylistTrack(from, plPos)
}

// reversePlaylistOrder reverses the displayed order and the shuffle permutation
// with it.
func (m *Model) reversePlaylistOrder() {
	n := len(m.playlist)
	if n < 2 {
		return
	}
	for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
		m.playlist[i], m.playlist[j] = m.playlist[j], m.playlist[i]
	}
	if m.shuffle && len(m.shuffleOrder) == n {
		for i, idx := range m.shuffleOrder {
			m.shuffleOrder[i] = n - 1 - idx
		}
	} else {
		m.shuffleOrder = nil
	}
	m.currentIndex = n - 1 - m.currentIndex
	m.updatePlaylist()
}

// enqueueTracks appends tracks to the end of the playback order. It returns
// the display index the inserted block starts at, and whether mpv is already
// playing (in which case the queue was mirrored into mpv; when it returns false
// the caller is expected to start playback).
func (m *Model) enqueueTracks(tracks []models.Track) (int, bool) {
	at := len(m.playlist)
	m.insertPlaylistTracks(at, len(m.shuffleOrder), tracks)
	playing := m.playing && m.mpvBackend.IsRunning()
	if playing {
		m.commitPlaylistChange()
	}
	return at, playing
}

// enqueueNextTracks inserts tracks so they play immediately after the current
// entry. See enqueueTracks for the return value.
func (m *Model) enqueueNextTracks(tracks []models.Track) bool {
	insertAt := m.currentIndex + 1
	if insertAt > len(m.playlist) {
		insertAt = len(m.playlist)
	}
	plPos := m.playbackPosOf(m.currentIndex) + 1
	if plPos < 1 {
		plPos = len(m.shuffleOrder)
	}
	m.insertPlaylistTracks(insertAt, plPos, tracks)
	if m.playing && m.mpvBackend.IsRunning() {
		m.commitPlaylistChange()
		return true
	}
	return false
}

// displayIndexPerm returns the mapping from old display index to new display
// index for "move the entry at from to to".
func displayIndexPerm(from, to int) func(int) int {
	if from < to {
		return func(i int) int {
			switch {
			case i == from:
				return to
			case i > from && i <= to:
				return i - 1
			default:
				return i
			}
		}
	}
	return func(i int) int {
		switch {
		case i == from:
			return to
		case i >= to && i < from:
			return i + 1
		default:
			return i
		}
	}
}
