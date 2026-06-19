package cronjob

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gxfdev/DevDash/server/internal/hostpath"
	"github.com/gxfdev/DevDash/server/internal/model"
	"github.com/gxfdev/DevDash/server/internal/store"
)

var validTypes = map[string]bool{
	"shell":     true,
	"systemd":   true,
	"scheduled": true,
}

var cronExprRegex = regexp.MustCompile(`^(\S+\s+){4}\S+$`)

type CronJob struct {
	ID         int    `json:"id"`
	NodeID     string `json:"node_id"`
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Command    string `json:"command"`
	Type       string `json:"type"`
	Enabled    bool   `json:"enabled"`
	LastRun    int64  `json:"last_run"`
}

func List(nodeID string, s *store.Store) []CronJob {
	jobs := s.ListCronJobs(nodeID)
	result := make([]CronJob, 0, len(jobs))
	for _, m := range jobs {
		job := CronJob{}
		if v, ok := m["id"].(float64); ok {
			job.ID = int(v)
		}
		if v, ok := m["node_id"].(string); ok {
			job.NodeID = v
		}
		if v, ok := m["name"].(string); ok {
			job.Name = v
		}
		if v, ok := m["expression"].(string); ok {
			job.Expression = v
		}
		if v, ok := m["command"].(string); ok {
			job.Command = v
		}
		if v, ok := m["type"].(string); ok {
			job.Type = v
		}
		if v, ok := m["enabled"].(bool); ok {
			job.Enabled = v
		}
		if v, ok := m["last_run"].(float64); ok {
			job.LastRun = int64(v)
		}
		result = append(result, job)
	}
	return result
}

func Create(job *CronJob, s *store.Store) error {
	if job.NodeID == "" {
		job.NodeID = "self"
	}
	if err := validateJobInput(job); err != nil {
		return err
	}
	jobID, err := s.SaveCronJob(map[string]any{
		"node_id":    job.NodeID,
		"name":       job.Name,
		"expression": job.Expression,
		"command":    job.Command,
		"type":       job.Type,
		"enabled":    job.Enabled,
	})
	if err != nil {
		return fmt.Errorf("failed to save cron job: %w", err)
	}
	job.ID = int(jobID)
	// 仅当任务启用时才注册OS调度器
	if job.Enabled {
		if err := registerOSJob(job); err != nil {
			log.Printf("[cron] Warning: OS scheduler registration failed for job %d: %v", job.ID, err)
			// 不回滚，仅记录警告。任务仍保存在数据库中，可手动运行
		}
	}
	return nil
}

func Update(job *CronJob, s *store.Store) error {
	if err := validateJobInput(job); err != nil {
		return err
	}
	// 先注销旧的OS任务
	unregisterOSJob(job.ID, job.NodeID)
	_, err := s.SaveCronJob(map[string]any{
		"id":         float64(job.ID),
		"node_id":    job.NodeID,
		"name":       job.Name,
		"expression": job.Expression,
		"command":    job.Command,
		"type":       job.Type,
		"enabled":    job.Enabled,
	})
	if err != nil {
		return fmt.Errorf("failed to update cron job: %w", err)
	}
	// 如果任务启用，重新注册OS任务
	if job.Enabled {
		if err := registerOSJob(job); err != nil {
			log.Printf("[cron] Warning: OS scheduler re-registration failed for job %d: %v", job.ID, err)
		}
	}
	return nil
}

func Delete(id int, nodeID string, s *store.Store) error {
	unregisterOSJob(id, nodeID)
	if err := s.DeleteCronJob(id); err != nil {
		return err
	}
	return nil
}

func SaveLog(s *store.Store, jobID int, nodeID, output string, exitCode int, durationMs int64) {
	if s == nil {
		return
	}
	result := "success"
	if exitCode != 0 {
		result = fmt.Sprintf("failed (exit %d)", exitCode)
	}
	_ = s.SaveAuditLog(&model.AuditLog{
		NodeID: nodeID,
		Action: "cronjob_execute",
		Detail: output,
		Result: result,
		Time:   time.Now(),
	})
}

