// Package execute carries out a plan, records a journal, and can undo it.
package execute

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"encoding/json"

	"rawsivr/internal/plan"
	"rawsivr/internal/scan"
	"rawsivr/internal/store"
)

// Recycler sends one file to the OS trash.
type Recycler func(path string) error

type Entry struct {
	plan.Op
	OK  bool   `json:"ok"`
	Err string `json:"err,omitempty"`
}

type Journal struct {
	Time    time.Time `json:"time"`
	Mode    plan.Mode `json:"mode"`
	Undone  bool      `json:"undone"`
	Entries []Entry   `json:"entries"`
	path    string
}

func (j *Journal) Failed() int {
	n := 0
	for _, e := range j.Entries {
		if !e.OK {
			n++
		}
	}
	return n
}

// Run executes every op. A failed op is recorded and the run continues.
// The journal is written even when the run is interrupted by an error.
func Run(root string, p plan.Plan, recycle Recycler, progress func(done, total int, op plan.Op)) (*Journal, error) {
	now := time.Now()
	j := &Journal{
		Time: now,
		Mode: p.Mode,
		path: filepath.Join(root, scan.DirState, "journal-"+now.Format("20060102-150405.000")+".json"),
	}
	for i, op := range p.Ops {
		if progress != nil {
			progress(i, len(p.Ops), op)
		}
		var err error
		switch op.Action {
		case plan.Copy:
			err = CopyVerified(op.Src, op.Dst)
		case plan.Move:
			err = moveFile(op.Src, op.Dst)
		case plan.Recycle:
			err = recycle(op.Src)
		default:
			err = fmt.Errorf("unknown action %q", op.Action)
		}
		e := Entry{Op: op, OK: err == nil}
		if err != nil {
			e.Err = err.Error()
		}
		j.Entries = append(j.Entries, e)
	}
	if len(j.Entries) == 0 {
		return j, nil
	}
	return j, store.WriteJSON(j.path, j)
}

// Undo reverses the most recent journal that has not been undone.
// Copies are removed, moves are moved back. Recycled files cannot be
// restored from here; they are returned so the caller can tell the user.
func Undo(root string) (j *Journal, recycled []string, err error) {
	matches, _ := filepath.Glob(filepath.Join(root, scan.DirState, "journal-*.json"))
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	for _, m := range matches {
		b, rerr := os.ReadFile(m)
		if rerr != nil {
			return nil, nil, rerr
		}
		cand := &Journal{path: m}
		if jerr := json.Unmarshal(b, cand); jerr != nil {
			return nil, nil, fmt.Errorf("%s: %w", m, jerr)
		}
		if !cand.Undone {
			j = cand
			break
		}
	}
	if j == nil {
		return nil, nil, errors.New("nothing to undo")
	}
	var errs []error
	for i := len(j.Entries) - 1; i >= 0; i-- {
		e := j.Entries[i]
		if !e.OK {
			continue
		}
		switch e.Action {
		case plan.Copy:
			// Only remove the copy while the original is still there.
			if _, serr := os.Stat(e.Src); serr != nil {
				errs = append(errs, fmt.Errorf("kept %s: original is gone", e.Dst))
				continue
			}
			if rerr := os.Remove(e.Dst); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
				errs = append(errs, rerr)
			}
		case plan.Move:
			if _, serr := os.Stat(e.Src); serr == nil {
				errs = append(errs, fmt.Errorf("cannot move back %s: a file is already there", e.Src))
				continue
			}
			if merr := moveFile(e.Dst, e.Src); merr != nil {
				errs = append(errs, merr)
			}
		case plan.Recycle:
			recycled = append(recycled, e.Src)
		}
	}
	for _, e := range j.Entries {
		if e.Dst != "" {
			os.Remove(filepath.Dir(e.Dst)) // only succeeds when empty
		}
	}
	if len(errs) == 0 {
		j.Undone = true
		if werr := store.WriteJSON(j.path, j); werr != nil {
			return j, recycled, werr
		}
	}
	return j, recycled, errors.Join(errs...)
}

func moveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("%s already exists", dst)
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	// Different drive: copy, verify, then remove the original.
	if err := CopyVerified(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// CopyVerified copies src to dst, never overwriting, then re-reads dst and
// compares its hash with what was read from src.
func CopyVerified(src, dst string) (err error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			out.Close()
			os.Remove(dst)
		}
	}()
	h := sha256.New()
	if _, err = io.Copy(io.MultiWriter(out, h), in); err != nil {
		return err
	}
	if err = out.Sync(); err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	check, err := os.Open(dst)
	if err != nil {
		return err
	}
	h2 := sha256.New()
	_, err = io.Copy(h2, check)
	check.Close()
	if err != nil {
		return err
	}
	if string(h.Sum(nil)) != string(h2.Sum(nil)) {
		err = fmt.Errorf("verification failed for %s", dst)
		return err
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}
