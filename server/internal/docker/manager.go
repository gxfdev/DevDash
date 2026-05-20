package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

type ContainerInfo struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Image         string                 `json:"image"`
	ImageID       string                 `json:"image_id"`
	State         string                 `json:"state"`
	Status        string                 `json:"status"`
	Created       time.Time              `json:"created"`
	Ports         []string               `json:"ports"`
	Networks      []string               `json:"networks"`
	Command       string                 `json:"command"`
	RestartPolicy string                 `json:"restart_policy"`
	Labels        map[string]string      `json:"labels"`
	Stats         *ContainerStats        `json:"stats,omitempty"`
}

type ContainerStats struct {
	CPUPercentage    float64 `json:"cpu_percentage"`
	MemoryUsage     uint64  `json:"memory_usage"`
	MemoryLimit     uint64  `json:"memory_limit"`
	MemoryPercentage float64 `json:"memory_percentage"`
	NetworkRx       uint64  `json:"network_rx"`
	NetworkTx       uint64  `json:"network_tx"`
	BlockRead       uint64  `json:"block_read"`
	BlockWrite      uint64  `json:"block_write"`
	PIDs            uint64  `json:"pids"`
}

type ImageInfo struct {
	ID          string    `json:"id"`
	RepoTags    []string  `json:"repo_tags"`
	Size        int64     `json:"size"`
	Created     time.Time `json:"created"`
	Labels      map[string]string `json:"labels"`
}

type NetworkInfo struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Scope      string            `json:"scope"`
	IPv4       string            `json:"ipv4"`
	IPv6       string            `json:"ipv6"`
	Containers []string          `json:"containers"`
	Labels     map[string]string `json:"labels"`
}

type VolumeInfo struct {
	Name       string   `json:"name"`
	Driver     string   `json:"driver"`
	Mountpoint string   `json:"mountpoint"`
	Created    time.Time `json:"created"`
	Labels     map[string]string `json:"labels"`
}

type DockerManager struct {
	cli *client.Client
}

func NewDockerManager() (*DockerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %v", err)
	}
	return &DockerManager{cli: cli}, nil
}

func withTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

func (dm *DockerManager) Ping() error {
	ctx, cancel := withTimeout()
	defer cancel()
	_, err := dm.cli.Ping(ctx)
	return err
}

func (dm *DockerManager) ListContainers(all bool) ([]ContainerInfo, error) {
	ctx, cancel := withTimeout()
	defer cancel()
	options := container.ListOptions{
		All: all,
	}
	
	containers, err := dm.cli.ContainerList(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %v", err)
	}
	
	result := make([]ContainerInfo, 0, len(containers))
	for _, c := range containers {
		info := ContainerInfo{
			ID:            c.ID[:12],
			Name:         strings.TrimPrefix(c.Names[0], "/"),
			Image:        c.Image,
			ImageID:      c.ImageID[:12],
			State:        string(c.State),
			Status:       c.Status,
			Created:       time.Unix(c.Created, 0),
			Command:       c.Command,
			Labels:       c.Labels,
		}
		
		for _, port := range c.Ports {
			if port.PublicPort != 0 {
				portStr := fmt.Sprintf("%d->%d/%s", port.PublicPort, port.PrivatePort, port.Type)
				info.Ports = append(info.Ports, portStr)
			}
		}
		
		if c.NetworkSettings != nil {
			for net := range c.NetworkSettings.Networks {
				info.Networks = append(info.Networks, net)
			}
		}
		
		result = append(result, info)
	}
	
	return result, nil
}

