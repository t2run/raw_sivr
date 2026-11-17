package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"rawsivr/internal/burst"
	"rawsivr/internal/execute"
	"rawsivr/internal/exif"
	"rawsivr/internal/importer"
	"rawsivr/internal/media"
	"rawsivr/internal/orphans"
	"rawsivr/internal/plan"
	"rawsivr/internal/scan"
	"rawsivr/internal/store"
	"rawsivr/internal/thumb"
	"rawsivr/internal/trash"
	"rawsivr/internal/video"
)

// App is the bridge between the UI and the engine.
type App struct {
	ctx     context.Context
	cancel  context.CancelFunc
	media   *media.Server
	proxies *video.Proxies
	mpv     *video.Mpv
	cfgPath string

	cfgMu        sync.Mutex
	cfg          Settings
	importCancel context.CancelFunc

	mu  sync.Mutex
	res scan.Result
	st  *store.Store
}

// Item is one photo or video as the UI sees it.
type Item struct {
	Burst     int               `json:"burst"`     // burst number, 0 when the photo is on its own
	BurstPos  int               `json:"burstPos"`  // position inside the burst, from 1
	BurstSize int               `json:"burstSize"` // photos in the burst
	ID        string            `json:"id"`
	Kind      string            `json:"kind"`
	Name      string            `json:"name"`
	HasJPG    bool              `json:"hasJpg"`
	HasRAW    bool              `json:"hasRaw"`
	Decision  string            `json:"decision"`
	Thumb     string            `json:"thumb"`
	Full      string            `json:"full"`            // photo: image for the single view
	Video     string            `json:"video,omitempty"` // original clip path
	Proxy     *video.ProxyState `json:"proxy,omitempty"`
	ProxyURL  string            `json:"proxyUrl,omitempty"`
}

type Folder struct {
	Root   string   `json:"root"`
	Items  []Item   `json:"items"`
	Strays []string `json:"strays"`
	HasMpv bool     `json:"hasMpv"`
}

type ApplyResult struct {
	OK     int      `json:"ok"`
	Failed []string `json:"failed"`

	FreedBytes int64 `json:"freedBytes"` // size of what went to the Recycle Bin
}

type UndoResult struct {
	Mode     string   `json:"mode"`
	Recycled []string `json:"recycled"`
}

// Settings is what the app remembers between runs.
type Settings struct {
	LastFolder string      `json:"LastFolder"`
	Library    string      `json:"library"` // where imports go
	Folders    scan.Layout `json:"folders"` // names of the folders picks go to

	Keys map[string]string `json:"keys"` // action -> shortcut key
}

func NewApp() *App { return &App{} }

func (a *App) loadSettings() {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if b, err := os.ReadFile(a.cfgPath); err == nil {
		json.Unmarshal(b, &a.cfg)
	}
	if a.cfg.Library == "" {
		if home, err := os.UserHomeDir(); err == nil {
			a.cfg.Library = filepath.Join(home, "Pictures", "RAW SIVR")
		}
	}
	if a.cfg.Folders.Validate() != nil {
		a.cfg.Folders = scan.DefaultLayout
	}
	keys, err := normalizeKeys(a.cfg.Keys)
	if err != nil {
		keys, _ = normalizeKeys(nil)
	}
	a.cfg.Keys = keys
}

// update changes the settings and writes them to disk.
func (a *App) update(change func(*Settings)) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	change(&a.cfg)
	if a.cfgPath == "" {
		return nil
	}
	return store.WriteJSON(a.cfgPath, a.cfg)
}

func (a *App) layout() scan.Layout {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	return a.cfg.Folders
}

// GetSettings returns the current settings.
func (a *App) GetSettings() Settings {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	return a.cfg
}

