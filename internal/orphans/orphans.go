// Package orphans finds RAW files whose JPG is gone. In a library that was
// culled by deleting the JPGs of unwanted photos, those RAWs are the
// leftovers that still take up most of the space.
package orphans

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

type Folder struct {
	Dir   string   `json:"dir"`
	Rel   string   `json:"rel"` // path relative to the scanned root, "." for the root
	Files []string `json:"files"`
	Bytes int64    `json:"bytes"`
}

type Report struct {
	Root    string   `json:"root"`
	Folders []Folder `json:"folders"`
	Files   int      `json:"files"`
	Bytes   int64    `json:"bytes"`
	RAWs    int      `json:"raws"` // all RAW files seen
}

var (
	rawExt = map[string]bool{".arw": true}
	jpgExt = map[string]bool{".jpg": true, ".jpeg": true, ".hif": true, ".heic": true}
)

// Find walks root and reports every RAW file for which no JPG with the same
// name exists anywhere under root. Matching across the whole tree keeps a
// RAW safe when its JPG simply lives in another folder (raw/ next to jpg/,
// selected_raw next to selected_jpg).
func Find(root string) (Report, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	type raw struct {
		path string
		size int64
	}
	jpgs := map[string]bool{}
	var raws []raw
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir // unreadable folder: leave it alone
			}
			return nil
		}
		if d.IsDir() {
			if path != root && (strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "$")) {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		base := strings.ToUpper(strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())))
		switch {
		case jpgExt[ext]:
			jpgs[base] = true
		case rawExt[ext]:
			if info, err := d.Info(); err == nil {
				raws = append(raws, raw{path, info.Size()})
			}
		}
		return nil
	})
	if err != nil {
		return Report{}, err
	}

	rep := Report{Root: root, Folders: []Folder{}, RAWs: len(raws)}
	byDir := map[string]*Folder{}
	for _, r := range raws {
		base := filepath.Base(r.path)
		if jpgs[strings.ToUpper(strings.TrimSuffix(base, filepath.Ext(base)))] {
			continue
		}
		dir := filepath.Dir(r.path)
		f := byDir[dir]
		if f == nil {
			rel, _ := filepath.Rel(root, dir)
			f = &Folder{Dir: dir, Rel: rel}
			byDir[dir] = f
		}
		f.Files = append(f.Files, r.path)
		f.Bytes += r.size
		rep.Files++
		rep.Bytes += r.size
	}
	for _, f := range byDir {
		rep.Folders = append(rep.Folders, *f)
	}
	sort.Slice(rep.Folders, func(i, j int) bool { return rep.Folders[i].Dir < rep.Folders[j].Dir })
	return rep, nil
}
