package engine

import (
	"context"
	"net/http"
	"net/url"
)

type ExecInspect struct {
	ID          string `json:"ID"`
	Running     bool   `json:"Running"`
	ExitCode    int    `json:"ExitCode"`
	ProcessPID  int    `json:"Pid"`
	ContainerID string `json:"ContainerID"`
}

func (c *Client) ExecInspect(ctx context.Context, execID string) (*ExecInspect, error) {
	var out ExecInspect
	if err := c.doJSON(ctx, http.MethodGet, "/exec/"+url.PathEscape(execID)+"/json", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
