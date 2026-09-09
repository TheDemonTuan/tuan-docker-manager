package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"docker-panel/internal/compose"
	"docker-panel/internal/docker"
	"docker-panel/internal/gpu"
	"docker-panel/internal/metrics"
	"docker-panel/internal/models"
)

type AgentHandler struct {
	dockerClient *docker.Client
	hostColl     *metrics.HostCollector
	gpuColl      *gpu.Collector
	composeRun   *compose.Runner
	version      string
	storageMu    sync.Mutex
	storageCache *models.StorageSnapshot
	storageAt    time.Time
}

func NewAgentHandler(
	dockerClient *docker.Client,
	hostColl *metrics.HostCollector,
	gpuColl *gpu.Collector,
	composeRun *compose.Runner,
	version string,
) *AgentHandler {
	return &AgentHandler{
		dockerClient: dockerClient,
		hostColl:     hostColl,
		gpuColl:      gpuColl,
		composeRun:   composeRun,
		version:      version,
	}
}

func (h *AgentHandler) Router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /ping", h.handlePing)

	// Containers
	mux.HandleFunc("POST /actions/containers/list", h.handleListContainers)
	mux.HandleFunc("POST /actions/containers/inspect", h.handleInspectContainer)
	mux.HandleFunc("POST /actions/storage/snapshot", h.handleStorageSnapshot)
	mux.HandleFunc("POST /actions/containers/action", h.handleContainerAction)
	mux.HandleFunc("GET /actions/containers/logs", h.handleContainerLogs)
	mux.HandleFunc("POST /actions/containers/stats", h.handleContainerStats)
	mux.HandleFunc("POST /actions/containers/stats-batch", h.handleBatchContainerStats)

	// Compose
	mux.HandleFunc("POST /actions/compose/action", h.handleComposeAction)
	mux.HandleFunc("POST /actions/compose/discover", h.handleDiscoverStacks)
	mux.HandleFunc("POST /actions/compose/read-file", h.handleReadFile)

	// Images
	mux.HandleFunc("POST /actions/images/list", h.handleListImages)
	mux.HandleFunc("POST /actions/images/pull", h.handlePullImage)
	mux.HandleFunc("POST /actions/images/delete", h.handleDeleteImage)
	mux.HandleFunc("POST /actions/images/prune", h.handlePruneImages)

	// Volumes
	mux.HandleFunc("POST /actions/volumes/list", h.handleListVolumes)
	mux.HandleFunc("POST /actions/volumes/delete", h.handleDeleteVolume)
	mux.HandleFunc("POST /actions/volumes/prune", h.handlePruneVolumes)

	// Networks
	mux.HandleFunc("POST /actions/networks/list", h.handleListNetworks)
	mux.HandleFunc("POST /actions/networks/delete", h.handleDeleteNetwork)

	// Metrics
	mux.HandleFunc("GET /metrics/host", h.handleHostMetrics)
	mux.HandleFunc("GET /metrics/gpu", h.handleGPUMetrics)

	// Events
	mux.HandleFunc("GET /events", h.handleEvents)

	return mux
}

func (h *AgentHandler) handlePing(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	dockerVer := "unknown"
	v, err := h.dockerClient.GetVersion(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("docker engine unavailable: %w", err))
		return
	}
	if verStr, ok := v["Version"].(string); ok {
		dockerVer = verStr
	}

	writeJSON(w, http.StatusOK, PingResponse{
		Status:        "ok",
		Version:       h.version,
		DockerVersion: dockerVer,
		OS:            runtime.GOOS,
		Hostname:      hostname,
	})
}

func (h *AgentHandler) handleListContainers(w http.ResponseWriter, r *http.Request) {
	var req ListContainersRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	containers, err := h.dockerClient.ListContainers(r.Context(), req.All)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if req.StackName != "" {
		filtered := make([]models.ContainerInfo, 0)
		for _, c := range containers {
			if strings.EqualFold(c.StackName, req.StackName) {
				filtered = append(filtered, c)
			}
		}
		containers = filtered
	}

	writeJSON(w, http.StatusOK, ListContainersResponse{Containers: containers})
}

