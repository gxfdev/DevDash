package terminal

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type resizeMsg struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

type Session struct {
	ptmx   *os.File
	cmd    *exec.Cmd
	conn   *websocket.Conn
	closed bool
	mu     sync.Mutex
}

func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	if s.conn != nil {
		s.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		s.conn.Close()
	}
	if s.ptmx != nil {
		s.ptmx.Close()
	}
	if s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd.Wait()
	}
}

func (s *Session) safeWrite(msg []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.conn == nil {
		return
	}
	s.conn.WriteMessage(websocket.BinaryMessage, msg)
}

func HandleTerminal(c *gin.Context) {
	shell := c.Query("shell")
	if shell == "" {
		shell = "/bin/bash"
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[terminal] upgrade failed: %v", err)
		return
	}

	ptmx, cmd, err := StartPty(shell, 24, 80)
	if err != nil {
		log.Printf("[terminal] pty start failed: %v", err)
		conn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[1;31m✗ 终端不可用（仅支持 Linux）\x1b[0m\r\n"))
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(4500, "pty not available"))
		conn.Close()
		return
	}

	sess := &Session{ptmx: ptmx, cmd: cmd, conn: conn}
	defer sess.Close()

	done := make(chan struct{})

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				select {
				case <-done:
				default:
					close(done)
				}
				return
			}
			sess.safeWrite(buf[:n])
		}
	}()

	conn.SetReadDeadline(time.Time{})
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			select {
			case <-done:
			default:
				close(done)
			}
			return
		}

		if msgType == websocket.TextMessage {
			var rm resizeMsg
			if json.Unmarshal(data, &rm) == nil && rm.Rows > 0 && rm.Cols > 0 {
				ResizePty(ptmx, rm.Rows, rm.Cols)
				continue
			}
		}

		if msgType == websocket.BinaryMessage || msgType == websocket.TextMessage {
			sess.mu.Lock()
			if _, writeErr := ptmx.Write(data); writeErr != nil {
				sess.mu.Unlock()
				select {
				case <-done:
				default:
					close(done)
				}
				return
			}
			sess.mu.Unlock()
		}
	}
}
