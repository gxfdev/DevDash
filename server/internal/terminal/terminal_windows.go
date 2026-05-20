//go:build windows

package terminal

import (
	"os/exec"
)

func setSysProcAttr(cmd *exec.Cmd) {
}