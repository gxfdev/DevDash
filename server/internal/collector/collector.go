package collector

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"devdash/internal/model"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	psnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

type Collector struct {
	mu          sync.RWMutex
	lastNet     psnet.IOCountersStat
	lastNetTime time.Time
	lastRecv    uint64
	lastSent    uint64
	gpu         *GPUCollector
}

func NewCollector() *Collector {
	return &Collector{
		gpu: NewGPUCollector(),
	}
}

func (c *Collector) Collect() (*model.Snapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	snap := &model.Snapshot{Timestamp: time.Now()}
	var wg sync.WaitGroup
	wg.Add(6)
	go func() {
		defer wg.Done()
		defer func() { recover() }()
		snap.CPU = c.collectCPU(ctx)
	}()
	go func() {
		defer wg.Done()
		defer func() { recover() }()
		snap.Memory = c.collectMemory(ctx)
	}()
	go func() {
		defer wg.Done()
		defer func() { recover() }()
		snap.Disk = c.collectDisk(ctx)
	}()
	go func() {
		defer wg.Done()
		defer func() { recover() }()
		snap.Network = c.collectNetwork(ctx)
	}()
	go func() {
		defer wg.Done()
		defer func() { recover() }()
		snap.Load = c.collectLoad(ctx)
	}()
	go func() {
		defer wg.Done()
		defer func() { recover() }()
		snap.Host = c.collectHost(ctx)
	}()
	wg.Wait()

	func() {
		defer func() { recover() }()
		snap.Processes = c.collectProcesses(ctx)
	}()
	func() {
		defer func() { recover() }()
		snap.Containers = c.collectContainers(ctx)
	}()
	func() {
		defer func() { recover() }()
		snap.GPU = c.collectGPU(ctx)
	}()
	func() {
		defer func() { recover() }()
		snap.Sensors = c.collectSensors(ctx)
	}()
	func() {
		defer func() { recover() }()
		snap.DiskIO = c.collectDiskIO(ctx)
	}()
	func() {
		defer func() { recover() }()
		snap.TCPConns = c.collectTCPConns(ctx)
	}()
	return snap, nil
}

func (c *Collector) collectCPU(_ context.Context) model.CPUMetrics {
	m := model.CPUMetrics{Cores: runtime.NumCPU()}
	if p, err := cpu.Percent(time.Second, false); err == nil && len(p) > 0 {
		m.UsagePercent = round(p[0])
	}
	if p, err := cpu.Percent(time.Second, true); err == nil {
		for _, v := range p {
			m.PerCoreUsage = append(m.PerCoreUsage, round(v))
		}
	}
	return m
}

func (c *Collector) collectMemory(_ context.Context) model.MemoryMetrics {
	m := model.MemoryMetrics{}
	if v, err := mem.VirtualMemory(); err == nil {
		m.TotalGB = gb(v.Total)
		m.UsedGB = gb(v.Used)
		m.AvailableGB = gb(v.Available)
		m.UsagePercent = round(v.UsedPercent)
	}
	if v, err := mem.SwapMemory(); err == nil {
		m.SwapTotalGB = gb(v.Total)
		m.SwapUsedGB = gb(v.Used)
	}
	return m
}

func (c *Collector) collectDisk(_ context.Context) model.DiskMetrics {
	m := model.DiskMetrics{}
	partitions, err := disk.Partitions(false)
	if err != nil {
		return m
	}
	for _, p := range partitions {
		if u, err := disk.Usage(p.Mountpoint); err == nil && u.Total > 0 {
			m.TotalGB = gb(u.Total)
			m.UsedGB = gb(u.Used)
			m.FreeGB = gb(u.Free)
			m.UsagePercent = round(u.UsedPercent)
			break
		}
	}
	return m
}

func (c *Collector) collectNetwork(_ context.Context) model.NetworkMetrics {
	m := model.NetworkMetrics{}
	io, err := psnet.IOCounters(false)
	if err != nil || len(io) == 0 {
		return m
	}
	cur := io[0]
	now := time.Now()
	m.BytesRecv = cur.BytesRecv
	m.BytesSent = cur.BytesSent
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastNetTime.IsZero() {
		c.lastNet = cur
		c.lastNetTime = now
		c.lastRecv = cur.BytesRecv
		c.lastSent = cur.BytesSent
		return m
	}
	elapsed := now.Sub(c.lastNetTime).Seconds()
	if elapsed > 0 {
		m.RecvRateMB = round(float64(cur.BytesRecv-c.lastRecv) / elapsed / 1024 / 1024)
		m.SentRateMB = round(float64(cur.BytesSent-c.lastSent) / elapsed / 1024 / 1024)
	}
	c.lastNet = cur
	c.lastNetTime = now
	c.lastRecv = cur.BytesRecv
	c.lastSent = cur.BytesSent
	return m
}

