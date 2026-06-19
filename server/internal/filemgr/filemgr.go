package filemgr

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/gxfdev/DevDash/server/internal/hostpath"
)

type FileInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Mode     string `json:"mode"`
	Modified string `json:"modified"`
	IsDir    bool   `json:"is_dir"`
	Perm     string `json:"perm"`
	Path     string `json:"path,omitempty"`
}

type FileOpCallback func(op, path, name, ext string, size int64, isDir bool)

var (
	mu              sync.RWMutex
	allowedBaseDirs []string
	opCallback      FileOpCallback
	dangerousPaths  = map[string]bool{
		"/etc": true, "/root": true, "/boot": true, "/sys": true,
		"/proc": true, "/dev": true, "/bin": true, "/sbin": true,
		"c:\\windows": true, "c:\\system32": true,
		"c:\\program files": true, "c:\\program files (x86)": true,
		"c:\\programdata": true,
	}
)

func SetOpCallback(cb FileOpCallback) {
	mu.Lock()
	defer mu.Unlock()
	opCallback = cb
}

func notifyOp(op, path, name, ext string, size int64, isDir bool) {
	mu.RLock()
	cb := opCallback
	mu.RUnlock()
	if cb != nil {
		cb(op, path, name, ext, size, isDir)
	}
}

func InitAllowedDirs(dirs []string) {
	mu.Lock()
	defer mu.Unlock()
	allowedBaseDirs = dirs
	if len(allowedBaseDirs) == 0 {
		allowedBaseDirs = []string{GetDefaultRoot()}
	}
}

func validatePath(userPath string) (string, error) {
	if userPath == "" {
		return GetDefaultRoot(), nil
	}

	cleanPath := filepath.Clean(userPath)
	absPath := cleanPath

	if !filepath.IsAbs(cleanPath) {
		absPath, _ = filepath.Abs(cleanPath)
	}

	absPath = filepath.Clean(absPath)

	mu.RLock()
	bases := make([]string, len(allowedBaseDirs))
	copy(bases, allowedBaseDirs)
	mu.RUnlock()

	for _, base := range bases {
		cleanBase := filepath.Clean(base)
		if strings.HasPrefix(strings.ToLower(absPath), strings.ToLower(cleanBase)) || strings.EqualFold(absPath, cleanBase) {
			resolvedPath, err := filepath.EvalSymlinks(absPath)
			if err == nil {
				if strings.HasPrefix(strings.ToLower(resolvedPath), strings.ToLower(cleanBase)) || strings.EqualFold(resolvedPath, cleanBase) {
					return resolvedPath, nil
				}
				return "", fmt.Errorf("symlink target outside allowed directory")
			}
			return absPath, nil
		}
	}

	return "", fmt.Errorf("access denied: path outside allowed directories")
}

// resolveHostPath 将用户请求的主机路径映射为容器内可访问的实际路径。
// 当运行在容器中且配置了 HOST_ROOT 时，将主机路径映射到挂载点。
// 验证通过后返回容器内实际路径。
func resolveHostPath(userPath string) (string, error) {
	validated, err := validatePath(userPath)
	if err != nil {
		return "", err
	}
	// 映射到容器内挂载点
	return hostpath.ToContainer(validated), nil
}

func checkDangerousPath(path string) bool {
	// 容器模式下（HOST_ROOT已配置），允许查看宿主机所有目录
	// 仅阻止对容器自身关键路径的写操作
	if hostpath.Enabled() {
		containerOnly := map[string]bool{
			"/proc": true, "/sys": true, "/dev": true,
		}
		lowerPath := strings.ToLower(filepath.Clean(path))
		for dangerous := range containerOnly {
			if strings.HasPrefix(lowerPath, dangerous) {
				return true
			}
		}
		return false
	}
	// 非容器模式：阻止访问系统关键目录
	lowerPath := strings.ToLower(filepath.Clean(path))
	for dangerous := range dangerousPaths {
		if strings.HasPrefix(lowerPath, strings.ToLower(dangerous)) {
			return true
		}
	}
	return false
}

func SanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return ""
	}
	name = filepath.Base(name)
	dangerousChars := []string{"\x00", "/", "\\", "|", "<", ">", ":", "*", "?", "\""}
	for _, c := range dangerousChars {
		name = strings.ReplaceAll(name, c, "_")
	}
	if len(name) > 255 {
		name = name[:255]
	}
	return name
}

func ListDir(path string) ([]FileInfo, error) {
	validatedPath, err := resolveHostPath(path)
	if err != nil {
		return nil, fmt.Errorf("path validation failed: %w", err)
	}

	if checkDangerousPath(path) {
		return nil, fmt.Errorf("access to system directory is restricted")
	}

	entries, err := os.ReadDir(validatedPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory: %w", err)
	}

	var result []FileInfo
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(path, e.Name())
		result = append(result, FileInfo{
			Name:     e.Name(),
			Size:     info.Size(),
			Modified: info.ModTime().Format("2006-01-02 15:04:05"),
			IsDir:    e.IsDir(),
			Mode:     info.Mode().String(),
			Path:     fullPath,
		})
	}
	return result, nil
}

