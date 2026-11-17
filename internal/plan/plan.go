// Package plan turns decisions into a list of file operations without
// touching the disk, so the result can be shown before anything happens.
package plan

import (
	"os"
	"path/filepath"

	"rawsivr/internal/scan"
	"rawsivr/internal/store"
)

type Mode string

const (
	ModeCopy    Mode = "copy"    // copy picks, touch nothing else
	ModeMove    Mode = "move"    // move picks, recycle rejects
	ModeCleanup Mode = "cleanup" // recycle rejects only
)

type Action string

const (
	Copy    Action = "copy"
	Move    Action = "move"
	Recycle Action = "recycle"
)

type Op struct {
	Action Action `json:"action"`
	ItemID string `json:"itemId"`
	Src    string `json:"src"`
	Dst    string `json:"dst,omitempty"`
}

type Plan struct {
	Mode      Mode     `json:"mode"`
	Ops       []Op     `json:"ops"`
	Done      []string `json:"done,omitempty"`      // destination already holds the same-size file
	Conflicts []string `json:"conflicts,omitempty"` // destination exists with a different size
	Picks     int      `json:"picks"`
	Rejects   int      `json:"rejects"`
	Unmarked  int      `json:"unmarked"`
	Bytes     int64    `json:"bytes"` // bytes to copy or move

	RecycleBytes int64 `json:"recycleBytes"` // bytes going to the Recycle Bin
}

func Build(res scan.Result, dec map[string]store.Decision, mode Mode, layout scan.Layout) Plan {
	p := Plan{Mode: mode}
	for _, it := range res.Items {
		switch dec[it.ID] {
		case store.Pick:
			p.Picks++
			if mode == ModeCleanup {
				continue
			}
			act := Copy
			if mode == ModeMove {
				act = Move
			}
			add := func(src, dir string) {
				if src == "" {
					return
				}
				dst := filepath.Join(res.Root, dir, filepath.Base(src))
				si, err := os.Stat(src)
				if err != nil {
					p.Conflicts = append(p.Conflicts, src+" (missing)")
					return
				}
				if di, err := os.Stat(dst); err == nil {
					if di.Size() == si.Size() && act == Copy {
						p.Done = append(p.Done, dst)
					} else {
						p.Conflicts = append(p.Conflicts, dst)
					}
					return
				}
				p.Ops = append(p.Ops, Op{Action: act, ItemID: it.ID, Src: src, Dst: dst})
				p.Bytes += si.Size()
			}
			add(it.Preview, layout.JPG)
			add(it.Raw, layout.RAW)
			add(it.Video, layout.Video)
			for _, sc := range it.Sidecars {
				add(sc, layout.Video)
			}
		case store.Reject:
			p.Rejects++
			if mode == ModeCopy {
				continue
			}
			for _, f := range it.Files() {
				p.Ops = append(p.Ops, Op{Action: Recycle, ItemID: it.ID, Src: f})
				if info, err := os.Stat(f); err == nil {
					p.RecycleBytes += info.Size()
				}
			}
		default:
			p.Unmarked++
		}
	}
	return p
}
