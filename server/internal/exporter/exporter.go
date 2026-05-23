package exporter

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"devdash/internal/collector"

	"github.com/gin-gonic/gin"
)

type Exporter struct {
	collector *collector.Collector
	mu        sync.RWMutex
	lastSnap  map[string]metricEntry
	nodeID    string
}

type metricEntry struct {
	Value     float64
	Timestamp int64
	Labels    map[string]string
}

func NewExporter(c *collector.Collector, nodeID string) *Exporter {
	return &Exporter{
		collector: c,
		nodeID:    nodeID,
		lastSnap:  make(map[string]metricEntry),
	}
}

func (e *Exporter) RegisterRoutes(r *gin.Engine) {
	r.GET("/metrics", e.metricsHandler)
	r.GET("/api/v1/exporter/metrics", e.metricsHandler)
}

func (e *Exporter) metricsHandler(c *gin.Context) {
	snap, err := e.collector.Collect()
	if err != nil {
		c.String(http.StatusInternalServerError, "# ERROR: collection failed\n")
		return
	}

	var b strings.Builder
	b.WriteString("# HELP devdash_up Whether the node is up (1=up, 0=down)\n")
	b.WriteString("# TYPE devdash_up gauge\n")
	b.WriteString(fmt.Sprintf("devdash_up{node=\"%s\"} 1 %d\n", e.nodeID, time.Now().UnixMilli()))

	b.WriteString("# HELP devdash_cpu_usage_percent CPU usage percentage\n")
	b.WriteString("# TYPE devdash_cpu_usage_percent gauge\n")
	b.WriteString(fmt.Sprintf("devdash_cpu_usage_percent{node=\"%s\",cores=\"%d\"} %.2f %d\n",
		e.nodeID, snap.CPU.Cores, snap.CPU.UsagePercent, time.Now().UnixMilli()))

	b.WriteString("# HELP devdash_cpu_cores Number of CPU cores\n")
	b.WriteString("# TYPE devdash_cpu_cores gauge\n")
	b.WriteString(fmt.Sprintf("devdash_cpu_cores{node=\"%s\"} %d %d\n",
		e.nodeID, snap.CPU.Cores, time.Now().UnixMilli()))

	b.WriteString("# HELP devdash_memory_usage_percent Memory usage percentage\n")
	b.WriteString("# TYPE devdash_memory_usage_percent gauge\n")
	b.WriteString(fmt.Sprintf("devdash_memory_usage_percent{node=\"%s\"} %.2f %d\n",
		e.nodeID, snap.Memory.UsagePercent, time.Now().UnixMilli()))

	b.WriteString("# HELP devdash_memory_total_bytes Total memory in bytes\n")
	b.WriteString("# TYPE devdash_memory_total_bytes gauge\n")
	b.WriteString(fmt.Sprintf("devdash_memory_total_bytes{node=\"%s\"} %.0f %d\n",
		e.nodeID, snap.Memory.TotalGB*1024*1024*1024, time.Now().UnixMilli()))

	b.WriteString("# HELP devdash_memory_used_bytes Used memory in bytes\n")
	b.WriteString("# TYPE devdash_memory_used_bytes gauge\n")
	b.WriteString(fmt.Sprintf("devdash_memory_used_bytes{node=\"%s\"} %.0f %d\n",
		e.nodeID, snap.Memory.UsedGB*1024*1024*1024, time.Now().UnixMilli()))

	b.WriteString("# HELP devdash_disk_usage_percent Disk usage percentage\n")
	b.WriteString("# TYPE devdash_disk_usage_percent gauge\n")
	b.WriteString(fmt.Sprintf("devdash_disk_usage_percent{node=\"%s\"} %.2f %d\n",
		e.nodeID, snap.Disk.UsagePercent, time.Now().UnixMilli()))

	b.WriteString("# HELP devdash_disk_total_bytes Total disk in bytes\n")
	b.WriteString("# TYPE devdash_disk_total_bytes gauge\n")
	b.WriteString(fmt.Sprintf("devdash_disk_total_bytes{node=\"%s\"} %.0f %d\n",
		e.nodeID, snap.Disk.TotalGB*1024*1024*1024, time.Now().UnixMilli()))

	b.WriteString("# HELP devdash_disk_used_bytes Used disk in bytes\n")
	b.WriteString("# TYPE devdash_disk_used_bytes gauge\n")
	b.WriteString(fmt.Sprintf("devdash_disk_used_bytes{node=\"%s\"} %.0f %d\n",
		e.nodeID, snap.Disk.UsedGB*1024*1024*1024, time.Now().UnixMilli()))

	if snap.DiskIO != nil {
		b.WriteString("# HELP devdash_disk_io_read_bytes Disk I/O read bytes\n")
		b.WriteString("# TYPE devdash_disk_io_read_bytes counter\n")
		b.WriteString(fmt.Sprintf("devdash_disk_io_read_bytes{node=\"%s\"} %.0f %d\n",
			e.nodeID, snap.DiskIO.ReadMB*1024*1024, time.Now().UnixMilli()))

		b.WriteString("# HELP devdash_disk_io_write_bytes Disk I/O write bytes\n")
		b.WriteString("# TYPE devdash_disk_io_write_bytes counter\n")
		b.WriteString(fmt.Sprintf("devdash_disk_io_write_bytes{node=\"%s\"} %.0f %d\n",
			e.nodeID, snap.DiskIO.WriteMB*1024*1024, time.Now().UnixMilli()))
	}

	b.WriteString("# HELP devdash_network_bytes_recv Network bytes received\n")
	b.WriteString("# TYPE devdash_network_bytes_recv counter\n")
	b.WriteString(fmt.Sprintf("devdash_network_bytes_recv{node=\"%s\"} %d %d\n",
		e.nodeID, snap.Network.BytesRecv, time.Now().UnixMilli()))

	b.WriteString("# HELP devdash_network_bytes_sent Network bytes sent\n")
	b.WriteString("# TYPE devdash_network_bytes_sent counter\n")
	b.WriteString(fmt.Sprintf("devdash_network_bytes_sent{node=\"%s\"} %d %d\n",
		e.nodeID, snap.Network.BytesSent, time.Now().UnixMilli()))

	if snap.Load.Load1 > 0 || snap.Load.Load5 > 0 || snap.Load.Load15 > 0 {
		b.WriteString("# HELP devdash_load1 System load average 1min\n")
		b.WriteString("# TYPE devdash_load1 gauge\n")
		b.WriteString(fmt.Sprintf("devdash_load1{node=\"%s\"} %.2f %d\n",
			e.nodeID, snap.Load.Load1, time.Now().UnixMilli()))

		b.WriteString("# HELP devdash_load5 System load average 5min\n")
		b.WriteString("# TYPE devdash_load5 gauge\n")
		b.WriteString(fmt.Sprintf("devdash_load5{node=\"%s\"} %.2f %d\n",
			e.nodeID, snap.Load.Load5, time.Now().UnixMilli()))

		b.WriteString("# HELP devdash_load15 System load average 15min\n")
		b.WriteString("# TYPE devdash_load15 gauge\n")
		b.WriteString(fmt.Sprintf("devdash_load15{node=\"%s\"} %.2f %d\n",
			e.nodeID, snap.Load.Load15, time.Now().UnixMilli()))
	}

	if snap.TCPConns != nil {
		b.WriteString("# HELP devdash_tcp_connections TCP connections by state\n")
		b.WriteString("# TYPE devdash_tcp_connections gauge\n")
		b.WriteString(fmt.Sprintf("devdash_tcp_connections{node=\"%s\",state=\"established\"} %d %d\n",
			e.nodeID, snap.TCPConns.Established, time.Now().UnixMilli()))
		b.WriteString(fmt.Sprintf("devdash_tcp_connections{node=\"%s\",state=\"time_wait\"} %d %d\n",
			e.nodeID, snap.TCPConns.TimeWait, time.Now().UnixMilli()))
		b.WriteString(fmt.Sprintf("devdash_tcp_connections{node=\"%s\",state=\"close_wait\"} %d %d\n",
			e.nodeID, snap.TCPConns.CloseWait, time.Now().UnixMilli()))
		b.WriteString(fmt.Sprintf("devdash_tcp_connections{node=\"%s\",state=\"listen\"} %d %d\n",
			e.nodeID, snap.TCPConns.Listen, time.Now().UnixMilli()))
	}

	if snap.GPU != nil {
		b.WriteString("# HELP devdash_gpu_usage_percent GPU usage percentage\n")
		b.WriteString("# TYPE devdash_gpu_usage_percent gauge\n")
		b.WriteString(fmt.Sprintf("devdash_gpu_usage_percent{node=\"%s\",gpu=\"%s\"} %.2f %d\n",
			e.nodeID, snap.GPU.Name, snap.GPU.UsagePercent, time.Now().UnixMilli()))

		b.WriteString("# HELP devdash_gpu_memory_used_bytes GPU memory used in bytes\n")
		b.WriteString("# TYPE devdash_gpu_memory_used_bytes gauge\n")
		b.WriteString(fmt.Sprintf("devdash_gpu_memory_used_bytes{node=\"%s\",gpu=\"%s\"} %.0f %d\n",
			e.nodeID, snap.GPU.Name, snap.GPU.MemUsedMB*1024*1024, time.Now().UnixMilli()))

		b.WriteString("# HELP devdash_gpu_temperature_celsius GPU temperature in celsius\n")
		b.WriteString("# TYPE devdash_gpu_temperature_celsius gauge\n")
		b.WriteString(fmt.Sprintf("devdash_gpu_temperature_celsius{node=\"%s\",gpu=\"%s\"} %.1f %d\n",
			e.nodeID, snap.GPU.Name, snap.GPU.Temperature, time.Now().UnixMilli()))
	}

	b.WriteString("# HELP devdash_uptime_seconds System uptime in seconds\n")
	b.WriteString("# TYPE devdash_uptime_seconds gauge\n")
	b.WriteString(fmt.Sprintf("devdash_uptime_seconds{node=\"%s\"} %d %d\n",
		e.nodeID, snap.Host.UptimeSeconds, time.Now().UnixMilli()))

	b.WriteString(fmt.Sprintf("\n# Build info: go%s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH))

	c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	c.String(http.StatusOK, b.String())
}

func (e *Exporter) StartPeriodicCollection(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			snap, err := e.collector.Collect()
			if err != nil {
				log.Printf("[exporter] collection failed: %v", err)
				continue
			}
			e.mu.Lock()
			now := time.Now().UnixMilli()
			e.lastSnap["cpu_usage"] = metricEntry{Value: snap.CPU.UsagePercent, Timestamp: now, Labels: map[string]string{"node": e.nodeID}}
			e.lastSnap["memory_usage"] = metricEntry{Value: snap.Memory.UsagePercent, Timestamp: now, Labels: map[string]string{"node": e.nodeID}}
			e.lastSnap["disk_usage"] = metricEntry{Value: snap.Disk.UsagePercent, Timestamp: now, Labels: map[string]string{"node": e.nodeID}}
			e.mu.Unlock()
		}
	}()
	log.Printf("[exporter] periodic collection started, interval=%s", interval)
}