// SaveSettings changes the library folder, the output folder names and the
// keyboard shortcuts.
func (a *App) SaveSettings(library string, folders scan.Layout, keys map[string]string) error {
	if err := folders.Validate(); err != nil {
		return err
	}
	keys, err := normalizeKeys(keys)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(library) {
		return errors.New("the library folder must be a full path")
	}
	return a.update(func(s *Settings) { s.Library, s.Folders, s.Keys = filepath.Clean(library), folders, keys })
}

// ImportSources lists the camera cards that are plugged in.
func (a *App) ImportSources() []importer.Source {
	out := []importer.Source{}
	for path, label := range importer.Cards() {
		if src, err := importer.Inspect(path); err == nil && src.Files > 0 {
			src.Label = label
			out = append(out, src)
		}
	}
	return out
}

// InspectSource counts the photos and videos in a folder chosen by hand.
func (a *App) InspectSource(path string) (importer.Source, error) {
	return importer.Inspect(path)
}

// Import copies a card or folder into a new dated folder in the library.
// The source is never changed.
func (a *App) Import(source, name string) (importer.Result, error) {
	ctx, cancel := context.WithCancel(a.ctx)
	defer cancel()
	a.cfgMu.Lock()
	if a.importCancel != nil {
		a.cfgMu.Unlock()
		return importer.Result{}, errors.New("an import is already running")
	}
	a.importCancel = cancel
	library := a.cfg.Library
	a.cfgMu.Unlock()
	defer func() {
		a.cfgMu.Lock()
		a.importCancel = nil
		a.cfgMu.Unlock()
	}()
	return importer.Run(ctx, source, library, name, func(p importer.Progress) {
		runtime.EventsEmit(a.ctx, "import", p)
	})
}

// CancelImport stops a running import after the current file.
func (a *App) CancelImport() {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if a.importCancel != nil {
		a.importCancel()
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx, a.cancel = context.WithCancel(ctx)
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	cache = filepath.Join(cache, "rawsivr")
	if cfg, err := os.UserConfigDir(); err == nil {
		a.cfgPath = filepath.Join(cfg, "rawsivr", "config.json")
	}
	a.loadSettings()
	a.media, err = media.Start(thumb.New(filepath.Join(cache, "thumbs")), cache)
	if err != nil {
		runtime.LogError(ctx, "media server: "+err.Error())
	}
	a.proxies = video.NewProxies(a.ctx, filepath.Join(cache, "proxies"), func(src string, st video.ProxyState) {
		ev := map[string]any{"video": src, "proxy": st}
		if st.Status == "ready" {
			ev["proxyUrl"] = a.media.FileURL(st.Path)
		}
		runtime.EventsEmit(a.ctx, "proxy", ev)
	})
	a.mpv = video.NewMpv(func(name string, value any) {
		runtime.EventsEmit(a.ctx, "mpv", map[string]any{"name": name, "value": value})
	})
}

func (a *App) shutdown(ctx context.Context) {
	a.cancel()
	a.mpv.Close()
}

// LastFolder returns the folder that was open last time, if it still exists.
func (a *App) LastFolder() string {
	last := a.GetSettings().LastFolder
	if info, err := os.Stat(last); err != nil || !info.IsDir() {
		return ""
	}
	return last
}

// ChooseFolder shows the folder picker; it returns "" when cancelled.
func (a *App) ChooseFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Open a shoot folder"})
}

