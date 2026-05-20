package node

import (
	"sync"
	"time"

	"devdash/internal/model"
	"devdash/internal/store"
)

type NodeManager struct {
	mu    sync.RWMutex
	nodes map[string]*model.Node
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
	if nm.s != nil {
		_ = nm.s.SaveNode(n)
	}
}

func (nm *NodeManager) UpdateHeartbeat(id string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	if n, ok := nm.nodes[id]; ok {
		n.LastHeartbeat = time.Now()
		n.Status = "online"
	}
	if nm.s != nil {
		_ = nm.s.UpdateNodeHeartbeat(id)
	}
}

func (nm *NodeManager) ListNodes() []*model.Node {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	result := make([]*model.Node, 0, len(nm.nodes))
	for _, n := range nm.nodes {
		status := "online"
		if time.Since(n.LastHeartbeat) > 2*time.Minute {
			status = "offline"
		}
		result = append(result, &model.Node{
			ID:            n.ID,
			Name:          n.Name,
			OS:            n.OS,
			Arch:          n.Arch,
			IP:            n.IP,
			Role:          n.Role,
			Token:         n.Token,
			Status:        status,
			LastHeartbeat: n.LastHeartbeat,
			CreatedAt:     n.CreatedAt,
		})
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

func (nm *NodeManager) SyncFromDB() {
	if nm.s == nil {
		return
	}
	nm.mu.Lock()
	defer nm.mu.Unlock()
	dbNodes, err := nm.s.ListNodes()
	if err != nil {
		return
	}
	for i := range dbNodes {
		n := &dbNodes[i]
		nm.nodes[n.ID] = n
	}
}
