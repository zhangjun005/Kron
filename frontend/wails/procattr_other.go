//go:build !windows

package main

import "syscall"

// hiddenProcAttr is a no-op on non-Windows platforms: there is no console
// window to suppress when a GUI spawns a child process.
func hiddenProcAttr() *syscall.SysProcAttr {
	return nil
}
