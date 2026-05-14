package playlist

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Playlist struct {
	Name     string
	FilePath string
	Tracks   []string
}

func Load(path string) (*Playlist, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open playlist: %w", err)
	}
	defer func() { _ = f.Close() }()

	p := &Playlist{
		Name:     strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		FilePath: path,
	}

	dir := filepath.Dir(path)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if filepath.IsAbs(line) {
			p.Tracks = append(p.Tracks, line)
		} else {
			p.Tracks = append(p.Tracks, filepath.Join(dir, line))
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading playlist: %w", err)
	}

	return p, nil
}

func Save(path string, tracks []string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create playlist: %w", err)
	}
	defer func() { _ = f.Close() }()

	writer := bufio.NewWriter(f)
	if _, err := writer.WriteString("#EXTM3U\n"); err != nil {
		return err
	}

	for _, track := range tracks {
		if _, err := writer.WriteString(track + "\n"); err != nil {
			return err
		}
	}

	return writer.Flush()
}
