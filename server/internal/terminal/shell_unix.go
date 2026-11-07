//go:build !windows

package terminal

import (
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

func hostShell() (string, []string) {
	if os.Getenv("AIDASH_HOST_NS") == "1" {
		if p, err := exec.LookPath("nsenter"); err == nil {
			return p, []string{
				"--target", "1", "--mount", "--uts", "--ipc", "--net", "--pid", "--",
				"/bin/sh", "-lc", "command -v bash >/dev/null 2>&1 && exec bash -l || exec sh -l",
			}
		}
	}
	return DefaultShell()
}

func startUnix(cols, rows int) (*Session, error) {
	shell, args := hostShell()
	cmd := exec.Command(shell, args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	cols, rows = resolvedDims(cols, rows)
	win := &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)}
	ptmx, err := pty.StartWithSize(cmd, win)
	if err != nil {
		return nil, err
	}
	s := &Session{
		rw:  ptmx,
		cmd: cmd,
		resize: func(c, r int) {
			c, r = resolvedDims(c, r)
			_ = pty.Setsize(ptmx, &pty.Winsize{Cols: uint16(c), Rows: uint16(r)})
		},
	}
	s.close = func() error {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		return ptmx.Close()
	}
	return s, nil
}

var _ io.ReadWriteCloser = (*Session)(nil)

func startPlatform(cols, rows int) (*Session, error) { return startUnix(cols, rows) }
