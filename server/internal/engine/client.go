package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a dependency-light Docker Engine API client. It speaks the
// documented HTTP API over a unix socket, a Windows named pipe, or TCP,
// which keeps AIDash working against any engine version >= 20.10.
type Client struct {
	host     string
	baseURL  string
	http     *http.Client
	dial     func(ctx context.Context, network, addr string) (net.Conn, error)
	dialAddr string
}

func New(host string) (*Client, error) {
	if host == "" {
		return nil, errors.New("empty docker host")
	}
	c := &Client{host: host}
	tr := &http.Transport{
		MaxIdleConns:        16,
		IdleConnTimeout:     60 * time.Second,
		DisableCompression:  true,
		MaxIdleConnsPerHost: 8,
	}

	switch {
	case strings.HasPrefix(host, "unix://"):
		socket := strings.TrimPrefix(host, "unix://")
		tr.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		}
		c.baseURL = "http://docker"
	case strings.HasPrefix(host, "npipe://"):
		dial, err := npipeDialer(host)
		if err != nil {
			return nil, err
		}
		tr.DialContext = dial
		c.baseURL = "http://docker"
	case strings.HasPrefix(host, "tcp://"):
		u, err := url.Parse(host)
		if err != nil {
			return nil, err
		}
		addr := u.Host
		if !strings.Contains(addr, ":") {
			addr += ":2375"
		}
		c.baseURL = "http://" + addr
	case strings.HasPrefix(host, "http://"), strings.HasPrefix(host, "https://"):
		c.baseURL = strings.TrimSuffix(host, "/")
	default:
		// Windows pipe shorthand: \\.\pipe\docker_engine
		if strings.Contains(host, `\\.\pipe\`) || strings.Contains(host, "//./pipe/") {
			dial, err := npipeDialer(host)
			if err != nil {
				return nil, err
			}
			tr.DialContext = dial
			c.baseURL = "http://docker"
		} else {
			c.baseURL = "http://" + host
		}
	}

	if u, err := url.Parse(c.baseURL); err == nil && u.Host != "" {
		c.dialAddr = u.Host
	}
	c.dial = tr.DialContext
	if c.dial == nil {
		var d net.Dialer
		c.dial = d.DialContext
	}
	c.http = &http.Client{Transport: tr}
	return c, nil
}

func (c *Client) Host() string { return c.host }

// APIError is a structured engine failure.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	if e.Status == 0 {
		return e.Message
	}
	return fmt.Sprintf("docker api %d: %s", e.Status, e.Message)
}

func (c *Client) Do(ctx context.Context, method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接 Docker 失败（%s）: %w", c.host, err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		msg := strings.TrimSpace(string(raw))
		var payload struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(raw, &payload) == nil && payload.Message != "" {
			msg = payload.Message
		}
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return nil, &APIError{Status: resp.StatusCode, Message: msg}
	}
	return resp, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, in any, out any) error {
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = strings.NewReader(string(raw))
	}
	resp, err := c.Do(ctx, method, path, body, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.Do(ctx, http.MethodGet, "/_ping", nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// ---------- version / info ----------

type VersionInfo struct {
	Version       string `json:"Version"`
	APIVersion    string `json:"ApiVersion"`
	MinAPIVersion string `json:"MinAPIVersion"`
	GoVersion     string `json:"GoVersion"`
	GitCommit     string `json:"GitCommit"`
	Os            string `json:"Os"`
	Arch          string `json:"Arch"`
	KernelVersion string `json:"KernelVersion"`
}

func (c *Client) Version(ctx context.Context) (*VersionInfo, error) {
	var v VersionInfo
	err := c.doJSON(ctx, http.MethodGet, "/version", nil, &v)
	return &v, err
}

type EngineInfo struct {
	ID                string `json:"ID"`
	Name              string `json:"Name"`
	ServerVersion     string `json:"ServerVersion"`
	Containers        int    `json:"Containers"`
	ContainersRunning int    `json:"ContainersRunning"`
	ContainersPaused  int    `json:"ContainersPaused"`
	ContainersStopped int    `json:"ContainersStopped"`
	Images            int    `json:"Images"`
	Driver            string `json:"Driver"`
	MemTotal          int64  `json:"MemTotal"`
	NCPU              int    `json:"NCPU"`
	OSType            string `json:"OSType"`
	Architecture      string `json:"Architecture"`
	KernelVersion     string `json:"KernelVersion"`
	OperatingSystem   string `json:"OperatingSystem"`
	DockerRootDir     string `json:"DockerRootDir"`
}

func (c *Client) Info(ctx context.Context) (*EngineInfo, error) {
	var i EngineInfo
	err := c.doJSON(ctx, http.MethodGet, "/info", nil, &i)
	return &i, err
}

type DiskUsage struct {
	LayersSize int64 `json:"LayersSize"`
	Images     []struct {
		ID         string   `json:"Id"`
		RepoTags   []string `json:"RepoTags"`
		Size       int64    `json:"Size"`
		Containers int64    `json:"Containers"`
	} `json:"Images"`
	Containers []struct {
		ID         string   `json:"Id"`
		Names      []string `json:"Names"`
		Image      string   `json:"Image"`
		State      string   `json:"State"`
		Status     string   `json:"Status"`
		SizeRw     int64    `json:"SizeRw"`
		SizeRootFs int64    `json:"SizeRootFs"`
	} `json:"Containers"`
	Volumes []struct {
		Name       string `json:"Name"`
		Driver     string `json:"Driver"`
		Mountpoint string `json:"Mountpoint"`
		UsageData  *struct {
			Size int64 `json:"Size"`
		} `json:"UsageData"`
	} `json:"Volumes"`
}

func (c *Client) DiskUsage(ctx context.Context) (*DiskUsage, error) {
	var d DiskUsage
	err := c.doJSON(ctx, http.MethodGet, "/system/df", nil, &d)
	return &d, err
}
