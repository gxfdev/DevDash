package engine

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

// CreateVolume ensures a named volume exists (idempotent).
func (c *Client) CreateVolume(ctx context.Context, name string) error {
	body := map[string]any{"Name": name, "Driver": "local"}
	resp, err := c.Do(ctx, http.MethodPost, "/volumes/create", jsonReader(body), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

type VolumeInfo struct {
	Name       string `json:"Name"`
	Driver     string `json:"Driver"`
	Mountpoint string `json:"Mountpoint"`
}

func (c *Client) InspectVolume(ctx context.Context, name string) (*VolumeInfo, error) {
	var v VolumeInfo
	if err := c.doJSON(ctx, http.MethodGet, "/volumes/"+url.PathEscape(name), nil, &v); err != nil {
		return nil, err
	}
	return &v, nil
}