func ReadFile(path string) ([]byte, error) {
	validatedPath, err := resolveHostPath(path)
	if err != nil {
		return nil, fmt.Errorf("path validation failed: %w", err)
	}

	if checkDangerousPath(path) {
		return nil, fmt.Errorf("access to system file is restricted")
	}

	info, err := os.Stat(validatedPath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("cannot read directory as file")
	}

	if info.Size() > 100*1024*1024 {
		return nil, fmt.Errorf("file too large (>100MB)")
	}

	data, err := os.ReadFile(validatedPath)
	if err != nil {
		return nil, fmt.Errorf("read failed: %w", err)
	}
	return data, nil
}

func WriteFile(path string, data []byte) error {
	validatedPath, err := resolveHostPath(path)
	if err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	if checkDangerousPath(path) {
		return fmt.Errorf("write to system directory is not allowed")
	}

	if len(data) > 100*1024*1024 {
		return fmt.Errorf("data too large (>100MB)")
	}

	dir := filepath.Dir(validatedPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("parent directory does not exist")
	}

	err = os.WriteFile(validatedPath, data, 0644)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}
	notifyOp("create", path, filepath.Base(path), filepath.Ext(path), int64(len(data)), false)
	return nil
}

func Delete(path string) error {
	validatedPath, err := resolveHostPath(path)
	if err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	if checkDangerousPath(path) {
		return fmt.Errorf("deletion of system files is not allowed")
	}

	defaultRoot := GetDefaultRoot()
	if path == defaultRoot || filepath.Dir(path) == defaultRoot {
		return fmt.Errorf("cannot delete root or home directory")
	}

	err = os.RemoveAll(validatedPath)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}
	notifyOp("delete", path, filepath.Base(path), filepath.Ext(path), 0, false)
	return nil
}

func Mkdir(path string) error {
	if path == "" {
		return fmt.Errorf("invalid directory name")
	}

	// 验证路径安全性，保留完整路径（不截断为文件名）
	cleanPath := filepath.Clean(path)
	if cleanPath == "." || cleanPath == ".." {
		return fmt.Errorf("invalid directory name")
	}

	validatedPath, err := resolveHostPath(cleanPath)
	if err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	if checkDangerousPath(path) {
		return fmt.Errorf("cannot create directory in system location")
	}

	err = os.MkdirAll(validatedPath, 0755)
	if err != nil {
		return fmt.Errorf("mkdir failed: %w", err)
	}
	notifyOp("create", path, filepath.Base(path), "", 0, true)
	return nil
}

func Rename(old, new string) error {
	oldPath, err := resolveHostPath(old)
	if err != nil {
		return fmt.Errorf("source path validation failed: %w", err)
	}

	newName := SanitizeFileName(new)
	if newName == "" {
		return fmt.Errorf("invalid destination name")
	}

	newPath := filepath.Join(filepath.Dir(oldPath), newName)
	newValidatedPath, err := validatePath(newPath)
	if err != nil {
		return fmt.Errorf("destination path validation failed: %w", err)
	}

	if checkDangerousPath(old) || checkDangerousPath(new) {
		return fmt.Errorf("operation on system paths is not allowed")
	}

	err = os.Rename(oldPath, newValidatedPath)
	if err != nil {
		return fmt.Errorf("rename failed: %w", err)
	}
	return nil
}

func GetHomeDir() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("USERPROFILE")
	}
	return os.Getenv("HOME")
}

func GetDefaultRoot() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("USERPROFILE")
	}
	home := os.Getenv("HOME")
	if home != "" {
		return home
	}
	return "/tmp/devdash"
}

func GetDriveLetters() []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	var drives []string
	for i := 'C'; i <= 'Z'; i++ {
		if _, err := os.Stat(string(i) + ":\\"); err == nil {
			drives = append(drives, string(i)+":\\")
		}
	}
	return drives
}

func Upload(path string, data []byte) error {
	validatedPath, err := resolveHostPath(path)
	if err != nil {
		return fmt.Errorf("upload path validation failed: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	// 仅阻止Windows可执行文件和Web脚本，允许.sh等Linux脚本
	dangerousExts := map[string]bool{
		".exe": true, ".bat": true, ".cmd": true,
		".php": true, ".jsp": true, ".asp": true,
		".com": true, ".scr": true, ".vbs": true,
	}
	if dangerousExts[ext] {
		return fmt.Errorf("upload of executable files (%s) is not allowed for security reasons", ext)
	}

	if len(data) > 50*1024*1024 {
		return fmt.Errorf("upload file too large (>50MB)")
	}

	err = os.WriteFile(validatedPath, data, 0644)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	notifyOp("upload", path, filepath.Base(path), filepath.Ext(path), int64(len(data)), false)
	return nil
}

func Download(path string) ([]byte, error) {
	return ReadFile(path)
}

func Chmod(path, mode string) error {
	validatedPath, err := resolveHostPath(path)
	if err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	var m os.FileMode
	n, err := fmt.Sscanf(mode, "%o", &m)
	if err != nil || n != 1 {
		return fmt.Errorf("invalid mode format: %s", mode)
	}

	if m > 0777 {
		return fmt.Errorf("mode too permissive (max 0777)")
	}

	err = os.Chmod(validatedPath, m)
	if err != nil {
		return fmt.Errorf("chmod failed: %w", err)
	}
	return nil
}
