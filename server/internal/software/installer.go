package software

import (
	"context"
	"os/exec"
	"runtime"
	"time"
)

func Install(nodeID, name, version string) (string, error) {
	os := runtime.GOOS
	arch := runtime.GOARCH
	cmd := GetInstallCommand(name, version, os, arch)
	if cmd == "" {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var out []byte
	var err error
	if os == "windows" {
		out, err = exec.CommandContext(ctx, "cmd", "/c", cmd).CombinedOutput()
	} else {
		out, err = exec.CommandContext(ctx, "sh", "-c", cmd).CombinedOutput()
	}
	if ctx.Err() == context.DeadlineExceeded {
		return "", context.DeadlineExceeded
	}
	return string(out), err
}

func Uninstall(nodeID, name string) (string, error) {
	os := runtime.GOOS
	var cmd string
	switch os {
	case "linux":
		cmd = "apt-get remove -y " + name
	case "windows":
		cmd = "choco uninstall " + name + " -y"
	}
	if cmd == "" {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var out []byte
	var err error
	if os == "windows" {
		out, err = exec.CommandContext(ctx, "cmd", "/c", cmd).CombinedOutput()
	} else {
		out, err = exec.CommandContext(ctx, "sh", "-c", cmd).CombinedOutput()
	}
	if ctx.Err() == context.DeadlineExceeded {
		return "", context.DeadlineExceeded
	}
	return string(out), err
}

func GetStatus(nodeID, name string) string {
	os := runtime.GOOS
	var cmd *exec.Cmd
	switch os {
	case "linux":
		cmd = exec.Command("sh", "-c", "which "+name)
	case "windows":
		cmd = exec.Command("cmd", "/c", "where "+name)
	}
	if cmd == nil || cmd.Run() != nil {
		return "not_installed"
	}
	return "installed"
}

func ServiceControl(nodeID, name, action string) (string, error) {
	os := runtime.GOOS
	var cmd string
	switch os {
	case "linux":
		switch action {
		case "start":
			cmd = "systemctl start " + name
		case "stop":
			cmd = "systemctl stop " + name
		case "restart":
			cmd = "systemctl restart " + name
		}
	case "windows":
		switch action {
		case "start":
			cmd = "net start " + name
		case "stop":
			cmd = "net stop " + name
		case "restart":
			cmd = "net stop " + name + " && net start " + name
		}
	}
	if cmd == "" {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var out []byte
	var err error
	if os == "windows" {
		out, err = exec.CommandContext(ctx, "cmd", "/c", cmd).CombinedOutput()
	} else {
		out, err = exec.CommandContext(ctx, "sh", "-c", cmd).CombinedOutput()
	}
	return string(out), err
}

func IsServiceRunning(nodeID, name string) bool {
	os := runtime.GOOS
	var cmd *exec.Cmd
	switch os {
	case "linux":
		cmd = exec.Command("sh", "-c", "systemctl is-active --quiet "+name)
	case "windows":
		cmd = exec.Command("cmd", "/c", "sc query "+name+" | findstr RUNNING >nul")
	}
	if cmd == nil {
		return false
	}
	return cmd.Run() == nil
}