// OpenFolder scans a folder and makes it the current shoot.
func (a *App) OpenFolder(root string) (Folder, error) {
	if a.media == nil {
		return Folder{}, errors.New("media server is not running")
	}
	res, err := scan.Scan(root, a.layout())
	if err != nil {
		return Folder{}, err
	}
	st, err := store.Load(res.Root)
	if err != nil {
		return Folder{}, err
	}
	a.mu.Lock()
	if a.res.Root != res.Root {
		a.media.Allow(res.Root)
	}
	a.res, a.st = res, st
	a.mu.Unlock()
	a.update(func(s *Settings) { s.LastFolder = res.Root })

	f := Folder{Root: res.Root, Strays: res.Strays, HasMpv: a.mpv.Available(), Items: []Item{}}
	for _, it := range res.Items {
		ui := Item{
			ID: it.ID, Kind: string(it.Kind), Name: it.Name,
			HasJPG: it.Preview != "", HasRAW: it.Raw != "",
			Decision: string(st.Decisions[it.ID]),
		}
		if it.Kind == scan.Video {
			ui.Video = it.Video
			ui.Thumb = a.media.ThumbURL(480, it.Video)
			ps := a.proxies.Ensure(it.Video)
			ui.Proxy = &ps
			if ps.Status == "ready" {
				ui.ProxyURL = a.media.FileURL(ps.Path)
			}
		} else {
			// The preview embedded in the RAW is small, so it makes a much
			// faster thumbnail source than the full-size JPG.
			ui.Thumb = a.media.ThumbURL(480, it.Raw, it.Preview)
			if it.Preview != "" {
				ui.Full = a.media.FileURL(it.Preview)
			} else {
				ui.Full = a.media.ThumbURL(2400, it.Raw)
			}
		}
		f.Items = append(f.Items, ui)
	}
	// Group photos shot within a couple of seconds of each other.
	times := make([]time.Time, len(res.Items))
	for i, it := range res.Items {
		if it.Kind != scan.Photo {
			continue
		}
		src := it.Preview
		if src == "" {
			src = it.Raw
		}
		if info, err := exif.Read(src); err == nil {
			times[i] = info.Time
		}
	}
	for i, m := range burst.Group(times) {
		f.Items[i].Burst, f.Items[i].BurstPos, f.Items[i].BurstSize = m.Group, m.Pos, m.Size
	}
	if f.Strays == nil {
		f.Strays = []string{}
	}
	return f, nil
}

// SetDecision marks an item "pick", "reject" or "" (unmarked).
func (a *App) SetDecision(id, decision string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.st == nil {
		return errors.New("no folder is open")
	}
	switch store.Decision(decision) {
	case store.Pick, store.Reject, "":
	default:
		return fmt.Errorf("unknown decision %q", decision)
	}
	a.st.Set(id, store.Decision(decision))
	return a.st.Save()
}

// PhotoInfo returns the shooting data (shutter, aperture, ISO…) of a photo.
func (a *App) PhotoInfo(id string) (exif.Info, error) {
	a.mu.Lock()
	var src string
	for _, it := range a.res.Items {
		if it.ID == id {
			if src = it.Preview; src == "" {
				src = it.Raw
			}
			break
		}
	}
	a.mu.Unlock()
	if src == "" {
		return exif.Info{}, errors.New("no photo with that name")
	}
	return exif.Read(src)
}

// SetDecisions marks several items at once, each "pick", "reject" or "".
func (a *App) SetDecisions(decisions map[string]string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.st == nil {
		return errors.New("no folder is open")
	}
	for id, d := range decisions {
		switch store.Decision(d) {
		case store.Pick, store.Reject, "":
			a.st.Set(id, store.Decision(d))
		default:
			return fmt.Errorf("unknown decision %q", d)
		}
	}
	return a.st.Save()
}

// OrphanResult reports what RecycleOrphans did.
type OrphanResult struct {
	Recycled int      `json:"recycled"`
	Bytes    int64    `json:"bytes"`
	Failed   []string `json:"failed"`
}

// FindOrphans lists RAW files under root that have no JPG anywhere under it.
func (a *App) FindOrphans(root string) (orphans.Report, error) {
	return orphans.Find(root)
}

