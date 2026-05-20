package software

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

var (
	safeNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._\-]{0,63}$`)
	catalog       = make(map[string]bool)
)

func init() {
	for _, s := range []string{
		"nginx", "apache", "caddy", "tomcat", "mysql", "postgresql",
		"mongodb", "redis", "sqlite", "nodejs", "python", "jdk",
		"go", "dotnet", "git", "vim", "htop", "curl", "wget",
		"docker", "certbot", "nginx-full", "apache2", "openjdk",
		"openjdk-11-jdk", "openjdk-17-jdk", "python3", "python3-pip",
		"nodejs", "npm", "yarn", "golang", "rustc", "cargo",
	} {
		catalog[s] = true
	}
}

func validateSoftwareName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	name = strings.TrimSpace(strings.ToLower(name))
	return safeNameRegex.MatchString(name)
}

func validateVersion(version string) bool {
	if version == "" {
		return true
	}
	version = strings.TrimSpace(version)
	if len(version) > 32 {
		return false
	}
	versionRegex := regexp.MustCompile(`^v?[0-9]+(\.[0-9]+)*(-[a-zA-Z0-9.+]+)?$`)
	return versionRegex.MatchString(version)
}

func sanitizeString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "{", "}", "[", "]", "<", ">", "\n", "\r", "\t", "\\", "'", "\""}
	for _, c := range dangerousChars {
		s = strings.ReplaceAll(s, c, "")
	}
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}

func isAllowedSoftware(name string) bool {
	name = strings.TrimSpace(strings.ToLower(name))
	return catalog[name]
}

func Install(nodeID, name, version string) (string, error) {
	if !validateSoftwareName(name) {
		return "", fmt.Errorf("invalid software name: %s (must be alphanumeric, dots, hyphens only)", name)
	}
	if !validateVersion(version) {
		return "", fmt.Errorf("invalid version format: %s", version)
	}
	if !isAllowedSoftware(name) && !isAllowedSoftware(strings.ToLower(name)) {
		return "", fmt.Errorf("software not in allowed catalog: %s", name)
	}

	osName := runtime.GOOS
	arch := runtime.GOARCH

	cmd := GetInstallCommand(name, version, osName, arch)
	if cmd == "" {
		return "", fmt.Errorf("no installation command available for %s on %s/%s", name, osName, arch)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var out []byte
	var err error
	if osName == "windows" {
		out, err = exec.CommandContext(ctx, "cmd", "/c", cmd).CombinedOutput()
	} else {
		out, err = exec.CommandContext(ctx, "sh", "-c", cmd).CombinedOutput()
	}

	if ctx.Err() == context.DeadlineExceeded {
		return "", context.DeadlineExceeded
	}
	if err != nil {
		return string(out), fmt.Errorf("installation failed for %s: %w", name, err)
	}
	return string(out), nil
}

func Uninstall(nodeID, name string) (string, error) {
	if !validateSoftwareName(name) {
		return "", fmt.Errorf("invalid software name: %s", name)
	}
	if !isAllowedSoftware(name) && !isAllowedSoftware(strings.ToLower(name)) {
		return "", fmt.Errorf("software not in allowed catalog: %s", name)
	}

	osName := runtime.GOOS
	var cmd string
	safeName := sanitizeString(name, 64)

	switch osName {
	case "linux":
		cmd = fmt.Sprintf("apt-get remove -y --purge %s 2>&1", safeName)
	case "windows":
		cmd = fmt.Sprintf("choco uninstall %s -y 2>&1", safeName)
	default:
		return "", fmt.Errorf("unsupported OS: %s", osName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var out []byte
	var err error
	if osName == "windows" {
		out, err = exec.CommandContext(ctx, "cmd", "/c", cmd).CombinedOutput()
	} else {
		out, err = exec.CommandContext(ctx, "sh", "-c", cmd).CombinedOutput()
	}

	if ctx.Err() == context.DeadlineExceeded {
		return "", context.DeadlineExceeded
	}
	if err != nil {
		return string(out), fmt.Errorf("uninstallation failed for %s: %w", name, err)
	}
	return string(out), nil
}

func GetStatus(nodeID, name string) string {
	if !validateSoftwareName(name) {
		return "invalid_name"
	}

	osName := runtime.GOOS
	safeName := sanitizeString(name, 64)

	var cmd *exec.Cmd
	switch osName {
	case "linux":
		cmd = exec.Command("sh", "-c", fmt.Sprintf("which %s 2>/dev/null && echo 'found' || echo 'not_found'", safeName))
	case "windows":
		cmd = exec.Command("cmd", "/c", fmt.Sprintf("where %s >nul 2>nul && echo found || echo not_found", safeName))
	default:
		return "not_installed"
	}

	if cmd == nil || cmd.Run() != nil {
		return "not_installed"
	}
	return "installed"
}

func ServiceControl(nodeID, name, action string) (string, error) {
	if !validateSoftwareName(name) {
		return "", fmt.Errorf("invalid software name: %s", name)
	}
	if !isAllowedSoftware(name) && !isAllowedSoftware(strings.ToLower(name)) {
		return "", fmt.Errorf("software not in allowed catalog: %s", name)
	}

	validActions := map[string]bool{"start": true, "stop": true, "restart": true, "status": true}
	action = strings.ToLower(strings.TrimSpace(action))
	if !validActions[action] {
		return "", fmt.Errorf("invalid action: %s (must be start/stop/restart/status)", action)
	}

	osName := runtime.GOOS
	safeName := sanitizeString(name, 64)

	var cmd string
	switch osName {
	case "linux":
		switch action {
		case "start":
			cmd = fmt.Sprintf("systemctl start %s 2>&1", safeName)
		case "stop":
			cmd = fmt.Sprintf("systemctl stop %s 2>&1", safeName)
		case "restart":
			cmd = fmt.Sprintf("systemctl restart %s 2>&1", safeName)
		case "status":
			cmd = fmt.Sprintf("systemctl status %s --no-pager 2>&1", safeName)
		}
	case "windows":
		switch action {
		case "start":
			cmd = fmt.Sprintf("net start %s 2>&1", safeName)
		case "stop":
			cmd = fmt.Sprintf("net stop %s 2>&1", safeName)
		case "restart":
			cmd = fmt.Sprintf("net stop %s 2>&1 & net start %s 2>&1", safeName, safeName)
		case "status":
			cmd = fmt.Sprintf("sc query %s 2>&1", safeName)
		}
	default:
		return "", fmt.Errorf("unsupported OS: %s", osName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var out []byte
	var err error
	if osName == "windows" {
		out, err = exec.CommandContext(ctx, "cmd", "/c", cmd).CombinedOutput()
	} else {
		out, err = exec.CommandContext(ctx, "sh", "-c", cmd).CombinedOutput()
	}

	if ctx.Err() == context.DeadlineExceeded {
		return "", context.DeadlineExceeded
	}
	if err != nil {
		return string(out), fmt.Errorf("service control failed (%s): %w", action, err)
	}
	return string(out), nil
}

func IsServiceRunning(nodeID, name string) bool {
	if !validateSoftwareName(name) {
		return false
	}

	osName := runtime.GOOS
	safeName := sanitizeString(name, 64)

	var cmd *exec.Cmd
	switch osName {
	case "linux":
		cmd = exec.Command("sh", "-c", fmt.Sprintf("systemctl is-active --quiet %s 2>/dev/null", safeName))
	case "windows":
		cmd = exec.Command("cmd", "/c", fmt.Sprintf("sc query %s | findstr RUNNING >nul 2>&1", safeName))
	default:
		return false
	}

	if cmd == nil {
		return false
	}
	return cmd.Run() == nil
}
