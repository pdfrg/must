package duration

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

func oggDuration(f *os.File) (float64, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	info, err := f.Stat()
	if err != nil {
		return 0, err
	}

	var sampleRate int64
	var lastGranule int64

	chunkSize := int64(65536)
	fileSize := info.Size()
	pos := fileSize

	for pos > 0 {
		seekPos := pos - chunkSize
		if seekPos < 0 {
			seekPos = 0
		}

		if _, err := f.Seek(seekPos, io.SeekStart); err != nil {
			break
		}

		buf := make([]byte, pos-seekPos)
		if _, err := io.ReadFull(f, buf); err != nil {
			break
		}

		for i := len(buf) - 4; i >= 0; i-- {
			if isOggPage(buf[i:]) {
				granule := int64(binary.LittleEndian.Uint64(buf[i+6 : i+14]))
				if granule > 0 {
					lastGranule = granule
					break
				}
			}
		}

		if lastGranule > 0 {
			break
		}
		pos = seekPos
	}

	if sampleRate == 0 {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return 0, err
		}
		sampleRate = findVorbisSampleRate(f)
	}

	if sampleRate == 0 {
		sampleRate = 44100
	}

	if lastGranule > 0 {
		return float64(lastGranule) / float64(sampleRate), nil
	}

	return 0, fmt.Errorf("could not determine OGG duration")
}

func isOggPage(data []byte) bool {
	if len(data) < 27 {
		return false
	}
	return data[0] == 'O' && data[1] == 'g' && data[2] == 'g' && data[3] == 'S'
}

func findVorbisSampleRate(f *os.File) int64 {
	header := make([]byte, 27)
	if _, err := io.ReadFull(f, header); err != nil {
		return 0
	}

	if !isOggPage(header) {
		return 0
	}

	segCount := int(header[26])
	segTable := make([]byte, segCount)
	if _, err := io.ReadFull(f, segTable); err != nil {
		return 0
	}

	bodySize := 0
	for _, s := range segTable {
		bodySize += int(s)
	}

	body := make([]byte, bodySize)
	if _, err := io.ReadFull(f, body); err != nil {
		return 0
	}

	if len(body) < 16 {
		return 0
	}

	// Opus identification header — always 48 kHz
	if body[0] == 'O' && string(body[:8]) == "OpusHead" {
		return 48000
	}

	// Vorbis identification header
	if body[0] != 0x01 {
		return 0
	}

	return int64(binary.LittleEndian.Uint32(body[12:16]))
}
