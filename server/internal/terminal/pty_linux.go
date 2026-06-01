//go:build linux

package terminal

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

func StartPty(shell string, rows, cols uint16) (*os.File, *exec.Cmd, error) {
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
	}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
	if err != nil {
		return nil, nil, err
	}
	return ptmx, cmd, nil
}

func ResizePty(f *os.File, rows, cols uint16) error {
	return pty.Setsize(f, &pty.Winsize{Rows: rows, Cols: cols})
}
