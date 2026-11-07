//go:build windows

package terminal

import (
	"io"
	"os/exec"
	"sync"
)

func startWindows(cols, rows int) (*Session, error) {
	shell, args := DefaultShell()
	cmd := exec.Command(shell, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	go func() {
		buf := make([]byte, 4096)
		for {
			n, rerr := stdout.Read(buf)
			if n > 0 {
				_, _ = pw.Write(buf[:n])
			}
			if rerr != nil {
				break
			}
		}
		for {
			n, rerr := stderr.Read(buf)
			if n > 0 {
				_, _ = pw.Write(buf[:n])
			}
			if rerr != nil {
				break
			}
		}
		_ = pw.Close()
	}()
	rw := &windowsRW{Reader: pr, w: stdin, once: &sync.Once{}}
	return &Session{
		rw:     rw,
		cmd:    cmd,
		resize: func(cols, rows int) {},
	}, nil
}

type windowsRW struct {
	io.Reader
	w    io.WriteCloser
	once *sync.Once
}

func (w *windowsRW) Write(p []byte) (int, error) { return w.w.Write(p) }
func (w *windowsRW) Close() error {
	var err error
	w.once.Do(func() { err = w.w.Close() })
	return err
}

func startPlatform(cols, rows int) (*Session, error) { return startWindows(cols, rows) }
