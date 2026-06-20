//go:build !windows

package terminal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"github.com/gxfdev/DevDash/server/internal/hostpath"
)

type ptyProcess struct {
	ptmx   *os.File
	cmd    *exec.Cmd
	pid    int
	mu     sync.Mutex
	closed bool
}

func createPtyProcess(shell string, cols, rows int16) (*ptyProcess, error) {
	var cmd *exec.Cmd
	if hostpath.Enabled() {
		// 容器内：通过 nsenter 进入主机命名空间执行 shell
		// 使用 /bin/sh 作为fallback，因为所有Linux发行版都有
		// 如果请求的shell在主机上不存在，nsenter会失败
		nsenterShell := shell
		if nsenterShell == "" {
			nsenterShell = "/bin/sh"
		}
		cmd = exec.Command("nsenter", "-m", "-u", "-i", "-n", "-p", "-t", "1", nsenterShell)
	} else {
		cmd = exec.Command(shell)
	}
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLUMNS="+fmt.Sprintf("%d", cols),
		"LINES="+fmt.Sprintf("%d", rows),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setctty: true,
		Setsid:  true,
	}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
	if err != nil {
		return nil, fmt.Errorf("pty start: %w", err)
	}

	return &ptyProcess{
		ptmx: ptmx,
		cmd:  cmd,
		pid:  cmd.Process.Pid,
	}, nil
}

func (p *ptyProcess) Read(buf []byte) (int, error) {
	return p.ptmx.Read(buf)
}

func (p *ptyProcess) Write(data []byte) (int, error) {
	return p.ptmx.Write(data)
}

func (p *ptyProcess) Resize(cols, rows int16) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	return pty.Setsize(p.ptmx, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
}

func (p *ptyProcess) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed = true

	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Signal(syscall.SIGTERM)
	}
	p.ptmx.Close()
}

func (p *ptyProcess) Wait(ctx context.Context) error {
	if p.cmd != nil {
		done := make(chan error, 1)
		go func() {
			done <- p.cmd.Wait()
		}()
		select {
		case err := <-done:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (p *ptyProcess) Pid() int {
	return p.pid
}

func (p *ptyProcess) IsConPty() bool {
	return true
}
