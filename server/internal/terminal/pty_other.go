//go:build !linux

package terminal

import (
	"errors"
	"os"
	"os/exec"
)

func StartPty(shell string, rows, cols uint16) (*os.File, *exec.Cmd, error) {
	return nil, nil, errors.New("terminal is only supported on Linux")
}

func ResizePty(f *os.File, rows, cols uint16) error {
	return errors.New("terminal is only supported on Linux")
}
