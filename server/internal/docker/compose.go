package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ComposeProject struct {
	Name        string            `json:"name"`
	Path        string            `json:"path"`
	Status      string            `json:"status"`
	Services    []ComposeService  `json:"services"`
	Created     time.Time         `json:"created"`
	ConfigFile  string            `json:"config_file"`
}

type ComposeService struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Health  string `json:"health"`
	Ports   []string `json:"ports"`
}

type ComposeManager struct {
	dockerManager *DockerManager
}

func NewComposeManager(dm *DockerManager) *ComposeManager {
	return &ComposeManager{dockerManager: dm}
}

func (cm *ComposeManager) CheckComposeInstalled() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker-compose", "version")
	err := cmd.Run()
	if err == nil {
		return true
	}
	
	cmd2 := exec.CommandContext(ctx, "docker", "compose", "version")
	err2 := cmd2.Run()
	return err2 == nil
}

func (cm *ComposeManager) ListProjects() ([]ComposeProject, error) {
	if !cm.CheckComposeInstalled() {
		return nil, fmt.Errorf("docker compose is not installed")
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "compose", "ls", "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list compose projects: %v, output: %s", err, string(output))
	}
	
	var projects []ComposeProject
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		var project struct {
			Name       string `json:"Name"`
			Status     string `json:"Status"`
			ConfigFiles string `json:"ConfigFiles"`
		}
		
		if err := json.Unmarshal([]byte(line), &project); err != nil {
			continue
		}
		
		configFile := ""
		if project.ConfigFiles != "" {
			files := strings.Split(project.ConfigFiles, ",")
			if len(files) > 0 {
				configFile = strings.TrimSpace(files[0])
			}
		}
		
		p := ComposeProject{
			Name:       project.Name,
			Status:     project.Status,
			ConfigFile: configFile,
		}
		
		if configFile != "" {
			p.Path = filepath.Dir(configFile)
		}
		
		services, err := cm.GetProjectServices(project.Name)
		if err == nil {
			p.Services = services
		}
		
		projects = append(projects, p)
	}
	
	return projects, nil
}

func (cm *ComposeManager) GetProjectServices(projectName string) ([]ComposeService, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", projectName, "ps", "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get services for project %s: %v", projectName, err)
	}
	
	var services []ComposeService
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		var svc struct {
			Name   string `json:"Name"`
			State  string `json:"State"`
			Health string `json:"Health"`
			Ports  string `json:"Publishers"`
		}
		
		if err := json.Unmarshal([]byte(line), &svc); err != nil {
			continue
		}
		
		service := ComposeService{
			Name:   svc.Name,
			State:  svc.State,
			Health: svc.Health,
		}
		
		if svc.Ports != "" {
			service.Ports = strings.Split(svc.Ports, ",")
		}
		
		services = append(services, service)
	}
	
	return services, nil
}

func (cm *ComposeManager) StartProject(composePath string) (io.ReadCloser, error) {
	if !isPathSafe(composePath) {
		return nil, fmt.Errorf("compose path must be absolute and must not contain path traversal")
	}
	
	dir := filepath.Dir(composePath)
	file := filepath.Base(composePath)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, w := io.Pipe()

	go func() {
		defer w.Close()
		cmd := exec.CommandContext(ctx, "docker", "compose", "-f", file, "up", "-d")
		cmd.Dir = dir
		cmd.Stdout = w
		cmd.Stderr = w

		if err := cmd.Run(); err != nil {
			fmt.Fprintf(w, "\nError: %v\n", err)
		}
	}()

	return r, nil
}

func (cm *ComposeManager) StopProject(composePath string) error {
	if !isPathSafe(composePath) {
		return fmt.Errorf("compose path must be absolute and must not contain path traversal")
	}
	
	dir := filepath.Dir(composePath)
	file := filepath.Base(composePath)
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", file, "down")
	cmd.Dir = dir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop project: %v, output: %s", err, string(output))
	}
	
	return nil
}

