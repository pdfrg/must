package scanner

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dhowden/tag"
	"github.com/pdfrg/must/internal/db"
	"github.com/pdfrg/must/internal/duration"
	imgpkg "github.com/pdfrg/must/internal/image"
	"github.com/pdfrg/must/internal/models"
)

const (
	audioExts = ".mp3.flac.ogg.opus.m4a.aac.wma.wav"
	batchSize = 500
)

var logger *log.Logger

func SetLogger(l *log.Logger) {
	logger = l
}

type ScanResult struct {
	TotalFiles   int
	NewFiles     int
	UpdatedFiles int
	RemovedFiles int
	Errors       int
	Duration     time.Duration
}

type Scanner struct {
	db      *db.LibraryDB
	mu      sync.Mutex
	stopped bool
}

func NewScanner(libraryDB *db.LibraryDB) *Scanner {
	return &Scanner{db: libraryDB}
}

func (s *Scanner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
}

func (s *Scanner) Scan(musicDirs []string) (*ScanResult, error) {
	start := time.Now()
	result := &ScanResult{}

	resetCount, resetErr := s.db.ResetZeroDurationModTimes()
	if resetErr != nil && logger != nil {
		logger.Printf("Warning: failed to reset zero-duration tracks: %v", resetErr)
	}
	if resetCount > 0 && logger != nil {
		logger.Printf("Resetting modtime for %d tracks with zero duration (will re-scan)", resetCount)
	}

	existingPaths := make(map[string]bool)
	var batch []*models.Track
	var batchIsUpdate []bool

	flushBatch := func() {
		if len(batch) == 0 {
			return
		}
		inserted, err := s.db.InsertTracksBatch(batch)
		if err != nil && logger != nil {
			logger.Printf("Error in batch insert: %v", err)
		}
		result.Errors += len(batch) - inserted
		for _, isUpdate := range batchIsUpdate {
			if isUpdate {
				result.UpdatedFiles++
			} else {
				result.NewFiles++
			}
		}
		batch = batch[:0]
		batchIsUpdate = batchIsUpdate[:0]
	}

	var walkErr error
	for _, musicDir := range musicDirs {
		err := filepath.WalkDir(musicDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				result.Errors++
				return nil
			}

			s.mu.Lock()
			stopped := s.stopped
			s.mu.Unlock()
			if stopped {
				return fmt.Errorf("scan stopped")
			}

			if d.IsDir() {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			if !strings.Contains(audioExts, ext) {
				return nil
			}

			result.TotalFiles++
			existingPaths[path] = true

			info, err := d.Info()
			if err != nil {
				result.Errors++
				return nil
			}
			modTime := info.ModTime().Unix()

			existing, err := s.db.GetTrackByPath(path)
			if err != nil {
				result.Errors++
				return nil
			}

			if existing != nil && existing.FileModTime == modTime {
				return nil
			}

			track, err := s.readTrack(path, modTime)
			if err != nil {
				if logger != nil {
					logger.Printf("Error reading %s: %v", path, err)
				}
				result.Errors++
				return nil
			}

			batch = append(batch, track)
			batchIsUpdate = append(batchIsUpdate, existing != nil)
			if len(batch) >= batchSize {
				flushBatch()
			}

			return nil
		})
		if err != nil {
			walkErr = err
		}
	}

	flushBatch()

	if walkErr != nil && walkErr.Error() != "scan stopped" {
		return result, walkErr
	}

	removed, err := s.db.DeleteMissingTracks(existingPaths)
	if err != nil && logger != nil {
		logger.Printf("Error removing missing tracks: %v", err)
	}
	result.RemovedFiles = removed

	result.Duration = time.Since(start)
	return result, nil
}

func (s *Scanner) readTrack(path string, modTime int64) (*models.Track, error) {
	track := &models.Track{
		Path:        path,
		FileModTime: modTime,
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open: %w", err)
	}
	defer func() { _ = f.Close() }()

	tags, err := tag.ReadFrom(f)
	if err != nil {
		track.Title = filepath.Base(path)
	} else {
		track.Title = tags.Title()
		track.Artist = tags.Artist()
		track.Album = tags.Album()
		track.AlbumArtist = tags.AlbumArtist()
		track.Year = tags.Year()
		track.Genre = tags.Genre()
		trackNum, totalTracks := tags.Track()
		discNum, totalDiscs := tags.Disc()
		track.TrackNum = trackNum
		track.DiscNum = discNum
		_, _ = totalTracks, totalDiscs
		track.HasCoverArt = tags.Picture() != nil
		if track.HasCoverArt {
			if err := imgpkg.CacheArtData(path, tags.Picture().Data); err != nil && logger != nil {
				logger.Printf("Warning: could not cache art for %s: %v", path, err)
			}
		}
	}

	if track.Title == "" {
		track.Title = filepath.Base(path)
	}

	dur, err := duration.FileDuration(path)
	if err != nil {
		if logger != nil {
			logger.Printf("Warning: could not get duration for %s: %v", path, err)
		}
	} else if dur <= 0 {
		if logger != nil {
			logger.Printf("Warning: zero duration for %s", path)
		}
	}
	track.Duration = dur

	return track, nil
}
