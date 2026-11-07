package api

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/gxfdev/aidash/server/internal/model"
	"github.com/gxfdev/aidash/server/internal/orchestrator"
	"github.com/gxfdev/aidash/server/internal/terminal"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type wsMessage struct {
	Type  string `json:"type"`
	Data  string `json:"data"`
	Cols  int    `json:"cols"`
	Rows  int    `json:"rows"`
	Token string `json:"token"`
}

// hostTerminal bridges an xterm.js session to a shell on the host.
func (h *Handler) hostTerminal(c *gin.Context) {
	claims, err := h.tokensFromRequest(c)
	if err != nil {
		fail(c, http.StatusUnauthorized, err.Error())
		return
	}
	_ = claims
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	cols, rows := 120, 32
	if v := c.Query("cols"); v != "" {
		if n, err := parseInt(v); err == nil && n > 0 {
			cols = n
		}
	}
	if v := c.Query("rows"); v != "" {
		if n, err := parseInt(v); err == nil && n > 0 {
			rows = n
		}
	}
	sess, err := terminal.Start(cols, rows)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n[错误] 无法启动终端: "+err.Error()+"\r\n"))
		return
	}
	defer sess.Close()
	pipeWS(conn, sess, func(cols, rows int) { sess.Resize(cols, rows) })
}

// serviceTerminal opens a shell inside a managed container.
func (h *Handler) serviceTerminal(c *gin.Context) {
	if _, err := h.tokensFromRequest(c); err != nil {
		fail(c, http.StatusUnauthorized, err.Error())
		return
	}
	svc, err := h.store.GetService(c.Param("id"))
	if err != nil {
		fail(c, http.StatusNotFound, "服务不存在")
		return
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	cols, rows := 120, 32
	if n, err := parseInt(c.DefaultQuery("cols", "120")); err == nil {
		cols = n
	}
	if n, err := parseInt(c.DefaultQuery("rows", "32")); err == nil {
		rows = n
	}
	stream, err := h.orch.ExecInteractive(c.Request.Context(), svc.ID, cols, rows)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n[错误] 无法进入容器: "+err.Error()+"\r\n"))
		return
	}
	defer stream.Close()
	var resize func(int, int)
	if rs, ok := stream.(interface{ Resize(int, int) }); ok {
		resize = rs.Resize
	}
	h.audit(c, "容器终端", svc.Name, "", "ok")
	pipeWS(conn, stream, resize)
}

type duplex struct {
	io.Reader
	io.Writer
}

func pipeWS(conn *websocket.Conn, stream io.ReadWriteCloser, resize func(cols, rows int)) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				_ = stream.Close()
				return
			}
			var msg wsMessage
			if json.Unmarshal(raw, &msg) == nil && msg.Type != "" {
				switch msg.Type {
				case "input":
					_, _ = stream.Write([]byte(msg.Data))
				case "resize":
					if resize != nil && msg.Cols > 0 && msg.Rows > 0 {
						resize(msg.Cols, msg.Rows)
					}
				case "ping":
					_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"pong"}`))
				}
				continue
			}
			_, _ = stream.Write(raw)
		}
	}()

	buf := make([]byte, 8192)
	for {
		n, err := stream.Read(buf)
		if n > 0 {
			if werr := conn.WriteMessage(websocket.TextMessage, buf[:n]); werr != nil {
				_ = stream.Close()
				return
			}
		}
		if err != nil {
			break
		}
	}
	_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n[连接已关闭]\r\n"))
	_ = stream.Close()
	<-timeoutOrDone(done)
}

func timeoutOrDone(done <-chan struct{}) <-chan struct{} {
	out := make(chan struct{})
	go func() {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
		close(out)
	}()
	return out
}

func (h *Handler) tokensFromRequest(c *gin.Context) (*model.User, error) {
	token := c.Query("token")
	if token == "" {
		header := c.GetHeader("Authorization")
		if len(header) > 7 && header[:7] == "Bearer " {
			token = header[7:]
		}
	}
	if token == "" {
		// Allow the session cookie style used by some proxies.
		return nil, errUnauthorized
	}
	claims, err := h.auth.Parse(token)
	if err != nil {
		return nil, errUnauthorized
	}
	user, err := h.store.GetUser(claims.UserID)
	if err != nil {
		return nil, errUnauthorized
	}
	return user, nil
}

var errUnauthorized = &authError{}

type authError struct{}

func (e *authError) Error() string { return "未授权：请重新登录" }

func parseInt(v string) (int, error) {
	n := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			return 0, errUnauthorized
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

var _ = orchestrator.StatusRunning
var _ duplex
