// Package thumb builds cached, correctly rotated JPEG thumbnails for photos
// (from the JPG or from the preview embedded in the RAW) and poster frames
// for videos.
package thumb

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"rawsivr/internal/exif"
	"rawsivr/internal/proc"
)

type Cache struct {
	Dir string
	sem chan struct{}
	mu  sync.Mutex
	run map[string]*sync.Mutex // one build per output file at a time
}

func New(dir string) *Cache {
	n := runtime.NumCPU() - 2
	if n < 2 {
		n = 2
	}
	return &Cache{Dir: dir, sem: make(chan struct{}, n), run: map[string]*sync.Mutex{}}
}

// Get returns the path of a cached thumbnail whose longest side is at most
// size pixels. Sources are tried in order; the first that works wins.
func (c *Cache) Get(size int, sources ...string) (string, error) {
	var lastErr error
	for _, src := range sources {
		if src == "" {
			continue
		}
		p, err := c.one(size, src)
		if err == nil {
			return p, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no source")
	}
	return "", lastErr
}

func (c *Cache) one(size int, src string) (string, error) {
	info, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	sum := sha1.Sum([]byte(fmt.Sprintf("%s|%d|%d|%d", src, info.Size(), info.ModTime().UnixNano(), size)))
	out := filepath.Join(c.Dir, hex.EncodeToString(sum[:])+".jpg")

	c.mu.Lock()
	lock := c.run[out]
	if lock == nil {
		lock = &sync.Mutex{}
		c.run[out] = lock
	}
	c.mu.Unlock()
	lock.Lock()
	defer lock.Unlock()

	if _, err := os.Stat(out); err == nil {
		return out, nil
	}
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return "", err
	}
	c.sem <- struct{}{}
	defer func() { <-c.sem }()

	tmp := out + ".tmp"
	switch strings.ToLower(filepath.Ext(src)) {
	case ".mp4", ".mov", ".mxf":
		err = poster(src, tmp, size)
	case ".jpg", ".jpeg":
		err = fromJPEG(src, tmp, size)
	default:
		err = fromRAW(src, tmp, size)
	}
	if err != nil {
		os.Remove(tmp)
		return "", err
	}
	return out, os.Rename(tmp, out)
}

func fromJPEG(src, out string, size int) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	orient := exif.JPEGOrientation(f)
	img, err := jpeg.Decode(f)
	if err != nil {
		return err
	}
	return save(img, orient, size, out)
}

func fromRAW(src, out string, size int) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	data, orient, err := exif.RawPreview(f)
	if err != nil {
		return err
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}
	return save(img, orient, size, out)
}

func save(img image.Image, orient, size int, out string) error {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if long := max(w, h); long > size {
		w, h = max(1, w*size/long), max(1, h*size/long)
	}
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	dst := rotate(shrink(rgba, w, h), orient)
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	if err := jpeg.Encode(f, dst, &jpeg.Options{Quality: 85}); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// shrink downscales by averaging the block of source pixels behind each
// output pixel, which is all a thumbnail needs.
func shrink(src *image.RGBA, w, h int) *image.RGBA {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if w >= sw && h >= sh {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		y0, y1 := y*sh/h, max((y+1)*sh/h, y*sh/h+1)
		for x := 0; x < w; x++ {
			x0, x1 := x*sw/w, max((x+1)*sw/w, x*sw/w+1)
			var r, g, b uint32
			for sy := y0; sy < y1; sy++ {
				p := src.Pix[sy*src.Stride+x0*4 : sy*src.Stride+x1*4]
				for i := 0; i < len(p); i += 4 {
					r, g, b = r+uint32(p[i]), g+uint32(p[i+1]), b+uint32(p[i+2])
				}
			}
			n := uint32((x1 - x0) * (y1 - y0))
			o := dst.Pix[y*dst.Stride+x*4:]
			o[0], o[1], o[2], o[3] = uint8(r/n), uint8(g/n), uint8(b/n), 255
		}
	}
	return dst
}

// rotate applies EXIF orientations 3, 6 and 8 (the ones cameras write).
func rotate(src *image.RGBA, orient int) *image.RGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	var dst *image.RGBA
	switch orient {
	case 3:
		dst = image.NewRGBA(image.Rect(0, 0, w, h))
	case 6, 8:
		dst = image.NewRGBA(image.Rect(0, 0, h, w))
	default:
		return src
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch orient {
			case 3:
				dx, dy = w-1-x, h-1-y
			case 6: // rotate 90 clockwise
				dx, dy = h-1-y, x
			case 8: // rotate 90 counter-clockwise
				dx, dy = y, w-1-x
			}
			dst.SetRGBA(dx, dy, src.RGBAAt(x, y))
		}
	}
	return dst
}

func poster(src, out string, size int) error {
	cmd := exec.Command("ffmpeg", "-hide_banner", "-v", "error", "-y", "-ss", "0.5", "-i", src,
		"-frames:v", "1", "-vf", fmt.Sprintf("scale=%d:-2:flags=fast_bilinear", size), "-f", "image2", "-c:v", "mjpeg", out)
	proc.Hide(cmd)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg poster: %v: %s", err, bytes.TrimSpace(b))
	}
	return nil
}