func validateJobInput(job *CronJob) error {
	if strings.TrimSpace(job.Name) == "" {
		return errors.New("name cannot be empty")
	}
	if !validTypes[job.Type] {
		return fmt.Errorf("invalid type: %s (allowed: shell, systemd, scheduled)", job.Type)
	}
	if !cronExprRegex.MatchString(strings.TrimSpace(job.Expression)) {
		return errors.New("invalid cron expression (must be 5 fields: minute hour day month weekday)")
	}
	return validateCommand(job.Command)
}

func validateCommand(cmd string) error {
	return ValidateCommand(cmd)
}

func ValidateCommand(cmd string) error {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return errors.New("command cannot be empty")
	}
	if len(cmd) > 1024 {
		return errors.New("command too long (max 1024 characters)")
	}
	// 允许常见的shell字符：管道、重定向、分号、引号、变量、通配符等
	allowedCmdPattern := regexp.MustCompile(`^[a-zA-Z0-9_/\-\.]+(\s+[a-zA-Z0-9_/\-\.\*\?\[\]=&%@!+,~^<>:;|'"$(){}]+)*$`)
	if !allowedCmdPattern.MatchString(cmd) {
		return errors.New("command contains disallowed characters. Only alphanumeric, spaces, and common shell characters are permitted")
	}
	// 仅阻止真正危险的系统级命令
	dangerousBinaries := []string{
		"rm", "dd", "mkfs", "shutdown", "reboot",
		"halt", "poweroff", "init", "passwd",
		"crontab", "iptables",
	}
	binaryName := strings.Fields(cmd)[0]
	for _, d := range dangerousBinaries {
		if binaryName == d {
			return fmt.Errorf("dangerous binary not allowed: %s", d)
		}
	}
	return nil
}

func registerOSJob(job *CronJob) error {
	osName := runtime.GOOS
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	switch osName {
	case "linux":
		cronCmd := fmt.Sprintf("(crontab -l 2>/dev/null | grep -v 'DevDash_%d'; echo '%s %s # DevDash_%d') | crontab -",
			job.ID, job.Expression, escapeShell(job.Command), job.ID)
		var cmd *exec.Cmd
		if hostpath.Enabled() {
			// 容器内：通过 nsenter 在主机上注册 crontab
			cmd = exec.CommandContext(ctx, "nsenter", "-m", "-u", "-i", "-n", "-p", "-t", "1",
				"/bin/sh", "-c", cronCmd)
		} else {
			cmd = exec.CommandContext(ctx, "sh", "-c", cronCmd)
		}
		err := cmd.Run()
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("registerOSJob timeout")
		}
		return err
	case "windows":
		return registerWindowsTask(ctx, job)
	default:
		log.Printf("[cron] unsupported OS for job registration: %s", osName)
		return nil
	}
}

func unregisterOSJob(id int, _ string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	switch runtime.GOOS {
	case "linux":
		if id <= 0 {
			return
		}
		cronCmd := fmt.Sprintf("crontab -l 2>/dev/null | grep -v 'DevDash_%d' | crontab -", id)
		var cmd *exec.Cmd
		if hostpath.Enabled() {
			cmd = exec.CommandContext(ctx, "nsenter", "-m", "-u", "-i", "-n", "-p", "-t", "1",
				"/bin/sh", "-c", cronCmd)
		} else {
			cmd = exec.CommandContext(ctx, "sh", "-c", cronCmd)
		}
		cmd.Run()
	case "windows":
		if id <= 0 {
			return
		}
		exec.CommandContext(ctx, "powershell", []string{
			"-NoProfile", "-NonInteractive", "-Command",
			fmt.Sprintf("Unregister-ScheduledTask -TaskName 'DevDash_*_%d' -Confirm:$false -ErrorAction SilentlyContinue", id),
		}...).Run()
	}
}

