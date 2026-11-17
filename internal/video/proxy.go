// Package video provides the two playback paths: small 1080p proxies that
// the webview can play, and an embedded mpv player for the originals.
package video

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"rawsivr/internal/proc"
)

type ProxyState struct {
	Status string `json:"status"` // queued, building, ready, error
	Path   string `json:"path,omitempty"`
	Err    string `json:"err,omitempty"`
}

// Proxies builds 1080p H.264 previews one at a time in the background.
type Proxies struct {
	dir      string
	ctx      context.Context
	onChange func(src string, st ProxyState)

	mu    sync.Mutex
	state map[string]ProxyState
	queue chan string
}

func NewProxies(ctx context.Context, dir string, onChange func(string, ProxyState)) *Proxies {
	p := &Proxies{dir: dir, ctx: ctx, onChange: onChange, state: map[string]ProxyState{}, queue: make(chan string, 4096)}
	go p.worker()
	return p
}

func (p *Proxies) target(src string) (string, error) {
	info, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	sum := sha1.Sum([]byte(fmt.Sprintf("%s|%d|%d", src, info.Size(), info.ModTime().UnixNano())))
	return filepath.Join(p.dir, hex.EncodeToString(sum[:])+".mp4"), nil
}

// Ensure returns the current state of a clip's proxy and queues a build
// when there is none yet.
func (p *Proxies) Ensure(src string) ProxyState {
	p.mu.Lock()
	defer p.mu.Unlock()
	if st, ok := p.state[src]; ok && st.Status != "error" {
		if st.Status != "ready" {
			return st
		}
		if _, err := os.Stat(st.Path); err == nil {
			return st
		}
	}
	out, err := p.target(src)
	if err != nil {
		st := ProxyState{Status: "error", Err: err.Error()}
		p.state[src] = st
		return st
	}
	st := ProxyState{Status: "queued"}
	if _, err := os.Stat(out); err == nil {
		st = ProxyState{Status: "ready", Path: out}
	} else {
		select {
		case p.queue <- src:
		default:
			st = ProxyState{Status: "error", Err: "queue full"}
		}
	}
	p.state[src] = st
	return st
}

func (p *Proxies) set(src string, st ProxyState) {
	p.mu.Lock()
	p.state[src] = st
	p.mu.Unlock()
	if p.onChange != nil {
		p.onChange(src, st)
	}
}

func (p *Proxies) worker() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case src := <-p.queue:
			p.set(src, ProxyState{Status: "building"})
			out, err := p.build(src)
			if err != nil {
				if p.ctx.Err() != nil {
					return
				}
				p.set(src, ProxyState{Status: "error", Err: err.Error()})
				continue
			}
			p.set(src, ProxyState{Status: "ready", Path: out})
		}
	}
}

func (p *Proxies) build(src string) (string, error) {
	out, err := p.target(src)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(p.dir, 0o755); err != nil {
		return "", err
	}
	tmp := out + ".part.mp4"
	// Decoding the camera original is the bottleneck, so the CPU encoder at
	// "veryfast" costs nothing extra and works on every machine.
	cmd := exec.CommandContext(p.ctx, "ffmpeg", "-hide_banner", "-v", "error", "-y", "-i", src,
		"-vf", "scale=-2:1080:flags=fast_bilinear,format=yuv420p",
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-g", "30",
		"-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart", tmp)
	proc.Hide(cmd)
	if b, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("ffmpeg: %v: %s", err, bytes.TrimSpace(b))
	}
	return out, os.Rename(tmp, out)
}
