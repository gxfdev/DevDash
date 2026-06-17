package alert

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gxfdev/DevDash/server/internal/model"
	"github.com/gxfdev/DevDash/server/internal/store"
)

type AlertConfig struct {
	Browser        bool   `json:"browser"`
	EmailEnabled   bool   `json:"email_enabled"`
	EmailSMTP      string `json:"email_smtp"`
	EmailPort      int    `json:"email_port"`
	EmailUser      string `json:"email_user"`
	EmailPassword  string `json:"email_password"`
	EmailFrom      string `json:"email_from"`
	EmailTo        string `json:"email_to"`
	WebhookEnabled bool   `json:"webhook_enabled"`
	WebhookURL     string `json:"webhook_url"`
	WebhookSecret  string `json:"webhook_secret"`
	Feishu         bool   `json:"feishu"`
	FeishuURL      string `json:"feishu_url"`
	DingTalk       bool   `json:"dingtalk"`
	DingTalkURL    string `json:"dingtalk_url"`
	DingTalkSecret string `json:"dingtalk_secret"`
}

type Engine struct {
	store       *store.Store
	lastAlerts  map[string]time.Time
	alertsMu    sync.RWMutex
	cooldownSec int
	config      AlertConfig
	configMu    sync.RWMutex
}

func NewEngine(s *store.Store) *Engine {
	e := &Engine{
		store:       s,
		lastAlerts:  make(map[string]time.Time),
		cooldownSec: 300,
		config:      AlertConfig{Browser: true},
	}
	e.loadConfig()
	return e
}

func (e *Engine) loadConfig() {
	if e.store == nil {
		return
	}
	data, err := e.store.GetKV("alert_config")
	if err != nil || data == "" {
		return
	}
	var cfg AlertConfig
	if err := json.Unmarshal([]byte(data), &cfg); err == nil {
		e.configMu.Lock()
		e.config = cfg
		e.configMu.Unlock()
		log.Printf("[alert] config loaded from store")
	}
}

func (e *Engine) SaveConfig() error {
	if e.store == nil {
		return fmt.Errorf("store not available")
	}
	cfg := e.GetConfig()
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return e.store.SetKV("alert_config", string(data))
}

func (e *Engine) GetConfig() AlertConfig {
	e.configMu.RLock()
	defer e.configMu.RUnlock()
	return e.config
}

func (e *Engine) SetConfig(cfg AlertConfig) {
	e.configMu.Lock()
	e.config = cfg
	e.configMu.Unlock()
	if err := e.SaveConfig(); err != nil {
		log.Printf("[alert] failed to save config: %v", err)
	}
}

