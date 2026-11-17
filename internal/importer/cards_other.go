//go:build !windows

package importer

import (
	"os"
	"path/filepath"
)

// Cards returns mounted volumes that look like a camera card (they have a
// DCIM folder), as path and display label.
func Cards() map[string]string {
	out := map[string]string{}
	user := os.Getenv("USER")
	for _, pattern := range []string{"/Volumes/*", "/media/" + user + "/*", "/run/media/" + user + "/*"} {
		mounts, _ := filepath.Glob(pattern)
		for _, m := range mounts {
			if info, err := os.Stat(filepath.Join(m, "DCIM")); err == nil && info.IsDir() {
				out[m] = filepath.Base(m)
			}
		}
	}
	return out
}
