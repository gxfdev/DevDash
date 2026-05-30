package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gxfdev/DevDash/server/internal/collector"
	"github.com/gxfdev/DevDash/server/internal/model"

	"github.com/gin-gonic/gin"
)

type RemoteHost struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	Token    string `json:"token,omitempty"`
	Status   string `json:"status"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
}

type HostMetrics struct {
	Snapshot   *model.Snapshot       `json:"snapshot,omitempty"`
	Containers []model.ContainerInfo `json:"containers,omitempty"`
}

type AgentManager struct {
	mu      sync.RWMutex
	hosts   map[string]*RemoteHost
	metrics map[string]*HostMetrics
}

func NewAgentManager() *AgentManager {
	return &AgentManager{
		hosts:   make(map[string]*RemoteHost),
		metrics: make(map[string]*HostMetrics),
	}
}

func (m *AgentManager) RegisterHost(host *RemoteHost) error {
	if host.ID == "" {
		return fmt.Errorf("host ID is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hosts[host.ID] = host
	m.metrics[host.ID] = &HostMetrics{}
	return nil
}

func (m *AgentManager) RemoveHost(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.hosts, id)
	delete(m.metrics, id)
}

func (m *AgentManager) GetHost(id string) (*RemoteHost, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, ok := m.hosts[id]
	return h, ok
}

func (m *AgentManager) ListHosts() []*RemoteHost {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*RemoteHost, 0, len(m.hosts))
	for _, h := range m.hosts {
		result = append(result, h)
	}
	return result
}

func (m *AgentManager) CollectFromHost(id string) (*HostMetrics, error) {
	m.mu.RLock()
	host, ok := m.hosts[id]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("host %s not found", id)
	}

	snap, containers, err := m.fetchFromEndpoint(host)
	if err != nil {
		m.mu.Lock()
		if h, exists := m.hosts[id]; exists {
			h.Status = "offline"
		}
		m.mu.Unlock()
		return nil, err
	}

	m.mu.Lock()
	if h, exists := m.hosts[id]; exists {
		h.Status = "online"
	}
	m.metrics[id] = &HostMetrics{
		Snapshot:   snap,
		Containers: containers,
	}
	hm := m.metrics[id]
	m.mu.Unlock()

	return hm, nil
}

func (m *AgentManager) GetHostMetrics(id string) (*HostMetrics, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	hm, ok := m.metrics[id]
	return hm, ok
}

func (m *AgentManager) GetHostContainers(id string) ([]model.ContainerInfo, error) {
	m.mu.RLock()
	host, ok := m.hosts[id]
	hm := m.metrics[id]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("host %s not found", id)
	}
	if hm != nil && len(hm.Containers) > 0 {
		return hm.Containers, nil
	}
	_, containers, err := m.fetchFromEndpoint(host)
	return containers, err
}

func (m *AgentManager) GetHostContainerStats(hostID, containerID string) (map[string]interface{}, error) {
	m.mu.RLock()
	host, ok := m.hosts[hostID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("host %s not found", hostID)
	}

	url := fmt.Sprintf("%s/api/v1/containers/%s/stats", host.Endpoint, containerID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if host.Token != "" {
		req.Header.Set("Authorization", "Bearer "+host.Token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}

func (m *AgentManager) CollectFromAllHosts() map[string]*HostMetrics {
	m.mu.RLock()
	ids := make([]string, 0, len(m.hosts))
	for id := range m.hosts {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	result := make(map[string]*HostMetrics)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(hostID string) {
			defer wg.Done()
			hm, err := m.CollectFromHost(hostID)
			if err != nil {
				log.Printf("[agent] collect from %s failed: %v", hostID, err)
				return
			}
			mu.Lock()
			result[hostID] = hm
			mu.Unlock()
		}(id)
	}
	wg.Wait()
	return result
}

func (m *AgentManager) GetAllMetrics() map[string]*HostMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*HostMetrics, len(m.metrics))
	for k, v := range m.metrics {
		result[k] = v
	}
	return result
}

func (m *AgentManager) StartPeriodicCollection(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			m.CollectFromAllHosts()
		}
	}()
	log.Printf("[agent] periodic collection started, interval=%s", interval)
}

func (m *AgentManager) fetchFromEndpoint(host *RemoteHost) (*model.Snapshot, []model.ContainerInfo, error) {
	url := fmt.Sprintf("%s/api/v1/metrics", host.Endpoint)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}
	if host.Token != "" {
		req.Header.Set("Authorization", "Bearer "+host.Token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read response: %w", err)
	}

	var snap model.Snapshot
	if err := json.Unmarshal(body, &snap); err != nil {
		return nil, nil, fmt.Errorf("decode snapshot: %w", err)
	}

	return &snap, snap.Containers, nil
}

type AgentServer struct {
	store     interface{}
	nodeMgr   interface{}
	collector *collector.Collector
}

func NewAgentServer(store interface{}, nodeMgr interface{}, c *collector.Collector) *AgentServer {
	return &AgentServer{
		store:     store,
		nodeMgr:   nodeMgr,
		collector: c,
	}
}

func (s *AgentServer) RegisterRoutes(r *gin.Engine) {
	agent := r.Group("/api/v1/agent")
	{
		agent.GET("/metrics", s.getMetrics)
		agent.GET("/health", s.healthCheck)
	}
}

func (s *AgentServer) getMetrics(c *gin.Context) {
	if s.collector == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "collector not available"})
		return
	}
	snap, err := s.collector.Collect()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snap)
}

func (s *AgentServer) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "timestamp": time.Now().Unix()})
}
