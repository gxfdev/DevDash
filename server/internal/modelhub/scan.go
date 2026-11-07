package modelhub

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gxfdev/aidash/server/internal/engine"
	"github.com/gxfdev/aidash/server/internal/model"
)

// parseScanOutput extracts (size, fileCount, ggufPath) from the output of
//
//	du -sb DIR; find DIR -type f | wc -l; find DIR -name '*.gguf' | head -1
func parseScanOutput(out string) (int64, int, string) {
	var size int64
	var files int
	gguf := ""
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			if n, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
				if size == 0 {
					size = n
				} else if files == 0 && n >= 0 {
					files = int(n)
				}
				continue
			}
		}
		if strings.HasSuffix(line, ".gguf") {
			gguf = line
		}
	}
	return size, files, gguf
}

// scanVolume inspects a downloaded model directory by mounting the shared
// volume into a short-lived helper container (works even after the
// downloader container has stopped).
func (h *Hub) scanVolume(ctx context.Context, asset *model.ModelAsset) (int64, int, string) {
	image := downloaderImage(asset.Source)
	dir := asset.LocalPath
	cmd := "du -sb " + dir + " 2>/dev/null; find " + dir + " -type f 2>/dev/null | wc -l; find " + dir + " -name '*.gguf' 2>/dev/null | head -1"
	create := engine.CreateRequest{
		Config: engine.Config{
			Image:  image,
			Cmd:    []string{"sh", "-lc", cmd},
			Labels: map[string]string{"aios.asset_id": asset.ID, "aios.role": "model-scan"},
		},
		HostConfig: engine.HostConfig{
			Binds:         []string{VolumeName + ":" + VolumeTarget},
			RestartPolicy: engine.RestartPolicy{Name: "no"},
		},
	}
	create.HostConfig.NanoCPUs = int64(1e9)
	create.HostConfig.Memory = 512 << 20
	created, err := h.docker.CreateContainer(ctx, "aidash-scan-"+asset.ID, create)
	if err != nil {
		return 0, 0, ""
	}
	defer h.cleanupContainer(context.Background(), created.ID)
	if err := h.docker.StartContainer(ctx, created.ID); err != nil {
		return 0, 0, ""
	}
	for i := 0; i < 60; i++ {
		time.Sleep(300 * time.Millisecond)
		ci, err := h.docker.InspectContainer(ctx, created.ID)
		if err == nil && !ci.State.Running {
			break
		}
	}
	logs, _ := h.docker.ContainerLogsText(context.Background(), created.ID, 100)
	return parseScanOutput(logs)
}
