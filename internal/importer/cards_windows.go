package importer

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	pGetLogicalDrives = kernel32.NewProc("GetLogicalDrives")
	pGetDriveType     = kernel32.NewProc("GetDriveTypeW")
	pGetVolumeInfo    = kernel32.NewProc("GetVolumeInformationW")
)

const driveRemovable = 2

// Cards returns the removable drives that look like a camera card (they
// have a DCIM folder), as path and display label.
func Cards() map[string]string {
	out := map[string]string{}
	mask, _, _ := pGetLogicalDrives.Call()
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		ptr, _ := syscall.UTF16PtrFromString(root)
		if t, _, _ := pGetDriveType.Call(uintptr(unsafe.Pointer(ptr))); t != driveRemovable {
			continue
		}
		if info, err := os.Stat(root + "DCIM"); err != nil || !info.IsDir() {
			continue
		}
		label := root[:2]
		var name [261]uint16
		if ok, _, _ := pGetVolumeInfo.Call(uintptr(unsafe.Pointer(ptr)), uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)), 0, 0, 0, 0, 0); ok != 0 {
			if v := syscall.UTF16ToString(name[:]); v != "" {
				label = v + " (" + root[:2] + ")"
			}
		}
		out[root] = label
	}
	return out
}
