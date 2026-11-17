// Package trash sends files to the operating system's recycle bin.
package trash

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	foDelete          = 3
	fofSilent         = 0x0004
	fofNoConfirmation = 0x0010
	fofAllowUndo      = 0x0040
	fofNoErrorUI      = 0x0400
)

// SHFILEOPSTRUCTW (64-bit layout).
type shFileOp struct {
	hwnd      uintptr
	wFunc     uint32
	pFrom     *uint16
	pTo       *uint16
	fFlags    uint16
	aborted   int32
	nameMaps  uintptr
	progTitle *uint16
}

var shFileOperation = syscall.NewLazyDLL("shell32.dll").NewProc("SHFileOperationW")

// Recycle moves one file to the Recycle Bin.
func Recycle(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	from, err := syscall.UTF16FromString(abs)
	if err != nil {
		return err
	}
	from = append(from, 0) // the list must end with a double NUL
	op := shFileOp{
		wFunc:  foDelete,
		pFrom:  &from[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofSilent | fofNoErrorUI,
	}
	ret, _, _ := shFileOperation.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return fmt.Errorf("recycle %s: shell error 0x%x", abs, ret)
	}
	if op.aborted != 0 {
		return fmt.Errorf("recycle %s: aborted", abs)
	}
	return nil
}