func (h *AgentHandler) handleStorageSnapshot(w http.ResponseWriter, r *http.Request) {
	h.storageMu.Lock()
	defer h.storageMu.Unlock()
	if h.storageCache != nil && time.Since(h.storageAt) < time.Minute {
		writeJSON(w, http.StatusOK, StorageSnapshotResponse{Storage: h.storageCache})
		return
	}

	ctx := r.Context()
	containers, err := h.dockerClient.ListContainers(ctx, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	volumes, err := h.dockerClient.ListVolumes(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	images, _ := h.dockerClient.ListImages(ctx, true)
	volumeUsage, usageErr := h.dockerClient.VolumeUsage(ctx)

	snapshot := &models.StorageSnapshot{
		MeasuredAt: time.Now().UTC(),
		Containers: make(map[string]models.ContainerStorage, len(containers)),
		Volumes:    make(map[string]models.VolumeStorage, len(volumes)),
		Stacks:     make(map[string]models.StackStorage),
		Status:     "complete",
		Scope:      "Docker storage snapshot: stack and container disk space includes container images, writable layers, and persistent volumes.",
	}
	if usageErr != nil {
		snapshot.Status = "partial"
		snapshot.Scope = "Docker storage snapshot (volume usage partially measured: " + usageErr.Error() + ")"
	}

	// Index image sizes by ID, sha-prefix-trimmed ID, and repo tags
	imageSizes := make(map[string]int64, len(images)*2)
	for _, img := range images {
		if img.Size > 0 {
			imageSizes[img.ID] = img.Size
			imageSizes[strings.TrimPrefix(img.ID, "sha256:")] = img.Size
			for _, tag := range img.RepoTags {
				imageSizes[tag] = img.Size
			}
		}
	}

	hostRoot := os.Getenv("HOST_ROOT")
	if hostRoot == "" {
		hostRoot = "/host/root"
	}

	for _, volume := range volumes {
		var size *int64
		if s, ok := volumeUsage[volume.Name]; ok && s >= 0 {
			size = &s
		} else if volume.Mountpoint != "" {
			p := volume.Mountpoint
			if runtime.GOOS != "windows" {
				p = filepath.Join(hostRoot, volume.Mountpoint)
			}
			if info, statErr := os.Stat(p); statErr == nil && info.IsDir() {
				s := measureDirSize(p)
				size = &s
			}
		}
		if size == nil && usageErr != nil {
			snapshot.Status = "partial"
		}
		snapshot.Volumes[volume.Name] = models.VolumeStorage{Bytes: size}
	}

	stackImages := make(map[string]map[string]int64)

	for _, container := range containers {
		var imgSize int64
		if s, ok := imageSizes[container.ImageID]; ok {
			imgSize = s
		} else if s, ok := imageSizes[container.Image]; ok {
			imgSize = s
		} else if s, ok := imageSizes[strings.TrimPrefix(container.ImageID, "sha256:")]; ok {
			imgSize = s
		}

		var rw int64
		if container.SizeRw != nil && *container.SizeRw >= 0 {
			rw = *container.SizeRw
		}

		var rootfs int64
		if container.SizeRootFS != nil && *container.SizeRootFS > 0 {
			rootfs = *container.SizeRootFS
		} else {
			rootfs = imgSize + rw
		}

		var volBytes int64
		var volNames []string
		for _, mount := range mustInspectMounts(ctx, h.dockerClient, container.ID) {
			if mount.Type != "volume" || mount.Name == "" {
				continue
			}
			volNames = append(volNames, mount.Name)
			vStorage := snapshot.Volumes[mount.Name]
			vStorage.RefCount++
			if vStorage.Bytes != nil {
				volBytes += *vStorage.Bytes
			}
			if container.StackName != "" && !containsString(vStorage.StackNames, container.StackName) {
				vStorage.StackNames = append(vStorage.StackNames, container.StackName)
			}
			snapshot.Volumes[mount.Name] = vStorage
		}

		storage := models.ContainerStorage{
			TotalBytes:    rootfs + volBytes,
			ImageBytes:    imgSize,
			WritableBytes: rw,
			RootFSBytes:   rootfs,
			VolumeBytes:   volBytes,
			VolumeNames:   volNames,
		}
		snapshot.Containers[container.ID] = storage

		if container.StackName != "" {
			st := snapshot.Stacks[container.StackName]
			st.WritableBytes += rw
			if container.SizeRw == nil {
				st.Incomplete = true
			}
			snapshot.Stacks[container.StackName] = st

			if _, ok := stackImages[container.StackName]; !ok {
				stackImages[container.StackName] = make(map[string]int64)
			}
			imgKey := container.ImageID
			if imgKey == "" {
				imgKey = container.Image
			}
			if imgKey != "" {
				stackImages[container.StackName][imgKey] = imgSize
			}
		}
	}

	// Deduplicate and sum image sizes for each stack
	for stackName, imgs := range stackImages {
		st := snapshot.Stacks[stackName]
		var totalImg int64
		for _, sz := range imgs {
			totalImg += sz
		}
		st.ImageBytes = totalImg
		snapshot.Stacks[stackName] = st
	}

	// Add volume usage to stacks
	for name, volume := range snapshot.Volumes {
		for _, stackName := range volume.StackNames {
			st := snapshot.Stacks[stackName]
			if !containsString(st.VolumeNames, name) {
				st.VolumeNames = append(st.VolumeNames, name)
			}
			if volume.Bytes == nil {
				st.Incomplete = true
			} else if volume.RefCount == 1 {
				st.ExclusiveVolumeBytes += *volume.Bytes
			} else {
				st.SharedVolumeBytes += *volume.Bytes
			}
			snapshot.Stacks[stackName] = st
		}
	}

	// Calculate total stack bytes
	for stackName, st := range snapshot.Stacks {
		st.TotalBytes = st.ImageBytes + st.WritableBytes + st.ExclusiveVolumeBytes
		snapshot.Stacks[stackName] = st
	}

	h.storageCache = snapshot
	h.storageAt = time.Now()
	writeJSON(w, http.StatusOK, StorageSnapshotResponse{Storage: snapshot})
}

func measureDirSize(root string) int64 {
	var total int64
	count := 0
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		count++
		if count > 50000 {
			return filepath.SkipAll
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

func normalizeStorageSize(value int64) *int64 {
	if value < 0 {
		return nil
	}
	return &value
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func mustInspectMounts(ctx context.Context, client *docker.Client, id string) []models.MountDetail {
	detail, err := client.InspectContainer(ctx, id)
	if err != nil || detail == nil {
		return nil
	}
	return detail.Mounts
}

func (h *AgentHandler) handleInspectContainer(w http.ResponseWriter, r *http.Request) {
	var req InspectContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("container id is required"))
		return
	}

	detail, err := h.dockerClient.InspectContainer(r.Context(), req.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}

	writeJSON(w, http.StatusOK, InspectContainerResponse{Container: detail})
}

func (h *AgentHandler) handleContainerAction(w http.ResponseWriter, r *http.Request) {
	var req ContainerActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid container action request"))
		return
	}

	ctx := r.Context()
	var err error
	switch strings.ToLower(req.Action) {
	case "start":
		err = h.dockerClient.StartContainer(ctx, req.ID)
	case "stop":
		err = h.dockerClient.StopContainer(ctx, req.ID)
	case "restart":
		err = h.dockerClient.RestartContainer(ctx, req.ID)
	case "kill":
		err = h.dockerClient.KillContainer(ctx, req.ID, req.Signal)
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("unsupported container action: %s", req.Action))
		return
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	h.invalidateStorageCache()
	writeJSON(w, http.StatusOK, ActionResponse{Success: true, Message: "action completed successfully"})
}

func (h *AgentHandler) invalidateStorageCache() {
	h.storageMu.Lock()
	defer h.storageMu.Unlock()
	h.storageCache = nil
	h.storageAt = time.Time{}
}

func (h *AgentHandler) handleContainerLogs(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("id is required"))
		return
	}

	follow := r.URL.Query().Get("follow") == "1"
	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "100"
	}
	timestamps := r.URL.Query().Get("timestamps") == "1"

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	writer := &flushWriter{w: w, f: flusher, ok: ok}

	_ = h.dockerClient.StreamLogs(r.Context(), id, true, true, follow, timestamps, tail, writer)
}

