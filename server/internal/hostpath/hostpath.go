// Package hostpath 提供容器内主机路径映射功能。
// 当 DevDash 运行在 Docker 容器中时，通过 HOST_ROOT 环境变量
// 将用户请求的主机路径映射到容器内挂载点，从而操作宿主机文件系统。
package hostpath

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// hostRoot 缓存 HOST_ROOT 环境变量值
var hostRoot string

func init() {
	hostRoot = os.Getenv("HOST_ROOT")
	if hostRoot != "" {
		hostRoot = filepath.Clean(hostRoot)
	}
}

// Enabled 返回是否启用了主机路径映射（即运行在容器中且配置了 HOST_ROOT）。
func Enabled() bool {
	return hostRoot != "" && runtime.GOOS != "windows"
}

// Root 返回配置的 HOST_ROOT 值。
func Root() string {
	return hostRoot
}

// ToContainer 将用户指定的主机路径映射为容器内可访问的实际路径。
// 例如：用户请求 /etc/hosts，HOST_ROOT=/host，则返回 /host/etc/hosts。
// 如果未启用映射，直接返回原路径。
func ToContainer(userPath string) string {
	if !Enabled() {
		return userPath
	}
	cleanPath := filepath.Clean(userPath)
	// Windows 路径不映射
	if !filepath.IsAbs(cleanPath) {
		cleanPath = "/" + cleanPath
	}
	// 将主机绝对路径映射到容器内挂载点
	return filepath.Join(hostRoot, cleanPath)
}

// ToHost 将容器内实际路径转换回用户可见的主机路径。
// 例如：容器内 /host/etc/hosts，HOST_ROOT=/host，则返回 /etc/hosts。
// 如果未启用映射，直接返回原路径。
func ToHost(containerPath string) string {
	if !Enabled() {
		return containerPath
	}
	cleanPath := filepath.Clean(containerPath)
	prefix := hostRoot + string(filepath.Separator)
	if cleanPath == hostRoot {
		return "/"
	}
	if strings.HasPrefix(cleanPath, prefix) {
		rel := strings.TrimPrefix(cleanPath, prefix)
		return "/" + rel
	}
	// 不在 HOST_ROOT 下，原样返回
	return cleanPath
}
