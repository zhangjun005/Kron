//go:build windows

package main

import "syscall"

const createNoWindow = 0x08000000

// hiddenProcAttr keeps a child console process (the `kron` CLI) from opening a
// console window. A windowsgui-subsystem GUI has no console of its own, so
// Windows would otherwise create one per child — visible as "several windows
// pop up when I switch to live".
func hiddenProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
