package filemgr

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	Mode    string    `json:"mode"`
}

type FileContent struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

var allowedRoot = "/"

func SetRoot(root string) {
	if root != "" {
		allowedRoot = root
	}
}

func isPathSafe(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path")
	}
	if !strings.HasPrefix(abs, allowedRoot) {
		return fmt.Errorf("access denied: path outside allowed root")
	}
	return nil
}

func ListDir(dirPath string) ([]FileInfo, error) {
	if err := isPathSafe(dirPath); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, FileInfo{
			Name:    e.Name(),
			Path:    filepath.Join(dirPath, e.Name()),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Mode:    info.Mode().String(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return files[i].Name < files[j].Name
	})
	return files, nil
}

func ReadFile(path string) (*FileContent, error) {
	if err := isPathSafe(path); err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory")
	}
	if info.Size() > 10*1024*1024 {
		return nil, fmt.Errorf("file too large (max 10MB)")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &FileContent{Path: path, Content: string(data)}, nil
}

func WriteFile(path, content string) error {
	if err := isPathSafe(path); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func DeletePath(path string) error {
	if err := isPathSafe(path); err != nil {
		return err
	}
	abs, _ := filepath.Abs(path)
	protected := []string{"/", "/etc", "/bin", "/sbin", "/usr", "/boot", "/dev", "/proc", "/sys"}
	for _, p := range protected {
		if abs == p {
			return fmt.Errorf("cannot delete protected path: %s", p)
		}
	}
	return os.RemoveAll(path)
}

func CreateDir(path string) error {
	if err := isPathSafe(path); err != nil {
		return err
	}
	return os.MkdirAll(path, 0755)
}

func Stat(path string) (*FileInfo, error) {
	if err := isPathSafe(path); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return &FileInfo{
		Name:    info.Name(),
		Path:    path,
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Mode:    info.Mode().String(),
	}, nil
}

func WalkDir(root string, maxDepth int) ([]FileInfo, error) {
	if err := isPathSafe(root); err != nil {
		return nil, err
	}
	var files []FileInfo
	rootDepth := len(strings.Split(filepath.Clean(root), string(os.PathSeparator)))
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		depth := len(strings.Split(filepath.Clean(path), string(os.PathSeparator))) - rootDepth
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files = append(files, FileInfo{
			Name:    d.Name(),
			Path:    path,
			IsDir:   d.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Mode:    info.Mode().String(),
		})
		return nil
	})
	return files, err
}
