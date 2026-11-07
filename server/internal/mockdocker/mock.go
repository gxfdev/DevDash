// Package mockdocker implements a subset of the Docker Engine HTTP API in
// memory. It lets AIDash's deployment pipeline, gateway and API be tested
// end-to-end on machines without a Docker daemon.
package mockdocker

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gxfdev/aidash/server/internal/engine"
)

type Container struct {
	ID         string
	Name       string
	Image      string
	Config     engine.CreateRequest
	State      string
	ExitCode   int
	Created    time.Time
	LogLines   []string
	Ports      map[string]int
	RestartCnt int
}

type Exec struct {
	ID          string
	ContainerID string
	Cmd         []string
	ExitCode    int
	Running     bool
}

type Mock struct {
	mu          sync.Mutex
	server      *httptest.Server
	containers  map[string]*Container
	order       []string
	execs       map[string]*Exec
	images      map[string]bool
	networks    []map[string]any
	volumes     []map[string]any
	ContainerIP string
	Requests    []string
	PullPlan    []string
	seq         int
}

func New() *Mock {
	m := &Mock{
		containers:  map[string]*Container{},
		execs:       map[string]*Exec{},
		images:      map[string]bool{"alpine:3.20": true},
		ContainerIP: "127.0.0.1",
		networks:    []map[string]any{{"Name": "bridge", "Id": "net_bridge", "Driver": "bridge", "Scope": "local"}},
	}
	m.server = httptest.NewServer(http.HandlerFunc(m.handle))
	return m
}

func (m *Mock) URL() string  { return m.server.URL }
func (m *Mock) Close()       { m.server.Close() }
func (m *Mock) Host() string { return strings.TrimPrefix(m.server.URL, "http://") }

func (m *Mock) RequestLog() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.Requests...)
}

func (m *Mock) ContainerList() []*Container {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Container, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.containers[id])
	}
	return out
}

func (m *Mock) SetContainerLogs(id string, lines ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.containers[id]; ok {
		c.LogLines = lines
	}
}

func (m *Mock) FakeImage(ref string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.images[ref] = true
}