func (c *Collector) collectLoad(_ context.Context) model.LoadMetrics {
	m := model.LoadMetrics{}
	if l, err := load.Avg(); err == nil {
		m.Load1 = round(l.Load1)
		m.Load5 = round(l.Load5)
		m.Load15 = round(l.Load15)
	}
	return m
}

func (c *Collector) collectHost(_ context.Context) model.HostInfo {
	m := model.HostInfo{}
	if i, err := host.Info(); err == nil {
		m.Hostname = i.Hostname
		m.OS = i.OS
		m.Platform = i.Platform
		m.PlatformVersion = i.PlatformVersion
		m.KernelVersion = i.KernelVersion
		m.UptimeSeconds = i.Uptime
	}
	return m
}

func (c *Collector) collectProcesses(ctx context.Context) []model.ProcessInfo {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil
	}
	var result []model.ProcessInfo
	for _, p := range procs {
		if len(result) >= 20 {
			break
		}
		name, _ := p.NameWithContext(ctx)
		cpuP, _ := p.CPUPercentWithContext(ctx)
		memP, _ := p.MemoryPercentWithContext(ctx)
		memInfo, _ := p.MemoryInfoWithContext(ctx)
		status, _ := p.StatusWithContext(ctx)
		var memMB float64
		if memInfo != nil {
			memMB = float64(memInfo.RSS) / 1024 / 1024
		}
		s := "unknown"
		if len(status) > 0 {
			s = status[0]
		}
		result = append(result, model.ProcessInfo{
			PID:        p.Pid,
			Name:       name,
			CPUPercent: round(cpuP),
			MemPercent: round(float64(memP)),
			MemMB:      round(memMB),
			Status:     s,
		})
	}
	return result
}

func (c *Collector) collectContainers(_ context.Context) []model.ContainerInfo { return nil }

func (c *Collector) collectGPU(ctx context.Context) *model.GPUMetrics {
	devices := c.gpu.Collect(ctx)
	if len(devices) == 0 {
		return nil
	}
	d := devices[0]
	total := &model.GPUMetrics{
		Name:         d.Name,
		UsagePercent: d.UsagePercent,
		MemUsedMB:    d.MemUsedMB,
		MemTotalMB:   d.MemTotalMB,
		Temperature:  d.TemperatureC,
		Devices:      make([]model.GPUDevice, 0, len(devices)),
	}
	for _, dev := range devices {
		total.Devices = append(total.Devices, model.GPUDevice{
			Index:        dev.Index,
			Name:         dev.Name,
			UsagePercent: dev.UsagePercent,
			MemUsedMB:    dev.MemUsedMB,
			MemTotalMB:   dev.MemTotalMB,
			Temperature:  dev.TemperatureC,
		})
	}
	if len(devices) > 1 {
		var totalUsage, totalMemUsed, totalMemTotal float64
		for _, dev := range devices {
			totalUsage += dev.UsagePercent
			totalMemUsed += dev.MemUsedMB
			totalMemTotal += dev.MemTotalMB
		}
		total.UsagePercent = round(totalUsage / float64(len(devices)))
		total.MemUsedMB = totalMemUsed
		total.MemTotalMB = totalMemTotal
	}
	return total
}

func (c *Collector) collectSensors(_ context.Context) *model.SensorInfo    { return nil }

func (c *Collector) collectDiskIO(_ context.Context) *model.DiskIOMetrics {
	io, err := disk.IOCounters()
	if err != nil || len(io) == 0 {
		return nil
	}
	var rB, wB uint64
	for _, v := range io {
		rB += v.ReadBytes
		wB += v.WriteBytes
	}
	return &model.DiskIOMetrics{
		ReadMB:  round(float64(rB) / 1024 / 1024),
		WriteMB: round(float64(wB) / 1024 / 1024),
	}
}

func (c *Collector) collectTCPConns(_ context.Context) *model.TCPConnectionMetrics {
	conns, err := psnet.Connections("tcp")
	if err != nil {
		return nil
	}
	m := &model.TCPConnectionMetrics{}
	for _, co := range conns {
		switch co.Status {
		case "ESTABLISHED":
			m.Established++
		case "TIME_WAIT":
			m.TimeWait++
		case "CLOSE_WAIT":
			m.CloseWait++
		case "LISTEN":
			m.Listen++
		}
	}
	return m
}

func gb(b uint64) float64 { return round(float64(b) / 1024 / 1024 / 1024) }
func round(v float64) float64 {
	s := fmt.Sprintf("%.2f", v)
	var r float64
	fmt.Sscanf(s, "%f", &r)
	return r
}
