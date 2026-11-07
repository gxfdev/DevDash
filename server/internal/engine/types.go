package engine

// ---------- create primitives ----------

type PortBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

type RestartPolicy struct {
	Name              string `json:"Name"`
	MaximumRetryCount int    `json:"MaximumRetryCount,omitempty"`
}

type DeviceRequest struct {
	Driver       string            `json:"Driver,omitempty"`
	Count        int               `json:"Count,omitempty"`
	DeviceIDs    []string          `json:"DeviceIDs,omitempty"`
	Capabilities [][]string        `json:"Capabilities,omitempty"`
	Options      map[string]string `json:"Options,omitempty"`
}

type Ulimit struct {
	Name string `json:"Name"`
	Soft int64  `json:"Soft"`
	Hard int64  `json:"Hard"`
}

type HealthConfig struct {
	Test        []string `json:"Test,omitempty"`
	Interval    int64    `json:"Interval,omitempty"`
	Timeout     int64    `json:"Timeout,omitempty"`
	Retries     int      `json:"Retries,omitempty"`
	StartPeriod int64    `json:"StartPeriod,omitempty"`
}

type HostConfig struct {
	Binds          []string                 `json:"Binds,omitempty"`
	PortBindings   map[string][]PortBinding `json:"PortBindings,omitempty"`
	RestartPolicy  RestartPolicy            `json:"RestartPolicy"`
	AutoRemove     bool                     `json:"AutoRemove,omitempty"`
	Privileged     bool                     `json:"Privileged,omitempty"`
	PidMode        string                   `json:"PidMode,omitempty"`
	IpcMode        string                   `json:"IpcMode,omitempty"`
	CgroupnsMode   string                   `json:"CgroupnsMode,omitempty"`
	NetworkMode    string                   `json:"NetworkMode,omitempty"`
	CapAdd         []string                 `json:"CapAdd,omitempty"`
	CapDrop        []string                 `json:"CapDrop,omitempty"`
	SecurityOpt    []string                 `json:"SecurityOpt,omitempty"`
	NanoCPUs       int64                    `json:"NanoCpus,omitempty"`
	Memory         int64                    `json:"Memory,omitempty"`
	MemorySwap     int64                    `json:"MemorySwap,omitempty"`
	ShmSize        int64                    `json:"ShmSize,omitempty"`
	DeviceRequests []DeviceRequest          `json:"DeviceRequests,omitempty"`
	Ulimits        []Ulimit                 `json:"Ulimits,omitempty"`
	Sysctls        map[string]string        `json:"Sysctls,omitempty"`
	ExtraHosts     []string                 `json:"ExtraHosts,omitempty"`
	ReadonlyRootfs bool                     `json:"ReadonlyRootfs,omitempty"`
	Runtime        string                   `json:"Runtime,omitempty"`
	GroupAdd       []string                 `json:"GroupAdd,omitempty"`
	LogConfig      *LogConfig               `json:"LogConfig,omitempty"`
	DNS            []string                 `json:"Dns,omitempty"`
}

type LogConfig struct {
	Type   string            `json:"Type"`
	Config map[string]string `json:"Config,omitempty"`
}

type Config struct {
	Hostname     string              `json:"Hostname,omitempty"`
	Domainname   string              `json:"Domainname,omitempty"`
	User         string              `json:"User,omitempty"`
	Tty          bool                `json:"Tty,omitempty"`
	OpenStdin    bool                `json:"OpenStdin,omitempty"`
	StdinOnce    bool                `json:"StdinOnce,omitempty"`
	AttachStdout bool                `json:"AttachStdout,omitempty"`
	AttachStderr bool                `json:"AttachStderr,omitempty"`
	Env          []string            `json:"Env,omitempty"`
	Cmd          []string            `json:"Cmd,omitempty"`
	Entrypoint   []string            `json:"Entrypoint,omitempty"`
	Image        string              `json:"Image"`
	WorkingDir   string              `json:"WorkingDir,omitempty"`
	Labels       map[string]string   `json:"Labels,omitempty"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`
	Volumes      map[string]struct{} `json:"Volumes,omitempty"`
	Healthcheck  *HealthConfig       `json:"Healthcheck,omitempty"`
	StopSignal   string              `json:"StopSignal,omitempty"`
	StopTimeout  int                 `json:"StopTimeout,omitempty"`
}

type CreateRequest struct {
	Config
	HostConfig HostConfig `json:"HostConfig"`
}

type CreateResponse struct {
	ID       string   `json:"Id"`
	Warnings []string `json:"Warnings"`
}

// ---------- views ----------

type PortView struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort int    `json:"private_port"`
	PublicPort  int    `json:"public_port,omitempty"`
	Type        string `json:"type"`
}

type ContainerView struct {
	ID         string            `json:"id"`
	Names      []string          `json:"names"`
	Image      string            `json:"image"`
	ImageID    string            `json:"image_id"`
	Command    string            `json:"command"`
	Created    int64             `json:"created"`
	State      string            `json:"state"`
	Status     string            `json:"status"`
	Ports      []PortView        `json:"ports"`
	Labels     map[string]string `json:"labels"`
	SizeRw     int64             `json:"size_rw"`
	Mounts     []string          `json:"mounts"`
	Network    string            `json:"network"`
	Managed    bool              `json:"managed"`
	ManagedID  string            `json:"managed_service_id,omitempty"`
	ManagedFor string            `json:"managed_for,omitempty"`
	HostAccess bool              `json:"host_access"`
}

