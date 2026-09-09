package agent

import (
	"docker-panel/internal/compose"
	"docker-panel/internal/models"
)

type ActionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type PingResponse struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	DockerVersion string `json:"docker_version,omitempty"`
	OS            string `json:"os"`
	Hostname      string `json:"hostname"`
}

type ListContainersRequest struct {
	All       bool              `json:"all"`
	Filters   map[string]string `json:"filters,omitempty"`
	StackName string            `json:"stack_name,omitempty"`
}

type ListContainersResponse struct {
	Containers []models.ContainerInfo `json:"containers"`
}

type InspectContainerRequest struct {
	ID string `json:"id"`
}

type InspectContainerResponse struct {
	Container *models.ContainerDetail `json:"container"`
}

type StorageSnapshotResponse struct {
	Storage *models.StorageSnapshot `json:"storage"`
}

type ContainerActionRequest struct {
	ID     string `json:"id"`
	Action string `json:"action"` // start, stop, restart, kill
	Signal string `json:"signal,omitempty"`
}

type ContainerStatsRequest struct {
	ID string `json:"id"`
}

type ContainerStatsResponse struct {
	Stats *models.ContainerStats `json:"stats"`
}

type BatchContainerStatsRequest struct {
	ContainerIDs []string `json:"container_ids,omitempty"`
}

type BatchContainerStatsResponse struct {
	Stats map[string]*models.ContainerStats `json:"stats"`
}

type ComposeActionRequest struct {
	StackName      string `json:"stack_name"`
	StackPath      string `json:"stack_path"`
	Action         string `json:"action"` // up, down, restart, pull, build, validate
	ComposeContent string `json:"compose_content,omitempty"`
	EnvContent     string `json:"env_content,omitempty"`
	RemoveVolumes  bool   `json:"remove_volumes,omitempty"`
}

type ComposeActionResponse struct {
	Success bool   `json:"success"`
	Logs    string `json:"logs"`
	Error   string `json:"error,omitempty"`
}

type DiscoverStacksResponse struct {
	Stacks []compose.DiscoveredStack `json:"stacks"`
}

type ListImagesRequest struct {
	All bool `json:"all"`
}

type ListImagesResponse struct {
	Images []models.ImageInfo `json:"images"`
}

type PullImageRequest struct {
	Image string `json:"image"`
}

type DeleteImageRequest struct {
	ID    string `json:"id"`
	Force bool   `json:"force"`
}

type PruneImagesRequest struct {
	All bool `json:"all"`
}

type ListVolumesResponse struct {
	Volumes []models.VolumeInfo `json:"volumes"`
}

type DeleteVolumeRequest struct {
	Name  string `json:"name"`
	Force bool   `json:"force"`
}

type PruneVolumesRequest struct {
	All bool `json:"all"`
}

type ListNetworksResponse struct {
	Networks []models.NetworkInfo `json:"networks"`
}

type DeleteNetworkRequest struct {
	ID string `json:"id"`
}

type ReadFileRequest struct {
	Path string `json:"path"`
}

type ReadFileResponse struct {
	Content string `json:"content"`
	Exists  bool   `json:"exists"`
	Error   string `json:"error,omitempty"`
}
