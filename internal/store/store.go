// Package store keeps pick/reject decisions in <shoot>/.rawsivr/decisions.json
// so a culling session can be closed and resumed.
package store

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"rawsivr/internal/scan"
)

type Decision string

const (
	Pick   Decision = "pick"
	Reject Decision = "reject"
)

type Store struct {
	path      string
	Decisions map[string]Decision `json:"decisions"`
}

func Load(root string) (*Store, error) {
	s := &Store{
		path:      filepath.Join(root, scan.DirState, "decisions.json"),
		Decisions: map[string]Decision{},
	}
	b, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, s); err != nil {
		return nil, err
	}
	if s.Decisions == nil {
		s.Decisions = map[string]Decision{}
	}
	return s, nil
}

// Set records a decision; an empty decision clears the mark.
func (s *Store) Set(id string, d Decision) {
	if d == "" {
		delete(s.Decisions, id)
		return
	}
	s.Decisions[id] = d
}

func (s *Store) Save() error {
	return WriteJSON(s.path, s)
}

// WriteJSON writes v atomically (temp file then rename).
func WriteJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
