package docker

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gxfdev/DevDash/server/internal/model"
	"github.com/gxfdev/DevDash/server/internal/logger"

	"gorm.io/gorm"
)

type HistoryStorage struct {
	db             *gorm.DB
	buffer         []model.ContainerHistoryRecord
	bufferMutex    sync.Mutex
	flushInterval  time.Duration
	maxBufferSize  int
	stopChan       chan struct{}
}

func NewHistoryStorage(db *gorm.DB) *HistoryStorage {
	return &HistoryStorage{
		db:            db,
		buffer:        make([]model.ContainerHistoryRecord, 0, 1000),
		flushInterval: 30 * time.Second,
		maxBufferSize: 1000,
		stopChan:      make(chan struct{}),
	}
}

func (hs *HistoryStorage) Start() error {
	logger.InfoLogger("Starting container metrics history storage")

	err := hs.autoMigrate()
	if err != nil {
		return fmt.Errorf("failed to migrate database: %v", err)
	}

	go hs.flushLoop()

	return nil
}

func (hs *HistoryStorage) Stop() {
	hs.flushBuffer()
	close(hs.stopChan)
	logger.InfoLogger("Container history storage stopped")
}

func (hs *HistoryStorage) autoMigrate() error {
	return hs.db.AutoMigrate(&model.ContainerHistoryRecord{})
}

func (hs *HistoryStorage) flushLoop() {
	ticker := time.NewTicker(hs.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hs.flushBuffer()
		case <-hs.stopChan:
			return
		}
	}
}

func (hs *HistoryStorage) StoreMetrics(metrics *model.ContainerMetrics) {
	record := model.ContainerHistoryRecord{
		ContainerID:   metrics.ContainerID,
		ContainerName: metrics.ContainerName,
		Timestamp:     metrics.Timestamp,
		CPUPercent:    metrics.CPU.UsagePercent,
		MemoryUsage:   metrics.Memory.Usage,
		MemoryLimit:   metrics.Memory.Limit,
		NetworkRx:     metrics.Network.BytesRecv,
		NetworkTx:     metrics.Network.BytesSent,
		BlockRead:     metrics.DiskIO.ReadBytes,
		BlockWrite:    metrics.DiskIO.WriteBytes,
	}

	rawData, err := json.Marshal(metrics)
	if err == nil {
		record.RawData = string(rawData)
	}

	hs.bufferMutex.Lock()
	hs.buffer = append(hs.buffer, record)

	if len(hs.buffer) >= hs.maxBufferSize {
		hs.bufferMutex.Unlock()
		hs.flushBuffer()
	} else {
		hs.bufferMutex.Unlock()
	}
}

func (hs *HistoryStorage) flushBuffer() {
	hs.bufferMutex.Lock()
	if len(hs.buffer) == 0 {
		hs.bufferMutex.Unlock()
		return
	}

	buffer := make([]model.ContainerHistoryRecord, len(hs.buffer))
	copy(buffer, hs.buffer)
	hs.buffer = hs.buffer[:0]
	hs.bufferMutex.Unlock()

	if err := hs.db.Create(&buffer).Error; err != nil {
		logger.ErrorLogger(err, "Failed to flush container metrics buffer")
		
		hs.bufferMutex.Lock()
		hs.buffer = append(buffer, hs.buffer...)
		hs.bufferMutex.Unlock()
	}
}

func (hs *HistoryStorage) GetContainerHistory(containerID string, startTime, endTime time.Time) ([]model.ContainerHistoryRecord, error) {
	var records []model.ContainerHistoryRecord

	query := hs.db.Where("container_id = ?", containerID)
	
	if !startTime.IsZero() {
		query = query.Where("timestamp >= ?", startTime)
	}
	
	if !endTime.IsZero() {
		query = query.Where("timestamp <= ?", endTime)
	}

	err := query.Order("timestamp ASC").Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get container history: %v", err)
	}

	return records, nil
}

func (hs *HistoryStorage) GetRecentHistory(containerID string, duration time.Duration) ([]model.ContainerHistoryRecord, error) {
	startTime := time.Now().Add(-duration)
	return hs.GetContainerHistory(containerID, startTime, time.Time{})
}

func (hs *HistoryStorage) GetAggregatedMetrics(containerID string, interval string, limit int) ([]map[string]interface{}, error) {
	validIntervals := map[string]string{
		"1m":  "1 MINUTE",
		"5m":  "5 MINUTE",
		"15m": "15 MINUTE",
		"1h":  "1 HOUR",
		"1d":  "1 DAY",
	}

	sqlInterval, ok := validIntervals[interval]
	if !ok {
		sqlInterval = "5 MINUTE"
	}

	query := fmt.Sprintf(`
		SELECT 
			time_bucket('%s', timestamp) AS bucket,
			AVG(cpu_percent) AS avg_cpu,
			MAX(cpu_percent) AS max_cpu,
			MIN(cpu_percent) AS min_cpu,
			AVG(memory_usage) AS avg_memory,
			MAX(memory_usage) AS max_memory,
			AVG(network_rx + network_tx) AS avg_network,
			COUNT(*) AS sample_count
		FROM container_history_records
		WHERE container_id = ?
			AND timestamp >= NOW() - INTERVAL '24 hours'
		GROUP BY bucket
		ORDER BY bucket DESC
		LIMIT ?
	`, sqlInterval)

	var results []map[string]interface{}
	if err := hs.db.Raw(query, containerID, limit).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get aggregated metrics: %v", err)
	}

	return results, nil
}

func (hs *HistoryStorage) CleanupOldRecords(retentionDays int) error {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)
	
	result := hs.db.Where("timestamp < ?", cutoffDate).Delete(&model.ContainerHistoryRecord{})
	if result.Error != nil {
		return fmt.Errorf("failed to cleanup old records: %v", result.Error)
	}

	logger.InfoLogger(fmt.Sprintf("Cleaned up %d old container metric records", result.RowsAffected))
	return nil
}

func (hs *HistoryStorage) GetTopContainersByCPU(limit int, duration time.Duration) ([]map[string]interface{}, error) {
	startTime := time.Now().Add(-duration)

	query := `
		SELECT 
			container_id,
			container_name,
			AVG(cpu_percent) AS avg_cpu,
			MAX(cpu_percent) AS peak_cpu,
			COUNT(*) AS samples
		FROM container_history_records
		WHERE timestamp >= ?
		GROUP BY container_id, container_name
		ORDER BY avg_cpu DESC
		LIMIT ?
	`

	var results []map[string]interface{}
	if err := hs.db.Raw(query, startTime, limit).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get top containers by CPU: %v", err)
	}

	return results, nil
}

func (hs *HistoryStorage) GetTopContainersByMemory(limit int, duration time.Duration) ([]map[string]interface{}, error) {
	startTime := time.Now().Add(-duration)

	query := `
		SELECT 
			container_id,
			container_name,
			AVG(memory_usage) AS avg_memory,
			MAX(memory_usage) AS peak_memory,
			AVG(memory_limit) AS avg_limit,
			COUNT(*) AS samples
		FROM container_history_records
		WHERE timestamp >= ?
		GROUP BY container_id, container_name
		ORDER BY avg_memory DESC
		LIMIT ?
	`

	var results []map[string]interface{}
	if err := hs.db.Raw(query, startTime, limit).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get top containers by memory: %v", err)
	}

	return results, nil
}
