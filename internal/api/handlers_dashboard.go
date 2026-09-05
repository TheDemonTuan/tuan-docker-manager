package api

import (
	"net/http"
	"strings"

	"docker-panel/internal/agent"
	"docker-panel/internal/compose"
	"docker-panel/internal/models"
)

type DashboardResponse struct {
	Host       *models.HostMetrics    `json:"host"`
	GPU        *models.GPUMetrics     `json:"gpu"`
	Stacks     []*models.Stack        `json:"stacks"`
	Containers []models.ContainerInfo `json:"containers"`
	Summary    DashboardSummary       `json:"summary"`
}

type DashboardSummary struct {
	TotalStacks       int `json:"total_stacks"`
	RunningStacks     int `json:"running_stacks"`
	TotalContainers   int `json:"total_containers"`
	RunningContainers int `json:"running_containers"`
	StoppedContainers int `json:"stopped_containers"`
	AverageSecurity   int `json:"average_security_score"`
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Host metrics
	hostMetrics, err := s.agentClient.GetHostMetrics(ctx)
	if err != nil {
		hostMetrics = &models.HostMetrics{}
	}

	// 2. GPU metrics
	gpuMetrics, err := s.agentClient.GetGPUMetrics(ctx)
	if err != nil {
		gpuMetrics = &models.GPUMetrics{Available: false}
	}

	// 3. Stacks from DB
	stacks, _ := s.db.GetAllStacks()
	if stacks == nil {
		stacks = make([]*models.Stack, 0)
	}

	// 4. Containers from Agent
	containers, err := s.agentClient.ListContainers(ctx, agent.ListContainersRequest{All: true})
	if err != nil {
		containers = make([]models.ContainerInfo, 0)
	}

	// Correlate containers to stacks
	compose.MatchContainersToStacks(stacks, containers)

	runningContainers := 0
	stoppedContainers := 0
	for _, c := range containers {
		if strings.EqualFold(c.State, "running") {
			runningContainers++
		} else {
			stoppedContainers++
		}
	}

	runningStacks := 0
	totalScore := 0
	for _, st := range stacks {
		if st.Status == models.StackStatusRunning {
			runningStacks++
		}
		totalScore += st.SecurityScore
	}

	avgScore := 100
	if len(stacks) > 0 {
		avgScore = totalScore / len(stacks)
	}

	writeJSON(w, http.StatusOK, DashboardResponse{
		Host:       hostMetrics,
		GPU:        gpuMetrics,
		Stacks:     stacks,
		Containers: containers,
		Summary: DashboardSummary{
			TotalStacks:       len(stacks),
			RunningStacks:     runningStacks,
			TotalContainers:   len(containers),
			RunningContainers: runningContainers,
			StoppedContainers: stoppedContainers,
			AverageSecurity:   avgScore,
		},
	})
}
