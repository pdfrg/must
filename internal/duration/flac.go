package duration

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

func flacDuration(f *os.File) (float64, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	header := make([]byte, 4)
	if _, err := io.ReadFull(f, header); err != nil {
		return 0, fmt.Errorf("reading FLAC marker: %w", err)
	}
	if string(header) != "fLaC" {
		return 0, fmt.Errorf("not a FLAC file")
	}

	for {
		blockHeader := make([]byte, 4)
		if _, err := io.ReadFull(f, blockHeader); err != nil {
			return 0, fmt.Errorf("reading block header: %w", err)
		}

		blockType := blockHeader[0] & 0x7f
		isLast := blockHeader[0]&0x80 != 0
		blockSize := int(binary.BigEndian.Uint32([]byte{0, blockHeader[1], blockHeader[2], blockHeader[3]}))

		if blockType == 0 {
			streamInfo := make([]byte, blockSize)
			if _, err := io.ReadFull(f, streamInfo); err != nil {
				return 0, fmt.Errorf("reading STREAMINFO: %w", err)
			}

			sampleRate := int(binary.BigEndian.Uint32([]byte{
				streamInfo[10], streamInfo[11], streamInfo[12], 0,
			}) >> 12)

			totalSamplesHi := int(streamInfo[13]&0x0F) << 32
			totalSamplesLo := int(binary.BigEndian.Uint32(streamInfo[14:18]))
			totalSamples := totalSamplesHi | totalSamplesLo

			if totalSamples == 0 || sampleRate == 0 {
				return 0, fmt.Errorf("invalid FLAC STREAMINFO: samples=%d rate=%d", totalSamples, sampleRate)
			}

			return float64(totalSamples) / float64(sampleRate), nil
		}

		if _, err := f.Seek(int64(blockSize), io.SeekCurrent); err != nil {
			return 0, err
		}

		if isLast {
			break
		}
	}

	return 0, fmt.Errorf("STREAMINFO block not found")
}
