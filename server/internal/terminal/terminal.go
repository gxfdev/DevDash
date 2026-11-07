package terminal

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
)

// Session is a duplex host shell used by the browser terminal.
type Session struct {
	rw     io.ReadWriteCloser
	resize func(cols, rows int)
	close  func() error
	cmd    *exec.Cmd
}

func (s *Session) Read(p []byte) (int, error)  { return s.rw.Read(p) }
func (s *Session) Write(p []byte) (int, error) { return s.rw.Write(p) }
func (s *Session) Resize(cols, rows int) {
	if s.resize != nil {
		s.resize(cols, rows)
	}
}
func (s *Session) Close() error {
	err1 := s.rw.Close()
	if s.close != nil {
		_ = s.close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return err1
}

// DefaultShell returns the shell binary and arguments for the platform.
func DefaultShell() (string, []string) {
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("pwsh"); err == nil {
			return "pwsh", []string{"-NoLogo"}
		}
		return "powershell.exe", []string{"-NoLogo", "-NoProfile"}
	}
	sh := os.Getenv("SHELL")
	if sh == "" {
		if _, err := os.Stat("/bin/bash"); err == nil {
			sh = "/bin/bash"
		} else {
			sh = "/bin/sh"
		}
	}
	return sh, []string{"-l"}
}

// Start launches the host shell. A real PTY is used on Unix; Windows
// falls back to a pipe-backed PowerShell (ConPTY-free, no CGO).
func Start(cols, rows int) (*Session, error) {
	return startPlatform(cols, rows)
}

func resolvedDims(cols, rows int) (int, int) {
	if cols <= 0 {
		cols = 120
	}
	if rows <= 0 {
		rows = 30
	}
	return cols, rows
}

var errClosed = fmt.Errorf("terminal closed")
