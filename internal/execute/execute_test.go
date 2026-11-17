package execute

import (
	"os"
	"path/filepath"
	"testing"

	"rawsivr/internal/plan"
	"rawsivr/internal/scan"
	"rawsivr/internal/store"
)

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func shoot(t *testing.T) (string, scan.Result, map[string]store.Decision) {
	t.Helper()
	root := t.TempDir()
	for _, f := range []string{
		"DSC001.ARW", "DSC001.JPG", // pick
		"DSC002.ARW", "DSC002.jpg", // reject
		"DSC003.ARW",               // RAW only, pick
		"DSC004.JPG", "DSC004.ARW", // unmarked
		"C0001.MP4", "C0001M01.XML", // video pick
		"C0009M01.XML", // stray
		"notes.txt",
	} {
		write(t, root, f, "data:"+f)
	}
	res, err := scan.Scan(root, scan.DefaultLayout)
	if err != nil {
		t.Fatal(err)
	}
	return root, res, map[string]store.Decision{
		"DSC001": store.Pick, "DSC002": store.Reject, "DSC003": store.Pick, "C0001": store.Pick,
	}
}

func TestScan(t *testing.T) {
	_, res, _ := shoot(t)
	if len(res.Items) != 5 {
		t.Fatalf("items = %d, want 5: %+v", len(res.Items), res.Items)
	}
	if len(res.Strays) != 1 {
		t.Fatalf("strays = %v", res.Strays)
	}
	for _, it := range res.Items {
		switch it.ID {
		case "DSC002":
			if it.Raw == "" || it.Preview == "" {
				t.Errorf("DSC002 not paired across extension case: %+v", it)
			}
		case "DSC003":
			if it.Preview != "" || it.Raw == "" {
				t.Errorf("DSC003 should be RAW only: %+v", it)
			}
		case "C0001":
			if it.Kind != scan.Video || len(it.Sidecars) != 1 {
				t.Errorf("C0001 video/sidecar wrong: %+v", it)
			}
		}
	}
}

func TestCopyThenUndo(t *testing.T) {
	root, res, dec := shoot(t)
	p := plan.Build(res, dec, plan.ModeCopy, scan.DefaultLayout)
	if len(p.Ops) != 5 { // 001 jpg+raw, 003 raw, video + sidecar
		t.Fatalf("ops = %d, want 5", len(p.Ops))
	}
	j, err := Run(root, p, func(string) error { t.Fatal("copy mode must not recycle"); return nil }, nil)
	if err != nil || j.Failed() != 0 {
		t.Fatalf("run: %v, failed %d", err, j.Failed())
	}
	for _, f := range []string{"selected_jpg/DSC001.JPG", "selected_raw/DSC001.ARW", "selected_raw/DSC003.ARW",
		"selected_video/C0001.MP4", "selected_video/C0001M01.XML", "DSC001.JPG", "DSC002.ARW"} {
		if !exists(filepath.Join(root, f)) {
			t.Errorf("missing %s", f)
		}
	}
	// A second plan has nothing left to do, and the outputs are not rescanned.
	res2, _ := scan.Scan(root, scan.DefaultLayout)
	if len(res2.Items) != 5 {
		t.Errorf("rescan items = %d, want 5", len(res2.Items))
	}
	if p2 := plan.Build(res2, dec, plan.ModeCopy, scan.DefaultLayout); len(p2.Ops) != 0 || len(p2.Done) != 5 {
		t.Errorf("second plan: ops %d done %d", len(p2.Ops), len(p2.Done))
	}
	if _, _, err := Undo(root); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(root, "selected_jpg")) || !exists(filepath.Join(root, "DSC001.JPG")) {
		t.Error("undo did not restore the folder")
	}
	if _, _, err := Undo(root); err == nil {
		t.Error("second undo should report nothing to undo")
	}
}

func TestMoveThenUndo(t *testing.T) {
	root, res, dec := shoot(t)
	var recycled []string
	p := plan.Build(res, dec, plan.ModeMove, scan.DefaultLayout)
	j, err := Run(root, p, func(p string) error { recycled = append(recycled, p); return os.Remove(p) }, nil)
	if err != nil || j.Failed() != 0 {
		t.Fatalf("run: %v, failed %d", err, j.Failed())
	}
	if len(recycled) != 2 {
		t.Errorf("recycled %v, want DSC002 jpg+raw", recycled)
	}
	if exists(filepath.Join(root, "DSC001.JPG")) || !exists(filepath.Join(root, "selected_raw", "DSC001.ARW")) {
		t.Error("pick was not moved")
	}
	if !exists(filepath.Join(root, "DSC004.ARW")) {
		t.Error("unmarked file was touched")
	}
	_, gone, err := Undo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(gone) != 2 || !exists(filepath.Join(root, "DSC001.ARW")) || !exists(filepath.Join(root, "C0001M01.XML")) {
		t.Errorf("undo: recycled %v", gone)
	}
}

func TestCustomFolderNames(t *testing.T) {
	root, _, dec := shoot(t)
	layout := scan.Layout{JPG: "keep jpg", RAW: "keep raw", Video: "keep video"}
	if err := layout.Validate(); err != nil {
		t.Fatal(err)
	}
	if (scan.Layout{JPG: "a", RAW: "A", Video: "v"}).Validate() == nil || (scan.Layout{JPG: "a/b", RAW: "r", Video: "v"}).Validate() == nil {
		t.Error("duplicate or nested names should be rejected")
	}
	res, _ := scan.Scan(root, layout)
	j, err := Run(root, plan.Build(res, dec, plan.ModeCopy, layout), nil, nil)
	if err != nil || j.Failed() != 0 {
		t.Fatalf("run: %v", err)
	}
	if !exists(filepath.Join(root, "keep jpg", "DSC001.JPG")) || !exists(filepath.Join(root, "keep video", "C0001.MP4")) {
		t.Error("picks did not go to the renamed folders")
	}
	if res2, _ := scan.Scan(root, layout); len(res2.Items) != 5 {
		t.Errorf("renamed output folders were scanned as photos: %d items", len(res2.Items))
	}
	if _, _, err := Undo(root); err != nil || exists(filepath.Join(root, "keep jpg")) {
		t.Errorf("undo left the renamed folder behind: %v", err)
	}
}

func TestConflictIsNotOverwritten(t *testing.T) {
	root, res, dec := shoot(t)
	os.Mkdir(filepath.Join(root, "selected_jpg"), 0o755)
	write(t, filepath.Join(root, "selected_jpg"), "DSC001.JPG", "something else entirely")
	p := plan.Build(res, dec, plan.ModeCopy, scan.DefaultLayout)
	if len(p.Conflicts) != 1 || len(p.Ops) != 4 {
		t.Fatalf("conflicts %v ops %d", p.Conflicts, len(p.Ops))
	}
}
