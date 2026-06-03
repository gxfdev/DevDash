package terminal

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 32768
	defaultCols    = 120
	defaultRows    = 30
)

type resizeMessage struct {
	Type string `json:"type"`
	Cols int16  `json:"cols"`
	Rows int16  `json:"rows"`
}

type CommandSaverFunc func(command string)

type TerminalSession struct {
	NodeID        string
	Shell         string
	Conn          *websocket.Conn
	proc          *ptyProcess
	mu            sync.Mutex
	closed        int32
	ctx           context.Context
	cancel        context.CancelFunc
	CommandSaver  CommandSaverFunc
}

func NewSession(nodeID string, shell string, conn *websocket.Conn) *TerminalSession {
	ctx, cancel := context.WithCancel(context.Background())
	return &TerminalSession{
		NodeID: nodeID,
		Shell:  shell,
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

	shell := s.resolvedShell()

	proc, err := createPtyProcess(shell, defaultCols, defaultRows)
	if err != nil {
		s.sendMsg("\r\n\x1b[1;31mError: " + err.Error() + "\x1b[0m\r\n")
		return
	}
	s.proc = proc

	ptyType := "PTY"
	if runtime.GOOS == "windows" {
		if proc.IsConPty() {
			ptyType = "ConPTY"
		} else {
			ptyType = "Pipe"
		}
	}

	log.Printf("[terminal] session started for node=%s pid=%d shell=%s mode=%s", s.NodeID, proc.Pid(), shell, ptyType)
	s.sendMsg("\x1b[1;32mDevDash Terminal Connected (" + runtime.GOOS + " - " + ptyType + " - " + shell + ")\r\n\x1b[0m")

	var wg sync.WaitGroup
	wg.Add(3)

	go s.pinger(&wg)
	go s.outputReader(&wg)
	go s.inputHandler(&wg)

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

func (s *TerminalSession) outputReader(wg *sync.WaitGroup) {
	defer wg.Done()

	buf := make([]byte, 8192)
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		n, err := s.proc.Read(buf)
		if n > 0 {
			data := buf[:n]
			s.mu.Lock()
			if atomic.LoadInt32(&s.closed) == 0 {
				if writeErr := s.Conn.WriteMessage(websocket.BinaryMessage, data); writeErr != nil {
					log.Printf("[terminal] write error: %v", writeErr)
					s.mu.Unlock()
					s.cancel()
					return
				}
			}
			s.mu.Unlock()
		}
		if err != nil {
			if err.Error() != "EOF" {
				log.Printf("[terminal] output read error: %v", err)
			}
			return
		}
	}
}

func (s *TerminalSession) inputHandler(wg *sync.WaitGroup) {
	defer wg.Done()

	var commandBuf strings.Builder
	var inEscape bool

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

		if len(msg) == 0 {
			continue
		}

		var resize resizeMessage
		if err := json.Unmarshal(msg, &resize); err == nil && resize.Type == "resize" {
			if resize.Cols > 0 && resize.Rows > 0 {
				if err := s.proc.Resize(resize.Cols, resize.Rows); err != nil {
					log.Printf("[terminal] resize error: %v", err)
				}
			}
			continue
		}

		for _, b := range msg {
			switch {
			case b == '\r' || b == '\n':
				cmd := strings.TrimSpace(commandBuf.String())
				if cmd != "" && s.CommandSaver != nil {
					s.CommandSaver(cmd)
				}
				commandBuf.Reset()
				inEscape = false
			case b == 3:
				commandBuf.Reset()
				inEscape = false
			case b == 127 || b == 8:
				if commandBuf.Len() > 0 {
					runes := []rune(commandBuf.String())
					if len(runes) > 0 {
						commandBuf.Reset()
						commandBuf.WriteString(string(runes[:len(runes)-1]))
					}
				}
			case b == 27:
				inEscape = true
			case inEscape:
				if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '[' || b == 'O' {
					inEscape = false
				}
			case b >= 32 && b <= 126:
				commandBuf.WriteByte(b)
			}
		}

		if _, err := s.proc.Write(msg); err != nil {
			log.Printf("[terminal] stdin write error: %v", err)
			return
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

	if s.proc != nil {
		s.proc.Close()
	}

	if s.Conn != nil {
		s.Conn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[1;31mDisconnected\x1b[0m\r\n"))
		s.Conn.Close()
	}
}

func (s *TerminalSession) sendMsg(msg string) {
	if s.Conn != nil && atomic.LoadInt32(&s.closed) == 0 {
		s.Conn.WriteMessage(websocket.TextMessage, []byte(msg))
	}
}

func (s *TerminalSession) resolvedShell() string {
	if s.Shell != "" {
		if !isAllowedShell(s.Shell) {
			log.Printf("[terminal] rejected disallowed shell: %s", s.Shell)
			s.Shell = ""
		}
	}
	if s.Shell != "" {
		if _, err := os.Stat(s.Shell); err == nil {
			return s.Shell
		}
	}

	shell := os.Getenv("DEVDASH_SHELL")
	if shell != "" {
		if isAllowedShell(shell) {
			if _, err := os.Stat(shell); err == nil {
				return shell
			}
		}
	}

	if runtime.GOOS == "windows" {
		pwsh := findWindowsShell([]string{
			os.Getenv("ProgramFiles") + `\PowerShell\7\pwsh.exe`,
			os.Getenv("ProgramFiles") + `\PowerShell\6\pwsh.exe`,
			`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`,
		})
		if pwsh != "" {
			return pwsh
		}
		return "cmd.exe"
	}

	envShell := os.Getenv("SHELL")
	if envShell != "" {
		if _, err := os.Stat(envShell); err == nil {
			return envShell
		}
	}

	candidates := []string{"/bin/zsh", "/bin/bash", "/usr/bin/zsh", "/usr/bin/bash", "/bin/sh", "/usr/bin/sh"}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	return "/bin/sh"
}

func isAllowedShell(shell string) bool {
	allowed := []string{
		"/bin/sh", "/bin/bash", "/bin/zsh", "/bin/dash", "/bin/fish", "/bin/csh", "/bin/tcsh", "/bin/ksh",
		"/usr/bin/sh", "/usr/bin/bash", "/usr/bin/zsh", "/usr/bin/dash", "/usr/bin/fish", "/usr/bin/csh", "/usr/bin/tcsh", "/usr/bin/ksh",
		"/usr/local/bin/fish", "/usr/local/bin/zsh", "/usr/local/bin/bash",
		"cmd.exe", "powershell.exe", "pwsh.exe",
	}
	lower := strings.ToLower(shell)
	for _, a := range allowed {
		if lower == strings.ToLower(a) {
			return true
		}
	}
	if runtime.GOOS == "windows" {
		lowerShell := strings.ToLower(shell)
		if strings.HasSuffix(lowerShell, `\powershell.exe`) ||
			strings.HasSuffix(lowerShell, `\pwsh.exe`) ||
			lowerShell == "cmd.exe" {
			return true
		}
	}
	return false
}

func findWindowsShell(candidates []string) string {
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

type ShellOption struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func AvailableShells() []ShellOption {
	var shells []ShellOption

	if runtime.GOOS == "windows" {
		winShells := []struct{ name, path string }{
			{"PowerShell", `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`},
			{"PowerShell 7", os.Getenv("ProgramFiles") + `\PowerShell\7\pwsh.exe`},
			{"PowerShell 6", os.Getenv("ProgramFiles") + `\PowerShell\6\pwsh.exe`},
			{"Command Prompt", "cmd.exe"},
		}
		for _, s := range winShells {
			if s.path == "cmd.exe" {
				shells = append(shells, ShellOption{Name: s.name, Path: s.path})
			} else if _, err := os.Stat(s.path); err == nil {
				shells = append(shells, ShellOption{Name: s.name, Path: s.path})
			}
		}
	} else {
		unixShells := []struct{ name, path string }{
			{"Zsh", "/bin/zsh"},
			{"Zsh", "/usr/bin/zsh"},
			{"Bash", "/bin/bash"},
			{"Bash", "/usr/bin/bash"},
			{"Fish", "/usr/bin/fish"},
			{"Fish", "/usr/local/bin/fish"},
			{"Dash", "/bin/dash"},
			{"Sh", "/bin/sh"},
		}
		seen := map[string]bool{}
		for _, s := range unixShells {
			if seen[s.name] {
				continue
			}
			if _, err := os.Stat(s.path); err == nil {
				shells = append(shells, ShellOption{Name: s.name, Path: s.path})
				seen[s.name] = true
			}
		}
	}

	return shells
}

func IsClosed(s *TerminalSession) bool {
	return atomic.LoadInt32(&s.closed) == 1
}