// RecycleOrphans sends the orphan RAWs of the chosen folders to the Recycle
// Bin. The folder is searched again first, so only files that are orphans
// right now are touched.
func (a *App) RecycleOrphans(root string, dirs []string) (OrphanResult, error) {
	rep, err := orphans.Find(root)
	if err != nil {
		return OrphanResult{}, err
	}
	chosen := map[string]bool{}
	for _, d := range dirs {
		chosen[d] = true
	}
	res := OrphanResult{Failed: []string{}}
	a.mpv.Hide()
	for _, f := range rep.Folders {
		if !chosen[f.Dir] {
			continue
		}
		for _, path := range f.Files {
			info, err := os.Stat(path)
			if err == nil {
				err = trash.Recycle(path)
			}
			if err != nil {
				res.Failed = append(res.Failed, filepath.Base(path)+": "+err.Error())
				continue
			}
			res.Recycled++
			res.Bytes += info.Size()
		}
	}
	return res, nil
}

func parseMode(mode string) (plan.Mode, error) {
	switch m := plan.Mode(mode); m {
	case plan.ModeCopy, plan.ModeMove, plan.ModeCleanup:
		return m, nil
	}
	return "", fmt.Errorf("unknown mode %q", mode)
}

// currentPlan rescans so the plan always reflects what is on disk now.
func (a *App) currentPlan(mode string) (plan.Plan, error) {
	m, err := parseMode(mode)
	if err != nil {
		return plan.Plan{}, err
	}
	if a.st == nil {
		return plan.Plan{}, errors.New("no folder is open")
	}
	res, err := scan.Scan(a.res.Root, a.layout())
	if err != nil {
		return plan.Plan{}, err
	}
	a.res = res
	return plan.Build(res, a.st.Decisions, m, a.layout()), nil
}

// Plan previews what Apply would do. Nothing on disk changes.
func (a *App) Plan(mode string) (plan.Plan, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.currentPlan(mode)
}

// Apply carries out the plan for the given mode.
func (a *App) Apply(mode string) (ApplyResult, error) {
	a.mpv.Hide() // release any clip the player holds open
	a.mu.Lock()
	defer a.mu.Unlock()
	p, err := a.currentPlan(mode)
	if err != nil {
		return ApplyResult{}, err
	}
	sizes := map[string]int64{} // of the files about to be recycled
	for _, op := range p.Ops {
		if op.Action == plan.Recycle {
			if info, err := os.Stat(op.Src); err == nil {
				sizes[op.Src] = info.Size()
			}
		}
	}
	j, err := execute.Run(a.res.Root, p, trash.Recycle, func(done, total int, op plan.Op) {
		runtime.EventsEmit(a.ctx, "progress", map[string]any{"done": done, "total": total})
	})
	if err != nil {
		return ApplyResult{}, err
	}
	r := ApplyResult{Failed: []string{}}
	for _, e := range j.Entries {
		if e.OK {
			r.OK++
			if e.Action == plan.Recycle {
				r.FreedBytes += sizes[e.Src]
			}
		} else {
			r.Failed = append(r.Failed, filepath.Base(e.Src)+": "+e.Err)
		}
	}
	return r, nil
}

// Undo reverses the most recent Apply.
func (a *App) Undo() (UndoResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.st == nil {
		return UndoResult{}, errors.New("no folder is open")
	}
	j, recycled, err := execute.Undo(a.res.Root)
	r := UndoResult{Recycled: recycled}
	if r.Recycled == nil {
		r.Recycled = []string{}
	}
	if j != nil {
		r.Mode = string(j.Mode)
	}
	return r, err
}

// MpvPlay plays a clip's original file in the embedded player.
func (a *App) MpvPlay(path string) error { return a.mpv.Play(windowTitle, path) }

// MpvRect positions the embedded player, in physical pixels of the window.
func (a *App) MpvRect(x, y, w, h int) { a.mpv.SetRect(x, y, w, h) }

func (a *App) MpvHide()                      { a.mpv.Hide() }
func (a *App) MpvPause(paused bool) error    { return a.mpv.Pause(paused) }
func (a *App) MpvSeek(seconds float64) error { return a.mpv.Seek(seconds) }
