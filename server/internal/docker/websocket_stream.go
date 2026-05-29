package docker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gxfdev/DevDash/server/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" || origin == "null" {
			return false
		}
		allowedOrigins := os.Getenv("CORS_ORIGINS")
		if allowedOrigins != "" {
			for _, o := range strings.Split(allowedOrigins, ",") {
				if strings.TrimSpace(o) == origin {
					return true
				}
			}
		}
		defaultOrigins := map[string]bool{
			"http://localhost:3000":  true,
			"http://localhost:5173":  true,
			"http://localhost:9090":  true,
			"http://127.0.0.1:3000": true,
			"http://127.0.0.1:5173": true,
			"http://127.0.0.1:9090": true,
		}
		return defaultOrigins[origin]
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type WSClient struct {
	conn     *websocket.Conn
	send     chan []byte
	hub      *WSHub
	containerID string
}

type WSHub struct {
	clients    map[*WSClient]bool
	register   chan *WSClient
	unregister chan *WSClient
	broadcast  chan []byte
	mutex      sync.RWMutex
}

func NewWSHub() *WSHub {
	return &WSHub{
		clients:    make(map[*WSClient]bool),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		broadcast:  make(chan []byte, 256),
	}
}

func (h *WSHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
			logger.InfoLogger(fmt.Sprintf("WebSocket client connected. Total clients: %d", len(h.clients)))

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mutex.Unlock()
			logger.InfoLogger(fmt.Sprintf("WebSocket client disconnected. Total clients: %d", len(h.clients)))

		case message := <-h.broadcast:
			h.mutex.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mutex.RUnlock()
		}
	}
}

func (c *WSClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err == nil {
			if action, ok := msg["action"].(string); ok {
				switch action {
				case "subscribe_container":
					if containerID, ok := msg["container_id"].(string); ok {
						c.containerID = containerID
					}
				case "unsubscribe":
					c.containerID = ""
				case "ping":
					c.send <- []byte(`{"type":"pong"}`)
				}
			}
		}
	}
}

func (c *WSClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte("\n"))
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

type ContainerStreamService struct {
	hub       *WSHub
	monitor   *ContainerMonitor
	storage   *HistoryStorage
}

func NewContainerStreamService(monitor *ContainerMonitor, storage *HistoryStorage) *ContainerStreamService {
	hub := NewWSHub()
	go hub.Run()

	return &ContainerStreamService{
		hub:     hub,
		monitor: monitor,
		storage: storage,
	}
}

func (s *ContainerStreamService) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.ErrorLogger(err, "Failed to upgrade connection to WebSocket")
		return
	}

	client := &WSClient{
		hub:   s.hub,
		conn:  conn,
		send:  make(chan []byte, 256),
	}

	s.hub.register <- client

	go client.writePump()
	go client.readPump()
}

func (s *ContainerStreamService) StartMetricsBroadcast() {
	subscription := s.monitor.Subscribe()

	go func() {
		defer s.monitor.Unsubscribe(subscription)

		for metrics := range subscription {
			s.storage.StoreMetrics(metrics)

			message := map[string]interface{}{
				"type":    "metrics_update",
				"payload": metrics,
			}

			messageData, err := json.Marshal(message)
			if err != nil {
				logger.ErrorLogger(err, "Failed to marshal metrics message")
				continue
			}

			s.hub.broadcast <- messageData
		}
	}()
}

func (s *ContainerStreamService) GetConnectedClientsCount() int {
	s.hub.mutex.RLock()
	defer s.hub.mutex.RUnlock()
	return len(s.hub.clients)
}
