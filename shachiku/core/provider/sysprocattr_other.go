//go:build !windows
// +build !windows

package provider

import (
	"os/exec"
)

func configureCmd(cmd *exec.Cmd) {
	// No-op on non-Windows OS
}
