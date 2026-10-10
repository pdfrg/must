package tui

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/pdfrg/must/internal/models"
	"github.com/pdfrg/must/internal/tui/widgets"
)

// queueModel builds a Model with just the queue state wired up, so the
// playlist mutation helpers can run without a terminal or an mpv process.
func queueModel(tracks ...models.Track) *Model {
	m := &Model{
		playlist:       append([]models.Track(nil), tracks...),
		playlistWidget: widgets.NewPlaylist(nil),
	}
	return m
}

// applyOps replays a reconciliation plan the way syncMPVPlaylist would,
// including mpv's drag semantics for playlist-move (a from>to move lands
// exactly at to, which is the only direction the planner emits).
func applyOps(current []string, ops []syncOp) []string {
	got := append([]string(nil), current...)
	for _, op := range ops {
		switch op.kind {
		case syncMove:
			if op.from < 0 || op.from >= len(got) || op.to < 0 || op.to > len(got)-1 {
				panic(fmt.Sprintf("bad move %+v on %v", op, got))
			}
			e := got[op.from]
			got = append(got[:op.from], got[op.from+1:]...)
			got = append(got[:op.to], append([]string{e}, got[op.to:]...)...)
		case syncInsert:
			if op.to < 0 || op.to > len(got) {
				panic(fmt.Sprintf("bad insert %+v on %v", op, got))
			}
			got = append(got[:op.to], append([]string{op.path}, got[op.to:]...)...)
		case syncRemove:
			if op.from < 0 || op.from >= len(got) {
				panic(fmt.Sprintf("bad remove %+v on %v", op, got))
			}
			got = append(got[:op.from], got[op.from+1:]...)
		}
	}
	return got
}

func TestPlanPlaylistSync(t *testing.T) {
	tests := []struct {
		name    string
		current []string
		want    []string
	}{
		{"already in sync", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"no ops needed for empty playlist", nil, nil},
		{"adjacent swap", []string{"a", "b", "c"}, []string{"b", "a", "c"}},
		{"move to top", []string{"a", "b", "c", "d"}, []string{"d", "a", "b", "c"}},
		{"move to bottom", []string{"a", "b", "c", "d"}, []string{"b", "c", "d", "a"}},
		{"reverse", []string{"a", "b", "c", "d"}, []string{"d", "c", "b", "a"}},
		{"insert in middle", []string{"a", "c"}, []string{"a", "b", "c"}},
		{"insert at front", []string{"b"}, []string{"a", "b"}},
		{"insert at end", []string{"a"}, []string{"a", "b"}},
		{"remove middle", []string{"a", "b", "c"}, []string{"a", "c"}},
		{"remove several", []string{"a", "b", "c", "d", "e"}, []string{"a", "e"}},
		{"rotation", []string{"a", "b", "c", "d"}, []string{"c", "d", "a", "b"}},
		{"duplicate inserted", []string{"a", "b"}, []string{"a", "b", "a"}},
		{"duplicate removed", []string{"a", "a", "b"}, []string{"a", "b"}},
		{"all new", nil, []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := planPlaylistSync(tt.current, tt.want)
			got := applyOps(tt.current, ops)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ops %+v turned %v into %v, want %v", ops, tt.current, got, tt.want)
			}
			for _, op := range ops {
				if op.kind == syncMove && op.from <= op.to {
					t.Errorf("move op must only ever move up (from > to), got %+v", op)
				}
			}
		})
	}
}

// TestPlanPlaylistSyncStable guards against the reconciler reshuffling a queue
// that is already correct, which would interrupt playback for no reason.
func TestPlanPlaylistSyncNoOpsWhenInSync(t *testing.T) {
	cur := []string{"a", "b", "c", "d", "e"}
	if ops := planPlaylistSync(cur, cur); len(ops) != 0 {
		t.Errorf("expected no ops for an in-sync playlist, got %+v", ops)
	}
}

