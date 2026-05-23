package ha

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Uptime    string            `json:"uptime"`
	Checks    map[string]Check  `json:"checks"`
	System    SystemInfo        `json:"system"`
}

type Check struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

type SystemInfo struct {
	GoVersion    string `json:"go_version"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	Goroutines   int    `json:"goroutines"`
	HeapAllocMB  string `json:"heap_alloc_mb"`
	NumCPU       int    `json:"num_cpu"`
}

var startTime = time.Now()

func GetUptime() string {
	d := time.Since(startTime)
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd%dh%dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func GetSystemInfo() SystemInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return SystemInfo{
		GoVersion:   runtime.Version(),
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		Goroutines:  runtime.NumGoroutine(),
		HeapAllocMB: fmt.Sprintf("%.1f", float64(m.HeapAlloc)/1024/1024),
		NumCPU:      runtime.NumCPU(),
	}
}

type BackupManager struct {
	backupDir string
	mu        sync.Mutex
}

func NewBackupManager(backupDir string) *BackupManager {
	if backupDir == "" {
		backupDir = filepath.Join(".", "backups")
	}
	return &BackupManager{backupDir: backupDir}
}

type BackupInfo struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
	Type      string `json:"type"`
}

func (bm *BackupManager) CreateBackup(dbPath string) (*BackupInfo, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if err := os.MkdirAll(bm.backupDir, 0755); err != nil {
		return nil, fmt.Errorf("create backup dir: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("backup_%s.json", timestamp)
	backupPath := filepath.Join(bm.backupDir, backupName)

	backupData := map[string]interface{}{
		"version":   "1.0",
		"timestamp": time.Now().Format(time.RFC3339),
		"source":    dbPath,
	}

	if _, err := os.Stat(dbPath); err == nil {
		src, err := os.Open(dbPath)
		if err != nil {
			return nil, fmt.Errorf("open source db: %w", err)
		}
		defer src.Close()

		content, err := io.ReadAll(src)
		if err != nil {
			return nil, fmt.Errorf("read source db: %w", err)
		}

		backupData["db_size"] = len(content)
		backupData["db_content_b64"] = encodeContent(content)
	}

	jsonData, err := json.MarshalIndent(backupData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal backup: %w", err)
	}

	if err := os.WriteFile(backupPath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("write backup: %w", err)
	}

	fi, _ := os.Stat(backupPath)
	log.Printf("[ha] backup created: %s (%d bytes)", backupName, fi.Size())

	return &BackupInfo{
		Name:      backupName,
		Size:      fi.Size(),
		CreatedAt: time.Now().Format(time.RFC3339),
		Type:      "full",
	}, nil
}

func (bm *BackupManager) ListBackups() []BackupInfo {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	entries, err := os.ReadDir(bm.backupDir)
	if err != nil {
		return nil
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "backup_") {
			continue
		}
		fi, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, BackupInfo{
			Name:      entry.Name(),
			Size:      fi.Size(),
			CreatedAt: fi.ModTime().Format(time.RFC3339),
			Type:      "full",
		})
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt > backups[j].CreatedAt
	})

	return backups
}

func (bm *BackupManager) RestoreBackup(name string, targetPath string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	backupPath := filepath.Join(bm.backupDir, name)
	jsonData, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("read backup: %w", err)
	}

	var backupData map[string]interface{}
	if err := json.Unmarshal(jsonData, &backupData); err != nil {
		return fmt.Errorf("parse backup: %w", err)
	}

	contentB64, ok := backupData["db_content_b64"].(string)
	if !ok {
		return fmt.Errorf("invalid backup format: missing db content")
	}

	content, err := decodeContent(contentB64)
	if err != nil {
		return fmt.Errorf("decode backup content: %w", err)
	}

	if targetPath == "" {
		return fmt.Errorf("target path is required")
	}

	if _, err := os.Stat(targetPath); err == nil {
		renamePath := targetPath + ".pre_restore_" + time.Now().Format("20060102_150405")
		if err := os.Rename(targetPath, renamePath); err != nil {
			return fmt.Errorf("rename existing db: %w", err)
		}
		log.Printf("[ha] existing db renamed to: %s", renamePath)
	}

	if err := os.WriteFile(targetPath, content, 0644); err != nil {
		return fmt.Errorf("write restored db: %w", err)
	}

	log.Printf("[ha] backup restored: %s -> %s", name, targetPath)
	return nil
}

func (bm *BackupManager) DeleteBackup(name string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if !strings.HasPrefix(name, "backup_") {
		return fmt.Errorf("invalid backup name")
	}

	backupPath := filepath.Join(bm.backupDir, name)
	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("delete backup: %w", err)
	}

	log.Printf("[ha] backup deleted: %s", name)
	return nil
}

func (bm *BackupManager) CleanupOldBackups(maxBackups int) {
	backups := bm.ListBackups()
	if len(backups) <= maxBackups {
		return
	}

	for i := maxBackups; i < len(backups); i++ {
		backupPath := filepath.Join(bm.backupDir, backups[i].Name)
		os.Remove(backupPath)
		log.Printf("[ha] old backup cleaned up: %s", backups[i].Name)
	}
}

func encodeContent(data []byte) string {
	encoded := make([]byte, len(data)*2)
	hexChars := []byte("0123456789abcdef")
	for i, b := range data {
		encoded[i*2] = hexChars[b>>4]
		encoded[i*2+1] = hexChars[b&0x0f]
	}
	return string(encoded)
}

func decodeContent(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("invalid hex length")
	}
	data := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		high := hexVal(s[i])
		low := hexVal(s[i+1])
		if high < 0 || low < 0 {
			return nil, fmt.Errorf("invalid hex char")
		}
		data[i/2] = byte(high<<4 | low)
	}
	return data, nil
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c - 'a' + 10)
	case c >= 'A' && c <= 'F':
		return int(c - 'A' + 10)
	default:
		return -1
	}
}
