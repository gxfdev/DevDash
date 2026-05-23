package software

import (
	"fmt"
	"os"
	"runtime"
)

type OSInfo struct {
	Name    string
	Family  string
	Arch    string
	Version string
}

func DetectOS() OSInfo {
	return OSInfo{Name: runtime.GOOS, Family: runtime.GOOS, Arch: runtime.GOARCH}
}

type SoftwareCatalog struct {
	Name         string
	Category     string
	Versions     []string
	InstallCmd   map[string]map[string]string
}

var Catalog = []SoftwareCatalog{
	{Name: "nginx", Category: "web_server", Versions: []string{"latest"}, InstallCmd: map[string]map[string]string{"linux": {"amd64": "apt-get install -y nginx", "arm64": "apt-get install -y nginx"}, "windows": {"amd64": "choco install nginx -y"}}},
	{Name: "mysql", Category: "database", Versions: []string{"8.0"}, InstallCmd: map[string]map[string]string{"linux": {"amd64": "apt-get install -y mysql-server", "arm64": "apt-get install -y mysql-server"}, "windows": {"amd64": "choco install mysql -y"}}},
	{Name: "redis", Category: "database", Versions: []string{"7.0"}, InstallCmd: map[string]map[string]string{"linux": {"amd64": "apt-get install -y redis-server", "arm64": "apt-get install -y redis-server"}, "windows": {"amd64": "choco install redis-64 -y"}}},
	{Name: "docker", Category: "container", Versions: []string{"latest"}, InstallCmd: map[string]map[string]string{"linux": {"amd64": "curl -fsSL https://get.docker.com | sh", "arm64": "curl -fsSL https://get.docker.com | sh"}, "windows": {"amd64": "choco install docker-desktop -y"}}},
	{Name: "nodejs", Category: "runtime", Versions: []string{"20", "18"}, InstallCmd: map[string]map[string]string{"linux": {"amd64": "curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && apt-get install -y nodejs", "arm64": "curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && apt-get install -y nodejs"}, "windows": {"amd64": "choco install nodejs-lts -y"}}},
	{Name: "python", Category: "runtime", Versions: []string{"3.11", "3.12"}, InstallCmd: map[string]map[string]string{"linux": {"amd64": "apt-get install -y python3 python3-pip python3-venv", "arm64": "apt-get install -y python3 python3-pip python3-venv"}, "windows": {"amd64": "choco install python312 -y"}}},
	{Name: "postgresql", Category: "database", Versions: []string{"16"}, InstallCmd: map[string]map[string]string{"linux": {"amd64": "apt-get install -y postgresql", "arm64": "apt-get install -y postgresql"}, "windows": {"amd64": "choco install postgresql -y"}}},
	{Name: "jdk", Category: "runtime", Versions: []string{"17", "11", "21"}, InstallCmd: map[string]map[string]string{"linux": {"amd64": "apt-get install -y openjdk-17-jdk", "arm64": "apt-get install -y openjdk-17-jdk"}, "windows": {"amd64": "choco install openjdk17 -y"}}},
	{Name: "caddy", Category: "web_server", Versions: []string{"2"}, InstallCmd: map[string]map[string]string{"linux": {"amd64": "curl -fsSL https://get.caddyserver.com | sh", "arm64": "curl -fsSL https://get.caddyserver.com | sh"}, "windows": {"amd64": "choco install caddy -y"}}},
}

func detectPackageManager() string {
	checks := []struct {
		path string
		name string
	}{
		{"/usr/bin/apt-get", "apt"},
		{"/usr/bin/dnf", "dnf"},
		{"/usr/bin/yum", "yum"},
		{"/usr/bin/pacman", "pacman"},
		{"/usr/bin/zypper", "zypper"},
		{"/usr/bin/apk", "apk"},
	}
	for _, c := range checks {
		if _, err := os.Stat(c.path); err == nil {
			return c.name
		}
	}
	return "apt"
}

func GetInstallCommand(name, version, osName, arch string) string {
	for _, s := range Catalog {
		if s.Name == name {
			if arches, ok := s.InstallCmd[osName]; ok {
				if cmd, ok := arches[arch]; ok {
					return cmd
				}
				if cmd, ok := arches["amd64"]; ok {
					return cmd
				}
			}
		}
	}

	safeName := sanitizeString(name, 64)
	versionFlag := ""
	if version != "" {
		safeVer := sanitizeString(version, 32)
		versionFlag = fmt.Sprintf("--version=%s", safeVer)
	}

	switch osName {
	case "linux":
		pm := detectPackageManager()
		switch pm {
		case "apt":
			return fmt.Sprintf("apt-get install -y %s %s 2>&1", safeName, versionFlag)
		case "dnf":
			return fmt.Sprintf("dnf install -y %s %s 2>&1", safeName, versionFlag)
		case "yum":
			return fmt.Sprintf("yum install -y %s %s 2>&1", safeName, versionFlag)
		case "pacman":
			return fmt.Sprintf("pacman -S --noconfirm %s %s 2>&1", safeName, versionFlag)
		case "zypper":
			return fmt.Sprintf("zypper install -y %s %s 2>&1", safeName, versionFlag)
		case "apk":
			return fmt.Sprintf("apk add %s %s 2>&1", safeName, versionFlag)
		default:
			return fmt.Sprintf("apt-get install -y %s %s 2>&1", safeName, versionFlag)
		}
	case "windows":
		return fmt.Sprintf("choco install %s %s -y 2>&1", safeName, versionFlag)
	case "darwin":
		return fmt.Sprintf("brew install %s %s 2>&1", safeName, versionFlag)
	default:
		return ""
	}
}

func ListByCategory(category string) []SoftwareCatalog {
	var result []SoftwareCatalog
	for _, s := range Catalog {
		if s.Category == category { result = append(result, s) }
	}
	return result
}