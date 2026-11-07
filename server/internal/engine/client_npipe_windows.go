//go:build windows

package engine

import (
	"context"
	"net"
	"strings"

	"github.com/Microsoft/go-winio"
)

func npipeDialer(host string) (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	path := host
	switch {
	case strings.HasPrefix(host, "npipe://"):
		path = strings.TrimPrefix(host, "npipe://")
	case strings.HasPrefix(host, `\\.\pipe\`):
		return func(ctx context.Context, _, _ string) (net.Conn, error) {
			return winio.DialPipeContext(ctx, host)
		}, nil
	}
	path = strings.ReplaceAll(path, "//./pipe/", `\\.\pipe\`)
	path = strings.ReplaceAll(path, "/", `\`)
	if !strings.HasPrefix(path, `\\.\pipe\`) {
		path = `\\.\pipe\` + strings.TrimPrefix(path, `\`)
	}
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		return winio.DialPipeContext(ctx, path)
	}, nil
}
