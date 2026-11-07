package engine_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gxfdev/aidash/server/internal/engine"
	"github.com/gxfdev/aidash/server/internal/mockdocker"
)

func TestClientLifecycle(t *testing.T) {
	mock := mockdocker.New()
	defer mock.Close()
	client, err := engine.New(mock.URL())
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	version, err := client.Version(ctx)
	if err != nil || version.Version == "" {
		t.Fatalf("version: %v", err)
	}
	info, err := client.Info(ctx)
	if err != nil || info.NCPU != 8 {
		t.Fatalf("info: %v %+v", err, info)
	}

	// Pull
	var statuses []string
	if err := client.PullImage(ctx, "ollama/ollama", "latest", func(p engine.PullProgress) {
		statuses = append(statuses, p.Status)
	}); err != nil {
		t.Fatalf("pull: %v", err)
	}
	if len(statuses) == 0 {
		t.Fatal("no pull progress")
	}

	// Create + start
	req := engine.CreateRequest{
		Config: engine.Config{
			Image:        "ollama/ollama:latest",
			Env:          []string{"A=1"},
			Labels:       map[string]string{"aios.managed": "true"},
			ExposedPorts: map[string]struct{}{"11434/tcp": {}},
		},
	}
	req.HostConfig.PortBindings = map[string][]engine.PortBinding{"11434/tcp": {{HostIP: "127.0.0.1", HostPort: "21434"}}}
	created, err := client.CreateContainer(ctx, "aidash-test", req)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := client.StartContainer(ctx, created.ID); err != nil {
		t.Fatalf("start: %v", err)
	}
	inspect, err := client.InspectContainer(ctx, created.ID)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if !inspect.State.Running {
		t.Fatal("container should be running")
	}
	if inspect.Config.Image != "ollama/ollama:latest" {
		t.Fatalf("unexpected image %s", inspect.Config.Image)
	}

	// Logs (framed stream -> demuxed)
	mock.SetContainerLogs(created.ID, "hello world", "second line")
	logs, err := client.ContainerLogsText(ctx, created.ID, 50)
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	if !strings.Contains(logs, "hello world") || !strings.Contains(logs, "second line") {
		t.Fatalf("unexpected logs: %q", logs)
	}

	// Stats
	stats, err := client.ContainerStats(ctx, created.ID)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.MemoryMB <= 0 || stats.CPUPercent <= 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	// Exec
	execID, err := client.ExecCreate(ctx, created.ID, engine.ExecCreateRequest{
		AttachStdout: true, AttachStderr: true, Tty: true, Cmd: []string{"sh", "-lc", "uname -a"},
	})
	if err != nil {
		t.Fatalf("exec create: %v", err)
	}
	stream, err := client.ExecStart(ctx, execID)
	if err != nil {
		t.Fatalf("exec start: %v", err)
	}
	buf := make([]byte, 256)
	n, _ := stream.Read(buf)
	stream.Close()
	if !strings.Contains(string(buf[:n]), "mock") {
		t.Fatalf("unexpected exec output: %q", string(buf[:n]))
	}
	execInfo, err := client.ExecInspect(ctx, execID)
	if err != nil || execInfo.ExitCode != 0 {
		t.Fatalf("exec inspect: %v %+v", err, execInfo)
	}

	// Lists
	containers, err := client.ListContainers(ctx, true)
	if err != nil || len(containers) != 1 {
		t.Fatalf("list containers: %v %d", err, len(containers))
	}
	if !containers[0].Managed || containers[0].ManagedFor != "" {
		t.Fatalf("managed flags wrong: %+v", containers[0])
	}
	if _, err := client.ListImages(ctx); err != nil {
		t.Fatalf("images: %v", err)
	}
	if _, err := client.ListNetworks(ctx); err != nil {
		t.Fatalf("networks: %v", err)
	}
	if _, err := client.DiskUsage(ctx); err != nil {
		t.Fatalf("df: %v", err)
	}

	// Stop / restart / remove
	if err := client.StopContainer(ctx, created.ID, 5); err != nil {
		t.Fatalf("stop: %v", err)
	}
	inspect, _ = client.InspectContainer(ctx, created.ID)
	if inspect.State.Running {
		t.Fatal("should be stopped")
	}
	if err := client.RestartContainer(ctx, created.ID, 5); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if err := client.RemoveContainer(ctx, created.ID, true, false); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := client.InspectContainer(ctx, created.ID); err == nil {
		t.Fatal("expected inspect failure after remove")
	}
}

func TestConnectFailureIsDescriptive(t *testing.T) {
	client, err := engine.New("http://127.0.0.1:1")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = client.Ping(ctx)
	if err == nil {
		t.Fatal("expected connection error")
	}
	if !strings.Contains(err.Error(), "Docker") {
		t.Fatalf("error should mention Docker: %v", err)
	}
}

func TestDemuxFramed(t *testing.T) {
	framed := []byte{
		1, 0, 0, 0, 0, 0, 0, 6, 'h', 'e', 'l', 'l', 'o', '\n',
		2, 0, 0, 0, 0, 0, 0, 3, 'e', 'r', 'r',
	}
	if got := engine.Demux(framed); got != "hello\nerr" {
		t.Fatalf("demux = %q", got)
	}
	if got := engine.Demux([]byte("plain text")); got != "plain text" {
		t.Fatalf("raw = %q", got)
	}
}
