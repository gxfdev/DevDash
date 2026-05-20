package cronjob

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"devdash/internal/model"
	"devdash/internal/store"
)

var validTypes = map[string]bool{
	"shell":    true,
	"systemd":  true,
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
	jobID, err := s.SaveCronJob(map[string]interface{}{
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
	if err := registerOSJob(job); err != nil {
		log.Printf("[cron] Warning: OS scheduler registration failed for job %d: %v", job.ID, err)
		_ = s.DeleteCronJob(job.ID)
		return fmt.Errorf("OS scheduler registration failed (job rolled back): %w", err)
	}
	return nil
}

func Update(job *CronJob, s *store.Store) error {
	if err := validateJobInput(job); err != nil {
		return err
	}
	_, err := s.SaveCronJob(map[string]interface{}{
		"id":         float64(job.ID),
		"node_id":    job.NodeID,
		"name":       job.Name,
		"expression": job.Expression,
		"command":    job.Command,
		"type":       job.Type,
		"enabled":    job.Enabled,
	})
	return err
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
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return errors.New("command cannot be empty")
	}
	if len(cmd) > 1024 {
		return errors.New("command too long (max 1024 characters)")
	}
	allowedCmdPattern := regexp.MustCompile(`^[a-zA-Z0-9_/\-\.]+(\s+[a-zA-Z0-9_/\-\.\*\?\[\]=&%@!+,~^<>:]+)*$`)
	if !allowedCmdPattern.MatchString(cmd) {
		return errors.New("command contains disallowed characters. Only alphanumeric, spaces, and common path/symbol characters are permitted")
	}
	dangerousBinaries := []string{
		"rm", "dd", "mkfs", "shutdown", "reboot",
		"halt", "poweroff", "init", "passwd",
		"crontab", "iptables", "nc", "ncat", "socat",
		"wget", "curl",
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
	os := runtime.GOOS
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	switch os {
	case "linux":
		cmd := exec.CommandContext(ctx, "sh", "-c",
			fmt.Sprintf("(crontab -l 2>/dev/null | grep -v 'DevDash_%d'; echo '%s %s # DevDash_%d') | crontab -",
				job.ID, job.Expression, escapeShell(job.Command), job.ID))
		err := cmd.Run()
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("registerOSJob timeout")
		}
		return err
	case "windows":
		trigger := parseCronToTrigger(job.Expression)
		safeName := sanitizeTaskName(job.Name)
		safeCmd := sanitizeTaskArg(job.Command)
		psArgs := []string{
			"-NoProfile", "-NonInteractive", "-Command",
			fmt.Sprintf(
				"Register-ScheduledTask -TaskName 'DevDash_%s_%d' -Trigger (New-ScheduledTaskTrigger -%s) -Action (New-ScheduledTaskAction -Execute 'cmd.exe' -Argument '/c %s') -Description 'DevDash cronjob' -Force",
				safeName, job.ID, trigger, safeCmd),
		}
		cmd := exec.CommandContext(ctx, "powershell", psArgs...)
		err := cmd.Run()
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("registerOSJob timeout")
		}
		return err
	default:
		log.Printf("[cron] unsupported OS for job registration: %s", os)
		return nil
	}
}

func unregisterOSJob(id int, nodeID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	switch runtime.GOOS {
	case "linux":
		if id <= 0 {
			return
		}
		exec.CommandContext(ctx, "sh", "-c",
			fmt.Sprintf("crontab -l 2>/dev/null | grep -v 'DevDash_%d' | crontab -", id)).Run()
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

func parseCronToTrigger(expr string) string {
	parts := strings.Fields(expr)
	if len(parts) < 5 {
		return "Once -At (Get-Date).AddMinutes(1)"
	}
	minute, hour, day, month, weekday := parts[0], parts[1], parts[2], parts[3], parts[4]
	isDaily := day == "*" && month == "*"
	isWeekly := isDaily && weekday != "*"
	isMonthly := day != "*" && month == "*" && weekday == "*"
	switch {
	case isWeekly:
		dayMap := map[string]string{"0": "Sun", "1": "Mon", "2": "Tue", "3": "Wed", "4": "Thu", "5": "Fri", "6": "Sat"}
		d, ok := dayMap[weekday]
		if !ok {
			d = "Mon"
		}
		return fmt.Sprintf("Weekly -DaysOfWeek %s -At '%s:%s'", d, hour, minute)
	case isMonthly:
		return fmt.Sprintf("Monthly -DaysOfMonth %s -At '%s:%s'", day, hour, minute)
	case isDaily || (day == "*" && month == "*" && weekday == "*"):
		return fmt.Sprintf("Daily -At '%s:%s'", hour, minute)
	default:
		return fmt.Sprintf("Daily -At '%s:%s'", hour, minute)
	}
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
