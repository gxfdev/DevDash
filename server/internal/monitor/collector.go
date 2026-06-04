package monitor

import (
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

type SystemInfo struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Platform string `json:"platform"`
	Arch     string `json:"arch"`
	Uptime   uint64 `json:"uptime"`
	Time     string `json:"time"`
}

type CPUInfo struct {
	Count  int       `json:"count"`
	Usage  []float64 `json:"usage"`
	Load1  float64   `json:"load1"`
	Load5  float64   `json:"load5"`
	Load15 float64   `json:"load15"`
}

type MemoryInfo struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Available   uint64  `json:"available"`
	UsedPercent float64 `json:"usedPercent"`
	SwapTotal   uint64  `json:"swapTotal"`
	SwapUsed    uint64  `json:"swapUsed"`
}

type DiskInfo struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"usedPercent"`
}

type NetworkIO struct {
	BytesSent   uint64 `json:"bytesSent"`
	BytesRecv   uint64 `json:"bytesRecv"`
	PacketsSent uint64 `json:"packetsSent"`
	PacketsRecv uint64 `json:"packetsRecv"`
}

type Status struct {
	System  SystemInfo `json:"system"`
	CPU     CPUInfo    `json:"cpu"`
	Memory  MemoryInfo `json:"memory"`
	Disk    DiskInfo   `json:"disk"`
	Network NetworkIO  `json:"network"`
}

func Collect() (*Status, error) {
	s := &Status{}

	hostInfo, err := host.Info()
	if err == nil {
		s.System = SystemInfo{
			Hostname: hostInfo.Hostname,
			OS:       hostInfo.OS,
			Platform: hostInfo.Platform + " " + hostInfo.PlatformVersion,
			Arch:     runtime.GOARCH,
			Uptime:   hostInfo.Uptime,
			Time:     time.Now().Format("2006-01-02 15:04:05"),
		}
	}

	cpuPercents, _ := cpu.Percent(0, true)
	s.CPU = CPUInfo{
		Count: runtime.NumCPU(),
		Usage: cpuPercents,
	}

	if lv, err := load.Avg(); err == nil {
		s.CPU.Load1 = lv.Load1
		s.CPU.Load5 = lv.Load5
		s.CPU.Load15 = lv.Load15
	}

	if vm, err := mem.VirtualMemory(); err == nil {
		s.Memory = MemoryInfo{
			Total:       vm.Total,
			Used:        vm.Used,
			Available:   vm.Available,
			UsedPercent: vm.UsedPercent,
			SwapTotal:   0,
			SwapUsed:    0,
		}
		if sm, err := mem.SwapMemory(); err == nil {
			s.Memory.SwapTotal = sm.Total
			s.Memory.SwapUsed = sm.Used
		}
	}

	if du, err := disk.Usage("/"); err == nil {
		s.Disk = DiskInfo{
			Total:       du.Total,
			Used:        du.Used,
			Free:        du.Free,
			UsedPercent: du.UsedPercent,
		}
	}

	if nc, err := net.IOCounters(false); err == nil && len(nc) > 0 {
		s.Network = NetworkIO{
			BytesSent:   nc[0].BytesSent,
			BytesRecv:   nc[0].BytesRecv,
			PacketsSent: nc[0].PacketsSent,
			PacketsRecv: nc[0].PacketsRecv,
		}
	}

	return s, nil
}
