package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

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
	mux.HandleFunc("POST /actions/containers/action", h.handleContainerAction)
	mux.HandleFunc("GET /actions/containers/logs", h.handleContainerLogs)
	mux.HandleFunc("POST /actions/containers/stats", h.handleContainerStats)

	// Compose
	mux.HandleFunc("POST /actions/compose/action", h.handleComposeAction)

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
	if v, err := h.dockerClient.GetVersion(r.Context()); err == nil {
		if verStr, ok := v["Version"].(string); ok {
			dockerVer = verStr
		}
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

	writeJSON(w, http.StatusOK, ActionResponse{Success: true, Message: "action completed successfully"})
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

	logs, err := h.composeRun.Execute(r.Context(), req.StackName, action, req.RemoveVolumes)
	if err != nil {
		writeJSON(w, http.StatusOK, ComposeActionResponse{
			Success: false,
			Logs:    logs,
			Error:   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, ComposeActionResponse{
		Success: true,
		Logs:    logs,
	})
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
