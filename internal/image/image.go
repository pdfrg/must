package image

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	termimg "github.com/blacktop/go-termimg"
	"github.com/dhowden/tag"
	"github.com/pdfrg/must/internal/config"
)

type Renderer struct {
	protocol termimg.Protocol
	artCache string
}

func NewRenderer() *Renderer {
	return &Renderer{
		protocol: termimg.DetectProtocol(),
		artCache: config.GetArtCacheDir(),
	}
}

func (r *Renderer) Protocol() string {
	return r.protocol.String()
}

func (r *Renderer) GetArtForTrack(trackPath string) (image.Image, error) {
	img, err := r.getEmbeddedArt(trackPath)
	if err == nil && img != nil {
		return img, nil
	}

	dir := filepath.Dir(trackPath)
	img, err = r.getLocalArt(dir)
	if err == nil && img != nil {
		return img, nil
	}

	parentDir := filepath.Dir(dir)
	img, err = r.getLocalArt(parentDir)
	if err == nil && img != nil {
		return img, nil
	}

	return nil, fmt.Errorf("no album art found for %s", trackPath)
}

func (r *Renderer) getEmbeddedArt(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	tags, err := tag.ReadFrom(f)
	if err != nil {
		return nil, err
	}

	picture := tags.Picture()
	if picture == nil || len(picture.Data) == 0 {
		return nil, fmt.Errorf("no embedded art")
	}

	img, _, err := image.Decode(bytes.NewReader(picture.Data))
	if err != nil {
		return nil, err
	}

	return img, nil
}

func (r *Renderer) getLocalArt(dir string) (image.Image, error) {
	coverNames := []string{
		"cover.jpg", "cover.jpeg", "cover.png",
		"folder.jpg", "folder.jpeg", "folder.png",
		"front.jpg", "front.jpeg", "front.png",
		"artwork.jpg", "artwork.jpeg", "artwork.png",
	}

	for _, name := range coverNames {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			img, err := termimg.Open(path)
			if err == nil && img != nil {
				return img.Source, nil
			}
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		lower := strings.ToLower(entry.Name())
		if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".png") {
			path := filepath.Join(dir, entry.Name())
			f, err := os.Open(path)
			if err != nil {
				continue
			}
			img, _, decodeErr := image.Decode(f)
			_ = f.Close()
			if decodeErr == nil {
				return img, nil
			}
		}
	}

	return nil, fmt.Errorf("no local art in %s", dir)
}

func (r *Renderer) RenderImage(img image.Image, width, height int) (string, error) {
	if img == nil {
		return "", fmt.Errorf("nil image")
	}

	ti := termimg.New(img)
	ti.Width(width)
	ti.Height(height)
	ti.Scale(termimg.ScaleFit)

	rendered, err := ti.Render()
	if err != nil {
		return "", err
	}

	return rendered, nil
}

func (r *Renderer) ClearArt() string {
	return termimg.ClearAllString()
}

func (r *Renderer) CacheArt(trackPath string, data []byte) error {
	if r.artCache == "" {
		return nil
	}

	if err := os.MkdirAll(r.artCache, 0755); err != nil {
		return err
	}

	key := cacheKey(trackPath)
	cachePath := filepath.Join(r.artCache, key+".jpg")

	f, err := os.Create(cachePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = io.Copy(f, bytes.NewReader(data))
	return err
}

func (r *Renderer) GetCachedArt(trackPath string) (image.Image, error) {
	key := cacheKey(trackPath)
	cachePath := filepath.Join(r.artCache, key+".jpg")

	f, err := os.Open(cachePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	img, _, err := image.Decode(f)
	return img, err
}

func cacheKey(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func LoadImageFromPath(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	img, _, err := image.Decode(f)
	return img, err
}

func LoadImageFromBytes(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}
