package node

import (
	"sync"
	"time"

	"devdash/internal/model"
	"devdash/internal/store"
)

type NodeManager struct {
	mu    sync.RWMutex
	nodes map[string]*model.Node // in-memory cache for fast heartbeat updates
	s     *store.Store
}

func NewNodeManager(s *store.Store) *NodeManager {
	return &NodeManager{
		nodes: make(map[string]*model.Node),
		s:     s,
	}
}

func (nm *NodeManager) Register(n *model.Node) {
	n.Status = "online"
	n.LastHeartbeat = time.Now()
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.nodes[n.ID] = n
	// Persist to DB so main collection goroutine can find it
	if nm.s != nil {
		nm.s.CreateNode(n)
	}
}

func (nm *NodeManager) UpdateHeartbeat(id string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	if n, ok := nm.nodes[id]; ok {
		n.LastHeartbeat = time.Now()
		n.Status = "online"
	}
	// Also update in DB for persistence
	if nm.s != nil {
		nm.s.UpdateNodeStatus(id, "online")
	}
}

func (nm *NodeManager) ListNodes() []*model.Node {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	result := make([]*model.Node, 0, len(nm.nodes))
	for _, n := range nm.nodes {
		if time.Since(n.LastHeartbeat) > 2*time.Minute {
			n.Status = "offline"
		}
		result = append(result, n)
	}
	return result
}

func (nm *NodeManager) GetNode(id string) *model.Node {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.nodes[id]
}

func (nm *NodeManager) RemoveNode(id string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	delete(nm.nodes, id)
}

// SyncFromDB reloads nodes from DB into in-memory cache.
// Call this on startup to recover nodes that were persisted but not in memory.
func (nm *NodeManager) SyncFromDB() {
	if nm.s == nil {
		return
	}
	nm.mu.Lock()
	defer nm.mu.Unlock()
	dbNodes := nm.s.ListNodes()
	for _, n := range dbNodes {
		if m, ok := n.(map[string]interface{}); ok {
			node := &model.Node{
				ID:     getString(m, "id"),
				Name:   getString(m, "name"),
				OS:     getString(m, "os"),
				Arch:   getString(m, "arch"),
				IP:     getString(m, "ip"),
				Role:   getString(m, "role"),
				Token:  getString(m, "token"),
				Status: getString(m, "status"),
			}
			nm.nodes[node.ID] = node
		}
	}
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}