func (dm *DockerManager) GetContainer(id string) (*ContainerInfo, error) {
	ctx, cancel := withTimeout()
	defer cancel()
	containerJSON, err := dm.cli.ContainerInspect(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %s: %v", id, err)
	}
	
	info := &ContainerInfo{
		ID:            containerJSON.ID[:12],
		Name:         strings.TrimPrefix(containerJSON.Name, "/"),
		Image:        containerJSON.Config.Image,
		ImageID:      containerJSON.Image[:12],
		State:        string(containerJSON.State.Status),
		Status:       string(containerJSON.State.Status),
		Command:       strings.Join(containerJSON.Config.Cmd, " "),
		RestartPolicy: string(containerJSON.HostConfig.RestartPolicy.Name),
		Labels:       containerJSON.Config.Labels,
	}
	
	if t, err := time.Parse(time.RFC3339, containerJSON.Created); err == nil {
		info.Created = t
	} else if t, err := time.Parse(time.RFC3339Nano, containerJSON.Created); err == nil {
		info.Created = t
	}
	
	for _, port := range containerJSON.NetworkSettings.Ports {
		for _, p := range port {
			if p.HostPort != "" {
				portStr := fmt.Sprintf("%s->%s", p.HostPort, port)
				info.Ports = append(info.Ports, portStr)
			}
		}
	}
	
	for network := range containerJSON.NetworkSettings.Networks {
		info.Networks = append(info.Networks, network)
	}
	
	return info, nil
}

func (dm *DockerManager) StartContainer(id string) error {
	ctx, cancel := withTimeout()
	defer cancel()
	err := dm.cli.ContainerStart(ctx, id, container.StartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container %s: %v", id, err)
	}
	return nil
}

func (dm *DockerManager) StopContainer(id string, timeout *int) error {
	ctx, cancel := withTimeout()
	defer cancel()
	options := container.StopOptions{Timeout: timeout}
	err := dm.cli.ContainerStop(ctx, id, options)
	if err != nil {
		return fmt.Errorf("failed to stop container %s: %v", id, err)
	}
	return nil
}

func (dm *DockerManager) RestartContainer(id string, timeout *int) error {
	ctx, cancel := withTimeout()
	defer cancel()
	options := container.StopOptions{Timeout: timeout}
	err := dm.cli.ContainerRestart(ctx, id, options)
	if err != nil {
		return fmt.Errorf("failed to restart container %s: %v", id, err)
	}
	return nil
}

func (dm *DockerManager) RemoveContainer(id string, force, removeVolumes bool) error {
	ctx, cancel := withTimeout()
	defer cancel()
	options := container.RemoveOptions{Force: force, RemoveVolumes: removeVolumes}
	err := dm.cli.ContainerRemove(ctx, id, options)
	if err != nil {
		return fmt.Errorf("failed to remove container %s: %v", id, err)
	}
	return nil
}

func (dm *DockerManager) GetContainerLogs(id string, tail string, follow bool) (io.ReadCloser, error) {
	ctx, cancel := withTimeout()
	defer cancel()
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
		Follow:     follow,
		Timestamps: true,
	}
	
	reader, err := dm.cli.ContainerLogs(ctx, id, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs for container %s: %v", id, err)
	}
	
	return reader, nil
}

func (dm *DockerManager) GetContainerStats(id string) (*ContainerStats, error) {
	ctx, cancel := withTimeout()
	defer cancel()
	stats, err := dm.cli.ContainerStats(ctx, id, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats for container %s: %v", id, err)
	}
	defer stats.Body.Close()
	
	var stat container.StatsResponse
	if err := json.NewDecoder(stats.Body).Decode(&stat); err != nil {
		return nil, fmt.Errorf("failed to decode stats: %v", err)
	}
	
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
	
	memPercent := 0.0
	if stat.MemoryStats.Limit != 0 {
		memPercent = float64(stat.MemoryStats.Usage) / float64(stat.MemoryStats.Limit) * 100.0
	}
	
	result := &ContainerStats{
		CPUPercentage:    cpuPercent,
		MemoryUsage:     stat.MemoryStats.Usage,
		MemoryLimit:     stat.MemoryStats.Limit,
		MemoryPercentage: memPercent,
		PIDs:            stat.PidsStats.Current,
	}
	
	if stat.Networks != nil {
		for _, v := range stat.Networks {
			result.NetworkRx += v.RxBytes
			result.NetworkTx += v.TxBytes
		}
	}
	
	if len(stat.BlkioStats.IoServiceBytesRecursive) > 0 {
		for _, bio := range stat.BlkioStats.IoServiceBytesRecursive {
			switch bio.Op {
			case "read", "Read":
				result.BlockRead += bio.Value
			case "write", "Write":
				result.BlockWrite += bio.Value
			}
		}
	}
	
	return result, nil
}

