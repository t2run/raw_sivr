package orphans

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFind(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		"trip/raw/DSC001.ARW", "trip/jpg/DSC001.JPG", // JPG lives in a sibling folder: not an orphan
		"trip/raw/DSC002.ARW",                  // JPG deleted: orphan
		"trip/raw/DSC003.arw",                  // orphan, lower-case extension
		"other/DSC010.ARW", "other/dsc010.jpg", // paired, different case
		"other/DSC011.ARW", // orphan
		".rawsivr/x/DSC099.ARW",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte("12345"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rep, err := Find(root)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Files != 3 || rep.Bytes != 15 || rep.RAWs != 5 || len(rep.Folders) != 2 {
		t.Fatalf("report = %+v", rep)
	}
	if rep.Folders[0].Rel != "other" || len(rep.Folders[0].Files) != 1 || len(rep.Folders[1].Files) != 2 {
		t.Errorf("folders = %+v", rep.Folders)
	}
}
