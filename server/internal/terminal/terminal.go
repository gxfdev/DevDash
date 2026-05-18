package terminal

import (
	"log"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type TerminalSession struct {
	NodeID string
	Conn   *websocket.Conn
	cmd    *exec.Cmd
	stdin  interface {
		Write(p []byte) (n int, err error)
		Close() error
	}
	mu     sync.Mutex
	closed bool
}

func NewSession(nodeID string, conn *websocket.Conn) *TerminalSession {
	return &TerminalSession{NodeID: nodeID, Conn: conn}
}

func (s *TerminalSession) Handle() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe")
	} else {
		cmd = exec.Command("/bin/bash")
		if _, err := os.Stat("/bin/bash"); os.IsNotExist(err) {
			cmd = exec.Command("/bin/sh")
		}
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("[terminal] stdin pipe error: %v", err)
		s.Conn.WriteMessage(websocket.TextMessage, []byte("\r\nError: "+err.Error()+"\r\n"))
		return
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("[terminal] stdout pipe error: %v", err)
		s.Conn.WriteMessage(websocket.TextMessage, []byte("\r\nError: "+err.Error()+"\r\n"))
		return
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("[terminal] stderr pipe error: %v", err)
		s.Conn.WriteMessage(websocket.TextMessage, []byte("\r\nError: "+err.Error()+"\r\n"))
		return
	}

	s.cmd = cmd
	s.stdin = stdinPipe

	if err := cmd.Start(); err != nil {
		log.Printf("[terminal] start error: %v", err)
		s.Conn.WriteMessage(websocket.TextMessage, []byte("\r\nError: "+err.Error()+"\r\n"))
		return
	}

	s.Conn.WriteMessage(websocket.TextMessage, []byte("\x1b[1;32mDevDash Terminal Connected\r\n\x1b[0m"))

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := stdoutPipe.Read(buf)
			if err != nil {
				s.safeClose()
				return
			}
			if n > 0 {
				s.mu.Lock()
				if !s.closed {
					s.Conn.WriteMessage(websocket.TextMessage, buf[:n])
				}
				s.mu.Unlock()
			}
		}
	}()

	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := stderrPipe.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				s.mu.Lock()
				if !s.closed {
					s.Conn.WriteMessage(websocket.TextMessage, buf[:n])
				}
				s.mu.Unlock()
			}
		}
	}()

	go func() {
		for {
			_, msg, err := s.Conn.ReadMessage()
			if err != nil {
				s.safeClose()
				return
			}
			if len(msg) > 0 {
				data := string(msg)
				if data == "\x04" {
					stdinPipe.Write([]byte("exit\r\n"))
					time.Sleep(100 * time.Millisecond)
					s.safeClose()
					return
				}
				stdinPipe.Write(msg)
			}
		}
	}()

	wg.Wait()
}

func (s *TerminalSession) safeClose() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
	s.Conn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[1;31mDisconnected\x1b[0m\r\n"))
	s.Conn.Close()
}

func (s *TerminalSession) Write(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.Conn.WriteMessage(websocket.TextMessage, data)
	}
}