type flushWriter struct {
	w  io.Writer
	f  http.Flusher
	ok bool
}

func (fw *flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if fw.ok {
		fw.f.Flush()
	}
	return n, err
}

func (h *AgentHandler) handleContainerStats(w http.ResponseWriter, r *http.Request) {
	var req ContainerStatsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("id is required"))
		return
	}

	stats, err := h.dockerClient.GetContainerStats(r.Context(), req.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ContainerStatsResponse{Stats: stats})
}

func (h *AgentHandler) handleBatchContainerStats(w http.ResponseWriter, r *http.Request) {
	var req BatchContainerStatsRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	targetIDs := req.ContainerIDs
	if len(targetIDs) == 0 {
		// If no IDs provided, fetch running containers
		ctrs, err := h.dockerClient.ListContainers(r.Context(), false)
		if err == nil {
			for _, c := range ctrs {
				targetIDs = append(targetIDs, c.ID)
			}
		}
	}

	statsMap := make(map[string]*models.ContainerStats)
	if len(targetIDs) == 0 {
		writeJSON(w, http.StatusOK, BatchContainerStatsResponse{Stats: statsMap})
		return
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	// Concurrently query container stats with bounded concurrency (up to 16 workers)
	sem := make(chan struct{}, 16)

	for _, cid := range targetIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			st, err := h.dockerClient.GetContainerStats(r.Context(), id)
			if err == nil && st != nil {
				mu.Lock()
				statsMap[id] = st
				// Also key by short 12-char ID for convenience
				if len(id) >= 12 {
					statsMap[id[:12]] = st
				}
				mu.Unlock()
			}
		}(cid)
	}

	wg.Wait()
	writeJSON(w, http.StatusOK, BatchContainerStatsResponse{Stats: statsMap})
}

