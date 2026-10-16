package tui

import (
	"testing"

	"github.com/pdfrg/must/internal/tui/modals"
)

func TestAllocateExploreHybrid(t *testing.T) {
	counts := map[modals.ExploreSource]int{
		modals.ExploreLocal:    5000,
		modals.ExploreSubsonic: 2000,
		modals.ExploreTemp:     10,
	}
	available := []modals.ExploreSource{modals.ExploreLocal, modals.ExploreSubsonic, modals.ExploreTemp}

	alloc := allocateExplore(20, counts, available)

	total := 0
	for _, s := range available {
		if alloc[s] < 1 {
			t.Fatalf("source %v got no guaranteed slot: %+v", s, alloc)
		}
		total += alloc[s]
	}
	if total != 20 {
		t.Fatalf("allocation sums to %d, want 20: %+v", total, alloc)
	}
	if alloc[modals.ExploreLocal] <= alloc[modals.ExploreSubsonic] {
		t.Fatalf("local (largest) should dominate: %+v", alloc)
	}
	if alloc[modals.ExploreSubsonic] <= alloc[modals.ExploreTemp] {
		t.Fatalf("subsonic should outrank temp: %+v", alloc)
	}
	// Reserve(1 each) + 17 proportional with largest-remainder rounding.
	if alloc[modals.ExploreLocal] != 13 || alloc[modals.ExploreSubsonic] != 6 || alloc[modals.ExploreTemp] != 1 {
		t.Fatalf("unexpected hybrid split: %+v", alloc)
	}
}

func TestAllocateExploreSingleSource(t *testing.T) {
	counts := map[modals.ExploreSource]int{modals.ExploreLocal: 42}
	alloc := allocateExplore(20, counts, []modals.ExploreSource{modals.ExploreLocal})
	if alloc[modals.ExploreLocal] != 20 {
		t.Fatalf("single source should take the whole batch: %+v", alloc)
	}
}

func TestAllocateExploreFewerSlotsThanSources(t *testing.T) {
	counts := map[modals.ExploreSource]int{
		modals.ExploreLocal:    5,
		modals.ExploreSubsonic: 5,
		modals.ExploreTemp:     5,
	}
	available := []modals.ExploreSource{modals.ExploreLocal, modals.ExploreSubsonic, modals.ExploreTemp}
	alloc := allocateExplore(2, counts, available)
	if len(alloc) != 2 {
		t.Fatalf("expected 2 allocated sources, got %+v", alloc)
	}
}

func TestDedupeShuffleExploreRemovesDuplicates(t *testing.T) {
	pool := []modals.ExploreAlbum{
		{Name: "Same", Artist: "A", Source: modals.ExploreLocal},
		{Name: "same", Artist: "a", Source: modals.ExploreSubsonic},
		{Name: "Other", Artist: "A", Source: modals.ExploreLocal},
		{Name: "Third", Artist: "B", Source: modals.ExploreLocal},
	}
	got := dedupeShuffleExplore(pool, 10)
	if len(got) != 3 {
		t.Fatalf("expected 3 unique albums, got %d: %+v", len(got), got)
	}
	seen := map[string]bool{}
	for _, a := range got {
		key := a.Artist + "/" + a.Name
		if seen[key] {
			t.Fatalf("duplicate survived: %s", key)
		}
		seen[key] = true
	}
}

func TestDedupeShuffleExploreRespectsLimit(t *testing.T) {
	pool := make([]modals.ExploreAlbum, 0, 30)
	for i := 0; i < 30; i++ {
		pool = append(pool, modals.ExploreAlbum{Name: string(rune('A' + i)), Artist: "X"})
	}
	if got := dedupeShuffleExplore(pool, 12); len(got) != 12 {
		t.Fatalf("expected 12 albums, got %d", len(got))
	}
}
