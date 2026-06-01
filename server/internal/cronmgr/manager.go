package cronmgr

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
)

type CrontabEntry struct {
	Schedule string `json:"schedule"`
	Command  string `json:"command"`
	Comment  string `json:"comment,omitempty"`
}

func ListCrontab() ([]CrontabEntry, error) {
	cmd := exec.Command("crontab", "-l")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return []CrontabEntry{}, nil
		}
		return nil, fmt.Errorf("read crontab: %w", err)
	}

	var entries []CrontabEntry
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 6 {
			continue
		}
		entries = append(entries, CrontabEntry{
			Schedule: strings.Join(parts[:5], " "),
			Command:  strings.Join(parts[5:], " "),
		})
	}
	return entries, nil
}

func WriteCrontab(entries []CrontabEntry) error {
	var sb strings.Builder
	sb.WriteString("# Managed by WebShell\n")
	for _, e := range entries {
		if e.Comment != "" {
			sb.WriteString("# " + e.Comment + "\n")
		}
		sb.WriteString(e.Schedule + " " + e.Command + "\n")
	}

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(sb.String())
	return cmd.Run()
}

func AddCrontab(schedule, command, comment string) error {
	entries, err := ListCrontab()
	if err != nil {
		return err
	}
	entries = append(entries, CrontabEntry{
		Schedule: schedule,
		Command:  command,
		Comment:  comment,
	})
	return WriteCrontab(entries)
}

func RemoveCrontabByCommand(schedule, command string) error {
	entries, err := ListCrontab()
	if err != nil {
		return err
	}
	var filtered []CrontabEntry
	for _, e := range entries {
		if e.Schedule == schedule && e.Command == command {
			continue
		}
		filtered = append(filtered, e)
	}
	return WriteCrontab(filtered)
}

type SyncJob struct {
	Schedule string
	Command  string
	Enabled  bool
}

func SyncCrontab(jobs []SyncJob) error {
	entries, err := ListCrontab()
	if err != nil {
		entries = []CrontabEntry{}
	}

	managed := make(map[string]bool)
	for _, j := range jobs {
		if !j.Enabled {
			continue
		}
		key := j.Schedule + "|" + j.Command
		managed[key] = true
		found := false
		for _, e := range entries {
			if e.Schedule == j.Schedule && e.Command == j.Command {
				found = true
				break
			}
		}
		if !found {
			entries = append(entries, CrontabEntry{
				Schedule: j.Schedule,
				Command:  j.Command,
			})
		}
	}

	var filtered []CrontabEntry
	for _, e := range entries {
		for _, j := range jobs {
			if e.Schedule == j.Schedule && e.Command == j.Command {
				if j.Enabled {
					filtered = append(filtered, e)
				}
				break
			}
		}
		if _, isManaged := managed[e.Schedule+"|"+e.Command]; !isManaged {
			filtered = append(filtered, e)
		}
	}

	return WriteCrontab(filtered)
}

func IsCrontabAvailable() bool {
	_, err := exec.LookPath("crontab")
	return err == nil
}
