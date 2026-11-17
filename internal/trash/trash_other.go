//go:build !windows

// Package trash sends files to the operating system's recycle bin.
package trash

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Recycle moves one file into the user's trash: ~/.Trash on macOS, the
// freedesktop.org home trash (with its .trashinfo record, so file managers
// can restore the file) elsewhere. A file on another volume than the home
// folder cannot be renamed into the trash and returns an error; it is never
// deleted instead.
func Recycle(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(abs); err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		return moveUnique(abs, filepath.Join(home, ".Trash"), nil)
	}
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		data = filepath.Join(home, ".local", "share")
	}
	trash := filepath.Join(data, "Trash")
	info := filepath.Join(trash, "info")
	if err := os.MkdirAll(info, 0o700); err != nil {
		return err
	}
	return moveUnique(abs, filepath.Join(trash, "files"), func(name string) error {
		record := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
			(&url.URL{Path: abs}).EscapedPath(), time.Now().Format("2006-01-02T15:04:05"))
		return os.WriteFile(filepath.Join(info, name+".trashinfo"), []byte(record), 0o600)
	})
}

// moveUnique renames src into dir under a name that is not taken yet.
// record, when set, is called with the chosen name before the move.
func moveUnique(src, dir string, record func(name string) error) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	name := filepath.Base(src)
	for i := 1; ; i++ {
		if _, err := os.Lstat(filepath.Join(dir, name)); os.IsNotExist(err) {
			break
		}
		ext := filepath.Ext(src)
		name = fmt.Sprintf("%s.%d%s", filepath.Base(src[:len(src)-len(ext)]), i, ext)
	}
	if record != nil {
		if err := record(name); err != nil {
			return err
		}
	}
	if err := os.Rename(src, filepath.Join(dir, name)); err != nil {
		return fmt.Errorf("could not move %s to the trash: %w", src, err)
	}
	return nil
}
