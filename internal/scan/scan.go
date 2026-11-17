// Package scan walks a shoot folder and groups files into items:
// a photo is a preview (JPG/HIF) and/or a RAW sharing a base name,
// a video is a clip plus its Sony sidecar XML files.
package scan

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Kind string

const (
	Photo Kind = "photo"
	Video Kind = "video"
)

// Output folder names. They are skipped while scanning.
const (
	DirSelectedJPG   = "selected_jpg"
	DirSelectedRAW   = "selected_raw"
	DirSelectedVideo = "selected_video"
	DirState         = ".rawsivr"
)

type Item struct {
	ID       string   `json:"id"` // relative dir + "/" + base name, forward slashes
	Kind     Kind     `json:"kind"`
	Name     string   `json:"name"` // base name without extension
	Preview  string   `json:"preview,omitempty"`
	Raw      string   `json:"raw,omitempty"`
	Video    string   `json:"video,omitempty"`
	Sidecars []string `json:"sidecars,omitempty"`
}

// Files returns every file on disk that belongs to the item.
func (it Item) Files() []string {
	var out []string
	for _, f := range []string{it.Preview, it.Raw, it.Video} {
		if f != "" {
			out = append(out, f)
		}
	}
	return append(out, it.Sidecars...)
}

type Result struct {
	Root   string   `json:"root"`
	Items  []Item   `json:"items"`
	Strays []string `json:"strays,omitempty"` // sidecars whose clip is missing
}

var (
	previewExt = map[string]bool{".jpg": true, ".jpeg": true, ".hif": true, ".heic": true}
	rawExt     = map[string]bool{".arw": true}
	videoExt   = map[string]bool{".mp4": true, ".mov": true, ".mxf": true}
	sidecarRe  = regexp.MustCompile(`(?i)^(.+)M\d\d$`)
)

// Layout names the folders, inside the shoot folder, that picks go to.
type Layout struct {
	JPG   string `json:"jpg"`
	RAW   string `json:"raw"`
	Video string `json:"video"`
}

var DefaultLayout = Layout{JPG: DirSelectedJPG, RAW: DirSelectedRAW, Video: DirSelectedVideo}

// Validate checks that the three names are usable, distinct folder names.
func (l Layout) Validate() error {
	seen := map[string]bool{}
	for _, n := range []string{l.JPG, l.RAW, l.Video} {
		if n == "" || n != strings.TrimSpace(n) || strings.HasPrefix(n, ".") || strings.HasSuffix(n, ".") ||
			strings.ContainsAny(n, `\/:*?"<>|`) {
			return fmt.Errorf("%q is not a valid folder name", n)
		}
		if seen[strings.ToLower(n)] {
			return fmt.Errorf("the three folder names must be different (%q is used twice)", n)
		}
		seen[strings.ToLower(n)] = true
	}
	return nil
}

// isOutput reports whether a folder is one the app writes to. The default
// names stay excluded even after the user renames the folders, so results
// of earlier runs are never mistaken for new photos.
func (l Layout) isOutput(name string) bool {
	for _, n := range []string{l.JPG, l.RAW, l.Video, DirSelectedJPG, DirSelectedRAW, DirSelectedVideo} {
		if strings.EqualFold(name, n) {
			return true
		}
	}
	return strings.HasPrefix(name, ".")
}

func Scan(root string, layout Layout) (Result, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Result{}, err
	}
	res := Result{Root: root}
	items := map[string]*Item{} // key: dir + "|" + upper(base)
	var order []string
	type xmlFile struct{ dir, base, path string }
	var xmls []xmlFile

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && layout.isOutput(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		base := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		dir := filepath.Dir(path)
		if ext == ".xml" {
			xmls = append(xmls, xmlFile{dir, base, path})
			return nil
		}
		isPreview, isRaw, isVideo := previewExt[ext], rawExt[ext], videoExt[ext]
		if !isPreview && !isRaw && !isVideo {
			return nil
		}
		key := dir + "|" + strings.ToUpper(base)
		it := items[key]
		if it == nil {
			rel, _ := filepath.Rel(root, dir)
			id := base
			if rel != "." {
				id = filepath.ToSlash(rel) + "/" + base
			}
			it = &Item{ID: id, Name: base, Kind: Photo}
			items[key] = it
			order = append(order, key)
		}
		switch {
		case isVideo:
			it.Video, it.Kind = path, Video
		case isRaw:
			it.Raw = path
		case it.Preview == "":
			it.Preview = path
		}
		return nil
	})
	if err != nil {
		return res, err
	}

	for _, x := range xmls {
		m := sidecarRe.FindStringSubmatch(x.base)
		if m != nil {
			if it := items[x.dir+"|"+strings.ToUpper(m[1])]; it != nil && it.Kind == Video {
				it.Sidecars = append(it.Sidecars, x.path)
				continue
			}
			res.Strays = append(res.Strays, x.path)
		}
	}

	for _, k := range order {
		res.Items = append(res.Items, *items[k])
	}
	sort.Slice(res.Items, func(i, j int) bool { return res.Items[i].ID < res.Items[j].ID })
	return res, nil
}
