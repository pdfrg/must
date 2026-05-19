package visualizer

import (
	"fmt"
	"log"
	"math"
)

var logger *log.Logger

func SetLogger(l *log.Logger) {
	logger = l
}

const (
	DefaultBandCount = 10
	DefaultRows      = 5
	maxSilentTicks   = 60
	maxRetries       = 3
)

var barBlocks = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

const (
	ModeBars VisMode = iota
	ModeBraille
	ModeClassicPeak
	ModeWave
	ModeStars
	ModeBrailleBars
	ModeRain
	ModeSegmented
	ModeBinary
	ModeCount
)

type VisMode int

type visEntry struct {
	name string
}

var visModes = [ModeCount]visEntry{
	ModeBars:        {"Bars"},
	ModeBraille:     {"Braille"},
	ModeClassicPeak: {"ClassicPeak"},
	ModeWave:        {"Wave"},
	ModeStars:       {"Stars"},
	ModeBrailleBars: {"BrailleBars"},
	ModeRain:        {"Rain"},
	ModeSegmented:   {"Segmented"},
	ModeBinary:      {"Binary"},
}

func ModeNames() []string {
	names := make([]string, ModeCount)
	for i := range visModes {
		names[i] = visModes[i].name
	}
	return names
}

type Visualizer struct {
	bands          []float64
	prevBands      []float64
	mode           VisMode
	rows           int
	frame          uint64
	refreshPending bool
	seed           uint64
	peakState      *peakState

	colorLow  string
	colorHigh string
	colorDim  string

	audioTap    *AudioTap
	analyzer    *Analyzer
	realAudio   bool
	audioReady  bool
	sampleBuf   []float32
	silentTicks int
	retryCount  int
	retryStatus string
}

func New(seed uint64) *Visualizer {
	v := &Visualizer{
		bands:     make([]float64, DefaultBandCount),
		prevBands: make([]float64, DefaultBandCount),
		rows:      DefaultRows,
		seed:      seed,
		colorLow:  "#89b4fa",
		colorHigh: "#f5c2e7",
		colorDim:  "#6c7086",
	}
	v.initSpectrum()
	return v
}

func (v *Visualizer) SetColors(low, high, dim string) {
	v.colorLow = low
	v.colorHigh = high
	v.colorDim = dim
	v.refreshPending = true
}

func (v *Visualizer) Mode() VisMode { return v.mode }

func (v *Visualizer) ModeName() string {
	if v.mode >= 0 && int(v.mode) < len(visModes) {
		return visModes[v.mode].name
	}
	return "Unknown"
}

func (v *Visualizer) CycleMode() {
	v.mode = (v.mode + 1) % ModeCount
	v.refreshPending = true
}

func (v *Visualizer) CycleModeReverse() {
	v.mode = (v.mode - 1 + ModeCount) % ModeCount
	v.refreshPending = true
}

func (v *Visualizer) RequestRefresh() {
	v.refreshPending = true
}

func (v *Visualizer) ConsumeRefresh() bool {
	if v == nil || !v.refreshPending {
		return false
	}
	v.refreshPending = false
	return true
}

func (v *Visualizer) Bands() []float64 { return v.bands }

func (v *Visualizer) Frame() uint64 { return v.frame }

func (v *Visualizer) Tick(playing bool, paused bool) {
	if v == nil {
		return
	}

	if paused {
		for i := range v.bands {
			v.bands[i] *= 0.85
			v.prevBands[i] = v.bands[i]
		}
		v.refreshPending = true
		return
	}

	v.frame++
	if playing {
		if v.realAudio && v.analyzer != nil && v.audioTap != nil {
			v.updateFromAudio()
		} else {
			v.updateSpectrum()
		}
	}
}

func (v *Visualizer) updateFromAudio() {
	if !v.audioTap.IsAlive() {
		if logger != nil {
			logger.Printf("Visualizer: audio tap process died, reconnecting")
		}
		v.reconnectAudioTap()
		return
	}

	if len(v.sampleBuf) < fftSize {
		v.sampleBuf = make([]float32, fftSize)
	}

	n := v.audioTap.ReadSamples(v.sampleBuf)
	if n < fftSize {
		return
	}

	bands := v.analyzer.Analyze(v.sampleBuf[:n])
	if bands == nil {
		return
	}

	hasEnergy := false
	for _, b := range bands {
		if b > 0.001 {
			hasEnergy = true
			break
		}
	}

	if !hasEnergy {
		v.silentTicks++
		if v.silentTicks > maxSilentTicks {
			if logger != nil {
				logger.Printf("Visualizer: no audio energy for %d ticks, will retry", v.silentTicks)
			}
			v.reconnectAudioTap()
		}
		return
	}

	if !v.audioReady {
		if logger != nil {
			logger.Printf("Visualizer: audio data flowing (audioReady=true)")
		}
		v.audioReady = true
	}
	v.silentTicks = 0
	v.retryCount = 0
	v.retryStatus = ""

	for i := range bandCount {
		v.bands[i] = bands[i]
		v.prevBands[i] = bands[i]
	}
	v.refreshPending = true
}