type ContainerInspect struct {
	ID      string `json:"Id"`
	Name    string `json:"Name"`
	Created string `json:"Created"`
	State   struct {
		Status     string `json:"Status"`
		Running    bool   `json:"Running"`
		Paused     bool   `json:"Paused"`
		Restarting bool   `json:"Restarting"`
		OOMKilled  bool   `json:"OOMKilled"`
		Dead       bool   `json:"Dead"`
		Pid        int    `json:"Pid"`
		ExitCode   int    `json:"ExitCode"`
		Error      string `json:"Error"`
		StartedAt  string `json:"StartedAt"`
		FinishedAt string `json:"FinishedAt"`
		Health     *struct {
			Status        string `json:"Status"`
			FailingStreak int    `json:"FailingStreak"`
		} `json:"Health,omitempty"`
	} `json:"State"`
	Config struct {
		Image  string            `json:"Image"`
		Cmd    []string          `json:"Cmd"`
		Env    []string          `json:"Env"`
		Labels map[string]string `json:"Labels"`
		Tty    bool              `json:"Tty"`
	} `json:"Config"`
	HostConfig struct {
		Privileged bool     `json:"Privileged"`
		PidMode    string   `json:"PidMode"`
		Binds      []string `json:"Binds"`
		NanoCPUs   int64    `json:"NanoCpus"`
		Memory     int64    `json:"Memory"`
		ShmSize    int64    `json:"ShmSize"`
		CapAdd     []string `json:"CapAdd"`
	} `json:"HostConfig"`
	NetworkSettings struct {
		IPAddress string                                `json:"IPAddress"`
		Ports     map[string][]PortBinding              `json:"Ports"`
		Networks  map[string]struct{ IPAddress string } `json:"Networks"`
	} `json:"NetworkSettings"`
	Mounts []struct {
		Type        string `json:"Type"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
		Mode        string `json:"Mode"`
		RW          bool   `json:"RW"`
	} `json:"Mounts"`
	RestartCount int `json:"RestartCount"`
}

func (c *ContainerInspect) DisplayName() string {
	if len(c.Name) > 0 && c.Name[0] == '/' {
		return c.Name[1:]
	}
	return c.Name
}

type ContainerStats struct {
	Read     string `json:"read"`
	CPUStats struct {
		CPUUsage struct {
			TotalUsage  uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs  int    `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage  uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs  int    `json:"online_cpus"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
	PidsStats struct {
		Current uint64 `json:"current"`
	} `json:"pids_stats"`
	BlkioStats struct {
		IoServiceBytesRecursive []struct {
			Major uint64 `json:"major"`
			Minor uint64 `json:"minor"`
			Op    string `json:"op"`
			Value uint64 `json:"value"`
		} `json:"io_service_bytes_recursive"`
	} `json:"blkio_stats"`
}

type ContainerStatView struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryMB      float64 `json:"memory_mb"`
	MemoryLimitMB float64 `json:"memory_limit_mb"`
	MemoryPercent float64 `json:"memory_percent"`
	NetRxMB       float64 `json:"net_rx_mb"`
	NetTxMB       float64 `json:"net_tx_mb"`
	BlkReadMB     float64 `json:"blk_read_mb"`
	BlkWriteMB    float64 `json:"blk_write_mb"`
	Pids          uint64  `json:"pids"`
	DiskMB        float64 `json:"disk_mb"`
}

type ImageView struct {
	ID       string   `json:"id"`
	Tags     []string `json:"tags"`
	SizeMB   float64  `json:"size_mb"`
	Created  int64    `json:"created"`
	InUse    int      `json:"in_use"`
	IsAIBase bool     `json:"is_ai_base"`
}

type NetworkView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Driver  string `json:"driver"`
	Scope   string `json:"scope"`
	Subnet  string `json:"subnet,omitempty"`
	Gateway string `json:"gateway,omitempty"`
}

type VolumeView struct {
	Name       string  `json:"name"`
	Driver     string  `json:"driver"`
	Mountpoint string  `json:"mountpoint"`
	SizeMB     float64 `json:"size_mb"`
	InUse      int     `json:"in_use"`
}

type Event struct {
	Status string `json:"status"`
	ID     string `json:"id"`
	From   string `json:"from"`
	Type   string `json:"Type"`
	Action string `json:"Action"`
	Actor  struct {
		ID         string            `json:"ID"`
		Attributes map[string]string `json:"Attributes"`
	} `json:"Actor"`
	Time     int64 `json:"time"`
	TimeNano int64 `json:"timeNano"`
}

type PullProgress struct {
	Status         string `json:"status"`
	ID             string `json:"id"`
	Progress       string `json:"progress"`
	ProgressDetail struct {
		Current int64 `json:"current"`
		Total   int64 `json:"total"`
	} `json:"progressDetail"`
	Error string `json:"error"`
}
