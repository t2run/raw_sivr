// Package exif reads the few EXIF fields the app needs from JPEG files and
// TIFF-based RAW files (Sony ARW), without decoding any image data.
package exif

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	tagMake         = 0x010F
	tagModel        = 0x0110
	tagOrientation  = 0x0112
	tagJPEGOffset   = 0x0201
	tagJPEGLength   = 0x0202
	tagExifIFD      = 0x8769
	tagExposureTime = 0x829A
	tagFNumber      = 0x829D
	tagISO          = 0x8827
	tagDateOriginal = 0x9003
	tagFocalLength  = 0x920A
	tagSubSecOrig   = 0x9291
	tagLensModel    = 0xA434
)

type entry struct {
	typ   uint16
	count uint32
	val   []byte // the 4 value/offset bytes
}

type tiff struct {
	r    io.ReaderAt
	base int64
	bo   binary.ByteOrder
}

// open reads a TIFF header at base and returns the first directory.
func open(r io.ReaderAt, base int64) (*tiff, map[uint16]entry, error) {
	var hdr [8]byte
	if _, err := r.ReadAt(hdr[:], base); err != nil {
		return nil, nil, err
	}
	t := &tiff{r: r, base: base}
	switch string(hdr[:2]) {
	case "II":
		t.bo = binary.LittleEndian
	case "MM":
		t.bo = binary.BigEndian
	default:
		return nil, nil, errors.New("not a TIFF header")
	}
	dir, err := t.dir(t.bo.Uint32(hdr[4:]))
	return t, dir, err
}

func (t *tiff) dir(off uint32) (map[uint16]entry, error) {
	var nb [2]byte
	if _, err := t.r.ReadAt(nb[:], t.base+int64(off)); err != nil {
		return nil, err
	}
	n := int(t.bo.Uint16(nb[:]))
	buf := make([]byte, n*12)
	if _, err := t.r.ReadAt(buf, t.base+int64(off)+2); err != nil {
		return nil, err
	}
	out := make(map[uint16]entry, n)
	for i := 0; i < n; i++ {
		e := buf[i*12 : i*12+12]
		out[t.bo.Uint16(e)] = entry{typ: t.bo.Uint16(e[2:]), count: t.bo.Uint32(e[4:]), val: e[8:12]}
	}
	return out, nil
}

func (t *tiff) uint(e entry) uint32 {
	switch e.typ {
	case 3:
		return uint32(t.bo.Uint16(e.val))
	case 4:
		return t.bo.Uint32(e.val)
	}
	return 0
}

func (t *tiff) str(e entry) string {
	if e.typ != 2 || e.count == 0 || e.count > 1024 {
		return ""
	}
	buf := make([]byte, e.count)
	if e.count <= 4 {
		copy(buf, e.val)
	} else if _, err := t.r.ReadAt(buf, t.base+int64(t.bo.Uint32(e.val))); err != nil {
		return ""
	}
	return strings.TrimSpace(string(bytes.TrimRight(buf, "\x00")))
}

func (t *tiff) rational(e entry) (num, den uint32) {
	if e.typ != 5 || e.count == 0 {
		return 0, 0
	}
	var buf [8]byte
	if _, err := t.r.ReadAt(buf[:], t.base+int64(t.bo.Uint32(e.val))); err != nil {
		return 0, 0
	}
	return t.bo.Uint32(buf[:]), t.bo.Uint32(buf[4:])
}

// jpegTIFFBase finds the EXIF block inside a JPEG.
func jpegTIFFBase(r io.ReaderAt) (int64, bool) {
	head := make([]byte, 128<<10)
	n, _ := r.ReadAt(head, 0)
	head = head[:n]
	if len(head) < 4 || head[0] != 0xFF || head[1] != 0xD8 {
		return 0, false
	}
	for p := 2; p+4 <= len(head) && head[p] == 0xFF; {
		if head[p+1] == 0xE1 && p+10 <= len(head) && bytes.Equal(head[p+4:p+10], []byte("Exif\x00\x00")) {
			return int64(p + 10), true
		}
		if head[p+1] == 0xDA {
			break
		}
		p += 2 + int(binary.BigEndian.Uint16(head[p+2:]))
	}
	return 0, false
}

