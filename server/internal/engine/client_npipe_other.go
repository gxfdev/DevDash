//go:build !windows

package engine

import (
	"context"
	"errors"
	"net"
)

func npipeDialer(host string) (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	return nil, errors.New("named pipe docker host is only supported on Windows")
}
