package playlist

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pdfrg/must/internal/navidrome"
)

func TestHealSubsonicIDs(t *testing.T) {
	oldID := "e3b7fc2ae9447bbec37a13bf916e3cf6"
	newID := "6VHl3uR4kss6sUPKA8Cwnk"
	url := "http://navidrome.lan:4533/rest/stream.view?id=" + oldID + "&format=raw&u=mds&t=abc"
	local := "../music/Artist/Album/01 - Track.mp3"
	other := "http://example.com/stream?id=not-a-navidrome-id"

	content := strings.Join([]string{
		"#EXTM3U",
		"#EXTINF:123,Artist - Title",
		url,
		local,
		other,
		"",
	}, "\n")

	path := filepath.Join(t.TempDir(), "playlist.m3u")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	n, err := HealSubsonicIDs(path, navidrome.Canonical)
	if err != nil {
		t.Fatalf("HealSubsonicIDs: %v", err)
	}
	if n != 1 {
		t.Fatalf("rewrote %d entries, want 1", n)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(content, oldID, newID, 1)
	if string(got) != want {
		t.Errorf("file content mismatch:\n got: %q\nwant: %q", got, want)
	}

	// Second pass must be a no-op.
	if n, err := HealSubsonicIDs(path, navidrome.Canonical); err != nil || n != 0 {
		t.Errorf("second pass rewrote %d entries, err=%v; want 0, nil", n, err)
	}
	if got2, _ := os.ReadFile(path); string(got2) != want {
		t.Errorf("second pass changed file:\n got: %q\nwant: %q", got2, want)
	}
}
