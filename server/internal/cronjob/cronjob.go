package cronjob

import (
	"context"
	"devdash/internal/store"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

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
	for _, j := range jobs {
		if m, ok := j.(map[string]interface{}); ok {
			result = append(result, CronJob{
				ID:         int(m["id"].(float64)),
				NodeID:     m["node_id"].(string),
				Name:       m["name"].(string),
				Expression: m["expression"].(string),
				Command:    m["command"].(string),
				Enabled:    m["enabled"].(bool),
			})
		}
	}
	return result
}

func Create(job *CronJob, s *store.Store) error {
	job.NodeID = job.NodeID
	if err := s.SaveCronJob(map[string]interface{}{
		"node_id":   job.NodeID,
		"name":      job.Name,
		"expression": job.Expression,
		"command":   job.Command,
		"type":      job.Type,
		"enabled":   job.Enabled,
	}); err != nil {
		return err
	}
	// Also register with OS scheduler
	if err := registerOSJob(job); err != nil {
		log.Printf("Warning: OS scheduler registration failed: %v", err)
	}
	return nil
}

func Update(job *CronJob, s *store.Store) error {
	return s.SaveCronJob(map[string]interface{}{
		"id":         float64(job.ID),
		"node_id":    job.NodeID,
		"name":       job.Name,
		"expression": job.Expression,
		"command":    job.Command,
		"type":       job.Type,
		"enabled":    job.Enabled,
	})
}

func Delete(id int, nodeID string, s *store.Store) error {
	if err := s.DeleteCronJob(id); err != nil {
		return err
	}
	// Also unregister from OS scheduler
	unregisterOSJob(id, nodeID)
	return nil
}

// SaveLog records a cron job execution result to the audit log
func SaveLog(s *store.Store, jobID int, nodeID, output string, exitCode int, durationMs int64) {
	if s == nil {
		return
	}
	result := "success"
	if exitCode != 0 {
		result = fmt.Sprintf("failed (exit %d)", exitCode)
	}
	_ = s.SaveAuditLog(map[string]interface{}{
		"job_id":  float64(jobID),
		"node_id": nodeID,
		"action":  "cronjob_execute",
		"detail":  output,
		"result":  result,
		"duration_ms": float64(durationMs),
	})
}

func registerOSJob(job *CronJob) error {
	os := runtime.GOOS
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	switch os {
	case "linux":
		// Validate command doesn't contain dangerous patterns before executing
		if err := validateCommand(job.Command); err != nil {
			return fmt.Errorf("command validation failed: %w", err)
		}
		cmd := fmt.Sprintf("(crontab -l 2>/dev/null | grep -v '%s'; echo '%s %s') | crontab -",
			escapeShell(job.Command), job.Expression, escapeShell(job.Command))
		err = exec.CommandContext(ctx, "sh", "-c", cmd).Run()
	case "windows":
		trigger := parseCronToTrigger(job.Expression)
		// Validate command before any shell execution
		if err := validateCommand(job.Command); err != nil {
			return fmt.Errorf("command validation failed: %w", err)
		}
		safeName := escapePowerShell(job.Name)
		safeCmd := escapePowerShell(job.Command)
		cmd := fmt.Sprintf(`powershell -Command "Register-ScheduledTask -TaskName 'DevDash_%s_%d' -Trigger (New-ScheduledTaskTrigger -%s) -Action (New-ScheduledTaskAction -Execute 'cmd.exe' -Argument '/c %s') -Description 'DevDash cronjob' -Force"`,
			safeName, job.ID, trigger, safeCmd)
		err = exec.CommandContext(ctx, "cmd", "/c", cmd).Run()
	}
	if ctx.Err() == context.DeadlineExceeded {
		log.Printf("[cron] registerOSJob timeout: job %d", job.ID)
		return ctx.Err()
	}
	return err
}

// validateCommand checks for dangerous shell patterns before executing
func validateCommand(cmd string) error {
	if cmd == "" {
		return errors.New("command cannot be empty")
	}
	dangerous := []string{"rm -rf /", "dd if=", "mkfs", ":(){ :|:\u0026 };:"}
	for _, d := range dangerous {
		if strings.Contains(cmd, d) {
			return fmt.Errorf("dangerous pattern detected: %s", d)
		}
	}
	return nil
}

func unregisterOSJob(id int, nodeID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	os := runtime.GOOS
	switch os {
	case "linux":
		// Best effort removal by pattern match
	case "windows":
		// Validate id is safe before embedding in shell command
		if id <= 0 {
			return
		}
		cmd := fmt.Sprintf(`powershell -Command "Unregister-ScheduledTask -TaskName 'DevDash_*_%d' -Confirm:$false -ErrorAction SilentlyContinue"`, id)
		exec.CommandContext(ctx, "cmd", "/c", cmd).Run()
	}
}

func parseCronToTrigger(expr string) string {
	// Very basic: handle common daily/weekly patterns
	parts := strings.Fields(expr)
	if len(parts) < 5 {
		return "TimeSpan -Once -At '09:00AM'"
	}
	minute, hour := parts[0], parts[1]
	return fmt.Sprintf("Daily -At '%s:%s'", hour, minute)
}

func escapeShell(s string) string {
	// Single-quote escaping: close quote, escape, reopen
	s = strings.ReplaceAll(s, "'", "'\\''")
	return s
}

func escapePowerShell(s string) string {
	s = strings.ReplaceAll(s, "'", "''")
	s = strings.ReplaceAll(s, "`", "``")
	s = strings.ReplaceAll(s, "$", "`$")
	s = strings.ReplaceAll(s, "\"", "`\"")
	s = strings.ReplaceAll(s, "\n", "`n")
	s = strings.ReplaceAll(s, "\r", "`r")
	return s
}