func (dm *DockerManager) ListImages() ([]ImageInfo, error) {
	ctx, cancel := withTimeout()
	defer cancel()
	images, err := dm.cli.ImageList(ctx, image.ListOptions{All: false})
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %v", err)
	}
	
	result := make([]ImageInfo, 0, len(images))
	for _, img := range images {
		info := ImageInfo{
			ID:      img.ID[7:19],
			Size:    img.Size,
			Created: time.Unix(img.Created, 0),
			Labels:  img.Labels,
		}
		if len(img.RepoTags) > 0 {
			info.RepoTags = img.RepoTags
		} else {
			info.RepoTags = []string{"<none>:<none>"}
		}
		result = append(result, info)
	}
	
	return result, nil
}

func (dm *DockerManager) PullImage(imageRef string) (io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	reader, err := dm.cli.ImagePull(ctx, imageRef, image.PullOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to pull image %s: %v", imageRef, err)
	}
	return reader, nil
}

func (dm *DockerManager) RemoveImage(id string, force bool) ([]image.DeleteResponse, error) {
	ctx, cancel := withTimeout()
	defer cancel()
	options := image.RemoveOptions{
		Force: force,
	}
	items, err := dm.cli.ImageRemove(ctx, id, options)
	if err != nil {
		return nil, fmt.Errorf("failed to remove image %s: %v", id, err)
	}
	return items, nil
}

func (dm *DockerManager) ListNetworks() ([]NetworkInfo, error) {
	ctx, cancel := withTimeout()
	defer cancel()
	networks, err := dm.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %v", err)
	}
	
	result := make([]NetworkInfo, 0, len(networks))
	for _, net := range networks {
		info := NetworkInfo{
			ID:     net.ID[:12],
			Name:   net.Name,
			Driver: net.Driver,
			Scope:  net.Scope,
			Labels: net.Labels,
		}
		
		if len(net.IPAM.Config) > 0 {
			info.IPv4 = net.IPAM.Config[0].Subnet
			if len(net.IPAM.Config) > 1 {
				info.IPv6 = net.IPAM.Config[1].Subnet
			}
		}
		
		if net.Containers != nil {
			for c := range net.Containers {
				info.Containers = append(info.Containers, c[:12])
			}
		}
		
		result = append(result, info)
	}
	
	return result, nil
}

func (dm *DockerManager) ListVolumes() ([]VolumeInfo, error) {
	ctx, cancel := withTimeout()
	defer cancel()
	volumes, err := dm.cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %v", err)
	}
	
	result := make([]VolumeInfo, 0, len(volumes.Volumes))
	for _, vol := range volumes.Volumes {
		info := VolumeInfo{
			Name:       vol.Name,
			Driver:     vol.Driver,
			Mountpoint: vol.Mountpoint,
			Labels:     vol.Labels,
		}
		if t, err := time.Parse(time.RFC3339, vol.CreatedAt); err == nil {
			info.Created = t
		} else if t, err := time.Parse(time.RFC3339Nano, vol.CreatedAt); err == nil {
			info.Created = t
		}
		result = append(result, info)
	}
	
	return result, nil
}

func (dm *DockerManager) SystemInfo() (system.Info, error) {
	ctx, cancel := withTimeout()
	defer cancel()
	info, err := dm.cli.Info(ctx)
	if err != nil {
		return system.Info{}, fmt.Errorf("failed to get docker info: %v", err)
	}
	return info, nil
}

func (dm *DockerManager) DiskUsage() (types.DiskUsage, error) {
	ctx, cancel := withTimeout()
	defer cancel()
	usage, err := dm.cli.DiskUsage(ctx, types.DiskUsageOptions{Types: []types.DiskUsageObject{"container", "image", "volume"}})
	if err != nil {
		return types.DiskUsage{}, fmt.Errorf("failed to get disk usage: %v", err)
	}
	return usage, nil
}