func (h *AgentHandler) handleComposeAction(w http.ResponseWriter, r *http.Request) {
	var req ComposeActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.StackName == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("stack_name is required"))
		return
	}

	if err := h.composeRun.ValidateStackName(req.StackName); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	action := strings.ToLower(req.Action)

	// Delete action: compose down (with volumes if requested) + remove stack directory
	if action == "delete" {
		logs, _ := h.composeRun.Execute(r.Context(), req.StackName, "down", req.RemoveVolumes)
		if err := h.composeRun.DeleteStackDir(req.StackName); err != nil {
			writeJSON(w, http.StatusOK, ComposeActionResponse{
				Success: false,
				Logs:    logs,
				Error:   err.Error(),
			})
			return
		}
		h.invalidateStorageCache()
		writeJSON(w, http.StatusOK, ComposeActionResponse{
			Success: true,
			Logs:    fmt.Sprintf("Stack %s deleted successfully\n%s", req.StackName, logs),
		})
		return
	}

	// Validate action
	if action == "validate" {
		if req.ComposeContent == "" {
			var err error
			req.ComposeContent, _, err = h.composeRun.ReadStackFiles(req.StackName)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		}
		if err := h.composeRun.ValidateComposeYAML(req.ComposeContent); err != nil {
			writeJSON(w, http.StatusOK, ComposeActionResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, ComposeActionResponse{
			Success: true,
			Logs:    "Compose YAML is valid.",
		})
		return
	}

	// If compose content is provided with up/build, save first
	if req.ComposeContent != "" {
		if err := h.composeRun.ValidateComposeYAML(req.ComposeContent); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid compose yaml: %w", err))
			return
		}
		if err := h.composeRun.SaveStackFiles(req.StackName, req.ComposeContent, req.EnvContent); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to save stack files: %w", err))
			return
		}
	}

	logs, err := h.composeRun.ExecuteInDir(r.Context(), req.StackName, req.StackPath, action, req.RemoveVolumes)
	if err != nil {
		writeJSON(w, http.StatusOK, ComposeActionResponse{
			Success: false,
			Logs:    logs,
			Error:   err.Error(),
		})
		return
	}

	h.invalidateStorageCache()
	writeJSON(w, http.StatusOK, ComposeActionResponse{
		Success: true,
		Logs:    logs,
	})
}

