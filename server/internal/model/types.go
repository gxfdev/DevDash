package model

import "time"

// ---------- identity ----------

type User struct {
	ID            int64      `json:"id"`
	Username      string     `json:"username"`
	PasswordHash  string     `json:"-"`
	Role          string     `json:"role"` // admin | operator | viewer
	MustChangePwd bool       `json:"must_change_pwd"`
	CreatedAt     time.Time  `json:"created_at"`
	LastLoginAt   *time.Time `json:"last_login_at,omitempty"`
}

type APIKey struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Prefix       string     `json:"prefix"`
	KeyHash      string     `json:"-"`
	Role         string     `json:"role"`
	CreatedAt    time.Time  `json:"created_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	Revoked      bool       `json:"revoked"`
	TokenLimit   int64      `json:"token_limit"`
	TokensUsed   int64      `json:"tokens_used"`
	RequestsUsed int64      `json:"requests_used"`
}

// ---------- deployment ----------

type PortMap struct {
	HostPort   int    `json:"host_port"`
	ContPort   int    `json:"container_port"`
	Protocol   string `json:"protocol"`
	BindHost   string `json:"bind_host,omitempty"`
	Accessible bool   `json:"accessible"`
}

type VolumeMount struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only"`
}

type ResourceSpec struct {
	CPUs     float64 `json:"cpus"`
	MemoryMB int64   `json:"memory_mb"`
	ShmMB    int64   `json:"shm_mb"`
	GPUCount int     `json:"gpu_count"`
	GPUKind  string  `json:"gpu_kind,omitempty"`
}

// HostAccess describes the "运维模式": the workload keeps running in a
// container but is granted host-level reach (host pid, docker socket,
// host root mount). It is implemented with plain Docker, so the same
// capability works on a single host without an orchestrator.
type HostAccess struct {
	Enabled           bool          `json:"enabled"`
	Privileged        bool          `json:"privileged"`
	HostPID           bool          `json:"host_pid"`
	MountHostFS       bool          `json:"mount_host_fs"`
	MountDockerSocket bool          `json:"mount_docker_socket"`
	ExtraMounts       []VolumeMount `json:"extra_mounts,omitempty"`
	CapAdd            []string      `json:"cap_add,omitempty"`
}

type ServiceSpec struct {
	Name         string            `json:"name"`
	Kind         string            `json:"kind"` // model | app | custom
	CatalogID    string            `json:"catalog_id"`
	Image        string            `json:"image"`
	Command      []string          `json:"command,omitempty"`
	Entrypoint   []string          `json:"entrypoint,omitempty"`
	Env          map[string]string `json:"env"`
	Ports        []PortMap         `json:"ports"`
	Volumes      []VolumeMount     `json:"volumes"`
	Resources    ResourceSpec      `json:"resources"`
	HostAccess   HostAccess        `json:"host_access"`
	Restart      string            `json:"restart"`
	ModelName    string            `json:"model_name,omitempty"` // name exposed on the AI gateway
	Description  string            `json:"description,omitempty"`
	AutoStart    bool              `json:"auto_start"`
	ModelAssetID string            `json:"model_asset_id,omitempty"`
	ModelRuntime string            `json:"model_runtime,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

type Service struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Kind        string      `json:"kind"`
	CatalogID   string      `json:"catalog_id"`
	Image       string      `json:"image"`
	ModelName   string      `json:"model_name,omitempty"`
	Description string      `json:"description,omitempty"`
	Spec        ServiceSpec `json:"spec"`
	Status      string      `json:"status"` // pending | pulling | creating | starting | running | stopped | error | removing
	ContainerID string      `json:"container_id,omitempty"`
	Health      string      `json:"health"` // unknown | healthy | unhealthy
	Endpoints   []string    `json:"endpoints"`
	Error       string      `json:"error,omitempty"`
	Owner       string      `json:"owner"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	StartedAt   *time.Time  `json:"started_at,omitempty"`
	HostAccess  bool        `json:"host_access"`
}

type ServiceEvent struct {
	ID        int64     `json:"id"`
	ServiceID string    `json:"service_id"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditLog struct {
	ID        int64     `json:"id"`
	User      string    `json:"user"`
	Role      string    `json:"role"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Detail    string    `json:"detail"`
	IP        string    `json:"ip"`
	Result    string    `json:"result"`
	CreatedAt time.Time `json:"created_at"`
}

type TokenUsage struct {
	ID               int64     `json:"id"`
	TS               time.Time `json:"ts"`
	KeyID            string    `json:"key_id"`
	KeyName          string    `json:"key_name"`
	ServiceID        string    `json:"service_id"`
	Model            string    `json:"model"`
	PromptTokens     int64     `json:"prompt_tokens"`
	CompletionTokens int64     `json:"completion_tokens"`
	TotalTokens      int64     `json:"total_tokens"`
	Status           int       `json:"status"`
	LatencyMS        int64     `json:"latency_ms"`
}

// ---------- observability ----------

type HostSnapshot struct {
	Timestamp  time.Time        `json:"timestamp"`
	CPU        CPU              `json:"cpu"`
	Memory     Memory           `json:"memory"`
	Disk       Disk             `json:"disk"`
	Network    Network          `json:"network"`
	Load       Load             `json:"load"`
	Host       HostInfo         `json:"host"`
	GPU        *GPU             `json:"gpu,omitempty"`
	Containers ContainerSummary `json:"containers"`
}

