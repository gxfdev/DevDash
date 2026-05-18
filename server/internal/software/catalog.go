package software

import "runtime"

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

func GetInstallCommand(name, version, os, arch string) string {
	for _, s := range Catalog {
		if s.Name == name {
			if arches, ok := s.InstallCmd[os]; ok {
				if cmd, ok := arches[arch]; ok { return cmd }
				if cmd, ok := arches["amd64"]; ok { return cmd }
			}
		}
	}
	return ""
}

func ListByCategory(category string) []SoftwareCatalog {
	var result []SoftwareCatalog
	for _, s := range Catalog {
		if s.Category == category { result = append(result, s) }
	}
	return result
}