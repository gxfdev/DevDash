package collector

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"devdash/internal/logger"
)

type GPUCollector struct {
	available bool
	checked   bool
}

func NewGPUCollector() *GPUCollector {
	return &GPUCollector{}
}

func (g *GPUCollector) isAvailable() bool {
	if g.checked {
		return g.available
	}
	g.checked = true
	if runtime.GOOS == "windows" {
		g.available = g.checkCommand("nvidia-smi")
	} else {
		g.available = g.checkCommand("nvidia-smi")
	}
	if !g.available {
		logger.DebugLogger("nvidia-smi not found, GPU monitoring disabled")
	}
	return g.available
}

func (g *GPUCollector) checkCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

type GPUDeviceMetrics struct {
	Index           int     `json:"gpu_index"`
	Name            string  `json:"name"`
	UsagePercent    float64 `json:"usage_percent"`
	MemUsedMB       float64 `json:"mem_used_mb"`
	MemTotalMB      float64 `json:"mem_total_mb"`
	TemperatureC    float64 `json:"temperature_celsius"`
	PowerDrawW      float64 `json:"power_draw_w,omitempty"`
	PowerLimitW     float64 `json:"power_limit_w,omitempty"`
	FanSpeedPercent float64 `json:"fan_speed_percent,omitempty"`
}

func (g *GPUCollector) Collect(ctx context.Context) []GPUDeviceMetrics {
	if !g.isAvailable() {
		return nil
	}

	result, err := g.queryNvidiaSMI(ctx)
	if err != nil {
		logger.DebugLogger(fmt.Sprintf("nvidia-smi query failed: %v", err))
		g.available = false
		g.checked = false
		return nil
	}
	return result
}

func (g *GPUCollector) queryNvidiaSMI(ctx context.Context) ([]GPUDeviceMetrics, error) {
	cmd := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=index,name,utilization.gpu,memory.used,memory.total,temperature.gpu,power.draw,power.limit,fan.speed",
		"--format=csv,noheader,nounits",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("nvidia-smi execution failed: %w, stderr: %s", err, stderr.String())
	}

	reader := csv.NewReader(&stdout)
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse nvidia-smi CSV output: %w", err)
	}

	var devices []GPUDeviceMetrics
	for _, record := range records {
		if len(record) < 6 {
			continue
		}
		device := GPUDeviceMetrics{}

		if v, err := strconv.Atoi(strings.TrimSpace(record[0])); err == nil {
			device.Index = v
		}
		device.Name = strings.TrimSpace(record[1])
		if v, err := strconv.ParseFloat(strings.TrimSpace(record[2]), 64); err == nil {
			device.UsagePercent = v
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(record[3]), 64); err == nil {
			device.MemUsedMB = v
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(record[4]), 64); err == nil {
			device.MemTotalMB = v
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(record[5]), 64); err == nil {
			device.TemperatureC = v
		}
		if len(record) > 6 {
			if v, err := strconv.ParseFloat(strings.TrimSpace(record[6]), 64); err == nil {
				device.PowerDrawW = v
			}
		}
		if len(record) > 7 {
			if v, err := strconv.ParseFloat(strings.TrimSpace(record[7]), 64); err == nil {
				device.PowerLimitW = v
			}
		}
		if len(record) > 8 {
			if v, err := strconv.ParseFloat(strings.TrimSpace(record[8]), 64); err == nil {
				device.FanSpeedPercent = v
			}
		}

		devices = append(devices, device)
	}

	return devices, nil
}

func init() {
	log.SetFlags(0)
}
