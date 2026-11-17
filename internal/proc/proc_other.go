//go:build !windows

// Package proc holds small helpers for running external tools.
package proc

import "os/exec"

// Hide is only needed on Windows.
func Hide(cmd *exec.Cmd) {}
