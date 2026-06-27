package mpv

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/models"
)

var logger *log.Logger

func SetLogger(l *log.Logger) {
	logger = l
}

type MPVBackend struct {
	mu             sync.Mutex
	process        *exec.Cmd
	processExited  chan struct{}
	currentPaths   []string
	isPaused       bool
	pauseStartTime time.Time
	lastPos        PlaybackPosition
	socketPath     string
	socketTimeout  time.Duration
	pulseServer    string
	replayGainMode string
}

type PlaybackPosition struct {
	TimePos    float64
	PercentPos float64
}

type IPCCommand struct {
	Command []any `json:"command"`
}

type IPCResponse struct {
	Error string `json:"error"`
	Data  any    `json:"data"`
}

func NewMPVBackend() *MPVBackend {
	return &MPVBackend{
		socketPath:    config.GetMPVSocketPath(),
		socketTimeout: 2 * time.Second,
	}
}

func (m *MPVBackend) SetPulseServer(server string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pulseServer = server
}

func (m *MPVBackend) checkPulseServerReachable() error {
	addr := m.pulseServer
	if !strings.HasPrefix(addr, "tcp:") {
		return fmt.Errorf("unsupported PULSE_SERVER format: %s", addr)
	}
	hostPort := strings.TrimPrefix(addr, "tcp:")
	conn, err := net.DialTimeout("tcp", hostPort, 2*time.Second)
	if err != nil {
		return fmt.Errorf("cannot reach PULSE_SERVER %s: %w", addr, err)
	}
	_ = conn.Close()
	return nil
}

func (m *MPVBackend) Start(paths []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_ = m.stopLocked()

	if runtime.GOOS != "windows" {
		socketDir := filepath.Dir(m.socketPath)
		if err := os.MkdirAll(socketDir, 0700); err != nil {
			return fmt.Errorf("failed to create socket directory: %w", err)
		}
		_ = os.Remove(m.socketPath)
	}

	if m.pulseServer != "" {
		if err := m.checkPulseServerReachable(); err != nil {
			if logger != nil {
				logger.Printf("Warning: PULSE_SERVER not reachable: %v", err)
			}
		}
	}

	args := []string{
		"--no-video",
		"--force-window=no",
		"--no-terminal",
		"--gapless-audio=yes",
		fmt.Sprintf("--input-ipc-server=%s", m.socketPath),
	}
	if m.pulseServer != "" {
		args = append(args, "--ao=pulse")
	}
	if m.replayGainMode != "" && m.replayGainMode != "off" {
		args = append(args, fmt.Sprintf("--replaygain=%s", m.replayGainMode))
	}
	args = append(args, paths...)

	if logger != nil {
		logger.Printf("MPV Start: socket=%s, paths=%d", m.socketPath, len(paths))
	}

	binaryName := "mpv"
	if runtime.GOOS == "windows" {
		binaryName = "mpv.exe"
	}
	m.process = exec.Command(binaryName, args...)
	m.process.Stdout = nil

	if m.pulseServer != "" {
		m.process.Env = append(os.Environ(), "PULSE_SERVER="+m.pulseServer)
	}

	stderrPipe, err := m.process.StderrPipe()
	if err != nil && logger != nil {
		logger.Printf("Failed to get stderr pipe: %v", err)
	}

	if err := m.process.Start(); err != nil {
		return fmt.Errorf("failed to start MPV: %w", err)
	}

	if logger != nil {
		logger.Printf("MPV started with PID %d", m.process.Process.Pid)
	}

	if stderrPipe != nil {
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := stderrPipe.Read(buf)
				if n > 0 && logger != nil {
					logger.Printf("MPV stderr: %s", string(buf[:n]))
				}
				if err != nil {
					break
				}
			}
		}()
	}

	m.currentPaths = paths
	m.isPaused = false
	m.pauseStartTime = time.Time{}

	m.processExited = make(chan struct{})
	go func() {
		_ = m.process.Wait()
		close(m.processExited)
	}()

	time.Sleep(200 * time.Millisecond)

	if runtime.GOOS != "windows" {
		if _, err := os.Stat(m.socketPath); os.IsNotExist(err) {
			if logger != nil {
				logger.Printf("WARNING: MPV socket not created at %s", m.socketPath)
			}
		}
	}

	return nil
}

