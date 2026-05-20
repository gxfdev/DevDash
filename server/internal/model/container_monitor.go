package model

import "time"

type ContainerMetrics struct {
	ContainerID   string                    `json:"container_id"`
	ContainerName string                    `json:"container_name"`
	Image         string                    `json:"image"`
	Timestamp     time.Time                 `json:"timestamp"`
	CPU           ContainerCPUMetrics       `json:"cpu"`
	Memory        ContainerMemoryMetrics    `json:"memory"`
	Network       ContainerNetworkMetrics   `json:"network"`
	DiskIO        ContainerDiskIOMetrics    `json:"disk_io"`
	PIDs          uint64                    `json:"pids"`
}

type ContainerCPUMetrics struct {
	UsagePercent     float64 `json:"usage_percent"`
	UsageNano        uint64  `json:"usage_nano"`
	SystemNano       uint64  `json:"system_nano"`
	OnlineCPUs       int32   `json:"online_cpus"`
	ThrottledPeriods uint64  `json:"throttled_periods"`
	ThrottledTime    uint64  `json:"throttled_time"`
}

type ContainerMemoryMetrics struct {
	Usage        uint64            `json:"usage"`
	UsagePercent float64           `json:"usage_percent"`
	Limit        uint64            `json:"limit"`
	MaxUsage     uint64            `json:"max_usage"`
	Stats        ContainerMemStats `json:"stats"`
}

type ContainerMemStats struct {
	Cache          uint64 `json:"cache"`
	RSS            uint64 `json:"rss"`
	SwapUsage      uint64 `json:"swap_usage"`
	SwapLimit      uint64 `json:"swap_limit"`
	KernelUsage    uint64 `json:"kernel_usage"`
	KernelTCPUsage uint64 `json:"kernel_tcp_usage"`
}

type ContainerNetworkMetrics struct {
	BytesRecv    uint64  `json:"bytes_recv"`
	BytesSent    uint64  `json:"bytes_sent"`
	PacketsRecv  uint64  `json:"packets_recv"`
	PacketsSent  uint64  `json:"packets_sent"`
	RecvErrors   uint64  `json:"recv_errors"`
	SendErrors   uint64  `json:"send_errors"`
	RecvDropped  uint64  `json:"recv_dropped"`
	SendDropped  uint64  `json:"send_dropped"`
	RecvRateMBPS float64 `json:"recv_rate_mbps"`
	SentRateMBPS float64 `json:"sent_rate_mbps"`
}

type ContainerDiskIOMetrics struct {
	ReadBytes    uint64  `json:"read_bytes"`
	WriteBytes   uint64  `json:"write_bytes"`
	ReadCount    uint64  `json:"read_count"`
	WriteCount   uint64  `json:"write_count"`
	ReadRateMBPS float64 `json:"read_rate_mbps"`
	WriteRateMBPS float64 `json:"write_rate_mbps"`
	IOPS         float64 `json:"iops"`
}

type ContainerHistoryRecord struct {
	ID            uint64    `json:"id" db:"id"`
	ContainerID   string    `json:"container_id" db:"container_id"`
	ContainerName string    `json:"container_name" db:"container_name"`
	Timestamp     time.Time `json:"timestamp" db:"timestamp"`
	CPUPercent    float64   `json:"cpu_percent" db:"cpu_percent"`
	MemoryUsage   uint64    `json:"memory_usage" db:"memory_usage"`
	MemoryLimit   uint64    `json:"memory_limit" db:"memory_limit"`
	NetworkRx     uint64    `json:"network_rx" db:"network_rx"`
	NetworkTx     uint64    `json:"network_tx" db:"network_tx"`
	BlockRead     uint64    `json:"block_read" db:"block_read"`
	BlockWrite    uint64    `json:"block_write" db:"block_write"`
	RawData       string    `json:"raw_data,omitempty" db:"raw_data"`
}

