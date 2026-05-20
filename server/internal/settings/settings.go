package settings

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

type AlertSettings struct {
	Browser      bool   `json:"browser"`
	Feishu       bool   `json:"feishu"`
	FeishuURL    string `json:"feishuUrl"`
	CPUThreshold int    `json:"cpuThreshold"`
	MemThreshold int    `json:"memThreshold"`
	DiskThresh   int    `json:"diskThreshold"`
	CooldownMin  int    `json:"cooldownMin"`
}

type SystemSettings struct {
	ServerPort     string        `json:"server_port"`
	CollectInterval int          `json:"collect_interval"`
	Alert          AlertSettings `json:"alert"`
}

var (
	instance *SystemSettings
	once     sync.Once
	mu       sync.RWMutex
)

func Default() *SystemSettings {
	return &SystemSettings{
		ServerPort:      "9090",
		CollectInterval: 10,
		Alert: AlertSettings{
			Browser:      true,
			Feishu:       false,
			FeishuURL:    "",
			CPUThreshold: 90,
			MemThreshold: 90,
			DiskThresh:   90,
			CooldownMin:  5,
		},
	}
}

func Get() *SystemSettings {
	once.Do(func() {
		instance = Default()
		instance.loadFromFile()
	})
	mu.RLock()
	defer mu.RUnlock()
	return instance
}

func Update(s *SystemSettings) {
	mu.Lock()
	defer mu.Unlock()
	instance = s
	instance.saveToFile()
}

func GetAlertSettings() AlertSettings {
	s := Get()
	mu.RLock()
	defer mu.RUnlock()
	return s.Alert
}

func UpdateAlertSettings(a AlertSettings) {
	mu.Lock()
	defer mu.Unlock()
	instance.Alert = a
	instance.saveToFile()
}

func (s *SystemSettings) loadFromFile() {
	data, err := os.ReadFile("devdash-settings.json")
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, s); err != nil {
		log.Printf("[settings] failed to parse settings file: %v", err)
	}
}

func (s *SystemSettings) saveToFile() {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		log.Printf("[settings] failed to marshal settings: %v", err)
		return
	}
	if err := os.WriteFile("devdash-settings.json", data, 0600); err != nil {
		log.Printf("[settings] failed to write settings file: %v", err)
	}
}
