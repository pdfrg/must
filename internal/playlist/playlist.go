package playlist

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pdfrg/must/internal/models"
)

type Playlist struct {
	Name     string
	FilePath string
	Tracks   []string
}

type SaveOptions struct {
	UseEXTINF     bool
	RelativePaths bool
	Tracks        []models.Track // metadata for EXTINF lines; must match length of paths passed to Save
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

func Save(path string, paths []string, opts *SaveOptions) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create playlist: %w", err)
	}
	defer func() { _ = f.Close() }()

	writer := bufio.NewWriter(f)
	if _, err := writer.WriteString("#EXTM3U\n"); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	if opts == nil {
		opts = &SaveOptions{}
	}

	for i, trackPath := range paths {
		entryPath := trackPath

		if opts.RelativePaths {
			rel, err := filepath.Rel(dir, trackPath)
			if err == nil {
				entryPath = rel
			}
		}

		if opts.UseEXTINF && i < len(opts.Tracks) {
			t := opts.Tracks[i]
			duration := int(t.Duration)
			var label string
			if t.Artist != "" && t.Title != "" {
				label = t.Artist + " - " + t.Title
			} else if t.Title != "" {
				label = t.Title
			} else {
				label = filepath.Base(trackPath)
			}
			extinf := fmt.Sprintf("#EXTINF:%d,%s\n", duration, label)
			if _, err := writer.WriteString(extinf); err != nil {
				_ = os.Remove(tmpPath)
				return err
			}
		}

		if _, err := writer.WriteString(entryPath + "\n"); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
	}

	if err := writer.Flush(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func SaveLegacy(path string, paths []string) error {
	return Save(path, paths, nil)
}
