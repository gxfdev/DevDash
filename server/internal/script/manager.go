package script

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type ExecResult struct {
	ExitCode  int    `json:"exitCode"`
	Output    string `json:"output"`
	Error     string `json:"error"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

func Execute(interpreter, content string, timeout time.Duration) *ExecResult {
	start := time.Now()
	result := &ExecResult{
		StartTime: start.Format("2006-01-02 15:04:05"),
	}

	tmpDir := os.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "webshell-script-*.sh")
	if err != nil {
		result.Error = fmt.Sprintf("create temp file: %v", err)
		result.ExitCode = -1
		result.EndTime = time.Now().Format("2006-01-02 15:04:05")
		return result
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		result.Error = fmt.Sprintf("write temp file: %v", err)
		result.ExitCode = -1
		result.EndTime = time.Now().Format("2006-01-02 15:04:05")
		tmpFile.Close()
		return result
	}
	tmpFile.Close()
	os.Chmod(tmpFile.Name(), 0755)

	if interpreter == "" {
		interpreter = "/bin/bash"
	}

	cmd := exec.Command(interpreter, tmpFile.Name())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if timeout > 0 {
		timer := time.AfterFunc(timeout, func() {
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		})
		defer timer.Stop()
	}

	err = cmd.Run()
	result.EndTime = time.Now().Format("2006-01-02 15:04:05")
	result.Output = stdout.String()
	result.Error = stderr.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			if result.Error == "" {
				result.Error = err.Error()
			}
		}
	}

	return result
}

func ExecuteFile(scriptPath string, args []string, timeout time.Duration) *ExecResult {
	start := time.Now()
	result := &ExecResult{
		StartTime: start.Format("2006-01-02 15:04:05"),
	}

	if _, err := os.Stat(scriptPath); err != nil {
		result.Error = fmt.Sprintf("script not found: %v", err)
		result.ExitCode = -1
		result.EndTime = time.Now().Format("2006-01-02 15:04:05")
		return result
	}

	args = append([]string{scriptPath}, args...)
	cmd := exec.Command("/bin/bash", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if timeout > 0 {
		timer := time.AfterFunc(timeout, func() {
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		})
		defer timer.Stop()
	}

	err := cmd.Run()
	result.EndTime = time.Now().Format("2006-01-02 15:04:05")
	result.Output = stdout.String()
	result.Error = stderr.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
	}

	return result
}

func SaveScript(dataDir, name, content string) (string, error) {
	scriptsDir := filepath.Join(dataDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return "", err
	}
	safeName := filepath.Base(name)
	path := filepath.Join(scriptsDir, safeName)
	return path, os.WriteFile(path, []byte(content), 0755)
}

func ReadScript(dataDir, name string) (string, error) {
	safeName := filepath.Base(name)
	path := filepath.Join(dataDir, "scripts", safeName)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
