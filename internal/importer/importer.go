// Package importer copies a shoot from a memory card (or any folder) into
// a dated folder in the photo library. It never changes the source.
package importer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rawsivr/internal/execute"
	"rawsivr/internal/scan"
)

// VideoDir is the subfolder of the shoot folder that clips are copied to.
const VideoDir = "video"

// Source describes what an import would bring in.
type Source struct {
	Path   string `json:"path"`
	Label  string `json:"label"`
	Photos int    `json:"photos"`
	Videos int    `json:"videos"`
	Files  int    `json:"files"`
	Bytes  int64  `json:"bytes"`
	Date   string `json:"date"` // day of the oldest file, YYYY-MM-DD

	files []file
}

type file struct {
	src   string
	video bool
	size  int64
}

// Folders on a Sony card that hold thumbnails and low-resolution proxies of
// the real clips; they are not worth importing.
func skipped(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		switch strings.ToUpper(part) {
		case "THMBNL", "SUB":
			return true
		}
	}
	return false
}

// Inspect lists the photos and videos under a folder.
func Inspect(path string) (Source, error) {
	res, err := scan.Scan(path, scan.DefaultLayout)
	if err != nil {
		return Source{}, err
	}
	s := Source{Path: res.Root, Label: filepath.Base(res.Root)}
	var oldest time.Time
	for _, it := range res.Items {
		if rel, _ := filepath.Rel(res.Root, it.Files()[0]); skipped(rel) {
			continue
		}
		if it.Kind == scan.Video {
			s.Videos++
		} else {
			s.Photos++
		}
		for _, f := range it.Files() {
			info, err := os.Stat(f)
			if err != nil {
				return Source{}, err
			}
			s.files = append(s.files, file{src: f, video: it.Kind == scan.Video, size: info.Size()})
			s.Bytes += info.Size()
			if oldest.IsZero() || info.ModTime().Before(oldest) {
				oldest = info.ModTime()
			}
		}
	}
	s.Files = len(s.files)
	if oldest.IsZero() {
		oldest = time.Now()
	}
	s.Date = oldest.Format("2006-01-02")
	return s, nil
}

// FolderName builds "2026-11-16_Name" (or just the date when name is empty).
func FolderName(date, name string) (string, error) {
	name = strings.TrimSpace(name)
	if strings.ContainsAny(name, `\/:*?"<>|`) || strings.HasSuffix(name, ".") {
		return "", fmt.Errorf("the name cannot contain \\ / : * ? \" < > | or end with a dot")
	}
	if name == "" {
		return date, nil
	}
	return date + "_" + strings.ReplaceAll(name, " ", "_"), nil
}

type Progress struct {
	Done       int    `json:"done"`
	Total      int    `json:"total"`
	BytesDone  int64  `json:"bytesDone"`
	BytesTotal int64  `json:"bytesTotal"`
	File       string `json:"file"`
}

type Result struct {
	Dest      string   `json:"dest"`
	Copied    int      `json:"copied"`
	Skipped   int      `json:"skipped"` // already in the destination
	Failed    []string `json:"failed"`
	Cancelled bool     `json:"cancelled"`
}

// Run copies everything Inspect found into library/<date>_<name>. Photos go
// into the folder itself, clips and their sidecars into its video subfolder.
// A file that is already there with the same size is skipped, so an
// interrupted import can simply be run again.
func Run(ctx context.Context, sourcePath, library, name string, progress func(Progress)) (Result, error) {
	src, err := Inspect(sourcePath)
	if err != nil {
		return Result{}, err
	}
	if src.Files == 0 {
		return Result{}, errors.New("no photos or videos found there")
	}
	if library == "" {
		return Result{}, errors.New("no library folder is set")
	}
	folder, err := FolderName(src.Date, name)
	if err != nil {
		return Result{}, err
	}
	res := Result{Dest: filepath.Join(library, folder), Failed: []string{}}
	if rel, err := filepath.Rel(src.Path, res.Dest); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
		return Result{}, errors.New("the library folder is inside the folder being imported")
	}
	if err := os.MkdirAll(res.Dest, 0o755); err != nil {
		return Result{}, err
	}
	p := Progress{Total: src.Files, BytesTotal: src.Bytes}
	for _, f := range src.files {
		if ctx.Err() != nil {
			res.Cancelled = true
			break
		}
		p.File = filepath.Base(f.src)
		if progress != nil {
			progress(p)
		}
		dst := filepath.Join(res.Dest, filepath.Base(f.src))
		if f.video {
			dst = filepath.Join(res.Dest, VideoDir, filepath.Base(f.src))
		}
		if info, err := os.Stat(dst); err == nil {
			if info.Size() == f.size {
				res.Skipped++
			} else {
				res.Failed = append(res.Failed, p.File+": a different file with this name is already there")
			}
		} else if err := execute.CopyVerified(f.src, dst); err != nil {
			res.Failed = append(res.Failed, p.File+": "+err.Error())
		} else {
			res.Copied++
		}
		p.Done++
		p.BytesDone += f.size
	}
	if progress != nil {
		progress(p)
	}
	return res, nil
}