func (v *Visualizer) reconnectAudioTap() {
	v.retryCount++
	if v.retryCount > maxRetries {
		v.retryStatus = "Audio connection failed. Press 'v' to retry."
		if logger != nil {
			logger.Printf("Visualizer: audio reconnection failed after %d attempts", maxRetries)
		}
		return
	}
	v.silentTicks = 0
	v.retryStatus = fmt.Sprintf("Retrying audio connection (attempt %d)...", v.retryCount)
	if logger != nil {
		logger.Printf("Visualizer: reconnecting audio tap (attempt %d)", v.retryCount)
	}
	if v.audioTap != nil {
		v.audioTap.Close()
	}
	v.audioTap = NewAudioTap()
	if v.audioTap == nil {
		v.retryStatus = "Audio connection failed. Press 'v' to retry."
	}
}

func (v *Visualizer) RetryStatus() string {
	return v.retryStatus
}

func (v *Visualizer) ResetRetry() {
	v.silentTicks = 0
	v.retryCount = 0
	v.retryStatus = ""
	v.audioReady = false
}

func (v *Visualizer) EnableRealAudio(enabled bool) string {
	if v == nil {
		return "Simulated"
	}

	if enabled && v.audioTap == nil {
		v.audioTap = NewAudioTap()
		if v.audioTap != nil {
			v.analyzer = NewAnalyzer()
			v.realAudio = true
			v.audioReady = false
			v.silentTicks = 0
			v.retryCount = 0
			v.retryStatus = ""
			return ActiveBackend()
		}
	}

	if !enabled && v.audioTap != nil {
		v.audioTap.Close()
		v.audioTap = nil
		v.analyzer = nil
		v.realAudio = false
		v.audioReady = false
		v.silentTicks = 0
		v.retryCount = 0
		v.retryStatus = ""
	}

	if v.realAudio {
		return ActiveBackend()
	}
	return "Simulated"
}

func (v *Visualizer) AudioSource() string {
	if v == nil {
		return "Simulated"
	}
	if v.realAudio {
		return ActiveBackend()
	}
	return "Simulated"
}

func (v *Visualizer) AudioReady() bool {
	if v == nil {
		return false
	}
	if !v.realAudio {
		return false
	}
	return v.audioReady
}

func (v *Visualizer) AvailableSamples() uint64 {
	if v == nil || v.audioTap == nil {
		return 0
	}
	return v.audioTap.AvailableSamples()
}

func (v *Visualizer) Close() {
	if v == nil {
		return
	}
	if v.audioTap != nil {
		v.audioTap.Close()
		v.audioTap = nil
		v.analyzer = nil
		v.realAudio = false
		v.silentTicks = 0
		v.retryCount = 0
		v.retryStatus = ""
	}
}

func (v *Visualizer) Render(width int) string {
	if v == nil {
		return ""
	}
	switch v.mode {
	case ModeBars:
		return v.renderBars(width)
	case ModeBraille:
		return v.renderBarsDot(width)
	case ModeClassicPeak:
		return v.renderClassicPeak(width)
	case ModeWave:
		return v.renderWave(width)
	case ModeStars:
		return v.renderStars(width)
	case ModeBrailleBars:
		return v.renderBrailleBars(width)
	case ModeRain:
		return v.renderRain(width)
	case ModeSegmented:
		return v.renderSegmented(width)
	case ModeBinary:
		return v.renderBinary(width)
	default:
		return v.renderBars(width)
	}
}

func (v *Visualizer) SetSeed(seed uint64) {
	v.seed = seed
	v.peakState = nil
	v.initSpectrum()
	v.refreshPending = true
}

func (v *Visualizer) SetRows(rows int) {
	if rows > 0 {
		v.rows = rows
	}
}

func (v *Visualizer) SetMode(mode VisMode) {
	if mode >= 0 && mode < ModeCount {
		v.mode = mode
		v.refreshPending = true
	}
}

func (v *Visualizer) initSpectrum() {
	seed := v.seed
	for i := range v.bands {
		seed = seed*6364136223846793005 + 1442695040888963407
		val := float64(seed>>33) / float64(1<<31)
		v.bands[i] = val*0.6 + 0.1
		v.prevBands[i] = v.bands[i]
	}
}

func (v *Visualizer) updateSpectrum() {
	for i := range v.bands {
		seed := v.seed + uint64(i)*104729 + uint64(v.frame)*3571
		seed = seed*6364136223846793005 + 1442695040888963407
		raw := float64(seed>>33) / float64(1<<31)

		target := math.Sin(raw*math.Pi*2+float64(i)*0.7)*0.3 + 0.5
		target = math.Max(0, math.Min(1, target))

		if target > v.bands[i] {
			v.bands[i] = target*0.6 + v.prevBands[i]*0.4
		} else {
			v.bands[i] = target*0.25 + v.prevBands[i]*0.75
		}
		v.prevBands[i] = v.bands[i]
	}
	v.refreshPending = true
}

func ModeFromString(name string) VisMode {
	for i, entry := range visModes {
		if entry.name == name {
			return VisMode(i)
		}
	}
	return ModeBars
}
