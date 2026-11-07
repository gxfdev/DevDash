package collector

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gxfdev/aidash/server/internal/model"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// Collector samples the host (or the mounted host when running in a
// container with HOST_PROC/HOST_SYS set) and derives rate metrics.
type Collector struct {
	mu          sync.Mutex
	lastNetRecv uint64
	lastNetSent uint64
	lastRead    uint64
	lastWrite   uint64
	lastAt      time.Time
	gpuOK       bool
	gpuChecked  bool
}

func New() *Collector {
	c := &Collector{}
	if p := os.Getenv("HOST_PROC"); p != "" {
		_ = os.Setenv("HOST_PROC", p)
	}
	return c
}

func (c *Collector) Collect() (*model.HostSnapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	snap := &model.HostSnapshot{Timestamp: time.Now()}

	// CPU
	if pcts, err := cpu.Percent(0, false); err == nil && len(pcts) > 0 {
		snap.CPU.UsagePercent = round2(pcts[0])
	}
	snap.CPU.Cores = runtime.NumCPU()
	if per, err := cpu.Percent(0, true); err == nil {
		for _, p := range per {
			snap.CPU.PerCore = append(snap.CPU.PerCore, round2(p))
		}
	}

	// Memory
	if vm, err := mem.VirtualMemory(); err == nil {
		snap.Memory = model.Memory{
			TotalGB:      round2(float64(vm.Total) / 1e9),
			UsedGB:       round2(float64(vm.Used) / 1e9),
			AvailableGB:  round2(float64(vm.Available) / 1e9),
			UsagePercent: round2(vm.UsedPercent),
			SwapTotalGB:  round2(float64(vm.SwapTotal) / 1e9),
			SwapUsedGB:   round2(float64(vm.SwapCached) / 1e9),
		}
	}

	// Disk usage of the busiest mount
	if parts, err := disk.Partitions(false); err == nil {
		var best *disk.UsageStat
		for _, p := range parts {
			if p.Fstype == "" || strings.HasPrefix(p.Fstype, "devfs") || strings.HasPrefix(p.Fstype, "tmpfs") || p.Fstype == "squashfs" {
				continue
			}
			u, err := disk.Usage(p.Mountpoint)
			if err != nil || u.Total < 1<<30 {
				continue
			}
			if best == nil || u.Total > best.Total {
				best = u
			}
		}
		if best != nil {
			snap.Disk.TotalGB = round2(float64(best.Total) / 1e9)
			snap.Disk.UsedGB = round2(float64(best.Used) / 1e9)
			snap.Disk.FreeGB = round2(float64(best.Free) / 1e9)
			snap.Disk.UsagePercent = round2(best.UsedPercent)
		}
	}

	// Disk IO rate
	if counters, err := disk.IOCounters(); err == nil {
		var read, write uint64
		for _, v := range counters {
			read += v.ReadBytes
			write += v.WriteBytes
		}
		now := time.Now()
		if !c.lastAt.IsZero() {
			dt := now.Sub(c.lastAt).Seconds()
			if dt > 0 && read >= c.lastRead && write >= c.lastWrite {
				snap.Disk.ReadRateMB = round2(float64(read-c.lastRead) / 1e6 / dt)
				snap.Disk.WriteRateMB = round2(float64(write-c.lastWrite) / 1e6 / dt)
			}
		}
		c.lastRead, c.lastWrite = read, write
	}

	// Network
	if counters, err := net.IOCounters(false); err == nil && len(counters) > 0 {
		nc := counters[0]
		snap.Network.BytesRecv = nc.BytesRecv
		snap.Network.BytesSent = nc.BytesSent
		now := time.Now()
		if !c.lastAt.IsZero() {
			dt := now.Sub(c.lastAt).Seconds()
			if dt > 0 && nc.BytesRecv >= c.lastNetRecv && nc.BytesSent >= c.lastNetSent {
				snap.Network.RecvRateMB = round2(float64(nc.BytesRecv-c.lastNetRecv) / 1e6 / dt)
				snap.Network.SentRateMB = round2(float64(nc.BytesSent-c.lastNetSent) / 1e6 / dt)
			}
		}
		c.lastNetRecv, c.lastNetSent = nc.BytesRecv, nc.BytesSent
	}
	c.lastAt = time.Now()

	// Load & host info
	if avg, err := load.Avg(); err == nil {
		snap.Load = model.Load{Load1: round2(avg.Load1), Load5: round2(avg.Load5), Load15: round2(avg.Load15)}
	}
	if h, err := host.Info(); err == nil {
		snap.Host = model.HostInfo{
			Hostname:        h.Hostname,
			OS:              h.OS,
			Platform:        h.Platform,
			PlatformVersion: h.PlatformVersion,
			KernelVersion:   h.KernelVersion,
			UptimeSeconds:   h.Uptime,
			Arch:            h.KernelArch,
		}
	}

	// GPU (best effort, cached availability)
	snap.GPU = c.collectGPU()
	return snap, nil
}

func (c *Collector) collectGPU() *model.GPU {
	if !c.gpuChecked {
		c.gpuChecked = true
		_, err := exec.LookPath("nvidia-smi")
		c.gpuOK = err == nil
	}
	if !c.gpuOK {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=index,name,utilization.gpu,memory.used,memory.total,temperature.gpu,driver_version",
		"--format=csv,noheader,nounits").Output()
	if err != nil {
		return nil
	}
	gpu := &model.GPU{}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	var totalMem, usedMem float64
	var utilSum float64
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 7 {
			continue
		}
		idx, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		name := strings.TrimSpace(parts[1])
		util, _ := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
		mUsed, _ := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
		mTotal, _ := strconv.ParseFloat(strings.TrimSpace(parts[4]), 64)
		temp, _ := strconv.ParseFloat(strings.TrimSpace(parts[5]), 64)
		driver := strings.TrimSpace(parts[6])
		gpu.Devices = append(gpu.Devices, model.GPUDevice{
			Index: idx, Name: name, UsagePercent: util,
			MemUsedMB: mUsed, MemTotalMB: mTotal, Temperature: temp,
		})
		if gpu.Name == "" {
			gpu.Name = name
		}
		if gpu.Driver == "" {
			gpu.Driver = driver
		}
		totalMem += mTotal
		usedMem += mUsed
		utilSum += util
	}
	if len(gpu.Devices) == 0 {
		return nil
	}
	gpu.MemTotalMB = round2(totalMem)
	gpu.MemUsedMB = round2(usedMem)
	gpu.UsagePercent = round2(utilSum / float64(len(gpu.Devices)))
	if gpu.MemTotalMB > 0 {
		gpu.Temperature = gpu.Devices[0].Temperature
	}
	return gpu
}

func round2(v float64) float64 {
	if v < 0 {
		v = 0
	}
	iv := int64(v*100 + 0.5)
	return float64(iv) / 100
}