func TestMpvPathKey(t *testing.T) {
	tests := []struct{ in, want string }{
		{"http://server/rest/stream?id=abc&u=x", "http://server/rest/stream?id=abc&u=x"},
		{"https://server/rest/stream", "https://server/rest/stream"},
		{"/music/a.flac", "/music/a.flac"},
		{"/music/./a.flac", "/music/a.flac"},
		{"/music/x/../a.flac", "/music/a.flac"},
		{"./rel/a.flac", ""}, // resolved against cwd; only the cleaning is asserted
	}
	for _, tt := range tests {
		got := mpvPathKey(tt.in)
		if tt.want != "" && got != tt.want {
			t.Errorf("mpvPathKey(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	// The whole point of the key: what must was given and what mpv echoes back
	// must compare equal even when one is relative.
	if mpvPathKey("./a.flac") != mpvPathKey("a.flac") {
		t.Error("relative spellings of the same path must map to the same key")
	}
	if mpvPathKey("/music/./a.flac") != mpvPathKey("/music/a.flac") {
		t.Error("dirty absolute paths must map to the same key as clean ones")
	}
}

func track(id string) models.Track {
	return models.Track{Path: "/music/" + id + ".flac", Title: id}
}

// displayIndexPerm must be a bijection over the playlist indices, otherwise
// the shuffle order would lose or duplicate entries after a move.
func TestDisplayIndexPermIsBijection(t *testing.T) {
	for n := 2; n <= 8; n++ {
		for from := 0; from < n; from++ {
			for to := 0; to < n; to++ {
				if from == to {
					continue
				}
				perm := displayIndexPerm(from, to)
				seen := make(map[int]bool, n)
				for i := 0; i < n; i++ {
					v := perm(i)
					if v < 0 || v >= n {
						t.Fatalf("n=%d move %d->%d: perm(%d)=%d out of range", n, from, to, i, v)
					}
					if seen[v] {
						t.Fatalf("n=%d move %d->%d: perm(%d)=%d already produced", n, from, to, i, v)
					}
					seen[v] = true
				}
				if got := perm(from); got != to {
					t.Errorf("n=%d move %d->%d: moved entry landed at %d", n, from, to, got)
				}
			}
		}
	}
}

// A move in the display order must leave the shuffle order a valid permutation
// of the new display order (this is what kept mpv and the UI apart).
func TestMovePlaylistTrackKeepsShuffleOrderValid(t *testing.T) {
	m := queueModel(track("a"), track("b"), track("c"), track("d"))
	m.shuffle = true
	m.shuffleOrder = []int{2, 0, 3, 1}

	playbackOrder := func() []string {
		out := make([]string, len(m.shuffleOrder))
		for i, idx := range m.shuffleOrder {
			out[i] = m.playlist[idx].Path
		}
		return out
	}
	before := playbackOrder()

	m.movePlaylistTrack(1, 3)

	if got, want := len(m.shuffleOrder), len(m.playlist); got != want {
		t.Fatalf("shuffleOrder length = %d, want %d", got, want)
	}
	seen := make(map[int]bool, len(m.shuffleOrder))
	for _, idx := range m.shuffleOrder {
		if idx < 0 || idx >= len(m.playlist) {
			t.Fatalf("shuffleOrder %v out of range for %d entries", m.shuffleOrder, len(m.playlist))
		}
		if seen[idx] {
			t.Fatalf("shuffleOrder %v has duplicate index %d", m.shuffleOrder, idx)
		}
		seen[idx] = true
	}
	if got := m.playlist[3].Title; got != "b" {
		t.Errorf("display[3] = %q, want \"b\"", got)
	}
	// A display reorder must not disturb what is queued to play: every entry
	// keeps its playback slot and only its display index is relabelled.
	after := playbackOrder()
	if !reflect.DeepEqual(before, after) {
		t.Errorf("playback order changed: before %v, after %v", before, after)
	}
}

func TestInsertRemoveKeepShuffleOrderValid(t *testing.T) {
	newModel := func() *Model {
		m := queueModel(track("a"), track("b"), track("c"), track("d"))
		m.shuffle = true
		m.shuffleOrder = []int{2, 0, 3, 1}
		m.currentIndex = 1
		return m
	}
	assertValid := func(t *testing.T, m *Model) {
		t.Helper()
		if len(m.shuffleOrder) != len(m.playlist) {
			t.Fatalf("shuffleOrder length = %d, want %d", len(m.shuffleOrder), len(m.playlist))
		}
		seen := make(map[int]bool, len(m.shuffleOrder))
		for _, idx := range m.shuffleOrder {
			if idx < 0 || idx >= len(m.playlist) || seen[idx] {
				t.Fatalf("shuffleOrder %v is not a permutation of %d entries", m.shuffleOrder, len(m.playlist))
			}
			seen[idx] = true
		}
	}

	m := newModel()
	// Enqueue next: the new tracks must sit right after the current entry in
	// the playback order.
	m.insertPlaylistTracks(m.currentIndex+1, m.playbackPosOf(m.currentIndex)+1, []models.Track{track("x"), track("y")})
	assertValid(t, m)
	if m.playlist[2].Title != "x" || m.playlist[3].Title != "y" {
		t.Errorf("insert at %d put %q,%q where x,y were expected", m.currentIndex+1, m.playlist[2].Title, m.playlist[3].Title)
	}
	if got := m.playbackPosOf(2); got != m.playbackPosOf(m.currentIndex)+1 {
		t.Errorf("new entry %d plays at %d, want %d", 2, got, m.playbackPosOf(m.currentIndex)+1)
	}
	if m.currentIndex != 1 {
		t.Errorf("currentIndex = %d, want 1 (the playing entry did not move)", m.currentIndex)
	}

	m = newModel()
	m.insertPlaylistTracks(len(m.playlist), len(m.shuffleOrder), []models.Track{track("z")})
	assertValid(t, m)
	if got := m.playbackPosOf(len(m.playlist) - 1); got != len(m.shuffleOrder)-1 {
		t.Errorf("entry %d plays at %d, want the last slot %d", len(m.playlist)-1, got, len(m.shuffleOrder)-1)
	}

	m = newModel()
	m.removePlaylistTrack(2)
	assertValid(t, m)
	if len(m.playlist) != 3 || m.playlist[2].Title != "d" {
		t.Errorf("after removing index 2: %v", m.playlist)
	}
	if m.currentIndex != 1 {
		t.Errorf("currentIndex = %d, want 1", m.currentIndex)
	}
}

func TestRemovePlaylistTrackFollowsCurrent(t *testing.T) {
	m := queueModel(track("a"), track("b"), track("c"), track("d"))
	m.currentIndex = 2
	// Removing an entry before the current one shifts the current index down.
	m.removePlaylistTrack(0)
	if m.currentIndex != 1 {
		t.Errorf("removing an entry before the current one: currentIndex = %d, want 1", m.currentIndex)
	}
	if got := m.playlist[m.currentIndex].Title; got != "c" {
		t.Errorf("current entry = %q, want \"c\"", got)
	}
	// Removing the current entry leaves the index where it was, now referring
	// to the entry that took its place.
	m.removePlaylistTrack(1)
	if m.currentIndex != 1 {
		t.Errorf("removing the current entry: currentIndex = %d, want 1", m.currentIndex)
	}
	if got := m.playlist[m.currentIndex].Title; got != "d" {
		t.Errorf("current entry = %q, want \"d\"", got)
	}
}

func TestMoveToPlaybackPos(t *testing.T) {
	m := queueModel(track("a"), track("b"), track("c"), track("d"))
	m.shuffle = true
	m.shuffleOrder = []int{2, 0, 3, 1}
	m.currentIndex = 0
	// Play entry 3 next.
	m.moveToPlaybackPos(3, m.playbackPosOf(m.currentIndex)+1)

	if got := m.playbackPosOf(3); got != m.playbackPosOf(m.currentIndex)+1 {
		t.Errorf("entry 3 now plays at %d, want %d", got, m.playbackPosOf(m.currentIndex)+1)
	}
	if got := len(m.shuffleOrder); got != len(m.playlist) {
		t.Errorf("shuffleOrder length = %d, want %d", got, len(m.playlist))
	}
}

func TestReversePlaylistOrder(t *testing.T) {
	m := queueModel(track("a"), track("b"), track("c"))
	m.shuffle = true
	m.shuffleOrder = []int{2, 0, 1}
	m.currentIndex = 2
	m.reversePlaylistOrder()
	if m.playlist[0].Title != "c" || m.playlist[2].Title != "a" {
		t.Errorf("reversed playlist = %v", []string{m.playlist[0].Title, m.playlist[1].Title, m.playlist[2].Title})
	}
	if m.currentIndex != 0 {
		t.Errorf("currentIndex = %d, want 0 (the playing track followed the move)", m.currentIndex)
	}
	// Reversing only relabels indices: the entry that played first still plays
	// first. Old order [2 0 1] means c, a, b; after the reverse c is display 0,
	// a is display 2 and b is display 1.
	if !reflect.DeepEqual(m.shuffleOrder, []int{0, 2, 1}) {
		t.Errorf("shuffleOrder = %v, want [0 2 1]", m.shuffleOrder)
	}
}

// buildMPVPlaylistPaths and the index mapping must stay exact inverses: this is
// the invariant the whole fix rests on.
func TestMPVIndexMappingRoundTrip(t *testing.T) {
	for _, n := range []int{1, 2, 5, 17} {
		m := &Model{playlist: make([]models.Track, n), shuffle: true}
		for i := range m.playlist {
			m.playlist[i] = track(fmt.Sprintf("t%d", i))
		}
		m.shuffleOrder = shuffleIndices(n)

		paths := m.buildMPVPlaylistPaths()
		if len(paths) != n {
			t.Fatalf("buildMPVPlaylistPaths returned %d paths, want %d", len(paths), n)
		}
		for i, p := range paths {
			if want := m.playlist[m.shuffleOrder[i]].Path; p != want {
				t.Fatalf("paths[%d] = %q, want %q", i, p, want)
			}
		}
		for displayIdx := 0; displayIdx < n; displayIdx++ {
			mpvIdx := m.playlistIndexToMPVIndex(displayIdx)
			if mpvIdx < 0 || mpvIdx >= n {
				t.Fatalf("playlistIndexToMPVIndex(%d) = %d out of range", displayIdx, mpvIdx)
			}
			if got := m.mpvIndexToPlaylistIndex(mpvIdx); got != displayIdx {
				t.Errorf("round trip of %d through mpv index %d gave %d", displayIdx, mpvIdx, got)
			}
			if want := m.playlist[displayIdx].Path; paths[mpvIdx] != want {
				t.Errorf("display %d (%s) maps to mpv %d which holds %q", displayIdx, want, mpvIdx, paths[mpvIdx])
			}
		}
	}
}
