//go:build !windows

package terminal

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

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

// canNsenter 检测当前容器环境是否支持 nsenter 进入主机命名空间
func canNsenter() bool {
	if !hostpath.Enabled() {
		return false
	}
	// 快速检测：检查能否读取 /proc/1 的信息
	_, err := os.Stat("/proc/1/ns/mnt")
	if err != nil {
		return false
	}
	// 检查 nsenter 是否可用
	cmd := exec.Command("nsenter", "--help")
	err = cmd.Run()
	if err != nil {
		log.Printf("[terminal] nsenter not available: %v", err)
		return false
	}
	return true
}

// createPtyProcess 创建 PTY 进程
// 优先尝试 nsenter（进入主机），失败后自动降级到容器本地 shell
func createPtyProcess(shell string, cols, rows int16) (*ptyProcess, error) {
	var cmd *exec.Cmd
	var useNsenter bool

	// 确定最终使用的 shell
	finalShell := resolveContainerShell(shell)

	// 尝试 nsenter 模式（仅当 HOST_ROOT 已配置且容器有权限时）
	useNsenter = canNsenter()
	if useNsenter {
		nsShell := shell
		if nsShell == "" {
			nsShell = "/bin/sh"
		}
		cmd = exec.Command("nsenter", "-m", "-u", "-i", "-n", "-p", "-t", "1", nsShell)
		log.Printf("[terminal] using nsenter mode (host namespace), shell=%s", nsShell)
	} else {
		cmd = exec.Command(finalShell)
		log.Printf("[terminal] using local mode (container shell), shell=%s", finalShell)
	}

	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLUMNS="+fmt.Sprintf("%d", cols),
		"LINES="+fmt.Sprintf("%d", rows),
		"HOME="+homeDir(),
	)

	// 设置进程组，避免信号传播问题
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setctty: true,
		Setsid:  true,
	}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
	if err != nil {
		// 如果是 nsenter 模式失败，自动降级到本地模式重试
		if useNsenter {
			log.Printf("[terminal] nsenter pty start failed (%v), falling back to local shell", err)
			return createPtyLocal(finalShell, cols, rows)
		}
		return nil, fmt.Errorf("pty start: %w", err)
	}

	return &ptyProcess{
		ptmx: ptmx,
		cmd:  cmd,
		pid:  cmd.Process.Pid,
	}, nil
}

// createPtyLocal 在容器内创建本地 PTY（不使用 nsenter）
func createPtyLocal(shell string, cols, rows int16) (*ptyProcess, error) {
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLUMNS="+fmt.Sprintf("%d", cols),
		"LINES="+fmt.Sprintf("%d", rows),
		"HOME="+homeDir(),
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
		return nil, fmt.Errorf("local pty start: %w", err)
	}

	log.Printf("[terminal] fallback to local mode success, pid=%d shell=%s", cmd.Process.Pid, shell)
	return &ptyProcess{
		ptmx: ptmx,
		cmd:  cmd,
		pid:  cmd.Process.Pid,
	}, nil
}

// resolveContainerShell 解析容器内可用的 shell 路径
func resolveContainerShell(preferred string) string {
	if preferred != "" && isAllowedShell(preferred) {
		if _, err := os.Stat(preferred); err == nil {
			return preferred
		}
	}

	// 容器内的 shell 候选列表（按优先级排序）
	candidates := []string{
		os.Getenv("DEVDASH_SHELL"),
		os.Getenv("SHELL"),
		"/bin/bash",
		"/usr/bin/bash",
		"/bin/sh",
		"/usr/bin/sh",
		"/bin/dash",
		"/usr/bin/dash",
		"/bin/ash", // Alpine 默认
		"/usr/bin/ash",
		"/bin/busybox", // Alpine busybox sh
	}

	for _, c := range candidates {
		if c == "" {
			continue
		}
		if isAllowedShell(c) {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}

	// 最终 fallback：直接用 /bin/sh（几乎所有 Linux 都有）
	return "/bin/sh"
}

// homeDir 返回有效的 HOME 目录
func homeDir() string {
	h := os.Getenv("HOME")
	if h != "" {
		if _, err := os.Stat(h); err == nil {
			return h
		}
	}
	// 容器环境下常见 home
	if u := os.Getenv("USER"); u != "" {
		candidate := "/home/" + u
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "/root"
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
		pid := p.cmd.Process.Pid
		// 先发 SIGTERM 让进程优雅退出
		p.cmd.Process.Signal(syscall.SIGTERM)

		// 等待进程退出（最多2秒）
		done := make(chan error, 1)
		go func() {
			done <- p.cmd.Wait()
		}()

		select {
		case <-done:
			log.Printf("[terminal] process %d exited gracefully", pid)
		case <-time.After(2 * time.Second):
			// 超时强制 kill
			p.cmd.Process.Signal(syscall.SIGKILL)
			log.Printf("[terminal] process %d killed after timeout", pid)
		}
	}

	// 关闭 PTY 主端
	if p.ptmx != nil {
		p.ptmx.Close()
	}
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
