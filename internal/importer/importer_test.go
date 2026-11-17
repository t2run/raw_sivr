package importer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func put(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 10, 26, 17, 0, 0, 0, time.Local)
	os.Chtimes(p, day, day)
}

func TestImportCardLayout(t *testing.T) {
	card, lib := t.TempDir(), t.TempDir()
	put(t, card, "DCIM/100MSDCF/DSC001.JPG", "jpg1")
	put(t, card, "DCIM/100MSDCF/DSC001.ARW", "raw1")
	put(t, card, "DCIM/100MSDCF/DSC002.ARW", "raw2")
	put(t, card, "PRIVATE/M4ROOT/CLIP/C0001.MP4", "clip")
	put(t, card, "PRIVATE/M4ROOT/CLIP/C0001M01.XML", "meta")
	put(t, card, "PRIVATE/M4ROOT/THMBNL/C0001T01.JPG", "thumb")
	put(t, card, "PRIVATE/M4ROOT/SUB/C0001S03.MP4", "proxy")

	src, err := Inspect(card)
	if err != nil {
		t.Fatal(err)
	}
	if src.Photos != 2 || src.Videos != 1 || src.Files != 5 || src.Date != "2026-10-26" {
		t.Fatalf("inspect = %+v", src)
	}

	var last Progress
	res, err := Run(context.Background(), card, lib, "Moon night", func(p Progress) { last = p })
	if err != nil {
		t.Fatal(err)
	}
	if res.Copied != 5 || len(res.Failed) != 0 || last.Done != 5 {
		t.Fatalf("result = %+v, progress = %+v", res, last)
	}
	dest := filepath.Join(lib, "2026-10-26_Moon_night")
	for _, f := range []string{"DSC001.JPG", "DSC001.ARW", "DSC002.ARW", "video/C0001.MP4", "video/C0001M01.XML"} {
		if _, err := os.Stat(filepath.Join(dest, filepath.FromSlash(f))); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	if _, err := os.Stat(filepath.Join(card, "DCIM", "100MSDCF", "DSC001.ARW")); err != nil {
		t.Error("the card was modified")
	}

	// Running again copies nothing.
	res, err = Run(context.Background(), card, lib, "Moon night", nil)
	if err != nil || res.Copied != 0 || res.Skipped != 5 {
		t.Fatalf("second run = %+v, %v", res, err)
	}

	// A cancelled import stops and says so.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err = Run(ctx, card, lib, "other", nil)
	if err != nil || !res.Cancelled || res.Copied != 0 {
		t.Fatalf("cancelled run = %+v, %v", res, err)
	}
}

func TestFolderName(t *testing.T) {
	if n, _ := FolderName("2026-11-16", ""); n != "2026-11-16" {
		t.Errorf("got %q", n)
	}
	if _, err := FolderName("2026-11-16", "a/b"); err == nil {
		t.Error("slash should be rejected")
	}
}
