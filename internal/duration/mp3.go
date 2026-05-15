package duration

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

func mp3Duration(f *os.File) (float64, error) {
	audioOffset := 0
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	if offset, err := findID3v2Tag(f); err == nil && offset > 0 {
		audioOffset = offset
	}

	if _, err := f.Seek(int64(audioOffset), io.SeekStart); err != nil {
		return 0, err
	}

	frameHeader := make([]byte, 4)
	if _, err := io.ReadFull(f, frameHeader); err != nil {
		return 0, fmt.Errorf("reading mp3 header: %w", err)
	}

	if frameHeader[0] != 0xFF || (frameHeader[1]&0xE0) != 0xE0 {
		return 0, fmt.Errorf("no valid MPEG frame at audio offset %d", audioOffset)
	}

	frameSize, sampleRate, err := parseMPEGFrameHeader(frameHeader)
	if err != nil || frameSize == 0 || sampleRate == 0 {
		return 0, fmt.Errorf("invalid MPEG frame header: %w", err)
	}

	versionBits := (frameHeader[1] >> 3) & 0x03
	layerBits := (frameHeader[1] >> 1) & 0x03
	layer := mpegLayer(layerBits)
	channelMode := frameHeader[3] >> 6
	sideInfoSize := 32
	if versionBits == 3 {
		if channelMode == 3 {
			sideInfoSize = 17
		}
	} else {
		if channelMode == 3 {
			sideInfoSize = 9
		} else {
			sideInfoSize = 17
		}
	}
	xingOffset := 4 + sideInfoSize

	if _, err := f.Seek(int64(audioOffset+xingOffset), io.SeekStart); err != nil {
		return 0, err
	}

	markerBuf := make([]byte, 4)
	if _, err := io.ReadFull(f, markerBuf); err != nil {
		return 0, fmt.Errorf("reading xing marker: %w", err)
	}

	marker := string(markerBuf)
	if marker == "Xing" || marker == "Info" {
		return parseXingHeader(f, int(versionBits), int(layer), sampleRate)
	}

	if marker == "VBRI" {
		return parseVBRIHeader(f, int(versionBits), int(layer), sampleRate)
	}

	return estimateMP3Duration(f, int64(audioOffset), frameSize, int(versionBits), int(layer), sampleRate)
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

func parseXingHeader(f *os.File, versionBits, layer, sampleRate int) (float64, error) {
	flagsBuf := make([]byte, 4)
	if _, err := io.ReadFull(f, flagsBuf); err != nil {
		return 0, err
	}
	flags := binary.BigEndian.Uint32(flagsBuf)

	hasFrames := flags&0x01 != 0

	if hasFrames {
		frameBytes := make([]byte, 4)
		if _, err := io.ReadFull(f, frameBytes); err != nil {
			return 0, err
		}
		numFrames := binary.BigEndian.Uint32(frameBytes)

		samplesPerFrame := mp3SamplesPerFrame(byte(versionBits), byte(layer))
		if sampleRate == 0 || samplesPerFrame == 0 {
			return 0, fmt.Errorf("invalid mp3 parameters: rate=%d spf=%d", sampleRate, samplesPerFrame)
		}

		totalSamples := uint64(numFrames) * uint64(samplesPerFrame)
		return float64(totalSamples) / float64(sampleRate), nil
	}

	return 0, fmt.Errorf("xing header without frame count")
}

func parseVBRIHeader(f *os.File, versionBits, layer, sampleRate int) (float64, error) {
	if _, err := f.Seek(6, io.SeekCurrent); err != nil {
		return 0, err
	}

	frameBytes := make([]byte, 4)
	if _, err := io.ReadFull(f, frameBytes); err != nil {
		return 0, err
	}
	numFrames := binary.BigEndian.Uint32(frameBytes)

	if numFrames == 0 {
		return 0, fmt.Errorf("vbri header with zero frames")
	}

	samplesPerFrame := mp3SamplesPerFrame(byte(versionBits), byte(layer))
	return float64(uint64(numFrames)*uint64(samplesPerFrame)) / float64(sampleRate), nil
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
	layer := mpegLayer(layerBits)
	br := mpegBitrate(versionBits, layer, bitrateIdx)

	if sr == 0 || br == 0 {
		return 0, 0, fmt.Errorf("invalid sample rate or bitrate")
	}

	if layer == 1 {
		frameSize = (12*br*1000)/sr + 4*int(padding)
	} else {
		frameSize = (144*br*1000)/sr + int(padding)
	}
	return frameSize, sr, nil
}

func mpegLayer(layerBits byte) byte {
	switch layerBits {
	case 1:
		return 3
	case 2:
		return 2
	case 3:
		return 1
	default:
		return 0
	}
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

func mp3SamplesPerFrame(versionBits, layer byte) int {
	switch layer {
	case 1:
		return 384
	case 2:
		return 1152
	case 3:
		if versionBits == 3 {
			return 1152
		}
		return 576
	default:
		return 0
	}
}

func estimateMP3Duration(f *os.File, audioOffset int64, firstFrameSize int, versionBits, layer, sampleRate int) (float64, error) {
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}

	if firstFrameSize == 0 || sampleRate == 0 {
		return 0, fmt.Errorf("cannot estimate mp3 duration")
	}

	audioSize := info.Size() - audioOffset
	samplesPerFrame := mp3SamplesPerFrame(byte(versionBits), byte(layer))

	avgFrameSize, numScanned, err := scanAverageFrameSize(f, audioOffset, info.Size())
	if err != nil || numScanned == 0 {
		approxFrames := int(audioSize) / firstFrameSize
		return float64(approxFrames*samplesPerFrame) / float64(sampleRate), nil
	}

	approxFrames := float64(audioSize) / avgFrameSize
	return approxFrames * float64(samplesPerFrame) / float64(sampleRate), nil
}

func scanAverageFrameSize(f *os.File, audioOffset int64, fileSize int64) (float64, int, error) {
	const maxScanFrames = 2000
	curOffset := audioOffset
	totalSize := 0
	numFrames := 0

	for i := 0; i < maxScanFrames && curOffset < fileSize-4; i++ {
		if _, err := f.Seek(curOffset, io.SeekStart); err != nil {
			break
		}

		header := make([]byte, 4)
		if _, err := io.ReadFull(f, header); err != nil {
			break
		}

		if header[0] != 0xFF || (header[1]&0xE0) != 0xE0 {
			break
		}

		versionBits := (header[1] >> 3) & 0x03
		layerBits := (header[1] >> 1) & 0x03
		bitrateIdx := (header[2] >> 4) & 0x0F
		sampleRateIdx := (header[2] >> 2) & 0x03
		padding := (header[2] >> 1) & 0x01

		if versionBits == 1 || layerBits == 0 || bitrateIdx == 0 || bitrateIdx == 15 || sampleRateIdx == 3 {
			break
		}

		sr := mpegSampleRate(versionBits, sampleRateIdx)
		layer := mpegLayer(layerBits)
		br := mpegBitrate(versionBits, layer, bitrateIdx)
		if sr == 0 || br == 0 {
			break
		}

		var frameSize int
		if layer == 1 {
			frameSize = (12*br*1000)/sr + 4*int(padding)
		} else {
			frameSize = (144*br*1000)/sr + int(padding)
		}
		if frameSize == 0 {
			break
		}

		totalSize += frameSize
		numFrames++
		curOffset += int64(frameSize)
	}

	if numFrames == 0 {
		return 0, 0, fmt.Errorf("no frames scanned")
	}

	return float64(totalSize) / float64(numFrames), numFrames, nil
}
