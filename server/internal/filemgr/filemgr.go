package filemgr

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

var (
	allowedBaseDirs []string
	dangerousPaths  = map[string]bool{
		"/etc": true, "/root": true, "/boot": true, "/sys": true,
		"/proc": true, "/dev": true, "bin": true, "sbin": true,
		"\\Windows": true, "\\System32": true, "\\Program Files": true,
		"\\Program Files (x86)": true, "\\ProgramData": true,
	}
)

func InitAllowedDirs(dirs []string) {
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
		homeDir := GetHomeDir()
		absPath = filepath.Join(homeDir, cleanPath)
	}

	absPath = filepath.Clean(absPath)

	for _, base := range allowedBaseDirs {
		cleanBase := filepath.Clean(base)
		if strings.HasPrefix(absPath, cleanBase) || absPath == cleanBase {
			resolvedPath, err := filepath.EvalSymlinks(absPath)
			if err == nil {
				if strings.HasPrefix(resolvedPath, cleanBase) || resolvedPath == cleanBase {
					return resolvedPath, nil
				}
				return "", fmt.Errorf("symlink target outside allowed directory")
			}
			return absPath, nil
		}
	}

	return "", fmt.Errorf("access denied: path outside allowed directories")
}

func checkDangerousPath(path string) bool {
	lowerPath := strings.ToLower(filepath.Clean(path))
	for dangerous := range dangerousPaths {
		if strings.HasPrefix(lowerPath, strings.ToLower(dangerous)) {
			return true
		}
	}
	return false
}

func sanitizeFileName(name string) string {
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
	validatedPath, err := validatePath(path)
	if err != nil {
		return nil, fmt.Errorf("path validation failed: %w", err)
	}

	if checkDangerousPath(validatedPath) && runtime.GOOS != "windows" {
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

		fullPath := filepath.Join(validatedPath, e.Name())
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
	validatedPath, err := validatePath(path)
	if err != nil {
		return nil, fmt.Errorf("path validation failed: %w", err)
	}

	if checkDangerousPath(validatedPath) {
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
	validatedPath, err := validatePath(path)
	if err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	if checkDangerousPath(validatedPath) {
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
	return nil
}

func Delete(path string) error {
	validatedPath, err := validatePath(path)
	if err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	if checkDangerousPath(validatedPath) {
		return fmt.Errorf("deletion of system files is not allowed")
	}

	defaultRoot := GetDefaultRoot()
	if validatedPath == defaultRoot || filepath.Dir(validatedPath) == defaultRoot {
		return fmt.Errorf("cannot delete root or home directory")
	}

	err = os.RemoveAll(validatedPath)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}
	return nil
}

func Mkdir(path string) error {
	safeName := sanitizeFileName(path)
	if safeName == "" {
		return fmt.Errorf("invalid directory name")
	}

	validatedPath, err := validatePath(safeName)
	if err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	if checkDangerousPath(validatedPath) {
		return fmt.Errorf("cannot create directory in system location")
	}

	err = os.MkdirAll(validatedPath, 0755)
	if err != nil {
		return fmt.Errorf("mkdir failed: %w", err)
	}
	return nil
}

func Rename(old, new string) error {
	oldPath, err := validatePath(old)
	if err != nil {
		return fmt.Errorf("source path validation failed: %w", err)
	}

	newName := sanitizeFileName(new)
	if newName == "" {
		return fmt.Errorf("invalid destination name")
	}

	newPath := filepath.Join(filepath.Dir(oldPath), newName)
	newValidatedPath, err := validatePath(newPath)
	if err != nil {
		return fmt.Errorf("destination path validation failed: %w", err)
	}

	if checkDangerousPath(oldPath) || checkDangerousPath(newValidatedPath) {
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
		return `C:\`
	}
	return "/"
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
	validatedPath, err := validatePath(path)
	if err != nil {
		return fmt.Errorf("upload path validation failed: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(validatedPath))
	dangerousExts := map[string]bool{
		".exe": true, ".bat": true, ".cmd": true, ".ps1": true,
		".sh": true, ".php": true, ".jsp": true, ".asp": true,
		".com": true, ".scr": true, ".vbs": true, ".js": true,
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
	return nil
}

func Download(path string) ([]byte, error) {
	return ReadFile(path)
}

func Chmod(path, mode string) error {
	validatedPath, err := validatePath(path)
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