func (m *MPVBackend) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopLocked()
}

func (m *MPVBackend) stopLocked() error {
	if m.process != nil {
		_ = m.process.Process.Kill()
		if m.processExited != nil {
			<-m.processExited
		}
		m.process = nil
	}
	m.currentPaths = nil
	m.isPaused = false
	m.pauseStartTime = time.Time{}
	if runtime.GOOS != "windows" {
		_ = os.Remove(m.socketPath)
	}
	return nil
}

func (m *MPVBackend) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil || m.process.Process == nil {
		return false
	}
	return m.process.ProcessState == nil || !m.process.ProcessState.Exited()
}

func (m *MPVBackend) IsPaused() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isPaused
}

func (m *MPVBackend) QueryPauseState() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	cmd := IPCCommand{Command: []any{"get_property", "pause"}}
	resp, err := m.sendIPCCommandLocked(cmd)
	if err != nil {
		return m.isPaused
	}
	if resp.Data != nil {
		if paused, ok := resp.Data.(bool); ok {
			m.isPaused = paused
		}
	}
	return m.isPaused
}

func (m *MPVBackend) IsPlaying() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil || m.process.Process == nil {
		return false
	}
	if m.process.ProcessState != nil && m.process.ProcessState.Exited() {
		return false
	}
	return !m.isPaused
}

func (m *MPVBackend) TogglePause() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil {
		return fmt.Errorf("MPV not running")
	}

	newPause := !m.isPaused
	cmd := IPCCommand{Command: []any{"set_property", "pause", newPause}}
	resp, err := m.sendIPCCommandLocked(cmd)
	if err != nil {
		if m.reconnectLocked() {
			resp, err = m.sendIPCCommandLocked(cmd)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	if resp != nil {
		m.isPaused = newPause
		if m.isPaused {
			m.pauseStartTime = time.Now()
		} else {
			m.pauseStartTime = time.Time{}
		}
	}
	return nil
}

func (m *MPVBackend) Pause(pause bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil {
		return fmt.Errorf("MPV not running")
	}

	cmd := IPCCommand{Command: []any{"set_property", "pause", pause}}
	resp, err := m.sendIPCCommandLocked(cmd)
	if err != nil {
		return err
	}
	if resp != nil {
		m.isPaused = pause
		if m.isPaused {
			m.pauseStartTime = time.Now()
		} else {
			m.pauseStartTime = time.Time{}
		}
	}
	return nil
}

func (m *MPVBackend) SkipNext() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil {
		return fmt.Errorf("MPV not running")
	}
	_, err := m.sendIPCCommandLocked(IPCCommand{Command: []any{"playlist-next"}})
	return err
}

func (m *MPVBackend) SkipPrev() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil {
		return fmt.Errorf("MPV not running")
	}
	_, err := m.sendIPCCommandLocked(IPCCommand{Command: []any{"playlist-prev"}})
	return err
}

func (m *MPVBackend) PlaylistPlayIndex(index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil {
		return fmt.Errorf("MPV not running")
	}
	_, err := m.sendIPCCommandLocked(IPCCommand{Command: []any{"set_property", "playlist-pos", index}})
	if err != nil && m.reconnectLocked() {
		_, err = m.sendIPCCommandLocked(IPCCommand{Command: []any{"set_property", "playlist-pos", index}})
	}
	return err
}

func (m *MPVBackend) RemoveFromPlaylist(index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil {
		return fmt.Errorf("MPV not running")
	}
	_, err := m.sendIPCCommandLocked(IPCCommand{Command: []any{"playlist-remove", index}})
	return err
}