type KubernetesCluster struct {
	ID             string    `json:"id" db:"id"`
	Name           string    `json:"name" db:"name"`
	APIEndpoint    string    `json:"api_endpoint" db:"api_endpoint"`
	Version        string    `json:"version" db:"version"`
	Status         string    `json:"status" db:"status"`
	NodeCount      int       `json:"node_count" db:"node_count"`
	NamespaceCount int       `json:"namespace_count" db:"namespace_count"`
	PodCount       int       `json:"pod_count" db:"pod_count"`
	LastSyncAt     time.Time `json:"last_sync_at" db:"last_sync_at"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

type K8sNode struct {
	Name             string          `json:"name"`
	Ready            bool            `json:"ready"`
	Architecture     string          `json:"architecture"`
	OSImage          string          `json:"os_image"`
	KernelVersion    string          `json:"kernel_version"`
	KubeletVersion   string          `json:"kubelet_version"`
	ContainerRuntime string          `json:"container_runtime"`
	CPU              K8sResourceInfo `json:"cpu"`
	Memory           K8sResourceInfo `json:"memory"`
	PodsAllocatable  int             `json:"pods_allocatable"`
	PodsCurrent      int             `json:"pods_current"`
	Conditions       []NodeCondition `json:"conditions"`
	Labels           map[string]string `json:"labels"`
	CreatedAt        time.Time       `json:"created_at"`
}

type K8sResourceInfo struct {
	Capacity    string  `json:"capacity"`
	Allocatable string  `json:"allocatable"`
	Used        string  `json:"used"`
	UsedPercent float64 `json:"used_percent"`
}

type NodeCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type K8sPod struct {
	Name             string            `json:"name"`
	Namespace        string            `json:"namespace"`
	Status           string            `json:"status"`
	Phase            string            `json:"phase"`
	NodeName         string            `json:"node_name"`
	IP               string            `json:"ip"`
	StartTime        time.Time         `json:"start_time"`
	RestartCount     int32             `json:"restart_count"`
	ReadyContainers  int32             `json:"ready_containers"`
	TotalContainers  int32             `json:"total_containers"`
	Labels           map[string]string `json:"labels"`
	Annotations      map[string]string `json:"annotations"`
	OwnerReference   string            `json:"owner_reference"`
	Containers       []K8sContainer    `json:"containers"`
	ResourceRequests K8sPodResources   `json:"resource_requests"`
	ResourceLimits   K8sPodResources   `json:"resource_limits"`
	Conditions       []PodCondition    `json:"conditions"`
	Events           []K8sEvent        `json:"events"`
	QOSClass         string            `json:"qos_class"`
}

type K8sContainer struct {
	Name            string               `json:"name"`
	Image           string               `json:"image"`
	ImageID         string               `json:"image_id"`
	Ready           bool                 `json:"ready"`
	RestartCount    int32                `json:"restart_count"`
	State           string               `json:"state"`
	CurrentMetrics  ContainerMetrics     `json:"current_metrics"`
	ResourceRequest ResourceSpec         `json:"resource_request"`
	ResourceLimit   ResourceSpec         `json:"resource_limit"`
	VolumeMounts    []VolumeMount        `json:"volume_mounts"`
	Ports           []ContainerPort      `json:"ports"`
}

type ResourceSpec struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
	SubPath   string `json:"sub_path"`
	ReadOnly  bool   `json:"read_only"`
}

type ContainerPort struct {
	Name          string `json:"name"`
	HostPort      int32  `json:"host_port"`
	ContainerPort int32  `json:"container_port"`
	Protocol      string `json:"protocol"`
}

type K8sPodResources struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

type PodCondition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	LastTransitionTime time.Time `json:"last_transition_time"`
	Reason             string    `json:"reason"`
	Message            string    `json:"message"`
}

type K8sEvent struct {
	Type      string    `json:"type"`
	Reason    string    `json:"reason"`
	Message   string    `json:"message"`
	Count     int32     `json:"count"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

type K8sNamespace struct {
	Name          string            `json:"name"`
	Status        string            `json:"status"`
	CreatedAt     time.Time         `json:"created_at"`
	Labels        map[string]string `json:"labels"`
	ResourceQuota *ResourceQuota    `json:"resource_quota,omitempty"`
}

type ResourceQuota struct {
	Hard K8sPodResources `json:"hard"`
	Used K8sPodResources `json:"used"`
}

type K8sDeployment struct {
	Name                 string               `json:"name"`
	Namespace            string               `json:"namespace"`
	Replicas             int32                `json:"replicas"`
	ReadyReplicas        int32                `json:"ready_replicas"`
	UpdatedReplicas      int32                `json:"updated_replicas"`
	AvailableReplicas    int32                `json:"available_replicas"`
	UnavailableReplicas  int32                `json:"unavailable_replicas"`
	Strategy             string               `json:"strategy"`
	Selector             map[string]string    `json:"selector"`
	Conditions           []DeploymentCondition `json:"conditions"`
	CreatedAt            time.Time            `json:"created_at"`
	Labels               map[string]string    `json:"labels"`
}

type DeploymentCondition struct {
	Type           string    `json:"type"`
	Status         string    `json:"status"`
	Reason         string    `json:"reason"`
	Message        string    `json:"message"`
	LastUpdateTime time.Time `json:"last_update_time"`
}

type K8sService struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	Type            string            `json:"type"`
	ClusterIP       string            `json:"cluster_ip"`
	ExternalIPs     []string          `json:"external_ips"`
	Ports           []ServicePort     `json:"ports"`
	Selector        map[string]string `json:"selector"`
	SessionAffinity string            `json:"session_affinity"`
	CreatedAt       time.Time         `json:"created_at"`
	Labels          map[string]string `json:"labels"`
}

type ServicePort struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort string `json:"target_port"`
	Protocol   string `json:"protocol"`
	NodePort   int32  `json:"node_port,omitempty"`
}

type ContainerMonitoringConfig struct {
	CollectionIntervalSeconds int  `json:"collection_interval_seconds"`
	HistoryRetentionDays      int  `json:"history_retention_days"`
	EnableRealtimeStreaming   bool `json:"enable_realtime_streaming"`
	MaxConcurrentCollectors   int  `json:"max_concurrent_collectors"`
}
