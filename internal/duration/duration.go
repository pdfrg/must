package duration

import (
	"fmt"
	"os"
	"strings"
)

func FileDuration(path string) (float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".flac"):
		return flacDuration(f)
	case strings.HasSuffix(lower, ".mp3"):
		return mp3Duration(f)
	case strings.HasSuffix(lower, ".m4a"), strings.HasSuffix(lower, ".aac"):
		return m4aDuration(f)
	case strings.HasSuffix(lower, ".ogg"), strings.HasSuffix(lower, ".opus"):
		return oggDuration(f)
	default:
		return 0, fmt.Errorf("unsupported format: %s", path)
	}
}