func (h *AgentHandler) handleReadFile(w http.ResponseWriter, r *http.Request) {
	var req ReadFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("path is required"))
		return
	}

	target := req.Path
	hostRoot := os.Getenv("HOST_ROOT")
	if hostRoot == "" {
		hostRoot = "/host/root"
	}

	// Try target path directly first
	info, err := os.Stat(target)
	if os.IsNotExist(err) || (err == nil && info.IsDir()) {
		// Try via hostRoot
		alt := filepath.Join(hostRoot, target)
		if info2, err2 := os.Stat(alt); err2 == nil && !info2.IsDir() {
			target = alt
			err = nil
		} else if !filepath.IsAbs(req.Path) {
			// Try via stacks root
			alt2 := filepath.Join(h.composeRun.StacksRoot(), req.Path)
			if info3, err3 := os.Stat(alt2); err3 == nil && !info3.IsDir() {
				target = alt2
				err = nil
			}
		}
	}

	if err != nil {
		writeJSON(w, http.StatusOK, ReadFileResponse{Exists: false})
		return
	}

	// Cap at 2MB
	f, err := os.Open(target)
	if err != nil {
		writeJSON(w, http.StatusOK, ReadFileResponse{Exists: false, Error: err.Error()})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, 2*1024*1024))
	if err != nil {
		writeJSON(w, http.StatusOK, ReadFileResponse{Exists: false, Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, ReadFileResponse{
		Content: string(data),
		Exists:  true,
	})
}

func (h *AgentHandler) handleDiscoverStacks(w http.ResponseWriter, r *http.Request) {
	discovered, err := compose.DiscoverStacks(h.composeRun.StacksRoot())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if discovered == nil {
		discovered = make([]compose.DiscoveredStack, 0)
	}
	writeJSON(w, http.StatusOK, DiscoverStacksResponse{Stacks: discovered})
}

func (h *AgentHandler) handleListImages(w http.ResponseWriter, r *http.Request) {
	var req ListImagesRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	imgs, err := h.dockerClient.ListImages(r.Context(), req.All)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, ListImagesResponse{Images: imgs})
}

func (h *AgentHandler) handlePullImage(w http.ResponseWriter, r *http.Request) {
	var req PullImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Image == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("image name is required"))
		return
	}

	var logBuf strings.Builder
	if err := h.dockerClient.PullImage(r.Context(), req.Image, &logBuf); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ActionResponse{Success: true, Message: logBuf.String()})
}

func (h *AgentHandler) handleDeleteImage(w http.ResponseWriter, r *http.Request) {
	var req DeleteImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("id is required"))
		return
	}

	if err := h.dockerClient.DeleteImage(r.Context(), req.ID, req.Force); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ActionResponse{Success: true})
}

func (h *AgentHandler) handlePruneImages(w http.ResponseWriter, r *http.Request) {
	res, err := h.dockerClient.PruneImages(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *AgentHandler) handleListVolumes(w http.ResponseWriter, r *http.Request) {
	vols, err := h.dockerClient.ListVolumes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, ListVolumesResponse{Volumes: vols})
}

func (h *AgentHandler) handleDeleteVolume(w http.ResponseWriter, r *http.Request) {
	var req DeleteVolumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	if err := h.dockerClient.DeleteVolume(r.Context(), req.Name, req.Force); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ActionResponse{Success: true})
}

func (h *AgentHandler) handlePruneVolumes(w http.ResponseWriter, r *http.Request) {
	res, err := h.dockerClient.PruneVolumes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *AgentHandler) handleListNetworks(w http.ResponseWriter, r *http.Request) {
	nets, err := h.dockerClient.ListNetworks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, ListNetworksResponse{Networks: nets})
}

func (h *AgentHandler) handleDeleteNetwork(w http.ResponseWriter, r *http.Request) {
	var req DeleteNetworkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("id is required"))
		return
	}

	if err := h.dockerClient.DeleteNetwork(r.Context(), req.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ActionResponse{Success: true})
}

func (h *AgentHandler) handleHostMetrics(w http.ResponseWriter, r *http.Request) {
	m, err := h.hostColl.Collect()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (h *AgentHandler) handleGPUMetrics(w http.ResponseWriter, r *http.Request) {
	m, err := h.gpuColl.Collect(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (h *AgentHandler) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	eventChan := make(chan models.DockerEvent, 20)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		_ = h.dockerClient.StreamEvents(ctx, eventChan)
		close(eventChan)
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-eventChan:
			if !ok {
				return
			}
			bytes, err := json.Marshal(event)
			if err == nil {
				fmt.Fprintf(w, "data: %s\n\n", string(bytes))
				flusher.Flush()
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": err.Error(),
	})
}
