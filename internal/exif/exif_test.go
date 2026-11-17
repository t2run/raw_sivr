package exif

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Run with RAWSIVR_SAMPLES=<folder of camera files> to read real files.
func TestReadSamples(t *testing.T) {
	dir := os.Getenv("RAWSIVR_SAMPLES")
	if dir == "" {
		t.Skip("RAWSIVR_SAMPLES not set")
	}
	for _, pattern := range []string{"*.JPG", "*.ARW"} {
		files, _ := filepath.Glob(filepath.Join(dir, pattern))
		if len(files) == 0 {
			t.Fatalf("no %s files in %s", pattern, dir)
		}
		if pattern == "*.JPG" {
			var prev time.Time
			for _, f := range files {
				i, _ := Read(f)
				t.Logf("%s  %s  gap %.2fs", filepath.Base(f), i.Time.Format("15:04:05.000"), i.Time.Sub(prev).Seconds())
				prev = i.Time
			}
		}
		info, err := Read(files[0])
		if err != nil {
			t.Fatalf("%s: %v", files[0], err)
		}
		t.Logf("%s: %+v", filepath.Base(files[0]), info)
		if info.Camera == "" || info.Shutter == "" || info.Aperture == "" || info.ISO == "" {
			t.Errorf("%s: missing fields: %+v", files[0], info)
		}
	}
}

func TestTrimFloat(t *testing.T) {
	for in, want := range map[float64]string{8: "8", 2.8: "2.8", 600: "600", 1.3: "1.3"} {
		if got := trimFloat(in); got != want {
			t.Errorf("trimFloat(%v) = %q, want %q", in, got, want)
		}
	}
}
