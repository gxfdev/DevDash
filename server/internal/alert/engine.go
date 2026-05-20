package alert

import (
	"fmt"
	"log"
	"sync"
	"time"

	"devdash/internal/model"
	"devdash/internal/store"
)

type Engine struct {
	store       *store.Store
	lastAlerts  map[string]time.Time
	alertsMu    sync.RWMutex
	cooldownSec int
}

func NewEngine(s *store.Store) *Engine {
	return &Engine{
		store:       s,
		lastAlerts:  make(map[string]time.Time),
		cooldownSec: 300,
	}
}

func (e *Engine) Evaluate(snap *model.Snapshot) {
	if snap == nil {
		return
	}

	rules := e.store.ListAlertRules()
	if len(rules) == 0 {
		return
	}

	for _, rule := range rules {
		enabled, _ := rule["enabled"].(bool)
		if !enabled {
			continue
		}

		metric, _ := rule["metric"].(string)
		op, _ := rule["op"].(string)
		threshold, _ := rule["threshold"].(float64)
		level, _ := rule["level"].(string)

		value := e.getMetricValue(snap, metric)
		if value == nil {
			continue
		}

		if e.checkCondition(*value, op, threshold) {
			alertKey := fmt.Sprintf("%s:%s", snap.NodeID, metric)

			if e.isInCooldown(alertKey) {
				continue
			}

			alert := map[string]interface{}{
				"node_id":   snap.NodeID,
				"node_name": snap.Host.Hostname,
				"metric":    metric,
				"level":     level,
				"message":   e.generateMessage(metric, *value, op, threshold),
				"value":     *value,
				"threshold": threshold,
				"time":      time.Now(),
				"status":    "firing",
			}

			e.store.SaveAlert(alert)

			e.setCooldown(alertKey)
			log.Printf("[alert] triggered: %s=%v %s %.1f [%s]",
				metric, *value, op, threshold, level)

			go e.sendNotifications(alert)
		}
	}
}

func (e *Engine) getMetricValue(snap *model.Snapshot, metric string) *float64 {
	switch metric {
	case "cpu":
		return &snap.CPU.UsagePercent
	case "memory":
		return &snap.Memory.UsagePercent
	case "disk":
		return &snap.Disk.UsagePercent
	case "load1":
		return &snap.Load.Load1
	case "load5":
		return &snap.Load.Load5
	default:
		return nil
	}
}

func (e *Engine) checkCondition(value float64, op string, threshold float64) bool {
	switch op {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return false
	}
}

func (e *Engine) generateMessage(metric string, value float64, op string, threshold float64) string {
	metricNames := map[string]string{
		"cpu":    "CPU",
		"memory": "Memory",
		"disk":   "Disk",
		"load1":  "Load1",
		"load5":  "Load5",
	}
	name, ok := metricNames[metric]
	if !ok {
		name = metric
	}
	return fmt.Sprintf("%s %.2f%% %s %.1f%%", name, value, op, threshold)
}

func (e *Engine) isInCooldown(key string) bool {
	e.alertsMu.RLock()
	defer e.alertsMu.RUnlock()
	lastTime, exists := e.lastAlerts[key]
	if !exists {
		return false
	}
	return time.Since(lastTime).Seconds() < float64(e.cooldownSec)
}

func (e *Engine) setCooldown(key string) {
	e.alertsMu.Lock()
	defer e.alertsMu.Unlock()
	e.lastAlerts[key] = time.Now()
}

func (e *Engine) sendNotifications(alert map[string]interface{}) {
	channels, _ := alert["channels"]
	if channels == nil {
		return
	}
	channelList, ok := channels.([]string)
	if !ok {
		return
	}
	for _, ch := range channelList {
		switch ch {
		case "browser":
			log.Printf("[alert] browser notification sent")
		case "feishu":
			go e.sendFeishuNotification(alert)
		case "email":
			log.Printf("[alert] email notification sent")
		case "webhook":
			go e.sendWebhookNotification(alert)
		}
	}
}

func (e *Engine) sendFeishuNotification(alert map[string]interface{}) {
	log.Printf("[alert] feishu notification: %v", alert["message"])
}

func (e *Engine) sendWebhookNotification(alert map[string]interface{}) {
	log.Printf("[alert] webhook notification sent")
}
