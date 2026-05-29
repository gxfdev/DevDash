package main

import (
	"log"
	"time"

	"github.com/gxfdev/DevDash/server/internal/collector"
	"github.com/gxfdev/DevDash/server/internal/config"
)

func main() {
	cfg := config.LoadAgent()
	c := collector.NewCollector()

	ticker := time.NewTicker(time.Duration(cfg.CollectInterval) * time.Second)
	for range ticker.C {
		snap, err := c.Collect()
		if err != nil {
			log.Printf("采集失败: %v", err)
			continue
		}
		snap.NodeID = cfg.NodeID
		log.Printf("已采集 node=%s", cfg.NodeID)
	}
}