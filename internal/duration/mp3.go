package duration

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

func mp3Duration(f *os.File) (float64, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	if offset, err := findID3v2Tag(f); err == nil && offset > 0 {
		if _, err := f.Seek(int64(offset), io.SeekStart); err != nil {
			return 0, err
		}
	} else {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return 0, err
		}
	}

	buf := make([]byte, 4)
	if _, err := io.ReadFull(f, buf); err != nil {
		return 0, fmt.Errorf("reading mp3 header: %w", err)
	}

	if buf[0] == 'X' && buf[1] == 'i' && buf[2] == 'n' && buf[3] == 'g' {
		return parseXingHeader(f)
	}

	if buf[0] == 'V' && buf[1] == 'B' && buf[2] == 'R' && buf[3] == 'I' {
		return parseVBRIHeader(f)
	}

	if _, err := f.Seek(-4, io.SeekCurrent); err != nil {
		return 0, err
	}
	return estimateMP3Duration(f)
}

func findID3v2Tag(f *os.File) (int, error) {
	header := make([]byte, 10)
	if _, err := io.ReadFull(f, header); err != nil {
		return 0, err
	}

	if header[0] != 'I' || header[1] != 'D' || header[2] != '3' {
		return 0, fmt.Errorf("no ID3v2 tag")
	}

	size := int(header[6])<<21 | int(header[7])<<14 | int(header[8])<<7 | int(header[9])
	return 10 + size, nil
}

func parseXingHeader(f *os.File) (float64, error) {
	flags := make([]byte, 4)
	if _, err := io.ReadFull(f, flags); err != nil {
		return 0, err
	}

	hasFrames := flags[0]&0x01 != 0

	if hasFrames {
		frameBytes := make([]byte, 4)
		if _, err := io.ReadFull(f, frameBytes); err != nil {
			return 0, err
		}
		numFrames := binary.BigEndian.Uint32(frameBytes)

		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return 0, err
		}

		frameSize, sampleRate, err := findFirstMP3FrameInfo(f)
		if err != nil {
			return 0, err
		}

		samplesPerFrame := mp3SamplesPerFrame(sampleRate)
		if sampleRate == 0 || samplesPerFrame == 0 {
			return 0, fmt.Errorf("invalid mp3 parameters: rate=%d spf=%d", sampleRate, samplesPerFrame)
		}

		totalSamples := uint64(numFrames) * uint64(samplesPerFrame)
		_ = frameSize
		return float64(totalSamples) / float64(sampleRate), nil
	}

	return 0, fmt.Errorf("xing header without frame count")
}

func parseVBRIHeader(f *os.File) (float64, error) {
	if _, err := f.Seek(10, io.SeekCurrent); err != nil {
		return 0, err
	}

	frameBytes := make([]byte, 4)
	if _, err := io.ReadFull(f, frameBytes); err != nil {
		return 0, err
	}
	numFrames := binary.BigEndian.Uint32(frameBytes)

	if _, err := f.Seek(6, io.SeekCurrent); err != nil {
		return 0, err
	}

	sampleRateBytes := make([]byte, 4)
	if _, err := io.ReadFull(f, sampleRateBytes); err != nil {
		return 0, err
	}
	sampleRate := binary.BigEndian.Uint32(sampleRateBytes)

	if sampleRate == 0 {
		return 0, fmt.Errorf("invalid VBRI sample rate")
	}

	samplesPerFrame := mp3SamplesPerFrame(int(sampleRate))
	return float64(uint64(numFrames)*uint64(samplesPerFrame)) / float64(sampleRate), nil
}

func findFirstMP3FrameInfo(f *os.File) (int, int, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, 0, err
	}

	buf := make([]byte, 4096)
	n, err := f.Read(buf)
	if err != nil {
		return 0, 0, err
	}

	for i := 0; i < n-3; i++ {
		if buf[i] == 0xFF && (buf[i+1]&0xE0) == 0xE0 {
			frameSize, sampleRate, err := parseMPEGFrameHeader(buf[i:])
			if err == nil && frameSize > 0 && sampleRate > 0 {
				return frameSize, sampleRate, nil
			}
		}
	}

	return 0, 0, fmt.Errorf("no valid MPEG frame found")
}

func parseMPEGFrameHeader(data []byte) (frameSize int, sampleRate int, err error) {
	if len(data) < 4 {
		return 0, 0, fmt.Errorf("insufficient data")
	}

	versionBits := (data[1] >> 3) & 0x03
	layerBits := (data[1] >> 1) & 0x03
	bitrateIdx := (data[2] >> 4) & 0x0F
	sampleRateIdx := (data[2] >> 2) & 0x03
	padding := (data[2] >> 1) & 0x01

	if versionBits == 1 || layerBits == 0 || bitrateIdx == 0 || bitrateIdx == 15 || sampleRateIdx == 3 {
		return 0, 0, fmt.Errorf("invalid frame header")
	}

	sr := mpegSampleRate(versionBits, sampleRateIdx)
	br := mpegBitrate(versionBits, layerBits, bitrateIdx)

	var samplesPerFrame int
	switch layerBits {
	case 1:
		samplesPerFrame = 384
	case 2:
		samplesPerFrame = 1152
	default:
		if versionBits == 3 {
			samplesPerFrame = 1152
		} else {
			samplesPerFrame = 576
		}
	}

	_ = samplesPerFrame
	frameSize = (144*br*1000)/sr + int(padding)
	return frameSize, sr, nil
}

func mpegSampleRate(versionBits, idx byte) int {
	switch versionBits {
	case 0:
		if idx > 2 {
			return 0
		}
		return []int{11025, 12000, 8000}[idx]
	case 2:
		if idx > 2 {
			return 0
		}
		return []int{22050, 24000, 16000}[idx]
	case 3:
		if idx > 2 {
			return 0
		}
		return []int{44100, 48000, 32000}[idx]
	default:
		return 0
	}
}

func mpegBitrate(versionBits, layerBits, idx byte) int {
	tables := map[[2]byte][15]int{
		{3, 1}: {0, 32, 64, 96, 128, 160, 192, 224, 256, 288, 320, 352, 384, 416, 448},
		{3, 2}: {0, 32, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384},
		{3, 3}: {0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320},
		{2, 1}: {0, 32, 48, 56, 64, 80, 96, 112, 128, 144, 160, 176, 192, 224, 256},
		{2, 2}: {0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160},
		{2, 3}: {0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160},
	}
	key := [2]byte{versionBits, layerBits}
	if t, ok := tables[key]; ok && idx < 15 {
		return t[idx]
	}
	return 0
}

func mp3SamplesPerFrame(sampleRate int) int {
	if sampleRate >= 32000 {
		return 1152
	}
	return 576
}

func estimateMP3Duration(f *os.File) (float64, error) {
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}

	frameSize, sampleRate, err := findFirstMP3FrameInfo(f)
	if err != nil {
		return 0, err
	}

	if frameSize == 0 || sampleRate == 0 {
		return 0, fmt.Errorf("cannot estimate mp3 duration")
	}

	samplesPerFrame := mp3SamplesPerFrame(sampleRate)
	approxFrames := int(info.Size()) / frameSize
	return float64(approxFrames*samplesPerFrame) / float64(sampleRate), nil
}