func (m *Mock) nextID(prefix string) string {
	m.seq++
	return fmt.Sprintf("%s%06d%s", prefix, m.seq, strings.Repeat("a", 12))
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (m *Mock) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	m.Requests = append(m.Requests, r.Method+" "+r.URL.String())
	m.mu.Unlock()

	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")

	switch {
	case path == "/_ping":
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	case path == "/version":
		writeJSON(w, 200, map[string]any{
			"Version": "27.0.0-mock", "ApiVersion": "1.46", "MinAPIVersion": "1.24",
			"Os": "linux", "Arch": "amd64", "KernelVersion": "6.0.0-mock",
		})
	case path == "/info":
		m.mu.Lock()
		n := len(m.containers)
		m.mu.Unlock()
		writeJSON(w, 200, map[string]any{
			"ID": "MOCK", "Name": "aidash-mock", "ServerVersion": "27.0.0-mock",
			"Containers": n, "ContainersRunning": m.runningCount(), "Images": len(m.images),
			"Driver": "overlay2", "MemTotal": int64(16 << 30), "NCPU": 8,
			"OSType": "linux", "Architecture": "x86_64", "OperatingSystem": "Mock Linux",
		})
	case path == "/system/df":
		m.mu.Lock()
		imgs := []map[string]any{}
		for ref := range m.images {
			imgs = append(imgs, map[string]any{"Id": "sha256:" + ref, "RepoTags": []string{ref}, "Size": int64(120 << 20)})
		}
		cts := []map[string]any{}
		for _, id := range m.order {
			c := m.containers[id]
			cts = append(cts, map[string]any{"Id": c.ID, "Names": []string{"/" + c.Name}, "Image": c.Image, "State": c.State, "SizeRw": int64(1 << 20)})
		}
		vols := []map[string]any{{"Name": "aidash-ollama", "Driver": "local", "Mountpoint": "/var/lib/docker/volumes/aidash-ollama/_data", "UsageData": map[string]any{"Size": int64(4 << 30)}}}
		m.mu.Unlock()
		writeJSON(w, 200, map[string]any{"LayersSize": int64(1 << 30), "Images": imgs, "Containers": cts, "Volumes": vols})
	case path == "/containers/json":
		m.listContainers(w, r)
	case path == "/containers/create":
		m.createContainer(w, r)
	case len(parts) >= 2 && parts[0] == "containers":
		action := ""
		if len(parts) > 2 {
			action = strings.Join(parts[2:], "/")
		}
		m.containerRoute(w, r, parts[1], action)
	case len(parts) >= 2 && parts[0] == "exec":
		m.execRoute(w, r, parts[1], strings.Join(parts[2:], "/"))
	case path == "/images/json":
		m.listImages(w)
	case path == "/images/create":
		m.pullImage(w, r)
	case len(parts) >= 2 && parts[0] == "images" && strings.HasSuffix(path, "/json"):
		ref := strings.Join(parts[1:len(parts)-1], "/")
		m.mu.Lock()
		_, ok := m.images[ref]
		m.mu.Unlock()
		if !ok {
			writeJSON(w, 404, map[string]any{"message": "No such image: " + ref})
			return
		}
		writeJSON(w, 200, map[string]any{"Id": "sha256:" + ref})
	case len(parts) >= 2 && parts[0] == "images":
		ref := strings.Join(parts[1:], "/")
		m.mu.Lock()
		delete(m.images, ref)
		m.mu.Unlock()
		w.WriteHeader(200)
	case path == "/networks":
		m.mu.Lock()
		nets := append([]map[string]any{}, m.networks...)
		m.mu.Unlock()
		writeJSON(w, 200, nets)
	case path == "/networks/create":
		var body struct {
			Name   string `json:"Name"`
			Driver string `json:"Driver"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		m.mu.Lock()
		id := m.nextID("net")
		m.networks = append(m.networks, map[string]any{"Name": body.Name, "Id": id, "Driver": body.Driver, "Scope": "local"})
		m.mu.Unlock()
		writeJSON(w, 201, map[string]any{"Id": id})
	case len(parts) == 2 && parts[0] == "networks":
		w.WriteHeader(204)
	case path == "/events":
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		if flusher != nil {
			flusher.Flush()
		}
		<-r.Context().Done()
	default:
		writeJSON(w, 404, map[string]any{"message": "mock: not implemented: " + path})
	}
}

func (m *Mock) runningCount() int {
	n := 0
	for _, c := range m.containers {
		if c.State == "running" {
			n++
		}
	}
	return n
}

func (m *Mock) listContainers(w http.ResponseWriter, r *http.Request) {
	all := r.URL.Query().Get("all") == "1"
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []map[string]any{}
	for _, id := range m.order {
		c := m.containers[id]
		if !all && c.State != "running" {
			continue
		}
		ports := []map[string]any{}
		for key, hostPort := range c.Ports {
			contPort, _ := strconv.Atoi(strings.Split(key, "/")[0])
			ports = append(ports, map[string]any{"IP": "0.0.0.0", "PrivatePort": contPort, "PublicPort": hostPort, "Type": "tcp"})
		}
		out = append(out, map[string]any{
			"Id": c.ID, "Names": []string{"/" + c.Name}, "Image": c.Image, "ImageID": "sha256:" + c.Image,
			"Command": strings.Join(c.Config.Cmd, " "), "Created": c.Created.Unix(),
			"State": c.State, "Status": strings.Title(c.State), "Ports": ports,
			"Labels": c.Config.Labels, "SizeRw": int64(1 << 20), "Mounts": []map[string]any{},
			"NetworkSettings": map[string]any{"Networks": map[string]any{"aidash-net": map[string]any{"IPAddress": m.ContainerIP}}},
		})
	}
	writeJSON(w, 200, out)
}

func (m *Mock) createContainer(w http.ResponseWriter, r *http.Request) {
	var req engine.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]any{"message": err.Error()})
		return
	}
	name := r.URL.Query().Get("name")
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.containers {
		if c.Name == name {
			writeJSON(w, 409, map[string]any{"message": "Conflict. The container name \"/" + name + "\" is already in use"})
			return
		}
	}
	id := m.nextID("ct")
	c := &Container{
		ID: id, Name: name, Image: req.Image, Config: req, State: "created",
		Created: time.Now(), Ports: map[string]int{},
		LogLines: []string{"mock container started", "listening on port"},
	}
	for key, bindings := range req.HostConfig.PortBindings {
		if len(bindings) > 0 {
			p, _ := strconv.Atoi(bindings[0].HostPort)
			c.Ports[key] = p
		}
	}
	m.containers[id] = c
	m.order = append(m.order, id)
	writeJSON(w, 201, map[string]any{"Id": id, "Warnings": []string{}})
}

func (m *Mock) containerRoute(w http.ResponseWriter, r *http.Request, id, action string) {
	m.mu.Lock()
	c, ok := m.containers[id]
	m.mu.Unlock()
	if !ok {
		// Allow prefix matching like the real engine.
		m.mu.Lock()
		for cid, cc := range m.containers {
			if strings.HasPrefix(cid, id) {
				c, ok = cc, true
				break
			}
		}
		m.mu.Unlock()
	}
	if !ok {
		writeJSON(w, 404, map[string]any{"message": "No such container: " + id})
		return
	}
	switch {
	case action == "start":
		m.mu.Lock()
		c.State = "running"
		m.mu.Unlock()
		w.WriteHeader(204)
	case action == "stop":
		m.mu.Lock()
		c.State = "exited"
		c.ExitCode = 0
		m.mu.Unlock()
		w.WriteHeader(204)
	case action == "restart":
		m.mu.Lock()
		c.State = "running"
		c.RestartCnt++
		m.mu.Unlock()
		w.WriteHeader(204)
	case action == "json":
		m.mu.Lock()
		defer m.mu.Unlock()
		env := c.Config.Env
		binds := c.Config.HostConfig.Binds
		ports := map[string]any{}
		for key, hp := range c.Ports {
			ports[key] = []map[string]any{{"HostIp": "0.0.0.0", "HostPort": strconv.Itoa(hp)}}
		}
		writeJSON(w, 200, map[string]any{
			"Id": c.ID, "Name": "/" + c.Name, "Created": c.Created.Format(time.RFC3339),
			"State": map[string]any{
				"Status": c.State, "Running": c.State == "running", "Paused": false,
				"Restarting": false, "OOMKilled": false, "Pid": 4242, "ExitCode": c.ExitCode,
				"StartedAt": c.Created.Format(time.RFC3339), "Health": nil,
			},
			"Config": map[string]any{
				"Image": c.Image, "Cmd": c.Config.Cmd, "Env": env, "Labels": c.Config.Labels, "Tty": false,
			},
			"HostConfig": map[string]any{
				"Privileged": c.Config.HostConfig.Privileged, "PidMode": c.Config.HostConfig.PidMode,
				"Binds": binds, "NanoCpus": c.Config.HostConfig.NanoCPUs,
				"Memory": c.Config.HostConfig.Memory, "ShmSize": c.Config.HostConfig.ShmSize,
				"CapAdd": c.Config.HostConfig.CapAdd,
			},
			"NetworkSettings": map[string]any{
				"IPAddress": m.ContainerIP,
				"Ports":     ports,
				"Networks":  map[string]any{"aidash-net": map[string]any{"IPAddress": m.ContainerIP}},
			},
			"Mounts": []map[string]any{}, "RestartCount": c.RestartCnt,
		})
	case action == "logs":
		m.mu.Lock()
		lines := append([]string{}, c.LogLines...)
		m.mu.Unlock()
		writeFramedLogs(w, lines)
	case action == "stats":
		writeJSON(w, 200, map[string]any{
			"read": time.Now().Format(time.RFC3339),
			"cpu_stats": map[string]any{
				"cpu_usage":        map[string]any{"total_usage": 200000000, "percpu_usage": []uint64{100, 100}},
				"system_cpu_usage": 1000000000, "online_cpus": 2,
			},
			"precpu_stats": map[string]any{
				"cpu_usage":        map[string]any{"total_usage": 100000000},
				"system_cpu_usage": 500000000,
			},
			"memory_stats": map[string]any{"usage": 256 << 20, "limit": 1 << 30, "stats": map[string]any{"inactive_file": 0}},
			"networks":     map[string]any{"eth0": map[string]any{"rx_bytes": 1 << 20, "tx_bytes": 2 << 20}},
			"pids_stats":   map[string]any{"current": 12},
			"blkio_stats":  map[string]any{"io_service_bytes_recursive": []map[string]any{{"op": "read", "value": 1 << 20}, {"op": "write", "value": 2 << 20}}},
		})
	case action == "top":
		writeJSON(w, 200, map[string]any{"Titles": []string{"PID", "CMD"}, "Processes": [][]string{{"1", c.Image}}})
	case r.Method == http.MethodDelete:
		m.mu.Lock()
		delete(m.containers, c.ID)
		for i, oid := range m.order {
			if oid == c.ID {
				m.order = append(m.order[:i], m.order[i+1:]...)
				break
			}
		}
		m.mu.Unlock()
		w.WriteHeader(204)
	case action == "exec":
		var req engine.ExecCreateRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		m.mu.Lock()
		execID := m.nextID("ex")
		m.execs[execID] = &Exec{ID: execID, ContainerID: c.ID, Cmd: req.Cmd}
		m.mu.Unlock()
		writeJSON(w, 201, map[string]any{"Id": execID})
	case action == "rename":
		w.WriteHeader(204)
	default:
		writeJSON(w, 404, map[string]any{"message": "mock: unsupported container action " + action})
	}
}

func (m *Mock) execRoute(w http.ResponseWriter, r *http.Request, execID, action string) {
	m.mu.Lock()
	ex, ok := m.execs[execID]
	m.mu.Unlock()
	if !ok {
		writeJSON(w, 404, map[string]any{"message": "No such exec instance"})
		return
	}
	switch action {
	case "start":
		_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 1<<20))
		hj, ok := w.(http.Hijacker)
		if !ok {
			writeJSON(w, 500, map[string]any{"message": "no hijack"})
			return
		}
		conn, brw, err := hj.Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = brw.Flush()
		_, _ = conn.Write([]byte("HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.raw-stream\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n"))
		m.mu.Lock()
		ex.Running = true
		ex.ExitCode = 0
		cmdline := strings.Join(ex.Cmd, " ")
		m.mu.Unlock()
		output := "mock exec output\n"
		switch {
		case strings.Contains(cmdline, "ollama list"):
			output = "NAME\tID\tSIZE\tMODIFIED\n"
		case strings.Contains(cmdline, "ollama pull"):
			output = "pulling manifest\npulling layer1... 100%\nsuccess\n"
		case strings.Contains(cmdline, "sh -lc"):
			output = "mock: " + cmdline + "\n"
		case strings.Contains(cmdline, "uname"):
			output = "Linux mock 6.0.0\n"
		}
		_, _ = conn.Write([]byte(output))
		if strings.Contains(cmdline, "command -v bash") || cmdline == "/bin/sh" {
			// Interactive shell: echo input back so the browser terminal can
			// be demonstrated, and keep the session alive like a real TTY.
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Minute))
			buf := make([]byte, 512)
			for {
				n, rerr := conn.Read(buf)
				if n > 0 {
					_, _ = conn.Write(buf[:n])
				}
				if rerr != nil {
					break
				}
			}
		}
		m.mu.Lock()
		ex.Running = false
		m.mu.Unlock()
	case "json":
		m.mu.Lock()
		defer m.mu.Unlock()
		writeJSON(w, 200, map[string]any{"ID": ex.ID, "Running": ex.Running, "ExitCode": ex.ExitCode, "Pid": 0, "ContainerID": ex.ContainerID})
	case "resize":
		w.WriteHeader(201)
	default:
		writeJSON(w, 404, map[string]any{"message": "mock: unsupported exec action " + action})
	}
}

func (m *Mock) listImages(w http.ResponseWriter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []map[string]any{}
	for ref := range m.images {
		out = append(out, map[string]any{"Id": "sha256:" + ref, "RepoTags": []string{ref}, "Size": int64(120 << 20), "Created": time.Now().Unix(), "Labels": map[string]string{}})
	}
	writeJSON(w, 200, out)
}

func (m *Mock) pullImage(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("fromImage")
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		tag = "latest"
	}
	ref := from + ":" + tag
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	for _, status := range []string{"Pulling from library/" + from, "Status: Downloaded newer image for " + ref} {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "id": tag})
		if flusher != nil {
			flusher.Flush()
		}
	}
	m.mu.Lock()
	m.images[ref] = true
	m.mu.Unlock()
}

func writeFramedLogs(w http.ResponseWriter, lines []string) {
	w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
	w.WriteHeader(200)
	var buf []byte
	for _, line := range lines {
		payload := []byte(line + "\n")
		header := make([]byte, 8)
		header[0] = 1
		binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
		buf = append(buf, header...)
		buf = append(buf, payload...)
	}
	_, _ = w.Write(buf)
}

// ---------- helpers used by tests ----------

// Dialer returns a net.Conn to the mock (not used, kept for interface parity).
func (m *Mock) Dialer() func() (net.Conn, error) { return nil }
