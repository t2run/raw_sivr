package video

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"

	"rawsivr/internal/proc"
)

// Mpv runs mpv.exe as a child process that draws into a child window of the
// app window, and talks to it over mpv's JSON IPC pipe. The child window is
// disabled so it never takes mouse or keyboard focus from the UI.
type Mpv struct {
	exe     string
	onEvent func(name string, value any)

	mu   sync.Mutex
	host uintptr
	cmd  *exec.Cmd
	conn net.Conn
}

var (
	user32            = syscall.NewLazyDLL("user32.dll")
	pFindWindow       = user32.NewProc("FindWindowW")
	pCreateWindowEx   = user32.NewProc("CreateWindowExW")
	pSetWindowPos     = user32.NewProc("SetWindowPos")
	pShowWindow       = user32.NewProc("ShowWindow")
	pGetMessage       = user32.NewProc("GetMessageW")
	pTranslateMessage = user32.NewProc("TranslateMessage")
	pDispatchMessage  = user32.NewProc("DispatchMessageW")
)

const (
	wsChild        = 0x40000000
	wsDisabled     = 0x08000000
	wsClipSiblings = 0x04000000
	wsClipChildren = 0x02000000
	swpNoActivate  = 0x0010
	swpShowWindow  = 0x0040
)

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	x, y    int32
}

// FindMpv returns the path of mpv.exe, or "" when it is not installed.
func FindMpv() string {
	if p, err := exec.LookPath("mpv"); err == nil {
		return p
	}
	for _, p := range []string{
		filepath.Join(os.Getenv("ProgramFiles"), "MPV Player", "mpv.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "mpv", "mpv.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "mpv", "mpv.exe"),
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func NewMpv(onEvent func(string, any)) *Mpv {
	return &Mpv{exe: FindMpv(), onEvent: onEvent}
}

func (m *Mpv) Available() bool { return m.exe != "" }

// start creates the host window inside the window with the given title and
// launches mpv into it. It does nothing when mpv is already running.
func (m *Mpv) start(windowTitle string) error {
	if m.conn != nil {
		return nil
	}
	if m.exe == "" {
		return errors.New("mpv is not installed")
	}
	if m.host == 0 {
		title, _ := syscall.UTF16PtrFromString(windowTitle)
		parent, _, _ := pFindWindow.Call(0, uintptr(unsafe.Pointer(title)))
		if parent == 0 {
			return errors.New("app window not found")
		}
		// A window belongs to the thread that created it, and that thread
		// has to keep pumping its messages.
		ready := make(chan uintptr)
		go func() {
			runtime.LockOSThread()
			class, _ := syscall.UTF16PtrFromString("Static")
			h, _, _ := pCreateWindowEx.Call(0, uintptr(unsafe.Pointer(class)), 0,
				wsChild|wsDisabled|wsClipSiblings|wsClipChildren, 0, 0, 16, 16, parent, 0, 0, 0)
			ready <- h
			if h == 0 {
				return
			}
			var msg winMsg
			for {
				r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
				if int32(r) <= 0 {
					return
				}
				pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
				pDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
			}
		}()
		if m.host = <-ready; m.host == 0 {
			return errors.New("could not create the video window")
		}
	}

	pipe := fmt.Sprintf(`\\.\pipe\rawsivr-mpv-%d-%d`, os.Getpid(), time.Now().UnixNano())
	cmd := exec.Command(m.exe, "--no-config", "--idle=yes", "--force-window=yes",
		"--wid="+strconv.FormatUint(uint64(m.host), 10), "--input-ipc-server="+pipe,
		"--hwdec=auto-safe", "--keep-open=yes", "--osc=no", "--osd-level=0",
		"--input-default-bindings=no", "--input-vo-keyboard=no", "--input-cursor=no", "--cursor-autohide=no")
	proc.Hide(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	var conn net.Conn
	var err error
	for i := 0; i < 50; i++ {
		timeout := 200 * time.Millisecond
		if conn, err = winio.DialPipe(pipe, &timeout); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("mpv did not start: %w", err)
	}
	m.cmd, m.conn = cmd, conn
	go m.read(conn, cmd)
	for i, prop := range []string{"time-pos", "duration", "pause", "frame-drop-count", "hwdec-current", "eof-reached"} {
		m.send("observe_property", i+1, prop)
	}
	return nil
}

func (m *Mpv) read(conn net.Conn, cmd *exec.Cmd) {
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	var lastPos time.Time
	for sc.Scan() {
		var ev struct {
			Event string `json:"event"`
			Name  string `json:"name"`
			Data  any    `json:"data"`
		}
		if json.Unmarshal(sc.Bytes(), &ev) != nil || ev.Event != "property-change" || m.onEvent == nil {
			continue
		}
		if ev.Name == "time-pos" { // arrives once per frame; ten a second is plenty
			if time.Since(lastPos) < 100*time.Millisecond {
				continue
			}
			lastPos = time.Now()
		}
		m.onEvent(ev.Name, ev.Data)
	}
	cmd.Wait()
	m.mu.Lock()
	if m.conn == conn {
		m.conn, m.cmd = nil, nil
	}
	m.mu.Unlock()
}

func (m *Mpv) send(args ...any) error {
	if m.conn == nil {
		return errors.New("mpv is not running")
	}
	b, _ := json.Marshal(map[string]any{"command": args})
	_, err := m.conn.Write(append(b, '\n'))
	return err
}

// Play loads a file into the player, starting mpv when needed.
func (m *Mpv) Play(windowTitle, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.start(windowTitle); err != nil {
		return err
	}
	m.send("set_property", "pause", false)
	return m.send("loadfile", path)
}

// SetRect places the video inside the app window, in physical pixels.
func (m *Mpv) SetRect(x, y, w, h int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.host != 0 {
		pSetWindowPos.Call(m.host, 0 /* HWND_TOP */, uintptr(x), uintptr(y), uintptr(w), uintptr(h), swpNoActivate|swpShowWindow)
	}
}

// Hide stops playback, releases the file and hides the video.
func (m *Mpv) Hide() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.host != 0 {
		pShowWindow.Call(m.host, 0)
	}
	if m.conn != nil {
		m.send("stop")
	}
}

func (m *Mpv) Pause(paused bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.send("set_property", "pause", paused)
}

func (m *Mpv) Seek(seconds float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.send("seek", seconds, "absolute")
}

// Close shuts mpv down.
func (m *Mpv) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.conn != nil {
		m.send("quit")
		m.conn.Close()
	}
	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Kill()
	}
}
