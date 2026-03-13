//go:build windows
// +build windows

package skills

import (
	"os/exec"
	"syscall"
)

func configureCmd(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
}
