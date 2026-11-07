package orchestrator

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/gxfdev/aidash/server/internal/engine"
)

// execCapture runs a command inside a container and returns its merged
// output. emit (optional) receives output lines as they arrive.
func (o *Orchestrator) execCapture(ctx context.Context, containerID string, cmd []string, timeout time.Duration, emit func(string)) (string, error) {
	if containerID == "" {
		return "", errors.New("容器不存在")
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	execID, err := o.docker.ExecCreate(runCtx, containerID, engine.ExecCreateRequest{
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          cmd,
	})
	if err != nil {
		return "", err
	}
	stream, err := o.docker.ExecStart(runCtx, execID)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	done := make(chan struct{})
	go func() {
		select {
		case <-runCtx.Done():
			_ = stream.Close()
		case <-done:
		}
	}()
	defer close(done)

	var buf bytes.Buffer
	var pending strings.Builder
	chunk := make([]byte, 2048)
	for {
		n, err := stream.Read(chunk)
		if n > 0 {
			buf.Write(chunk[:n])
			if emit != nil {
				pending.WriteString(string(chunk[:n]))
				text := pending.String()
				for {
					idx := strings.IndexByte(text, '\n')
					if idx < 0 {
						break
					}
					line := strings.TrimRight(strings.ReplaceAll(text[:idx], "\r", ""), " ")
					if line != "" {
						emit(line)
					}
					text = text[idx+1:]
				}
				pending.Reset()
				pending.WriteString(text)
			}
			if buf.Len() > 1<<20 {
				break
			}
		}
		if err != nil {
			if runCtx.Err() != nil {
				break
			}
			if err == io.EOF || err.Error() == "EOF" {
				break
			}
			// Docker sometimes reports "unexpected EOF"; the exec has finished.
			if strings.Contains(strings.ToLower(err.Error()), "eof") {
				break
			}
			return buf.String(), err
		}
	}
	if emit != nil && pending.Len() > 0 {
		line := strings.TrimSpace(pending.String())
		if line != "" {
			emit(line)
		}
	}
	info, err := o.docker.ExecInspect(ctx, execID)
	if err == nil && info.ExitCode != 0 {
		return buf.String(), &execError{code: info.ExitCode, output: strings.TrimSpace(buf.String())}
	}
	return buf.String(), nil
}

type execError struct {
	code   int
	output string
}

func (e *execError) Error() string {
	out := e.output
	if len(out) > 300 {
		out = out[:300]
	}
	if out == "" {
		return "命令执行失败 (exit " + itoa(e.code) + ")"
	}
	return "命令执行失败 (exit " + itoa(e.code) + "): " + out
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [12]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// ExecInteractive starts a TTY shell inside a managed container and
// returns a duplex stream for the websocket bridge.
func (o *Orchestrator) ExecInteractive(ctx context.Context, serviceID string, cols, rows int) (io.ReadWriteCloser, error) {
	svc, err := o.store.GetService(serviceID)
	if err != nil {
		return nil, err
	}
	if svc.ContainerID == "" {
		return nil, errors.New("容器尚未创建")
	}
	ci, err := o.docker.InspectContainer(ctx, svc.ContainerID)
	if err != nil {
		return nil, err
	}
	if !ci.State.Running {
		return nil, errors.New("容器未运行，请先启动服务")
	}
	shell := []string{"/bin/sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh"}
	if strings.EqualFold(ci.Config.Image, "alpine") || strings.Contains(ci.Config.Image, "alpine") {
		shell = []string{"/bin/sh"}
	}
	execID, err := o.docker.ExecCreate(ctx, svc.ContainerID, engine.ExecCreateRequest{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          shell,
		Env:          []string{"TERM=xterm-256color"},
	})
	if err != nil {
		return nil, err
	}
	stream, err := o.docker.ExecStart(ctx, execID)
	if err != nil {
		return nil, err
	}
	if cols > 0 && rows > 0 {
		_ = o.docker.ExecResize(ctx, execID, rows, cols)
	}
	return &execSession{stream: stream, docker: o.docker, execID: execID}, nil
}

type execSession struct {
	stream io.ReadWriteCloser
	docker *engine.Client
	execID string
	once   sync.Once
}

func (e *execSession) Read(p []byte) (int, error)  { return e.stream.Read(p) }
func (e *execSession) Write(p []byte) (int, error) { return e.stream.Write(p) }
func (e *execSession) Close() error {
	var err error
	e.once.Do(func() { err = e.stream.Close() })
	return err
}

func (e *execSession) Resize(cols, rows int) {
	_ = e.docker.ExecResize(context.Background(), e.execID, rows, cols)
}

// RunCommand executes a one-shot shell command inside a managed service
// (used by the Copilot sandbox and the service console).
func (o *Orchestrator) RunCommand(ctx context.Context, serviceID, command string, timeout time.Duration) (string, error) {
	svc, err := o.store.GetService(serviceID)
	if err != nil {
		return "", err
	}
	if svc.ContainerID == "" {
		return "", errors.New("容器尚未创建")
	}
	if strings.TrimSpace(command) == "" {
		return "", errors.New("命令为空")
	}
	return o.execCapture(ctx, svc.ContainerID, []string{"sh", "-lc", command}, timeout, nil)
}

// Logs returns the demultiplexed tail of a managed service's logs.
func (o *Orchestrator) Logs(ctx context.Context, serviceID string, tail int) (string, error) {
	svc, err := o.store.GetService(serviceID)
	if err != nil {
		return "", err
	}
	if svc.ContainerID == "" {
		return "（容器尚未创建）", nil
	}
	return o.docker.ContainerLogsText(ctx, svc.ContainerID, tail)
}

func (o *Orchestrator) Stats(ctx context.Context, serviceID string) (*engine.ContainerStatView, error) {
	svc, err := o.store.GetService(serviceID)
	if err != nil {
		return nil, err
	}
	if svc.ContainerID == "" {
		return &engine.ContainerStatView{}, nil
	}
	return o.docker.ContainerStats(ctx, svc.ContainerID)
}

// StreamLogs follows a service's logs, invoking emit with raw chunks.
func (o *Orchestrator) StreamLogs(ctx context.Context, serviceID string, tail int, emit func(string)) error {
	svc, err := o.store.GetService(serviceID)
	if err != nil {
		return err
	}
	if svc.ContainerID == "" {
		return errors.New("容器尚未创建")
	}
	return o.docker.ContainerLogsStream(ctx, svc.ContainerID, tail, emit)
}