type CPU struct {
	Cores        int       `json:"cores"`
	UsagePercent float64   `json:"usage_percent"`
	PerCore      []float64 `json:"per_core_usage"`
}

type Memory struct {
	TotalGB      float64 `json:"total_gb"`
	UsedGB       float64 `json:"used_gb"`
	AvailableGB  float64 `json:"available_gb"`
	UsagePercent float64 `json:"usage_percent"`
	SwapTotalGB  float64 `json:"swap_total_gb"`
	SwapUsedGB   float64 `json:"swap_used_gb"`
}

type Disk struct {
	TotalGB      float64 `json:"total_gb"`
	UsedGB       float64 `json:"used_gb"`
	FreeGB       float64 `json:"free_gb"`
	UsagePercent float64 `json:"usage_percent"`
	ReadRateMB   float64 `json:"read_rate_mb"`
	WriteRateMB  float64 `json:"write_rate_mb"`
}

type Network struct {
	BytesRecv  uint64  `json:"bytes_recv"`
	BytesSent  uint64  `json:"bytes_sent"`
	RecvRateMB float64 `json:"recv_rate_mb"`
	SentRateMB float64 `json:"sent_rate_mb"`
}

type Load struct {
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
	Arch            string `json:"arch"`
}

type GPU struct {
	Name         string      `json:"name"`
	UsagePercent float64     `json:"usage_percent"`
	MemUsedMB    float64     `json:"mem_used_mb"`
	MemTotalMB   float64     `json:"mem_total_mb"`
	Temperature  float64     `json:"temperature"`
	Devices      []GPUDevice `json:"devices"`
	Driver       string      `json:"driver,omitempty"`
}

type GPUDevice struct {
	Index        int     `json:"gpu_index"`
	Name         string  `json:"name"`
	UsagePercent float64 `json:"usage_percent"`
	MemUsedMB    float64 `json:"mem_used_mb"`
	MemTotalMB   float64 `json:"mem_total_mb"`
	Temperature  float64 `json:"temperature_celsius"`
}

type ContainerSummary struct {
	Total      int `json:"total"`
	Running    int `json:"running"`
	Stopped    int `json:"stopped"`
	AIServices int `json:"ai_services"`
}

type MetricPoint struct {
	TS     int64   `json:"ts"`
	CPU    float64 `json:"cpu"`
	Memory float64 `json:"memory"`
	Disk   float64 `json:"disk"`
	NetRX  float64 `json:"net_rx"`
	NetTX  float64 `json:"net_tx"`
	GPU    float64 `json:"gpu"`
	GPUMem float64 `json:"gpu_mem"`
}

// ---------- catalog ----------

type CatalogItem struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Category          string            `json:"category"`
	Vendor            string            `json:"vendor"`
	Kind              string            `json:"kind"` // model | app
	Icon              string            `json:"icon"`
	Accent            string            `json:"accent"`
	Summary           string            `json:"summary"`
	Description       string            `json:"description"`
	Tags              []string          `json:"tags"`
	Params            string            `json:"params,omitempty"`
	SizeGB            float64           `json:"size_gb,omitempty"`
	Image             string            `json:"image"`
	ImageTag          string            `json:"image_tag"`
	Command           []string          `json:"command,omitempty"`
	Env               map[string]string `json:"env,omitempty"`
	Ports             []PortMap         `json:"ports"`
	Volumes           []VolumeMount     `json:"volumes,omitempty"`
	Resources         ResourceSpec      `json:"resources"`
	RequiresGPU       bool              `json:"requires_gpu"`
	ModelName         string            `json:"model_name,omitempty"`
	Runtime           string            `json:"runtime,omitempty"`   // ollama | vllm | custom
	ModelRef          string            `json:"model_ref,omitempty"` // e.g. qwen2.5:7b
	DefaultHostAccess bool              `json:"default_host_access"`
	Copilot           bool              `json:"copilot"`
	DocsURL           string            `json:"docs_url,omitempty"`
	Managed           bool              `json:"managed"` // true -> managed by AIDash engine
	Order             int               `json:"order"`
}

// ---------- model artifacts ----------

type ModelAsset struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Source      string    `json:"source"` // modelscope | huggingface | ollama | local | url
	Repo        string    `json:"repo,omitempty"`
	Revision    string    `json:"revision,omitempty"`
	File        string    `json:"file,omitempty"` // optional single file (GGUF etc.)
	Mirror      string    `json:"mirror,omitempty"`
	LocalPath   string    `json:"local_path"` // path inside the panel/serving container, e.g. /models/<id>
	HostPath    string    `json:"host_path,omitempty"`
	Volume      string    `json:"volume,omitempty"`
	SizeBytes   int64     `json:"size_bytes"`
	FileCount   int       `json:"file_count"`
	Status      string    `json:"status"` // queued | downloading | completed | failed | canceled | registry
	Error       string    `json:"error,omitempty"`
	Log         string    `json:"log,omitempty"`
	ContainerID string    `json:"container_id,omitempty"`
	Runtime     string    `json:"runtime"` // llamacpp | vllm | ollama
	GGUF        string    `json:"gguf,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
