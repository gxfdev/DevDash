package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (c *Client) ListContainers(ctx context.Context, all bool) ([]ContainerView, error) {
	path := "/containers/json?all=0"
	if all {
		path = "/containers/json?all=1"
	}
	var raw []struct {
		ID      string            `json:"Id"`
		Names   []string          `json:"Names"`
		Image   string            `json:"Image"`
		ImageID string            `json:"ImageID"`
		Command string            `json:"Command"`
		Created int64             `json:"Created"`
		State   string            `json:"State"`
		Status  string            `json:"Status"`
		Ports   []PortView        `json:"-"`
		Labels  map[string]string `json:"Labels"`
		SizeRw  int64             `json:"SizeRw"`
		Mounts  []struct {
			Source      string `json:"Source"`
			Destination string `json:"Destination"`
		} `json:"Mounts"`
		NetworkSettings struct {
			Networks map[string]json.RawMessage `json:"Networks"`
		} `json:"NetworkSettings"`
		PortsRaw []struct {
			IP          string `json:"IP"`
			PrivatePort int    `json:"PrivatePort"`
			PublicPort  int    `json:"PublicPort"`
			Type        string `json:"Type"`
		} `json:"Ports"`
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return nil, err
	}
	out := make([]ContainerView, 0, len(raw))
	for _, r := range raw {
		v := ContainerView{
			ID: r.ID, Names: r.Names, Image: r.Image, ImageID: r.ImageID,
			Command: r.Command, Created: r.Created, State: r.State, Status: r.Status,
			Labels: r.Labels, SizeRw: r.SizeRw,
		}
		if v.Labels == nil {
			v.Labels = map[string]string{}
		}
		for _, p := range r.PortsRaw {
			v.Ports = append(v.Ports, PortView{IP: p.IP, PrivatePort: p.PrivatePort, PublicPort: p.PublicPort, Type: p.Type})
		}
		for _, m := range r.Mounts {
			v.Mounts = append(v.Mounts, m.Source+":"+m.Destination)
		}
		for name := range r.NetworkSettings.Networks {
			v.Network = name
			break
		}
		v.Managed = v.Labels["aios.managed"] == "true"
		v.ManagedID = v.Labels["aios.service_id"]
		v.ManagedFor = v.Labels["aios.service_name"]
		v.HostAccess = v.Labels["aios.host_access"] == "true"
		out = append(out, v)
	}
	return out, nil
}

func (c *Client) InspectContainer(ctx context.Context, id string) (*ContainerInspect, error) {
	var ci ContainerInspect
	if err := c.doJSON(ctx, http.MethodGet, "/containers/"+url.PathEscape(id)+"/json", nil, &ci); err != nil {
		return nil, err
	}
	return &ci, nil
}

func (c *Client) CreateContainer(ctx context.Context, name string, req CreateRequest) (*CreateResponse, error) {
	q := url.Values{}
	if name != "" {
		q.Set("name", name)
	}
	var resp CreateResponse
	err := c.doJSON(ctx, http.MethodPost, "/containers/create?"+q.Encode(), req, &resp)
	return &resp, err
}