// registerWindowsTask 在Windows上注册计划任务，支持所有cron表达式模式
func registerWindowsTask(ctx context.Context, job *CronJob) error {
	safeName := sanitizeTaskName(job.Name)
	taskName := fmt.Sprintf("DevDash_%s_%d", safeName, job.ID)
	safeCmd := sanitizeTaskArg(job.Command)

	// 解析cron表达式并生成PowerShell触发器脚本
	triggerScript := buildWindowsTrigger(job.Expression)

	// 构建完整的PowerShell命令
	psCmd := fmt.Sprintf(
		"$action = New-ScheduledTaskAction -Execute 'cmd.exe' -Argument '/c %s'; "+
			"$trigger = %s; "+
			"Register-ScheduledTask -TaskName '%s' -Action $action -Trigger $trigger -Description 'DevDash cronjob %d' -Force",
		safeCmd, triggerScript, taskName, job.ID)

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("powershell: %v, stderr: %s", err, stderr.String())
	}
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("registerWindowsTask timeout")
	}
	return nil
}

// buildWindowsTrigger 将cron表达式转换为PowerShell触发器创建代码
// 支持的模式:
//   - */N * * * *    → 每N分钟重复
//   - M * * * *      → 每小时第M分钟
//   - M H * * *      → 每天H:M
//   - M H * * D      → 每周D的H:M
//   - M H D * *      → 每月D号的H:M
//   - */N */M * * *  → 每M小时N分钟间隔
func buildWindowsTrigger(expr string) string {
	parts := strings.Fields(expr)
	if len(parts) < 5 {
		return "New-ScheduledTaskTrigger -Once -At (Get-Date).AddMinutes(1)"
	}

	minute, hour, day, month, weekday := parts[0], parts[1], parts[2], parts[3], parts[4]

	// 模式1: */N * * * * → 每N分钟重复
	if strings.HasPrefix(minute, "*/") && hour == "*" && day == "*" && month == "*" && weekday == "*" {
		n := parseInterval(minute)
		if n > 0 {
			return fmt.Sprintf(
				"$t = New-ScheduledTaskTrigger -Once -At (Get-Date); $t.Repetition = (New-ScheduledTaskTrigger -Once -At (Get-Date) -RepetitionInterval (New-TimeSpan -Minutes %d)).Repetition; $t",
				n)
		}
	}

	// 模式2: M * * * * → 每小时第M分钟
	if minute != "*" && !strings.HasPrefix(minute, "*/") && hour == "*" && day == "*" && month == "*" && weekday == "*" {
		m := parseIntSafe(minute, 0)
		return fmt.Sprintf(
			"$t = New-ScheduledTaskTrigger -Daily -At '00:%02d'; $t.Repetition = (New-ScheduledTaskTrigger -Once -At '00:%02d' -RepetitionInterval (New-TimeSpan -Hours 1)).Repetition; $t",
			m, m)
	}

	// 模式3: */N */M * * * → 每M小时，每N分钟
	if strings.HasPrefix(minute, "*/") && strings.HasPrefix(hour, "*/") && day == "*" && month == "*" && weekday == "*" {
		n := parseInterval(minute)
		m := parseInterval(hour)
		if n > 0 && m > 0 {
			totalMin := m * 60
			return fmt.Sprintf(
				"$t = New-ScheduledTaskTrigger -Once -At (Get-Date); $t.Repetition = (New-ScheduledTaskTrigger -Once -At (Get-Date) -RepetitionInterval (New-TimeSpan -Minutes %d)).Repetition; $t",
				totalMin)
		}
	}

	// 模式4: M H * * * → 每天H:M
	if minute != "*" && !strings.HasPrefix(minute, "*/") &&
		hour != "*" && !strings.HasPrefix(hour, "*/") &&
		day == "*" && month == "*" && weekday == "*" {
		h := parseIntSafe(hour, 0)
		m := parseIntSafe(minute, 0)
		return fmt.Sprintf("New-ScheduledTaskTrigger -Daily -At '%02d:%02d'", h, m)
	}

	// 模式5: M H * * D → 每周D的H:M
	if minute != "*" && !strings.HasPrefix(minute, "*/") &&
		hour != "*" && !strings.HasPrefix(hour, "*/") &&
		day == "*" && month == "*" && weekday != "*" {
		h := parseIntSafe(hour, 0)
		m := parseIntSafe(minute, 0)
		daysOfWeek := parseWeekdayToPowerShell(weekday)
		return fmt.Sprintf("New-ScheduledTaskTrigger -Weekly -DaysOfWeek %s -At '%02d:%02d'", daysOfWeek, h, m)
	}

	// 模式6: M H D * * → 每月D号的H:M (Windows Task Scheduler不直接支持月度，用Weekly近似)
	if minute != "*" && !strings.HasPrefix(minute, "*/") &&
		hour != "*" && !strings.HasPrefix(hour, "*/") &&
		day != "*" && month == "*" && weekday == "*" {
		h := parseIntSafe(hour, 0)
		m := parseIntSafe(minute, 0)
		// Windows不支持月度触发器，使用每周触发器作为近似
		return fmt.Sprintf("New-ScheduledTaskTrigger -Weekly -WeeksInterval 1 -DaysOfWeek Monday -At '%02d:%02d'", h, m)
	}

	// 默认: 尝试解析为每日时间
	h := parseIntSafe(hour, 0)
	m := parseIntSafe(minute, 0)
	if hour == "*" {
		h = 0
	}
	if minute == "*" {
		m = 0
	}
	return fmt.Sprintf("New-ScheduledTaskTrigger -Daily -At '%02d:%02d'", h, m)
}

