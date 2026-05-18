package settings

import (
	"encoding/json"
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
	s := Get()
	mu.Lock()
	defer mu.Unlock()
	s.Alert = a
	s.saveToFile()
}

func (s *SystemSettings) loadFromFile() {
	data, err := os.ReadFile("devdash-settings.json")
	if err != nil {
		return
	}
	json.Unmarshal(data, s)
}

func (s *SystemSettings) saveToFile() {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile("devdash-settings.json", data, 0644)
}
