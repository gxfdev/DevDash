package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"devdash/internal/model"
	"devdash/internal/logger"

	"github.com/docker/docker/api/types/container"
)

type ContainerMonitor struct {
	dm              *DockerManager
	metricsCache    map[string]*model.ContainerMetrics
	cacheMutex      sync.RWMutex
	collectInterval time.Duration
	stopChan        chan struct{}
	subscribers     map[chan *model.ContainerMetrics]bool
	subMutex        sync.RWMutex
	consecFailures  int
}

func NewContainerMonitor(dm *DockerManager) *ContainerMonitor {
	return &ContainerMonitor{
		dm:              dm,
		metricsCache:    make(map[string]*model.ContainerMetrics),
		collectInterval: 5 * time.Second,
		stopChan:        make(chan struct{}),
		subscribers:     make(map[chan *model.ContainerMetrics]bool),
	}
}

func (cm *ContainerMonitor) Start() error {
	logger.InfoLogger("Starting container metrics collector")
	go cm.collectLoop()
	return nil
}

func (cm *ContainerMonitor) Stop() {
	close(cm.stopChan)
	logger.InfoLogger("Container metrics collector stopped")
}

func (cm *ContainerMonitor) collectLoop() {
	ticker := time.NewTicker(cm.collectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.collectAllMetrics()
		case <-cm.stopChan:
			return
		}
	}
}

func (cm *ContainerMonitor) collectAllMetrics() {
	ctx := context.Background()
	containers, err := cm.dm.ListContainers(false)
	if err != nil {
		cm.consecFailures++
		if cm.consecFailures == 1 {
			logger.WarnLogger(fmt.Sprintf("Docker unavailable, container metrics disabled: %v", err))
		}
		return
	}
	cm.consecFailures = 0

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10)

	for _, c := range containers {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(containerID string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			metrics, err := cm.collectSingleMetrics(ctx, containerID)
			if err != nil {
				logger.ErrorLogger(err, fmt.Sprintf("Failed to collect metrics for container %s", containerID[:12]))
				return
			}

			cm.cacheMutex.Lock()
			cm.metricsCache[containerID] = metrics
			cm.cacheMutex.Unlock()

			cm.notifySubscribers(metrics)
		}(c.ID)
	}

	wg.Wait()
}

func (cm *ContainerMonitor) collectSingleMetrics(ctx context.Context, containerID string) (*model.ContainerMetrics, error) {
	stats, err := cm.dm.cli.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get container stats: %v", err)
	}
	defer stats.Body.Close()

	var stat container.StatsResponse
	if err := json.NewDecoder(stats.Body).Decode(&stat); err != nil {
		return nil, fmt.Errorf("failed to decode stats: %v", err)
	}

	containerInfo, err := cm.dm.GetContainer(containerID)
	if err != nil {
		containerInfo = &ContainerInfo{ID: containerID}
	}

	cpuMetrics := cm.calculateCPUMetrics(&stat)
	memMetrics := cm.calculateMemoryMetrics(&stat)
	networkMetrics := cm.calculateNetworkMetrics(&stat)
	diskIOMetrics := cm.calculateDiskIOMetrics(&stat)

	metrics := &model.ContainerMetrics{
		ContainerID:   containerID,
		ContainerName: containerInfo.Name,
		Image:         containerInfo.Image,
		Timestamp:     time.Now(),
		CPU:           cpuMetrics,
		Memory:        memMetrics,
		Network:       networkMetrics,
		DiskIO:        diskIOMetrics,
		PIDs:          stat.PidsStats.Current,
	}

	return metrics, nil
}

func (cm *ContainerMonitor) calculateCPUMetrics(stat *container.StatsResponse) model.ContainerCPUMetrics {
	cpuDelta := float64(stat.CPUStats.CPUUsage.TotalUsage - stat.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stat.CPUStats.SystemUsage - stat.PreCPUStats.SystemUsage)

	cpuPercent := 0.0
	if systemDelta > 0 && cpuDelta > 0 {
		numCPUs := float64(len(stat.CPUStats.CPUUsage.PercpuUsage))
		if numCPUs == 0 {
			numCPUs = float64(stat.CPUStats.OnlineCPUs)
		}
		cpuPercent = (cpuDelta / systemDelta) * numCPUs * 100.0
	}

	return model.ContainerCPUMetrics{
		UsagePercent:     cpuPercent,
		UsageNano:        stat.CPUStats.CPUUsage.TotalUsage,
		SystemNano:       stat.CPUStats.SystemUsage,
		OnlineCPUs:       int32(stat.CPUStats.OnlineCPUs),
		ThrottledPeriods: stat.CPUStats.ThrottlingData.Periods,
		ThrottledTime:    stat.CPUStats.ThrottlingData.ThrottledTime,
	}
}

