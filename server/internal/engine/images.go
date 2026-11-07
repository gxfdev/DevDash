package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (c *Client) ListImages(ctx context.Context) ([]ImageView, error) {
	var raw []struct {
		ID       string            `json:"Id"`
		RepoTags []string          `json:"RepoTags"`
		Size     int64             `json:"Size"`
		Created  int64             `json:"Created"`
		Labels   map[string]string `json:"Labels"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/images/json?all=0", nil, &raw); err != nil {
		return nil, err
	}
	inUse := map[string]int{}
	if containers, err := c.ListContainers(ctx, true); err == nil {
		for _, ct := range containers {
			inUse[ct.Image]++
		}
	}
	out := make([]ImageView, 0, len(raw))
	for _, r := range raw {
		v := ImageView{ID: r.ID, Tags: r.RepoTags, SizeMB: round2(float64(r.Size) / 1e6), Created: r.Created}
		if len(v.Tags) == 0 {
			v.Tags = []string{"<none>:<none>"}
		}
		for _, t := range v.Tags {
			if n, ok := inUse[t]; ok {
				v.InUse += n
			}
		}
		base := strings.Split(v.Tags[0], ":")[0]
		v.IsAIBase = strings.Contains(base, "ollama") || strings.Contains(base, "vllm") || strings.Contains(base, "open-webui") ||
			strings.Contains(base, "qdrant") || strings.Contains(base, "lobe") || strings.Contains(base, "dify") ||
			strings.Contains(base, "jupyter") || strings.Contains(base, "pytorch") || strings.Contains(base, "cuda")
		out = append(out, v)
	}
	return out, nil
}

// PullImage streams the pull progress through emit. It returns when the
// pull finishes or the context is cancelled.
func (c *Client) PullImage(ctx context.Context, image, tag string, emit func(PullProgress)) error {
	fromImage, fromTag := splitImage(image, tag)
	q := url.Values{}
	q.Set("fromImage", fromImage)
	if fromTag != "" {
		q.Set("tag", fromTag)
	}
	resp, err := c.Do(ctx, http.MethodPost, "/images/create?"+q.Encode(), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	for {
		var p PullProgress
		if err := dec.Decode(&p); err != nil {
			if err == io.EOF {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		if p.Error != "" {
			return fmt.Errorf("%s", p.Error)
		}
		if emit != nil && p.Status != "" {
			emit(p)
		}
	}
}

func (c *Client) ImageExists(ctx context.Context, image, tag string) (bool, error) {
	ref := image
	if tag != "" && !strings.Contains(image, ":") {
		ref = image + ":" + tag
	}
	resp, err := c.Do(ctx, http.MethodGet, "/images/"+url.PathEscape(ref)+"/json", nil, nil)
	if err != nil {
		if ae, ok := err.(*APIError); ok && ae.Status == 404 {
			return false, nil
		}
		return false, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return true, nil
}

func (c *Client) RemoveImage(ctx context.Context, id string, force bool) error {
	resp, err := c.Do(ctx, http.MethodDelete, "/images/"+url.PathEscape(id)+"?force="+strconv.FormatBool(force), nil, nil)
	if err != nil {
		if ae, ok := err.(*APIError); ok && ae.Status == 404 {
			return nil
		}
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func splitImage(image, tag string) (string, string) {
	if tag != "" {
		return image, tag
	}
	if i := strings.LastIndex(image, ":"); i > 0 && !strings.Contains(image[i:], "/") {
		return image[:i], image[i+1:]
	}
	return image, "latest"
}

// ---------- networks / volumes / events ----------

func (c *Client) ListNetworks(ctx context.Context) ([]NetworkView, error) {
	var raw []struct {
		ID     string `json:"Id"`
		Name   string `json:"Name"`
		Driver string `json:"Driver"`
		Scope  string `json:"Scope"`
		IPAM   struct {
			Config []struct {
				Subnet  string `json:"Subnet"`
				Gateway string `json:"Gateway"`
			} `json:"Config"`
		} `json:"IPAM"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/networks", nil, &raw); err != nil {
		return nil, err
	}
	out := make([]NetworkView, 0, len(raw))
	for _, n := range raw {
		v := NetworkView{ID: n.ID, Name: n.Name, Driver: n.Driver, Scope: n.Scope}
		if len(n.IPAM.Config) > 0 {
			v.Subnet = n.IPAM.Config[0].Subnet
			v.Gateway = n.IPAM.Config[0].Gateway
		}
		out = append(out, v)
	}
	return out, nil
}

func (c *Client) CreateNetwork(ctx context.Context, name, driver string) (string, error) {
	var resp struct {
		ID string `json:"Id"`
	}
	body := map[string]any{"Name": name, "Driver": driver, "CheckDuplicate": true}
	err := c.doJSON(ctx, http.MethodPost, "/networks/create", body, &resp)
	return resp.ID, err
}

func (c *Client) RemoveNetwork(ctx context.Context, id string) error {
	resp, err := c.Do(ctx, http.MethodDelete, "/networks/"+url.PathEscape(id), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *Client) ListVolumes(ctx context.Context) ([]VolumeView, error) {
	du, err := c.DiskUsage(ctx)
	if err != nil {
		return nil, err
	}
	inUse := map[string]int{}
	for _, ct := range du.Containers {
		_ = ct
	}
	out := make([]VolumeView, 0, len(du.Volumes))
	for _, v := range du.Volumes {
		vv := VolumeView{Name: v.Name, Driver: v.Driver, Mountpoint: v.Mountpoint}
		if v.UsageData != nil {
			vv.SizeMB = round2(float64(v.UsageData.Size) / 1e6)
		}
		vv.InUse = inUse[v.Name]
		out = append(out, vv)
	}
	return out, nil
}

func (c *Client) RemoveVolume(ctx context.Context, name string, force bool) error {
	path := fmt.Sprintf("/volumes/%s?force=%t", url.PathEscape(name), force)
	resp, err := c.Do(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		if ae, ok := err.(*APIError); ok && ae.Status == 404 {
			return nil
		}
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// StreamEvents follows engine events until the context is cancelled.
func (c *Client) StreamEvents(ctx context.Context, emit func(Event)) error {
	resp, err := c.Do(ctx, http.MethodGet, "/events?filters=%7B%22type%22%3A%5B%22container%22%2C%22image%22%5D%7D", nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	for {
		var ev Event
		if err := dec.Decode(&ev); err != nil {
			if ctx.Err() != nil || err == io.EOF {
				return nil
			}
			// transient decode issues should not kill the subscriber
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if emit != nil {
			emit(ev)
		}
	}
}

func round2(v float64) float64 {
	if v < 0 {
		v = 0
	}
	iv := int64(v*100 + 0.5)
	return float64(iv) / 100
}
