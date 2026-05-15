package api

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/models"
)

var notifyArtPath = filepath.Join(os.TempDir(), "must-notify.jpg")

func SendDesktopNotification(track *models.Track, cfg *config.Config, withImage bool) {
	title := "must"
	var body strings.Builder
	body.WriteString(track.Title)
	body.WriteString("\n")
	body.WriteString(track.Artist)
	if track.Album != "" {
		body.WriteString("\n")
		if track.Year > 0 {
			fmt.Fprintf(&body, "%s (%d)", track.Album, track.Year)
		} else {
			body.WriteString(track.Album)
		}
	}

	switch runtime.GOOS {
	case "linux":
		sendLinuxNotification(title, body.String(), withImage, cfg)
	case "darwin":
		sendMacOSNotification(title, body.String(), withImage)
	}
}

func sendLinuxNotification(title, body string, withImage bool, cfg *config.Config) {
	if _, err := exec.LookPath("notify-send"); err != nil {
		return
	}

	var args []string
	args = append(args, "-t", "5000")
	if withImage {
		if cfg.CopyAlbumArt && cfg.AlbumArtPath != "" {
			if _, err := os.Stat(cfg.AlbumArtPath); err == nil {
				args = append(args, "-i", cfg.AlbumArtPath)
			}
		} else if _, err := os.Stat(notifyArtPath); err == nil {
			args = append(args, "-i", notifyArtPath)
		}
	}
	args = append(args, "--", title, body)

	go func() {
		cmd := exec.Command("notify-send", args...)
		_ = cmd.Run()
	}()
}

func sendMacOSNotification(title, body string, withImage bool) {
	imgArg := ""
	if withImage {
		if _, err := os.Stat(notifyArtPath); err == nil {
			imgArg = fmt.Sprintf(` with image alias POSIX file "%s"`, notifyArtPath)
		}
	}

	escapedTitle := strings.ReplaceAll(title, `"`, `\"`)
	escapedBody := strings.ReplaceAll(body, `"`, `\"`)
	escapedBody = strings.ReplaceAll(escapedBody, "\n", "\\n")

	script := fmt.Sprintf(`display notification "%s" with title "%s"%s`, escapedBody, escapedTitle, imgArg)

	go func() {
		cmd := exec.Command("osascript", "-e", script)
		_ = cmd.Run()
	}()
}

func SaveNotifyArt(imageData []byte) {
	_ = os.WriteFile(notifyArtPath, imageData, 0644)
}
