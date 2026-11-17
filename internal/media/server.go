// Package media serves originals, thumbnails and video proxies to the UI
// over a loopback HTTP server. The webview's own asset handler cannot
// stream video with seeking, a plain HTTP server can.
package media

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"rawsivr/internal/thumb"
)

type Server struct {
	base   string
	token  string
	thumbs *thumb.Cache

	mu    sync.RWMutex
	roots []string // folders the server is allowed to read
}

func Start(thumbs *thumb.Cache, roots ...string) (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	tok := make([]byte, 16)
	if _, err := rand.Read(tok); err != nil {
		return nil, err
	}
	s := &Server{base: "http://" + ln.Addr().String(), token: hex.EncodeToString(tok), thumbs: thumbs, roots: roots}
	mux := http.NewServeMux()
	mux.HandleFunc("/file", s.file)
	mux.HandleFunc("/thumb", s.thumb)
	go http.Serve(ln, mux)
	return s, nil
}

// Allow adds a folder the server may read from.
func (s *Server) Allow(root string) {
	s.mu.Lock()
	s.roots = append(s.roots, root)
	s.mu.Unlock()
}

func (s *Server) FileURL(path string) string {
	return s.base + "/file?t=" + s.token + "&p=" + url.QueryEscape(path)
}

// ThumbURL builds a thumbnail URL; sources are tried in order.
func (s *Server) ThumbURL(size int, sources ...string) string {
	u := s.base + "/thumb?t=" + s.token + "&s=" + strconv.Itoa(size)
	for _, src := range sources {
		if src != "" {
			u += "&p=" + url.QueryEscape(src)
		}
	}
	return u
}

func (s *Server) allowed(path string) bool {
	path = filepath.Clean(path)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.roots {
		rel, err := filepath.Rel(r, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			return true
		}
	}
	return false
}

func (s *Server) check(w http.ResponseWriter, r *http.Request) ([]string, bool) {
	q := r.URL.Query()
	if q.Get("t") != s.token {
		http.Error(w, "forbidden", http.StatusForbidden)
		return nil, false
	}
	paths := q["p"]
	if len(paths) == 0 {
		http.Error(w, "missing path", http.StatusBadRequest)
		return nil, false
	}
	for _, p := range paths {
		if !s.allowed(p) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return nil, false
		}
	}
	return paths, true
}

func (s *Server) file(w http.ResponseWriter, r *http.Request) {
	if paths, ok := s.check(w, r); ok {
		http.ServeFile(w, r, paths[0])
	}
}

func (s *Server) thumb(w http.ResponseWriter, r *http.Request) {
	paths, ok := s.check(w, r)
	if !ok {
		return
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("s"))
	if size < 64 || size > 4096 {
		size = 400
	}
	out, err := s.thumbs.Get(size, paths...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Cache-Control", "max-age=86400")
	http.ServeFile(w, r, out)
}
