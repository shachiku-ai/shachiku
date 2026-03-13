//go:build !windows
// +build !windows

package skills

import (
	"os/exec"
)

func configureCmd(cmd *exec.Cmd) {
	// No-op on non-Windows OS
}
