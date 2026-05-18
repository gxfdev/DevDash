package filemgr

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type FileInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Mode     string `json:"mode"`
	Modified string `json:"modified"`
	IsDir    bool   `json:"is_dir"`
	Perm     string `json:"perm"`
}

func ListDir(path string) ([]FileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil { return nil, err }
	var result []FileInfo
	for _, e := range entries {
		info, err := e.Info()
		if err != nil { continue }
		result = append(result, FileInfo{
			Name: e.Name(), Size: info.Size(),
			Modified: info.ModTime().Format("2006-01-02 15:04:05"),
			IsDir: e.IsDir(), Mode: info.Mode().String(),
		})
	}
	return result, nil
}

func ReadFile(path string) ([]byte, error)       { return os.ReadFile(path) }
func WriteFile(path string, data []byte) error     { return os.WriteFile(path, data, 0644) }
func Delete(path string) error                    { return os.RemoveAll(path) }
func Mkdir(path string) error                     { return os.MkdirAll(path, 0755) }
func Rename(old, new string) error                { return os.Rename(old, new) }

func GetHomeDir() string {
	if runtime.GOOS == "windows" { return os.Getenv("USERPROFILE") }
	return os.Getenv("HOME")
}

func GetDefaultRoot() string {
	if runtime.GOOS == "windows" { return "C:\\" }
	return "/"
}

func GetDriveLetters() []string {
	if runtime.GOOS != "windows" { return nil }
	var drives []string
	for i := 'C'; i <= 'Z'; i++ {
		if _, err := os.Stat(string(i) + ":\\"); err == nil { drives = append(drives, string(i)) }
	}
	return drives
}

func Upload(path string, data []byte) error { return os.WriteFile(path, data, 0644) }
func Download(path string) ([]byte, error)  { return os.ReadFile(path) }

func Chmod(path, mode string) error {
	var m os.FileMode
	fmt.Sscanf(mode, "%o", &m)
	return os.Chmod(path, m)
}

func absPath(p string) string {
	if filepath.IsAbs(p) { return p }
	return filepath.Join(GetHomeDir(), p)
}