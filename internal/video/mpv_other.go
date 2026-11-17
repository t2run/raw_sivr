//go:build !windows

package video

import "errors"

// Mpv is not embedded on this platform yet; playback uses proxies only.
// TODO(phase 5): X11 embedding with --wid, and a macOS approach.
type Mpv struct{}

var errUnsupported = errors.New("original-quality playback is not available on this platform yet")

func FindMpv() string                              { return "" }
func NewMpv(onEvent func(string, any)) *Mpv        { return &Mpv{} }
func (m *Mpv) Available() bool                     { return false }
func (m *Mpv) Play(windowTitle, path string) error { return errUnsupported }
func (m *Mpv) SetRect(x, y, w, h int)              {}
func (m *Mpv) Hide()                               {}
func (m *Mpv) Pause(paused bool) error             { return errUnsupported }
func (m *Mpv) Seek(seconds float64) error          { return errUnsupported }
func (m *Mpv) Close()                              {}