func (cm *ComposeManager) RestartService(composePath, serviceName string) error {
	if !isPathSafe(composePath) {
		return fmt.Errorf("compose path must be absolute and must not contain path traversal")
	}
	if !isValidServiceName(serviceName) {
		return fmt.Errorf("invalid service name: %s", serviceName)
	}
	
	dir := filepath.Dir(composePath)
	file := filepath.Base(composePath)
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", file, "restart", serviceName)
	cmd.Dir = dir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart service %s: %v, output: %s", serviceName, err, string(output))
	}
	
	return nil
}

func (cm *ComposeManager) GetServiceLogs(composePath, serviceName string, tail string, follow bool) (io.ReadCloser, error) {
	if !isPathSafe(composePath) {
		return nil, fmt.Errorf("compose path must be absolute and must not contain path traversal")
	}
	if !isValidServiceName(serviceName) {
		return nil, fmt.Errorf("invalid service name: %s", serviceName)
	}
	if !isValidTailValue(tail) {
		return nil, fmt.Errorf("invalid tail value: %s", tail)
	}
	
	dir := filepath.Dir(composePath)
	file := filepath.Base(composePath)
	
	args := []string{"compose", "-f", file, "logs", "--tail", tail}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, serviceName)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, w := io.Pipe()

	go func() {
		defer w.Close()
		cmd := exec.CommandContext(ctx, "docker", args...)
		cmd.Dir = dir
		cmd.Stdout = w
		cmd.Stderr = w

		if err := cmd.Run(); err != nil {
			fmt.Fprintf(w, "\nError: %v\n", err)
		}
	}()
	
	return r, nil
}

func (cm *ComposeManager) CreateComposeFile(path string, content []byte) error {
	if !isPathSafe(path) {
		return fmt.Errorf("compose path must be absolute and must not contain path traversal")
	}
	dir := filepath.Dir(path)
	
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}
	
	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("failed to write compose file: %v", err)
	}
	
	return nil
}

func (cm *ComposeManager) ValidateCompose(content []byte) error {
	if len(content) > 1024*1024 {
		return fmt.Errorf("compose file too large (max 1MB)")
	}
	tmpFile, err := os.CreateTemp("", "docker-compose-*.yml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	
	if _, err := tmpFile.Write(content); err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}
	tmpFile.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", tmpFile.Name(), "config")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("invalid compose file: %v, output: %s", err, string(output))
	}
	
	return nil
}

