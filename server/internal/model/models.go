package model

import "time"

type Node struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	OS            string    `json:"os"`
	Arch          string    `json:"arch"`
	IP            string    `json:"ip"`
	Role          string    `json:"role"`
	Token         string    `json:"token,omitempty"`
	Status        string    `json:"status"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	CreatedAt     time.Time `json:"created_at"`
}

type Snapshot struct {
	NodeID     string                `json:"node_id"`
	Timestamp  time.Time             `json:"timestamp"`
	CPU        CPUMetrics            `json:"cpu"`
	Memory     MemoryMetrics         `json:"memory"`
	Disk       DiskMetrics           `json:"disk"`
	Network    NetworkMetrics        `json:"network"`
	Load       LoadMetrics           `json:"load"`
	Host       HostInfo              `json:"host"`
	Processes  []ProcessInfo         `json:"processes"`
	Containers []ContainerInfo       `json:"containers"`
	GPU        *GPUMetrics           `json:"gpu,omitempty"`
	Sensors    *SensorInfo           `json:"sensors,omitempty"`
	DiskIO     *DiskIOMetrics        `json:"disk_io,omitempty"`
	TCPConns   *TCPConnectionMetrics `json:"tcp_conns,omitempty"`
}

type CPUMetrics struct {
	Cores        int       `json:"cores"`
	UsagePercent float64   `json:"usage_percent"`
	PerCoreUsage []float64 `json:"per_core_usage"`
}

type MemoryMetrics struct {
	TotalGB      float64 `json:"total_gb"`
	UsedGB       float64 `json:"used_gb"`
	AvailableGB  float64 `json:"available_gb"`
	UsagePercent float64 `json:"usage_percent"`
	SwapTotalGB  float64 `json:"swap_total_gb"`
	SwapUsedGB   float64 `json:"swap_used_gb"`
}

type DiskMetrics struct {
	TotalGB      float64 `json:"total_gb"`
	UsedGB       float64 `json:"used_gb"`
	FreeGB       float64 `json:"free_gb"`
	UsagePercent float64 `json:"usage_percent"`
}

type NetworkMetrics struct {
	BytesRecv  uint64  `json:"bytes_recv"`
	BytesSent  uint64  `json:"bytes_sent"`
	RecvRateMB float64 `json:"recv_rate_mb"`
	SentRateMB float64 `json:"sent_rate_mb"`
}

type LoadMetrics struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

type HostInfo struct {
	Hostname        string `json:"hostname"`
	OS              string `json:"os"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	KernelVersion   string `json:"kernel_version"`
	UptimeSeconds   uint64 `json:"uptime_seconds"`
}

type ProcessInfo struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
	MemMB      float64 `json:"mem_mb"`
	Status     string  `json:"status"`
}

type ContainerInfo struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Image      string  `json:"image"`
	Status     string  `json:"status"`
	CPUPercent float64 `json:"cpu_percent"`
	MemUsageMB float64 `json:"mem_usage_mb"`
	MemLimitMB float64 `json:"mem_limit_mb"`
	Created    string  `json:"created"`
}

type GPUMetrics struct {
	Name         string      `json:"name"`
	UsagePercent float64     `json:"usage_percent"`
	MemUsedMB    float64     `json:"mem_used_mb"`
	MemTotalMB   float64     `json:"mem_total_mb"`
	Temperature  float64     `json:"temperature"`
	Devices      []GPUDevice `json:"devices,omitempty"`
}

type GPUDevice struct {
	Index        int     `json:"gpu_index"`
	Name         string  `json:"name"`
	UsagePercent float64 `json:"usage_percent"`
	MemUsedMB    float64 `json:"mem_used_mb"`
	MemTotalMB   float64 `json:"mem_total_mb"`
	Temperature  float64 `json:"temperature_celsius"`
}

type GPUMetricRow struct {
	ID           int64     `json:"id"`
	NodeID       string    `json:"node_id"`
	Timestamp    time.Time `json:"timestamp"`
	GPUIndex     int       `json:"gpu_index"`
	UsagePercent float64   `json:"usage_percent"`
	MemUsedMB    float64   `json:"mem_used_mb"`
	MemTotalMB   float64   `json:"mem_total_mb"`
	TemperatureC float64   `json:"temperature_celsius"`
}

type SensorInfo struct {
	CPUTemp     float64 `json:"cpu_temp"`
	CPUCoreTemp float64 `json:"cpu_core_temp"`
}

type DiskIOMetrics struct {
	ReadMB      float64 `json:"read_mb"`
	WriteMB     float64 `json:"write_mb"`
	IOPS        float64 `json:"iops"`
	ReadRateMB  float64 `json:"read_rate_mb"`
	WriteRateMB float64 `json:"write_rate_mb"`
}

type TCPConnectionMetrics struct {
	Established int `json:"established"`
	TimeWait    int `json:"time_wait"`
	CloseWait   int `json:"close_wait"`
	Listen      int `json:"listen"`
}

type Alert struct {
	ID        int       `json:"id"`
	NodeID    string    `json:"node_id"`
	NodeName  string    `json:"node_name"`
	Type      string    `json:"type"`
	Level     string    `json:"level"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Time      time.Time `json:"time"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
}

type User struct {
	ID            int    `json:"id"`
	Username      string `json:"username"`
	PasswordHash  string `json:"-"`
	Role          string `json:"role"`
	OTPEnabled    bool   `json:"otp_enabled"`
	MustChangePwd bool   `json:"must_change_pwd"`
}

type AuditLog struct {
	ID     int       `json:"id"`
	UserID int       `json:"user_id"`
	NodeID string    `json:"node_id"`
	Action string    `json:"action"`
	Detail string    `json:"detail"`
	Result string    `json:"result"`
	Time   time.Time `json:"time"`
}

type Software struct {
	ID      int    `json:"id"`
	NodeID  string `json:"node_id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Status  string `json:"status"`
}

type CronJob struct {
	ID         int    `json:"id"`
	NodeID     string `json:"node_id"`
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Command    string `json:"command"`
	Enabled    bool   `json:"enabled"`
}
