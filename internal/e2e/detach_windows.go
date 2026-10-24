//go:build windows

package e2e

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func detachControllingTTY(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NO_WINDOW,
	}
}
