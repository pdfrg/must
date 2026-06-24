package duration

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

func m4aDuration(f *os.File) (float64, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	for {
		boxHeader := make([]byte, 8)
		if _, err := io.ReadFull(f, boxHeader); err != nil {
			return 0, fmt.Errorf("reading atom header: %w", err)
		}

		boxSize := binary.BigEndian.Uint32(boxHeader[:4])
		boxType := string(boxHeader[4:8])

		if boxSize == 0 {
			return 0, fmt.Errorf("unsupported m4a atom size 0")
		}

		if boxSize == 1 {
			extHeader := make([]byte, 8)
			if _, err := io.ReadFull(f, extHeader); err != nil {
				return 0, err
			}
			boxSize = uint32(binary.BigEndian.Uint64(extHeader))
		}

		if boxType == "moov" {
			return parseMoovAtom(f, int64(boxSize)-8)
		}

		if boxSize < 8 {
			return 0, fmt.Errorf("invalid atom size")
		}

		if _, err := f.Seek(int64(boxSize)-8, io.SeekCurrent); err != nil {
			return 0, err
		}
	}
}

func parseMoovAtom(f *os.File, remaining int64) (float64, error) {
	startPos, _ := f.Seek(0, io.SeekCurrent)
	endPos := startPos + remaining

	var timescale int64
	var duration int64

	for {
		currentPos, _ := f.Seek(0, io.SeekCurrent)
		if currentPos >= endPos {
			break
		}

		atomHeader := make([]byte, 8)
		if _, err := io.ReadFull(f, atomHeader); err != nil {
			break
		}

		atomSize := int64(binary.BigEndian.Uint32(atomHeader[:4]))
		atomType := string(atomHeader[4:8])

		if atomSize == 1 {
			extHeader := make([]byte, 8)
			if _, err := io.ReadFull(f, extHeader); err != nil {
				break
			}
			atomSize = int64(binary.BigEndian.Uint64(extHeader))
		}

		if atomSize < 8 {
			break
		}

		if atomType == "mvhd" {
			version := make([]byte, 1)
			if _, err := io.ReadFull(f, version); err != nil {
				break
			}

			if version[0] == 0 {
				data := make([]byte, 99)
				if _, err := io.ReadFull(f, data); err != nil {
					break
				}
				timescale = int64(binary.BigEndian.Uint32(data[12:16]))
				duration = int64(binary.BigEndian.Uint32(data[16:20]))
			} else {
				data := make([]byte, 111)
				if _, err := io.ReadFull(f, data); err != nil {
					break
				}
				timescale = int64(binary.BigEndian.Uint32(data[20:24]))
				duration = int64(binary.BigEndian.Uint64(data[24:32]))
			}

			if timescale > 0 && duration > 0 {
				return float64(duration) / float64(timescale), nil
			}
			break
		}

		if _, err := f.Seek(currentPos+atomSize, io.SeekStart); err != nil {
			break
		}
	}

	return 0, fmt.Errorf("mdhd/mvhd duration not found")
}
