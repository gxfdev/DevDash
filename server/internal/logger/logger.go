package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	jsonOutput = false
	logFile    *os.File
	mu         sync.Mutex
)

type LogEntry struct {
	Level     string `json:"level"`
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
	TraceID   string `json:"trace_id,omitempty"`
	NodeID    string `json:"node_id,omitempty"`
}

func Setup() {
	if os.Getenv("LOG_FORMAT") == "json" {
		jsonOutput = true
	}

	logPath := os.Getenv("LOG_FILE")
	if logPath != "" {
		dir := filepath.Dir(logPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("[logger] cannot create log dir: %v", err)
		} else {
			f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				log.Printf("[logger] cannot open log file: %v", err)
			} else {
				logFile = f
				log.SetOutput(io.MultiWriter(os.Stdout, f))
			}
		}
	}

	log.SetFlags(0)
}

func Close() {
	if logFile != nil {
		logFile.Close()
	}
}

func writeJSON(level, msg, traceID, nodeID string) {
	entry := LogEntry{
		Level:     level,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Message:   msg,
		TraceID:   traceID,
		NodeID:    nodeID,
	}
	data, _ := json.Marshal(entry)
	mu.Lock()
	defer mu.Unlock()
	os.Stdout.Write(append(data, '\n'))
	if logFile != nil {
		logFile.Write(append(data, '\n'))
	}
}

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		if query != "" {
			path = path + "?" + query
		}

		if jsonOutput {
			data, _ := json.Marshal(map[string]interface{}{
				"level":     "info",
				"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
				"message":   "http_request",
				"trace_id":  c.GetString("trace_id"),
				"status":    statusCode,
				"latency":   latency.String(),
				"client_ip": clientIP,
				"method":    method,
				"path":      path,
			})
			mu.Lock()
			os.Stdout.Write(append(data, '\n'))
			if logFile != nil {
				logFile.Write(append(data, '\n'))
			}
			mu.Unlock()
		} else {
			log.Printf("[GIN] %d | %13v | %15s | %-7s %s",
				statusCode, latency, clientIP, method, path)
		}

		errStr := c.Errors.ByType(gin.ErrorTypePrivate).String()
		if errStr != "" {
			ErrorLogger(fmt.Errorf("gin_error: %s", errStr), "gin")
		}
	}
}

func ErrorLogger(err error, context string) {
	if jsonOutput {
		writeJSON("error", context+": "+err.Error(), "", "")
	} else {
		log.Printf("[ERROR] [%s] %v", context, err)
	}
}

func InfoLogger(message string) {
	if jsonOutput {
		writeJSON("info", message, "", "")
	} else {
		log.Printf("[INFO] %s", message)
	}
}

func WarnLogger(message string) {
	if jsonOutput {
		writeJSON("warn", message, "", "")
	} else {
		log.Printf("[WARN] %s", message)
	}
}

func DebugLogger(message string) {
	if jsonOutput {
		writeJSON("debug", message, "", "")
	} else {
		log.Printf("[DEBUG] %s", message)
	}
}

func InfoLoggerWithTrace(message, traceID, nodeID string) {
	if jsonOutput {
		writeJSON("info", message, traceID, nodeID)
	} else {
		log.Printf("[INFO] [trace=%s node=%s] %s", traceID, nodeID, message)
	}
}

func ErrorLoggerWithTrace(err error, context, traceID, nodeID string) {
	if jsonOutput {
		writeJSON("error", context+": "+err.Error(), traceID, nodeID)
	} else {
		log.Printf("[ERROR] [trace=%s node=%s] [%s] %v", traceID, nodeID, context, err)
	}
}