func (e *Engine) Evaluate(snap *model.Snapshot) {
	if snap == nil {
		return
	}

	rules := e.store.ListAlertRules()
	if len(rules) == 0 {
		e.evaluateDefaults(snap)
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

			nodeName := snap.NodeID
			if snap.Host.Hostname != "" {
				nodeName = snap.Host.Hostname
			}

			alert := map[string]interface{}{
				"node_id":   snap.NodeID,
				"node_name": nodeName,
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

	e.evaluateAnomalies(snap)
}

func (e *Engine) evaluateDefaults(snap *model.Snapshot) {
	defaults := []struct {
		metric    string
		op        string
		threshold float64
		level     string
	}{
		{"cpu", ">", 90, "critical"},
		{"cpu", ">", 80, "warning"},
		{"memory", ">", 90, "critical"},
		{"memory", ">", 80, "warning"},
		{"disk", ">", 90, "critical"},
		{"disk", ">", 80, "warning"},
		{"load1", ">", float64(snap.CPU.Cores) * 2, "critical"},
	}

	for _, d := range defaults {
		value := e.getMetricValue(snap, d.metric)
		if value == nil {
			continue
		}
		if !e.checkCondition(*value, d.op, d.threshold) {
			continue
		}

		alertKey := fmt.Sprintf("%s:%s:default", snap.NodeID, d.metric)
		if e.isInCooldown(alertKey) {
			continue
		}

		nodeName := snap.NodeID
		if snap.Host.Hostname != "" {
			nodeName = snap.Host.Hostname
		}

		alert := map[string]interface{}{
			"node_id":   snap.NodeID,
			"node_name": nodeName,
			"metric":    d.metric,
			"level":     d.level,
			"message":   e.generateMessage(d.metric, *value, d.op, d.threshold),
			"value":     *value,
			"threshold": d.threshold,
			"time":      time.Now(),
			"status":    "firing",
		}

		e.store.SaveAlert(alert)
		e.setCooldown(alertKey)
		log.Printf("[alert] default triggered: %s=%v %s %.1f [%s]",
			d.metric, *value, d.op, d.threshold, d.level)
		go e.sendNotifications(alert)
	}
}

func (e *Engine) evaluateAnomalies(snap *model.Snapshot) {
	anomalyMetrics := []string{"cpu", "memory", "disk"}
	for _, metric := range anomalyMetrics {
		value := e.getMetricValue(snap, metric)
		if value == nil {
			continue
		}

		baseline := e.computeBaseline(snap.NodeID, metric)
		if baseline == nil {
			continue
		}

		upperBound := baseline.mean + 2*baseline.stddev
		lowerBound := baseline.mean - 2*baseline.stddev

		if *value > upperBound || *value < lowerBound {
			alertKey := fmt.Sprintf("%s:%s:anomaly", snap.NodeID, metric)
			if e.isInCooldown(alertKey) {
				continue
			}

			nodeName := snap.NodeID
			if snap.Host.Hostname != "" {
				nodeName = snap.Host.Hostname
			}

			direction := "异常偏高"
			if *value < lowerBound {
				direction = "异常偏低"
			}

			alert := map[string]interface{}{
				"node_id":   snap.NodeID,
				"node_name": nodeName,
				"metric":    metric,
				"level":     "warning",
				"message":   fmt.Sprintf("%s %s(%.2f), 基线均值%.2f±2σ[%.2f,%.2f]", e.metricDisplayName(metric), direction, *value, baseline.mean, lowerBound, upperBound),
				"value":     *value,
				"threshold": upperBound,
				"time":      time.Now(),
				"status":    "firing",
			}

			e.store.SaveAlert(alert)
			e.setCooldown(alertKey)
			log.Printf("[alert] anomaly: %s=%v baseline=[%.2f,%.2f]", metric, *value, lowerBound, upperBound)
			go e.sendNotifications(alert)
		}
	}
}

type baseline struct {
	mean   float64
	stddev float64
}

func (e *Engine) computeBaseline(nodeID, metric string) *baseline {
	if e.store == nil {
		return nil
	}

	values := e.store.GetRecentMetricValues(nodeID, metric, 60)
	if len(values) < 10 {
		return nil
	}

	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	var variance float64
	for _, v := range values {
		variance += (v - mean) * (v - mean)
	}
	variance /= float64(len(values))
	stddev := math.Sqrt(variance)
	if stddev < 1.0 {
		stddev = 1.0
	}

	return &baseline{mean: mean, stddev: stddev}
}

func (e *Engine) metricDisplayName(metric string) string {
	names := map[string]string{
		"cpu":    "CPU使用率",
		"memory": "内存使用率",
		"disk":   "磁盘使用率",
	}
	if n, ok := names[metric]; ok {
		return n
	}
	return metric
}

func (e *Engine) getMetricValue(snap *model.Snapshot, metric string) *float64 {
	switch metric {
	case "cpu":
		return &snap.CPU.UsagePercent
	case "memory", "mem":
		return &snap.Memory.UsagePercent
	case "disk":
		return &snap.Disk.UsagePercent
	case "load1", "load":
		return &snap.Load.Load1
	case "load5":
		return &snap.Load.Load5
	case "load15":
		return &snap.Load.Load15
	case "net_recv_rate":
		v := snap.Network.RecvRateMB
		return &v
	case "net_sent_rate":
		v := snap.Network.SentRateMB
		return &v
	case "disk_read_rate":
		v := snap.DiskIO.ReadMB
		return &v
	case "disk_write_rate":
		v := snap.DiskIO.WriteMB
		return &v
	case "swap_usage":
		if snap.Memory.SwapTotalGB > 0 {
			v := (snap.Memory.SwapUsedGB / snap.Memory.SwapTotalGB) * 100
			return &v
		}
		return nil
	case "tcp_established":
		v := float64(snap.TCPConns.Established)
		return &v
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
		"cpu":             "CPU使用率",
		"memory":          "内存使用率",
		"mem":             "内存使用率",
		"disk":            "磁盘使用率",
		"load1":           "1分钟负载",
		"load5":           "5分钟负载",
		"load15":          "15分钟负载",
		"load":            "1分钟负载",
		"net_recv_rate":   "网络入流量",
		"net_sent_rate":   "网络出流量",
		"disk_read_rate":  "磁盘读取速率",
		"disk_write_rate": "磁盘写入速率",
		"swap_usage":      "Swap使用率",
		"tcp_established": "TCP连接数",
	}
	name, ok := metricNames[metric]
	if !ok {
		name = metric
	}
	unit := "%"
	if strings.HasPrefix(metric, "load") {
		unit = ""
	}
	return fmt.Sprintf("%s %.2f%s %s %.1f%s", name, value, unit, op, threshold, unit)
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

func (e *Engine) SendNotifications(alert map[string]interface{}) {
	e.sendNotifications(alert)
}

func (e *Engine) sendNotifications(alert map[string]interface{}) {
	cfg := e.GetConfig()

	if cfg.Browser {
		log.Printf("[alert] system notification: %s", alert["message"])
	}

	if cfg.EmailEnabled && cfg.EmailSMTP != "" && cfg.EmailTo != "" {
		go e.sendEmailNotification(cfg, alert)
	}

	if cfg.WebhookEnabled && cfg.WebhookURL != "" {
		go e.sendWebhookNotification(cfg, alert)
	}

	if cfg.Feishu && cfg.FeishuURL != "" {
		go e.sendFeishuNotification(cfg, alert)
	}

	if cfg.DingTalk && cfg.DingTalkURL != "" {
		go e.sendDingTalkNotification(cfg, alert)
	}
}

func (e *Engine) sendEmailNotification(cfg AlertConfig, alert map[string]interface{}) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[alert] email notification panic: %v", r)
		}
	}()

	subject := fmt.Sprintf("[DevDash Alert][%s] %s", alert["level"], alert["message"])
	body := fmt.Sprintf(`告警详情:

主机: %s (%s)
指标: %s
当前值: %.2f
阈值: %.1f
级别: %s
时间: %s
消息: %s

- DevDash 运维面板`,
		alert["node_name"], alert["node_id"],
		alert["metric"],
		alert["value"].(float64),
		alert["threshold"].(float64),
		alert["level"],
		alert["time"].(time.Time).Format("2006-01-02 15:04:05"),
		alert["message"],
	)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		cfg.EmailFrom, cfg.EmailTo, subject, body)

	addr := fmt.Sprintf("%s:%d", cfg.EmailSMTP, cfg.EmailPort)
	auth := smtp.PlainAuth("", cfg.EmailUser, cfg.EmailPassword, cfg.EmailSMTP)

	err := smtp.SendMail(addr, auth, cfg.EmailFrom, []string{cfg.EmailTo}, []byte(msg))
	if err != nil {
		log.Printf("[alert] email send failed: %v", err)
		return
	}
	log.Printf("[alert] email notification sent to %s", cfg.EmailTo)
}

