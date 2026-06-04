//go:build windows

package terminal

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type ptyProcess struct {
	cpty   *conPtyProcess
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.Reader
	pid    int
	mu     sync.Mutex
	closed bool
}

func createPtyProcess(shell string, cols, rows int16) (*ptyProcess, error) {
	// Try ConPTY first (native Windows pseudo console)
	cpty, err := createConPtyProcess(shell, cols, rows)
	if err != nil {
		log.Printf("[terminal] ConPTY failed: %v, falling back to pipe mode", err)
		return createPipeProcess(shell)
	}

	return &ptyProcess{
		cpty: cpty,
		pid:  cpty.Pid(),
	}, nil
}

func createPipeProcess(shell string) (*ptyProcess, error) {
	lower := strings.ToLower(shell)
	var cmd *exec.Cmd

	if strings.Contains(lower, "cmd") {
		cmd = exec.Command(shell, "/U", "/K", "prompt $P$G")
	} else if strings.Contains(lower, "powershell") || strings.Contains(lower, "pwsh") {
		// Use interactive mode with prompt for PowerShell in pipe mode
		cmd = exec.Command(shell, "-NoExit", "-Command", "prompt 'PS $PWD> '")
	} else {
		cmd = exec.Command(shell)
	}

	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start command: %w", err)
	}

	// Use a pipe to merge stdout and stderr
	mergedReader, mergedWriter := io.Pipe()
	go func() {
		io.Copy(mergedWriter, stdout)
		mergedWriter.Close()
	}()
	go func() {
		io.Copy(mergedWriter, stderr)
	}()

	return &ptyProcess{
		cmd:    cmd,
		stdin:  stdin,
		stdout: mergedReader,
		pid:    cmd.Process.Pid,
	}, nil
}

func (p *ptyProcess) Read(buf []byte) (int, error) {
	if p.cpty != nil {
		return p.cpty.Read(buf)
	}
	return p.stdout.Read(buf)
}

func (p *ptyProcess) Write(data []byte) (int, error) {
	if p.cpty != nil {
		return p.cpty.Write(data)
	}
	return p.stdin.Write(data)
}

func (p *ptyProcess) Resize(cols, rows int16) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	if p.cpty != nil {
		return p.cpty.Resize(cols, rows)
	}
	return nil
}

func (p *ptyProcess) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed = true

	if p.cpty != nil {
		p.cpty.Close()
		return
	}
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
	}
}

func (p *ptyProcess) Wait(ctx context.Context) error {
	if p.cpty != nil {
		return p.cpty.Wait(ctx)
	}
	if p.cmd != nil {
		return p.cmd.Wait()
	}
	return nil
}

func (p *ptyProcess) Pid() int {
	return p.pid
}

func (p *ptyProcess) IsConPty() bool {
	return p.cpty != nil
}

func buildWindowsCmdArgs(shell string) []string {
	lower := strings.ToLower(shell)
	if strings.Contains(lower, "cmd") {
		return []string{shell, "/U", "/K", "prompt $P$G"}
	}
	if strings.Contains(lower, "powershell") || strings.Contains(lower, "pwsh") {
		return []string{shell, "-NoLogo", "-NoProfile"}
	}
	return []string{shell}
}