func (cm *ContainerMonitor) calculateMemoryMetrics(stat *container.StatsResponse) model.ContainerMemoryMetrics {
	usagePercent := 0.0
	if stat.MemoryStats.Limit > 0 {
		usagePercent = float64(stat.MemoryStats.Usage) / float64(stat.MemoryStats.Limit) * 100.0
	}

	var cache uint64
	if v, ok := stat.MemoryStats.Stats["cache"]; ok {
		cache = v
	}
	var rss uint64
	if v, ok := stat.MemoryStats.Stats["rss"]; ok {
		rss = v
	}

	return model.ContainerMemoryMetrics{
		Usage:        stat.MemoryStats.Usage,
		UsagePercent: usagePercent,
		Limit:        stat.MemoryStats.Limit,
		MaxUsage:     stat.MemoryStats.MaxUsage,
		Stats: model.ContainerMemStats{
			Cache:          cache,
			RSS:            rss,
			SwapUsage:      0,
			SwapLimit:      0,
			KernelUsage:    0,
			KernelTCPUsage: 0,
		},
	}
}

func (cm *ContainerMonitor) calculateNetworkMetrics(stat *container.StatsResponse) model.ContainerNetworkMetrics {
	var rxBytes, txBytes, rxPackets, txPackets uint64
	var rxErrors, txErrors, rxDropped, txDropped uint64

	for _, netStat := range stat.Networks {
		rxBytes += netStat.RxBytes
		txBytes += netStat.TxBytes
		rxPackets += netStat.RxPackets
		txPackets += netStat.TxPackets
		rxErrors += netStat.RxErrors
		txErrors += netStat.TxErrors
		rxDropped += netStat.RxDropped
		txDropped += netStat.TxDropped
	}

	return model.ContainerNetworkMetrics{
		BytesRecv:   rxBytes,
		BytesSent:   txBytes,
		PacketsRecv: rxPackets,
		PacketsSent: txPackets,
		RecvErrors:  rxErrors,
		SendErrors:  txErrors,
		RecvDropped: rxDropped,
		SendDropped: txDropped,
	}
}

func (cm *ContainerMonitor) calculateDiskIOMetrics(stat *container.StatsResponse) model.ContainerDiskIOMetrics {
	var readBytes, writeBytes, readCount, writeCount uint64

	if len(stat.BlkioStats.IoServiceBytesRecursive) > 0 {
		for _, ioEntry := range stat.BlkioStats.IoServiceBytesRecursive {
			if strings.ToLower(ioEntry.Op) == "read" {
				readBytes += ioEntry.Value
			} else if strings.ToLower(ioEntry.Op) == "write" {
				writeBytes += ioEntry.Value
			}
		}

		for _, ioEntry := range stat.BlkioStats.IoServicedRecursive {
			if strings.ToLower(ioEntry.Op) == "read" {
				readCount += ioEntry.Value
			} else if strings.ToLower(ioEntry.Op) == "write" {
				writeCount += ioEntry.Value
			}
		}
	}

	return model.ContainerDiskIOMetrics{
		ReadBytes:  readBytes,
		WriteBytes: writeBytes,
		ReadCount:  readCount,
		WriteCount: writeCount,
		IOPS:       float64(readCount + writeCount),
	}
}

func (cm *ContainerMonitor) GetCachedMetrics(containerID string) (*model.ContainerMetrics, bool) {
	cm.cacheMutex.RLock()
	defer cm.cacheMutex.RUnlock()

	metrics, ok := cm.metricsCache[containerID]
	return metrics, ok
}

func (cm *ContainerMonitor) GetAllCachedMetrics() map[string]*model.ContainerMetrics {
	cm.cacheMutex.RLock()
	defer cm.cacheMutex.RUnlock()

	result := make(map[string]*model.ContainerMetrics, len(cm.metricsCache))
	for k, v := range cm.metricsCache {
		result[k] = v
	}
	return result
}

func (cm *ContainerMonitor) Subscribe() chan *model.ContainerMetrics {
	ch := make(chan *model.ContainerMetrics, 100)

	cm.subMutex.Lock()
	cm.subscribers[ch] = true
	cm.subMutex.Unlock()

	return ch
}

func (cm *ContainerMonitor) Unsubscribe(ch chan *model.ContainerMetrics) {
	cm.subMutex.Lock()
	delete(cm.subscribers, ch)
	cm.subMutex.Unlock()
	close(ch)
}

func (cm *ContainerMonitor) notifySubscribers(metrics *model.ContainerMetrics) {
	cm.subMutex.RLock()
	defer cm.subMutex.RUnlock()

	for ch := range cm.subscribers {
		select {
		case ch <- metrics:
		default:
			logger.InfoLogger(fmt.Sprintf("Subscriber channel full, dropping metrics for %s", metrics.ContainerName))
		}
	}
}

func (cm *ContainerMonitor) GetContainerSummary() []map[string]interface{} {
	cm.cacheMutex.RLock()
	defer cm.cacheMutex.RUnlock()

	summary := make([]map[string]interface{}, 0, len(cm.metricsCache))

	for _, metrics := range cm.metricsCache {
		item := map[string]interface{}{
			"container_id":   metrics.ContainerID,
			"container_name": metrics.ContainerName,
			"image":          metrics.Image,
			"timestamp":      metrics.Timestamp,
			"cpu_percent":    metrics.CPU.UsagePercent,
			"memory_usage":   metrics.Memory.Usage,
			"memory_limit":   metrics.Memory.Limit,
			"memory_percent": metrics.Memory.UsagePercent,
			"network_rx":     metrics.Network.BytesRecv,
			"network_tx":     metrics.Network.BytesSent,
			"pids":           metrics.PIDs,
		}
		summary = append(summary, item)
	}

	return summary
}