func orientation(t *tiff, dir map[uint16]entry) int {
	if o := t.uint(dir[tagOrientation]); o >= 1 && o <= 8 {
		return int(o)
	}
	return 1
}

// JPEGOrientation returns the EXIF orientation (1-8) of a JPEG, or 1.
func JPEGOrientation(r io.ReaderAt) int {
	base, ok := jpegTIFFBase(r)
	if !ok {
		return 1
	}
	t, dir, err := open(r, base)
	if err != nil {
		return 1
	}
	return orientation(t, dir)
}

// RawPreview returns the JPEG preview embedded in a TIFF-based RAW file
// (Sony ARW keeps it in the first directory) and the RAW's orientation.
func RawPreview(r io.ReaderAt) ([]byte, int, error) {
	t, dir, err := open(r, 0)
	if err != nil {
		return nil, 1, err
	}
	off, size := t.uint(dir[tagJPEGOffset]), t.uint(dir[tagJPEGLength])
	if off == 0 || size < 4 || size > 64<<20 {
		return nil, 1, errors.New("no embedded preview")
	}
	buf := make([]byte, size)
	if _, err := r.ReadAt(buf, int64(off)); err != nil {
		return nil, 1, err
	}
	if buf[0] != 0xFF || buf[1] != 0xD8 {
		return nil, 1, errors.New("embedded preview is not a JPEG")
	}
	return buf, orientation(t, dir), nil
}

// Info is the shooting data shown next to a photo. Fields are empty when
// the file does not record them.
type Info struct {
	Camera   string `json:"camera"`
	Lens     string `json:"lens"`
	Shutter  string `json:"shutter"`
	Aperture string `json:"aperture"`
	ISO      string `json:"iso"`
	Focal    string `json:"focal"`
	Taken    string `json:"taken"`

	// Time is Taken as a time (with fractions of a second when the camera
	// records them); zero when unknown.
	Time time.Time `json:"-"`
}

// Read returns the shooting data of a JPEG or RAW file.
func Read(path string) (Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return Info{}, err
	}
	defer f.Close()
	var base int64
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		b, ok := jpegTIFFBase(f)
		if !ok {
			return Info{}, errors.New("no EXIF data")
		}
		base = b
	}
	t, dir, err := open(f, base)
	if err != nil {
		return Info{}, err
	}
	info := Info{Camera: t.str(dir[tagModel])}
	if info.Camera == "" {
		info.Camera = t.str(dir[tagMake])
	}
	ptr, ok := dir[tagExifIFD]
	if !ok {
		return info, nil
	}
	ex, err := t.dir(t.uint(ptr))
	if err != nil {
		return info, nil
	}
	info.Lens = t.str(ex[tagLensModel])
	info.Taken = t.str(ex[tagDateOriginal])
	if tm, err := time.ParseInLocation("2006:01:02 15:04:05", info.Taken, time.Local); err == nil {
		if frac, err := strconv.ParseFloat("0."+t.str(ex[tagSubSecOrig]), 64); err == nil {
			tm = tm.Add(time.Duration(frac * float64(time.Second)))
		}
		info.Time = tm
	}
	if n, d := t.rational(ex[tagExposureTime]); n > 0 && d > 0 {
		if n >= d {
			info.Shutter = trimFloat(float64(n)/float64(d)) + " s"
		} else {
			info.Shutter = fmt.Sprintf("1/%.0f s", float64(d)/float64(n))
		}
	}
	if n, d := t.rational(ex[tagFNumber]); n > 0 && d > 0 {
		info.Aperture = "f/" + trimFloat(float64(n)/float64(d))
	}
	if iso := t.uint(ex[tagISO]); iso > 0 {
		info.ISO = fmt.Sprintf("ISO %d", iso)
	}
	if n, d := t.rational(ex[tagFocalLength]); n > 0 && d > 0 {
		info.Focal = trimFloat(float64(n)/float64(d)) + " mm"
	}
	return info, nil
}

func trimFloat(v float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.1f", v), "0"), ".")
}