func (m *MPVBackend) GetPlaylistCount() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil {
		return 0, fmt.Errorf("MPV not running")
	}
	resp, err := m.sendIPCCommandLocked(IPCCommand{Command: []any{"get_property", "playlist-count"}})
	if err != nil {
		return 0, err
	}
	if resp == nil || resp.Data == nil {
		return 0, nil
	}
	if count, ok := resp.Data.(float64); ok {
		return int(count), nil
	}
	return 0, nil
}

func (m *MPVBackend) SeekRelative(delta float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil {
		return fmt.Errorf("MPV not running")
	}
	_, err := m.sendIPCCommandLocked(IPCCommand{Command: []any{"seek", delta, "relative"}})
	if err != nil && m.reconnectLocked() {
		_, err = m.sendIPCCommandLocked(IPCCommand{Command: []any{"seek", delta, "relative"}})
	}
	return err
}

func (m *MPVBackend) SeekAbsolute(pos float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil {
		return fmt.Errorf("MPV not running")
	}
	_, err := m.sendIPCCommandLocked(IPCCommand{Command: []any{"seek", pos, "absolute"}})
	if err != nil && m.reconnectLocked() {
		_, err = m.sendIPCCommandLocked(IPCCommand{Command: []any{"seek", pos, "absolute"}})
	}
	return err
}

func (m *MPVBackend) SetVolume(vol float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil {
		return fmt.Errorf("MPV not running")
	}
	_, err := m.sendIPCCommandLocked(IPCCommand{Command: []any{"set_property", "volume", vol}})
	return err
}

func (m *MPVBackend) SetMute(muted bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil {
		return fmt.Errorf("MPV not running")
	}
	_, err := m.sendIPCCommandLocked(IPCCommand{Command: []any{"set_property", "mute", muted}})
	return err
}

func (m *MPVBackend) SetReplayGainMode(mode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.replayGainMode = mode
	if m.process == nil {
		return nil
	}
	_, err := m.sendIPCCommandLocked(IPCCommand{Command: []any{"set_property", "replaygain", mode}})
	return err
}

func (m *MPVBackend) GetPlaybackPosition() (PlaybackPosition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil {
		return PlaybackPosition{}, fmt.Errorf("MPV not running")
	}
	if m.isPaused {
		return m.lastPos, nil
	}

	timeResp, err := m.sendIPCCommandLocked(IPCCommand{Command: []any{"get_property", "time-pos"}})
	if err != nil {
		if m.reconnectLocked() {
			timeResp, err = m.sendIPCCommandLocked(IPCCommand{Command: []any{"get_property", "time-pos"}})
		}
		if err != nil {
			return m.lastPos, err
		}
	}
	timePos := 0.0
	if timeResp != nil && timeResp.Data != nil {
		if t, ok := timeResp.Data.(float64); ok {
			timePos = t
		}
	}

	percentResp, err := m.sendIPCCommandLocked(IPCCommand{Command: []any{"get_property", "percent-pos"}})
	if err != nil {
		return PlaybackPosition{TimePos: timePos, PercentPos: 0}, nil
	}
	percentPos := 0.0
	if percentResp != nil && percentResp.Data != nil {
		if p, ok := percentResp.Data.(float64); ok {
			percentPos = p
		}
	}

	pos := PlaybackPosition{TimePos: timePos, PercentPos: percentPos}
	m.lastPos = pos
	return pos, nil
}

func (m *MPVBackend) GetPlaylistPosition() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil {
		return -1, fmt.Errorf("MPV not running")
	}

	resp, err := m.sendIPCCommandLocked(IPCCommand{Command: []any{"get_property", "playlist-pos"}})
	if err != nil {
		return -1, err
	}
	if resp == nil || resp.Data == nil {
		return -1, nil
	}
	if pos, ok := resp.Data.(float64); ok {
		return int(pos), nil
	}
	return -1, nil
}

