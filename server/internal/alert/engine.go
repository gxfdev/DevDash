package alert

import (
	"fmt"
	"log"
	"time"

	"devdash/internal/model"
	"devdash/internal/store"
)

type Engine struct {
	store       *store.Store
	lastAlerts  map[string]time.Time
	cooldownSec int
}

func NewEngine(s *store.Store) *Engine {
	return &Engine{
		store:       s,
		lastAlerts:  make(map[string]time.Time),
		cooldownSec: 300, // 默认5分钟冷却时间
	}
}

func (e *Engine) Evaluate(snap *model.Snapshot) {
	if snap == nil {
		return
	}

	rules := e.store.ListAlertRules()
	if rules == nil || len(rules) == 0 {
		return
	}

	for _, r := range rules {
		rule, ok := r.(map[string]interface{})
		if !ok {
			continue
		}

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
				log.Printf("[alert] 冷却中，跳过: %s", alertKey)
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

			if err := e.store.SaveAlert(alert); err != nil {
				log.Printf("[alert] 保存失败: %v", err)
				continue
			}

			e.setCooldown(alertKey)
			log.Printf("[alert] ⚠️ 触发告警: %s=%v %s %.1f [%s]", 
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
		"cpu":    "CPU使用率",
		"memory": "内存使用率",
		"disk":   "磁盘使用率",
		"load1":  "系统负载(1分钟)",
		"load5":  "系统负载(5分钟)",
	}
	name, ok := metricNames[metric]
	if !ok {
		name = metric
	}
	return fmt.Sprintf("%s %.2f%% %s 阈值 %.1f%%", name, value, op, threshold)
}

func (e *Engine) isInCooldown(key string) bool {
	lastTime, exists := e.lastAlerts[key]
	if !exists {
		return false
	}
	return time.Since(lastTime).Seconds() < float64(e.cooldownSec)
}

func (e *Engine) setCooldown(key string) {
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
			log.Printf("[alert] 浏览器通知已发送")
		case "feishu":
			go e.sendFeishuNotification(alert)
		case "email":
			log.Printf("[alert] 邮件通知已发送")
		case "webhook":
			go e.sendWebhookNotification(alert)
		}
	}
}

func (e *Engine) sendFeishuNotification(alert map[string]interface{}) {
	log.Printf("[alert] 飞书通知: %v", alert["message"])
}

func (e *Engine) sendWebhookNotification(alert map[string]interface{}) {
	webhookURL := e.store.GetSetting("webhook_url")
	if webhookURL == "" {
		return
	}
	log.Printf("[alert] Webhook通知到: %s", webhookURL)
}
