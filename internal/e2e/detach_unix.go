//go:build !windows

package e2e

import (
	"os/exec"
	"syscall"
)

func detachControllingTTY(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