func (e *Engine) sendWebhookNotification(cfg AlertConfig, alert map[string]interface{}) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[alert] webhook notification panic: %v", r)
		}
	}()

	payload := map[string]interface{}{
		"alert":     alert,
		"source":    "devdash",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[alert] webhook marshal failed: %v", err)
		return
	}

	req, err := http.NewRequest("POST", cfg.WebhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[alert] webhook request create failed: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.WebhookSecret != "" {
		req.Header.Set("X-Webhook-Secret", cfg.WebhookSecret)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[alert] webhook send failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("[alert] webhook notification sent to %s (status %d)", cfg.WebhookURL, resp.StatusCode)
	} else {
		log.Printf("[alert] webhook returned status %d", resp.StatusCode)
	}
}

func (e *Engine) sendFeishuNotification(cfg AlertConfig, alert map[string]interface{}) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[alert] feishu notification panic: %v", r)
		}
	}()

	levelEmoji := "⚠️"
	if alert["level"] == "critical" {
		levelEmoji = "🔴"
	}

	payload := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]string{
					"tag":     "plain_text",
					"content": fmt.Sprintf("%s DevDash 告警 - %s", levelEmoji, alert["level"]),
				},
				"template": map[string]string{
					"tag": "blue",
				},
			},
			"elements": []map[string]interface{}{
				{
					"tag": "div",
					"text": map[string]string{
						"tag": "lark_md",
						"content": fmt.Sprintf("**主机**: %s\n**指标**: %s\n**当前值**: %.2f\n**阈值**: %.1f\n**消息**: %s",
							alert["node_name"], alert["metric"],
							alert["value"].(float64), alert["threshold"].(float64),
							alert["message"]),
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[alert] feishu marshal failed: %v", err)
		return
	}

	req, err := http.NewRequest("POST", cfg.FeishuURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[alert] feishu request create failed: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[alert] feishu send failed: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("[alert] feishu notification sent (status %d)", resp.StatusCode)
}

func (e *Engine) sendDingTalkNotification(cfg AlertConfig, alert map[string]interface{}) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[alert] dingtalk notification panic: %v", r)
		}
	}()

	levelEmoji := "⚠️"
	if alert["level"] == "critical" {
		levelEmoji = "🔴"
	}

	// 钉钉 markdown 消息格式
	title := fmt.Sprintf("%s DevDash告警 - %s", levelEmoji, alert["level"])
	text := fmt.Sprintf("### %s DevDash告警\n\n"+
		"**主机**: %s\n\n"+
		"**指标**: %s\n\n"+
		"**当前值**: %.2f\n\n"+
		"**阈值**: %.1f\n\n"+
		"**消息**: %s\n\n"+
		"**时间**: %s",
		levelEmoji, alert["level"],
		alert["node_name"],
		alert["metric"],
		alert["value"].(float64),
		alert["threshold"].(float64),
		alert["message"],
		alert["time"].(time.Time).Format("2006-01-02 15:04:05"))

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  text,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[alert] dingtalk marshal failed: %v", err)
		return
	}

	// 钉钉加签验证
	reqURL := cfg.DingTalkURL
	if cfg.DingTalkSecret != "" {
		timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
		stringToSign := timestamp + "\n" + cfg.DingTalkSecret
		mac := hmac.New(sha256.New, []byte(cfg.DingTalkSecret))
		mac.Write([]byte(stringToSign))
		signData := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		sign := url.QueryEscape(signData)
		if strings.Contains(reqURL, "?") {
			reqURL = fmt.Sprintf("%s&timestamp=%s&sign=%s", reqURL, timestamp, sign)
		} else {
			reqURL = fmt.Sprintf("%s?timestamp=%s&sign=%s", reqURL, timestamp, sign)
		}
	}

	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[alert] dingtalk request create failed: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[alert] dingtalk send failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("[alert] dingtalk notification sent (status %d)", resp.StatusCode)
	} else {
		log.Printf("[alert] dingtalk returned status %d", resp.StatusCode)
	}
}