// parseInterval 从 */N 格式提取N
func parseInterval(s string) int {
	if strings.HasPrefix(s, "*/") {
		return parseIntSafe(s[2:], 0)
	}
	return 0
}

// parseIntSafe 安全解析整数
func parseIntSafe(s string, defaultVal int) int {
	s = strings.TrimSpace(s)
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return defaultVal
}

// parseWeekdayToPowerShell 将cron星期转换为PowerShell DaysOfWeek
func parseWeekdayToPowerShell(weekday string) string {
	dayMap := map[string]string{
		"0": "Sunday", "1": "Monday", "2": "Tuesday", "3": "Wednesday",
		"4": "Thursday", "5": "Friday", "6": "Saturday", "7": "Sunday",
	}
	if d, ok := dayMap[weekday]; ok {
		return d
	}
	// 处理逗号分隔的多个星期
	if strings.Contains(weekday, ",") {
		var days []string
		for _, w := range strings.Split(weekday, ",") {
			if d, ok := dayMap[strings.TrimSpace(w)]; ok {
				days = append(days, d)
			}
		}
		if len(days) > 0 {
			return strings.Join(days, ",")
		}
	}
	return "Monday"
}

func escapeShell(s string) string {
	s = strings.ReplaceAll(s, "'", "'\\''")
	return s
}

func sanitizeTaskName(s string) string {
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "&", "")
	s = strings.ReplaceAll(s, "$", "")
	s = strings.ReplaceAll(s, ";", "")
	s = strings.ReplaceAll(s, "|", "")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	s = strings.ReplaceAll(s, "{", "")
	s = strings.ReplaceAll(s, "}", "")
	s = strings.ReplaceAll(s, "%", "")
	s = strings.ReplaceAll(s, "<", "")
	s = strings.ReplaceAll(s, ">", "")
	s = strings.ReplaceAll(s, "!", "")
	s = regexp.MustCompile(`[^\w\- ]`).ReplaceAllString(s, "")
	if len(s) > 32 {
		s = s[:32]
	}
	if s == "" {
		s = "unnamed"
	}
	return s
}

func sanitizeTaskArg(s string) string {
	s = strings.ReplaceAll(s, "'", "''")
	s = strings.ReplaceAll(s, "`", "``")
	s = strings.ReplaceAll(s, "$", "`$")
	s = strings.ReplaceAll(s, "\"", "`\"")
	s = strings.ReplaceAll(s, "%", "`%%")
	s = strings.ReplaceAll(s, ")", "`)")
	s = strings.ReplaceAll(s, "(", "`(")
	s = strings.ReplaceAll(s, "&", "^&")
	s = strings.ReplaceAll(s, "|", "^|")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return s
}