func (c *Client) StartContainer(ctx context.Context, id string) error {
	resp, err := c.Do(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/start", nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *Client) StopContainer(ctx context.Context, id string, timeout int) error {
	path := fmt.Sprintf("/containers/%s/stop?t=%d", url.PathEscape(id), timeout)
	resp, err := c.Do(ctx, http.MethodPost, path, nil, nil)
	if err != nil {
		if ae, ok := err.(*APIError); ok && ae.Status == 304 {
			return nil
		}
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *Client) RestartContainer(ctx context.Context, id string, timeout int) error {
	path := fmt.Sprintf("/containers/%s/restart?t=%d", url.PathEscape(id), timeout)
	resp, err := c.Do(ctx, http.MethodPost, path, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *Client) RemoveContainer(ctx context.Context, id string, force, volumes bool) error {
	path := fmt.Sprintf("/containers/%s?force=%t&v=%t", url.PathEscape(id), force, volumes)
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

func (c *Client) RenameContainer(ctx context.Context, id, name string) error {
	resp, err := c.Do(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/rename?name="+url.QueryEscape(name), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *Client) ContainerLogs(ctx context.Context, id string, tail int, follow bool) (io.ReadCloser, error) {
	if tail <= 0 {
		tail = 200
	}
	q := url.Values{}
	q.Set("stdout", "1")
	q.Set("stderr", "1")
	q.Set("timestamps", "0")
	q.Set("tail", strconv.Itoa(tail))
	if follow {
		q.Set("follow", "1")
	}
	resp, err := c.Do(ctx, http.MethodGet, "/containers/"+url.PathEscape(id)+"/logs?"+q.Encode(), nil, nil)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// ContainerLogsText returns the demultiplexed tail of container logs.
func (c *Client) ContainerLogsText(ctx context.Context, id string, tail int) (string, error) {
	body, err := c.ContainerLogs(ctx, id, tail, false)
	if err != nil {
		return "", err
	}
	defer body.Close()
	raw, err := io.ReadAll(io.LimitReader(body, 4<<20))
	if err != nil {
		return "", err
	}
	return Demux(raw), nil
}

// ContainerLogsStream follows logs and emits demultiplexed lines.
func (c *Client) ContainerLogsStream(ctx context.Context, id string, tail int, emit func(string)) error {
	body, err := c.ContainerLogs(ctx, id, tail, true)
	if err != nil {
		return err
	}
	defer body.Close()
	r := NewDemuxReader(body)
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			emit(string(buf[:n]))
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func (c *Client) ContainerStats(ctx context.Context, id string) (*ContainerStatView, error) {
	path := "/containers/" + url.PathEscape(id) + "/stats?stream=false&one-shot=true"
	var s ContainerStats
	subctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	if err := c.doJSON(subctx, http.MethodGet, path, nil, &s); err != nil {
		return nil, err
	}
	return computeStats(&s), nil
}

func computeStats(s *ContainerStats) *ContainerStatView {
	v := &ContainerStatView{}
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage) - float64(s.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(s.CPUStats.SystemUsage) - float64(s.PreCPUStats.SystemUsage)
	online := s.CPUStats.OnlineCPUs
	if online == 0 {
		online = len(s.CPUStats.CPUUsage.PercpuUsage)
	}
	if sysDelta > 0 && cpuDelta > 0 {
		v.CPUPercent = round2((cpuDelta / sysDelta) * float64(online) * 100)
	}
	usage := s.MemoryStats.Usage
	if cache, ok := s.MemoryStats.Stats["inactive_file"]; ok && usage > cache {
		usage -= cache
	} else if cache, ok := s.MemoryStats.Stats["cache"]; ok && usage > cache {
		usage -= cache
	}
	v.MemoryMB = round2(float64(usage) / 1e6)
	v.MemoryLimitMB = round2(float64(s.MemoryStats.Limit) / 1e6)
	if s.MemoryStats.Limit > 0 {
		v.MemoryPercent = round2(float64(usage) / float64(s.MemoryStats.Limit) * 100)
	}
	for _, n := range s.Networks {
		v.NetRxMB += float64(n.RxBytes) / 1e6
		v.NetTxMB += float64(n.TxBytes) / 1e6
	}
	v.NetRxMB = round2(v.NetRxMB)
	v.NetTxMB = round2(v.NetTxMB)
	for _, b := range s.BlkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(b.Op) {
		case "read":
			v.BlkReadMB += float64(b.Value) / 1e6
		case "write":
			v.BlkWriteMB += float64(b.Value) / 1e6
		}
	}
	v.BlkReadMB = round2(v.BlkReadMB)
	v.BlkWriteMB = round2(v.BlkWriteMB)
	v.Pids = s.PidsStats.Current
	return v
}

func (c *Client) ContainerTop(ctx context.Context, id string) ([]string, error) {
	var out struct {
		Titles    []string   `json:"Titles"`
		Processes [][]string `json:"Processes"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/containers/"+url.PathEscape(id)+"/top?ps_args=aux", nil, &out); err != nil {
		return nil, err
	}
	lines := []string{strings.Join(out.Titles, "\t")}
	for _, p := range out.Processes {
		lines = append(lines, strings.Join(p, "\t"))
	}
	return lines, nil
}

// ---------- exec ----------

type ExecCreateRequest struct {
	AttachStdin  bool     `json:"AttachStdin"`
	AttachStdout bool     `json:"AttachStdout"`
	AttachStderr bool     `json:"AttachStderr"`
	Tty          bool     `json:"Tty"`
	Cmd          []string `json:"Cmd"`
	Env          []string `json:"Env,omitempty"`
	WorkingDir   string   `json:"WorkingDir,omitempty"`
	User         string   `json:"User,omitempty"`
}

func (c *Client) ExecCreate(ctx context.Context, containerID string, req ExecCreateRequest) (string, error) {
	var resp struct {
		ID string `json:"Id"`
	}
	err := c.doJSON(ctx, http.MethodPost, "/containers/"+url.PathEscape(containerID)+"/exec", req, &resp)
	return resp.ID, err
}

func (c *Client) ExecResize(ctx context.Context, execID string, h, w int) error {
	path := fmt.Sprintf("/exec/%s/resize?h=%d&w=%d", url.PathEscape(execID), h, w)
	resp, err := c.Do(ctx, http.MethodPost, path, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// ExecStart hijacks an exec session and returns a duplex stream.
func (c *Client) ExecStart(ctx context.Context, execID string) (io.ReadWriteCloser, error) {
	if c.dial == nil {
		return nil, fmt.Errorf("docker host %s does not support interactive exec", c.host)
	}
	addr := c.dialAddr
	if addr == "" {
		addr = "docker"
	}
	conn, err := c.dial(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	payload := `{"Detach":false,"Tty":true}`
	req := "POST /exec/" + url.PathEscape(execID) + "/start HTTP/1.1\r\n" +
		"Host: docker\r\n" +
		"User-Agent: aidash\r\n" +
		"Content-Type: application/json\r\n" +
		"Content-Length: " + strconv.Itoa(len(payload)) + "\r\n" +
		"Connection: Upgrade\r\n" +
		"Upgrade: tcp\r\n\r\n" + payload
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, err
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols && resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		conn.Close()
		return nil, &APIError{Status: resp.StatusCode, Message: strings.TrimSpace(string(raw))}
	}
	return &hijackedConn{Conn: conn, r: br}, nil
}

type hijackedConn struct {
	net.Conn
	r *bufio.Reader
}

func (h *hijackedConn) Read(p []byte) (int, error) { return h.r.Read(p) }