func (cm *ComposeManager) DeployFromTemplate(templateType string, config map[string]string) (*ComposeProject, error) {
	validTemplates := map[string]bool{"nginx": true, "mysql": true, "redis": true, "wordpress": true}
	if !validTemplates[templateType] {
		return nil, fmt.Errorf("unknown template type: %s", templateType)
	}
	for k, v := range config {
		if len(k) > 64 || len(v) > 256 {
			return nil, fmt.Errorf("config key/value too long: %s", k)
		}
		if strings.ContainsAny(v, "\"'`\n\r\\") {
			return nil, fmt.Errorf("config value contains disallowed characters: %s", k)
		}
	}

	var composeContent bytes.Buffer
	
	switch templateType {
	case "nginx":
		composeContent.WriteString(`version: '3.8'
services:
  nginx:
    image: nginx:alpine
    container_name: devdash_nginx
    ports:
      - "` + getConfig(config, "port", "80") + `:80"
    volumes:
      - ./html:/usr/share/nginx/html:ro
      - ./conf/nginx.conf:/etc/nginx/nginx.conf:ro
    restart: unless-stopped
    networks:
      - devdash-net
networks:
  devdash-net:
    driver: bridge
`)
	case "mysql":
		composeContent.WriteString(`version: '3.8'
services:
  mysql:
    image: mysql:8.0
    container_name: devdash_mysql
    environment:
      MYSQL_ROOT_PASSWORD: "` + getConfig(config, "root_password", "root123") + `"
      MYSQL_DATABASE: "` + getConfig(config, "database", "appdb") + `"
      MYSQL_USER: "` + getConfig(config, "user", "appuser") + `"
      MYSQL_PASSWORD: "` + getConfig(config, "password", "apppass") + `"
    ports:
      - "` + getConfig(config, "port", "3306") + `:3306"
    volumes:
      - mysql_data:/var/lib/mysql
    restart: unless-stopped
    networks:
      - devdash-net
volumes:
  mysql_data:
networks:
  devdash-net:
    driver: bridge
`)
	case "redis":
		composeContent.WriteString(`version: '3.8'
services:
  redis:
    image: redis:7-alpine
    container_name: devdash_redis
    command: redis-server --requirepass "` + getConfig(config, "password", "redis123") + `"
    ports:
      - "` + getConfig(config, "port", "6379") + `:6379"
    volumes:
      - redis_data:/data
    restart: unless-stopped
    networks:
      - devdash-net
volumes:
  redis_data:
networks:
  devdash-net:
    driver: bridge
`)
	case "wordpress":
		composeContent.WriteString(`version: '3.8'
services:
  db:
    image: mysql:8.0
    container_name: devdash_wp_db
    environment:
      MYSQL_ROOT_PASSWORD: "` + getConfig(config, "db_root_password", "root123") + `"
      MYSQL_DATABASE: wordpress
      MYSQL_USER: wordpress
      MYSQL_PASSWORD: "` + getConfig(config, "db_password", "wp123") + `"
    volumes:
      - db_data:/var/lib/mysql
    restart: unless-stopped
    networks:
      - devdash-net
  
  wordpress:
    image: wordpress:latest
    container_name: devdash_wordpress
    depends_on:
      - db
    environment:
      WORDPRESS_DB_HOST: db:3306
      WORDPRESS_DB_USER: wordpress
      WORDPRESS_DB_PASSWORD: "` + getConfig(config, "db_password", "wp123") + `"
      WORDPRESS_DB_NAME: wordpress
    ports:
      - "` + getConfig(config, "port", "8080") + `:80"
    volumes:
      - wp_data:/var/www/html
    restart: unless-stopped
    networks:
      - devdash-net
volumes:
  db_data:
  wp_data:
networks:
  devdash-net:
    driver: bridge
`)
	default:
		return nil, fmt.Errorf("unknown template type: %s", templateType)
	}
	
	projectName := "devdash-" + templateType
	projectDir := filepath.Join(os.TempDir(), "devdash-deployments", projectName)
	composePath := filepath.Join(projectDir, "docker-compose.yml")
	
	if err := cm.CreateComposeFile(composePath, composeContent.Bytes()); err != nil {
		return nil, err
	}
	
	services, _ := cm.GetProjectServices(projectName)
	
	project := &ComposeProject{
		Name:       projectName,
		Path:       projectDir,
		Status:     "created",
		Services:   services,
		ConfigFile: composePath,
		Created:    time.Now(),
	}
	
	return project, nil
}

func getConfig(config map[string]string, key, defaultValue string) string {
	if val, ok := config[key]; ok && val != "" {
		return val
	}
	return defaultValue
}

var validServiceNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._\-]{0,63}$`)

func isValidServiceName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	return validServiceNameRegex.MatchString(name)
}

func isValidTailValue(tail string) bool {
	if tail == "" {
		return true
	}
	if tail == "all" {
		return true
	}
	n, err := strconv.Atoi(tail)
	if err != nil {
		return false
	}
	return n > 0 && n <= 10000
}

func isPathSafe(p string) bool {
	cleaned := filepath.Clean(p)
	if strings.Contains(cleaned, "..") {
		return false
	}
	if !filepath.IsAbs(cleaned) {
		return false
	}
	return true
}