func (m *MPVBackend) AppendToPlaylist(paths []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil {
		return fmt.Errorf("MPV not running")
	}
	for _, p := range paths {
		cmd := IPCCommand{Command: []any{"loadfile", p, "append"}}
		if _, err := m.sendIPCCommandLocked(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (m *MPVBackend) InsertInPlaylist(paths []string, afterIndex int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil {
		return fmt.Errorf("MPV not running")
	}
	insertAt := afterIndex + 1
	for i, p := range paths {
		cmd := IPCCommand{Command: []any{"loadfile", p, "insert-at", insertAt + i}}
		if _, err := m.sendIPCCommandLocked(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (m *MPVBackend) PlaylistMove(fromIndex, toIndex int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil {
		return fmt.Errorf("MPV not running")
	}
	cmd := IPCCommand{Command: []any{"playlist-move", fromIndex, toIndex}}
	_, err := m.sendIPCCommandLocked(cmd)
	return err
}

func (m *MPVBackend) GetAudioInfo() (*models.AudioInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil {
		return nil, fmt.Errorf("MPV not running")
	}

	info := &models.AudioInfo{}

	codecResp, err := m.sendIPCCommandLocked(IPCCommand{Command: []any{"get_property", "audio-codec-name"}})
	if err == nil && codecResp != nil && codecResp.Data != nil {
		if s, ok := codecResp.Data.(string); ok {
			info.Codec = s
		}
	}

	bitrateResp, err := m.sendIPCCommandLocked(IPCCommand{Command: []any{"get_property", "audio-bitrate"}})
	if err == nil && bitrateResp.Data != nil {
		if f, ok := bitrateResp.Data.(float64); ok {
			info.Bitrate = f / 1000.0
		}
	}

	sampleResp, err := m.sendIPCCommandLocked(IPCCommand{Command: []any{"get_property", "audio-params/samplerate"}})
	if err == nil && sampleResp.Data != nil {
		if f, ok := sampleResp.Data.(float64); ok {
			info.SampleRate = int(f)
		}
	}

	channelsResp, err := m.sendIPCCommandLocked(IPCCommand{Command: []any{"get_property", "audio-params/channel-count"}})
	if err == nil && channelsResp.Data != nil {
		if f, ok := channelsResp.Data.(float64); ok {
			info.Channels = int(f)
		}
	}

	bitsResp, err := m.sendIPCCommandLocked(IPCCommand{Command: []any{"get_property", "audio-bitdepth"}})
	if err == nil && bitsResp.Data != nil {
		if f, ok := bitsResp.Data.(float64); ok {
			info.BitDepth = int(f)
		}
	}

	return info, nil
}

func (m *MPVBackend) Restart() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.currentPaths == nil {
		return fmt.Errorf("no paths to restart with")
	}
	paths := make([]string, len(m.currentPaths))
	copy(paths, m.currentPaths)
	_ = m.stopLocked()
	return m.Start(paths)
}

func (m *MPVBackend) GetCurrentPaths() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.currentPaths == nil {
		return nil
	}
	paths := make([]string, len(m.currentPaths))
	copy(paths, m.currentPaths)
	return paths
}

func (m *MPVBackend) GetSocketPath() string {
	return m.socketPath
}

func (m *MPVBackend) sendIPCCommandLocked(cmd IPCCommand) (*IPCResponse, error) {
	conn, err := dialMPV(m.socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MPV socket: %w", err)
	}
	defer func() { _ = conn.Close() }()

	_ = conn.SetReadDeadline(time.Now().Add(m.socketTimeout))
	_ = conn.SetWriteDeadline(time.Now().Add(m.socketTimeout))

	cmdData, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}
	cmdData = append(cmdData, '\n')

	if _, err := conn.Write(cmdData); err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response IPCResponse
	if err := json.Unmarshal(line, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if response.Error != "" && response.Error != "success" {
		return &response, fmt.Errorf("MPV error: %s", response.Error)
	}

	return &response, nil
}

func (m *MPVBackend) reconnectLocked() bool {
	cmd := IPCCommand{Command: []any{"get_property", "pause"}}
	for i := 0; i < 3; i++ {
		resp, err := m.sendIPCCommandLocked(cmd)
		if err == nil && resp != nil && resp.Error == "success" {
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}
