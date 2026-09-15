package playlist

import (
	"net/url"
	"os"
	"strings"
)

// HealSubsonicIDs rewrites the `id` query parameter of Subsonic stream URLs in
// an m3u file using heal, preserving every other byte (comments, EXTINF lines,
// local paths, query-parameter order). It writes atomically and is a no-op when
// no entry changes. It returns the number of entries rewritten.
//
// This repairs playlists saved before a Navidrome 0.64 upgrade, whose id
// migration invalidated the ids embedded in stream URLs.
func HealSubsonicIDs(path string, heal func(string) string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	changed := 0
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		newLine, ok := healEntryLine(line, heal)
		if ok {
			lines[i] = newLine
			changed++
		}
	}
	if changed == 0 {
		return 0, nil
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		_ = os.Remove(tmpPath)
		return 0, err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return 0, err
	}
	return changed, nil
}

// healEntryLine replaces the value of the `id` query parameter in a stream URL
// line, preserving the rest of the line verbatim.
func healEntryLine(line string, heal func(string) string) (string, bool) {
	if !IsURL(strings.TrimSpace(line)) {
		return line, false
	}
	for _, sep := range []string{"?id=", "&id="} {
		i := strings.Index(line, sep)
		if i < 0 {
			continue
		}
		start := i + len(sep)
		end := len(line)
		if j := strings.IndexAny(line[start:], "&#\r\n"); j >= 0 {
			end = start + j
		}
		raw := line[start:end]
		decoded, err := url.QueryUnescape(raw)
		if err != nil {
			decoded = raw
		}
		newID := heal(decoded)
		if newID == decoded {
			return line, false
		}
		return line[:start] + url.QueryEscape(newID) + line[end:], true
	}
	return line, false
}
