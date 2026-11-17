// Package proc holds small helpers for running external tools.
package proc

import (
	"os/exec"
	"syscall"
)

// Hide stops a console window from flashing up when a GUI app starts a
// command-line tool.
func Hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
