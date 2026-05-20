package terminal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 8192
)

type TerminalSession struct {
	NodeID string
	Conn   *websocket.Conn
	cmd    *exec.Cmd
	mu     sync.Mutex
	closed int32
	ctx    context.Context
	cancel context.CancelFunc
}

func NewSession(nodeID string, conn *websocket.Conn) *TerminalSession {
	ctx, cancel := context.WithCancel(context.Background())
	return &TerminalSession{
		NodeID: nodeID,
		Conn:   conn,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *TerminalSession) Handle() {
	defer func() {
		s.cancel()
		s.safeClose()
	}()

	s.Conn.SetReadLimit(maxMessageSize)
	s.Conn.SetReadDeadline(time.Now().Add(pongWait))
	s.Conn.SetPongHandler(func(string) error {
		s.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	cmd, stdin, stdout, stderr, err := s.createCommand()
	if err != nil {
		s.sendMsg("\r\n\x1b[1;31mError: " + err.Error() + "\x1b[0m\r\n")
		return
	}
	s.cmd = cmd

	if err := cmd.Start(); err != nil {
		s.sendMsg("\r\n\x1b[1;31mFailed to start shell: " + err.Error() + "\x1b[0m\r\n")
		return
	}

	s.sendMsg("\x1b[1;32mDevDash Terminal Connected (" + runtime.GOOS + ")\r\n\x1b[0m")

	var wg sync.WaitGroup
	wg.Add(4)

	go s.pinger(&wg)
	go s.streamReader(stdout, &wg, false)
	go s.streamReader(stderr, &wg, true)
	go s.handleInput(stdin, &wg)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("[terminal] session ended normally")
	case <-s.ctx.Done():
		log.Printf("[terminal] session cancelled")
	}

	time.Sleep(100 * time.Millisecond)
}

func (s *TerminalSession) createCommand() (*exec.Cmd, io.WriteCloser, io.Reader, io.Reader, error) {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe")
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
		)
	} else {
		shell := "/bin/bash"
		if _, err := os.Stat(shell); os.IsNotExist(err) {
			shell = "/bin/sh"
		}
		cmd = exec.Command(shell)
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLUMNS=120",
			"LINES=30",
		)
		setSysProcAttr(cmd)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("stderr pipe: %w", err)
	}

	return cmd, stdin, stdout, stderr, nil
}

func (s *TerminalSession) streamReader(r io.Reader, wg *sync.WaitGroup, isStderr bool) {
	defer wg.Done()

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), maxMessageSize)

	for scanner.Scan() {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		text := scanner.Text()
		if runtime.GOOS == "windows" && !isStderr {
			text = text + "\r\n"
		} else if !isStderr {
			text = text + "\n"
		}

		s.mu.Lock()
		if atomic.LoadInt32(&s.closed) == 0 {
			if err := s.Conn.WriteMessage(websocket.TextMessage, []byte(text)); err != nil {
				log.Printf("[terminal] write error: %v", err)
				s.mu.Unlock()
				s.cancel()
				return
			}
		}
		s.mu.Unlock()
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[terminal] scanner error (%v): %v", map[bool]string{true: "stderr", false: "stdout"}[isStderr], err)
	}
}

func (s *TerminalSession) handleInput(w io.WriteCloser, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		if w != nil {
			w.Close()
		}
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		_, msg, err := s.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[terminal] read error: %v", err)
			}
			return
		}

		if len(msg) > 0 {
			data := string(msg)

			switch data {
			case "\x04":
				w.Close()
				time.Sleep(150 * time.Millisecond)
				s.cancel()
				return
			case "\x03":
				if s.cmd != nil && s.cmd.Process != nil {
					s.cmd.Process.Signal(os.Interrupt)
				}
				continue
			}

			if runtime.GOOS == "windows" {
				data = strings.ReplaceAll(data, "\r\n", "\n")
				data = strings.ReplaceAll(data, "\r", "\n")
				if !strings.HasSuffix(data, "\n") {
					data = data + "\r\n"
				} else {
					data = strings.TrimSuffix(data, "\n") + "\r\n"
				}
			}

			if _, err := w.Write([]byte(data)); err != nil {
				log.Printf("[terminal] stdin write error: %v", err)
				return
			}
		}
	}
}

func (s *TerminalSession) pinger(wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if atomic.LoadInt32(&s.closed) == 1 {
				return
			}
			s.mu.Lock()
			err := s.Conn.WriteMessage(websocket.PingMessage, nil)
			s.mu.Unlock()
			if err != nil {
				log.Printf("[terminal] ping error: %v", err)
				s.cancel()
				return
			}
		}
	}
}

func (s *TerminalSession) safeClose() {
	if !atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd != nil && s.cmd.Process != nil {
		if runtime.GOOS == "windows" {
			s.cmd.Process.Kill()
		} else {
			s.cmd.Process.Signal(os.Interrupt)
			time.Sleep(200 * time.Millisecond)
			s.cmd.Process.Kill()
		}
	}

	s.Conn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[1;31mDisconnected\x1b[0m\r\n"))
	s.Conn.Close()
}

func (s *TerminalSession) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if atomic.LoadInt32(&s.closed) == 0 {
		if err := s.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return 0, err
		}
		return len(data), nil
	}
	return 0, io.ErrClosedPipe
}

func (s *TerminalSession) sendMsg(msg string) {
	s.Conn.WriteMessage(websocket.TextMessage, []byte(msg))
}

func IsClosed(s *TerminalSession) bool {
	return atomic.LoadInt32(&s.closed) == 1
}