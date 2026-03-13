//go:build windows
// +build windows

package provider

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
