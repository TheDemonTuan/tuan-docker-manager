package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"docker-panel/internal/models"
)

type Client struct {
	httpClient *http.Client
	socketPath string
	mu         sync.RWMutex
}

func NewClient(socketPath string) *Client {
	if socketPath == "" {
		if runtime.GOOS == "windows" {
			socketPath = `//./pipe/docker_engine`
		} else {
			socketPath = "/var/run/docker.sock"
		}
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, proto, addr string) (net.Conn, error) {
			if strings.HasPrefix(socketPath, "tcp://") {
				return (&net.Dialer{}).DialContext(ctx, "tcp", strings.TrimPrefix(socketPath, "tcp://"))
			}
			// Windows named pipe or Unix socket
			return dialSocket(ctx, socketPath)
		},
		DisableKeepAlives: false,
	}

	return &Client{
		socketPath: socketPath,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   0, // Allow streaming
		},
	}
}

func (c *Client) IsAvailable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "http://docker/_ping", nil)
	if err != nil {
		return false
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (c *Client) GetVersion(ctx context.Context) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://docker/version", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("docker version failed (%d): %s", resp.StatusCode, string(body))
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}

func (c *Client) ListContainers(ctx context.Context, all bool) ([]models.ContainerInfo, error) {
	u := "http://docker/containers/json?size=true"
	if all {
		u += "&all=true"
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("docker list containers failed (%d): %s", resp.StatusCode, string(body))
	}

	var rawContainers []struct {
		ID      string   `json:"Id"`
		Names   []string `json:"Names"`
		Image   string   `json:"Image"`
		ImageID string   `json:"ImageID"`
		Command string   `json:"Command"`
		Created int64    `json:"Created"`
		State   string   `json:"State"`
		Status  string   `json:"Status"`
		Ports   []struct {
			IP          string `json:"IP"`
			PrivatePort uint16 `json:"PrivatePort"`
			PublicPort  uint16 `json:"PublicPort"`
			Type        string `json:"Type"`
		} `json:"Ports"`
		Labels        map[string]string `json:"Labels"`
		SizeRw        *int64            `json:"SizeRw"`
		SizeRootFS    *int64            `json:"SizeRootFs"`
		SizeRootfsAlt *int64            `json:"SizeRootfs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawContainers); err != nil {
		return nil, err
	}

	result := make([]models.ContainerInfo, 0, len(rawContainers))
	for _, raw := range rawContainers {
		if raw.SizeRootFS == nil && raw.SizeRootfsAlt != nil {
			raw.SizeRootFS = raw.SizeRootfsAlt
		}
		ports := make([]models.PortMapping, 0, len(raw.Ports))
		for _, p := range raw.Ports {
			exp := "INTERNAL"
			if p.PublicPort > 0 {
				if p.IP == "127.0.0.1" || p.IP == "localhost" || p.IP == "::1" {
					exp = "LOCALHOST"
				} else {
					exp = "PUBLIC"
				}
			}
			ports = append(ports, models.PortMapping{
				IP:          p.IP,
				PrivatePort: p.PrivatePort,
				PublicPort:  p.PublicPort,
				Type:        p.Type,
				Exposure:    exp,
			})
		}

		cleanNames := make([]string, 0, len(raw.Names))
		for _, n := range raw.Names {
			cleanNames = append(cleanNames, strings.TrimPrefix(n, "/"))
		}

		stackName := raw.Labels["com.docker.compose.project"]
		serviceName := raw.Labels["com.docker.compose.service"]
		composeFile := raw.Labels["com.docker.compose.project.config_files"]
		workingDir := raw.Labels["com.docker.compose.project.working_dir"]

		result = append(result, models.ContainerInfo{
			ID:          raw.ID,
			Names:       cleanNames,
			Image:       raw.Image,
			ImageID:     raw.ImageID,
			Command:     raw.Command,
			Created:     raw.Created,
			State:       raw.State,
			Status:      raw.Status,
			Ports:       ports,
			Labels:      raw.Labels,
			StackName:   stackName,
			ServiceName: serviceName,
			ComposeFile: composeFile,
			WorkingDir:  workingDir,
			SizeRw:      normalizeSize(raw.SizeRw),
			SizeRootFS:  normalizeSize(raw.SizeRootFS),
		})
	}

	return result, nil
}

func normalizeSize(value *int64) *int64 {
	if value == nil || *value < 0 {
		return nil
	}
	return value
}

func (c *Client) InspectContainer(ctx context.Context, id string) (*models.ContainerDetail, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://docker/containers/%s/json?size=true", id), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("docker inspect failed (%d): %s", resp.StatusCode, string(body))
	}

	var raw struct {
		ID      string   `json:"Id"`
		Created string   `json:"Created"`
		Path    string   `json:"Path"`
		Args    []string `json:"Args"`
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
				Status string `json:"Status"`
			} `json:"Health"`
		} `json:"State"`
		Image        string `json:"Image"`
		RestartCount int    `json:"RestartCount"`
		HostConfig   struct {
			RestartPolicy struct {
				Name string `json:"Name"`
			} `json:"RestartPolicy"`
		} `json:"HostConfig"`
		Config struct {
			Image  string            `json:"Image"`
			Cmd    []string          `json:"Cmd"`
			Env    []string          `json:"Env"`
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		NetworkSettings struct {
			IPAddress string `json:"IPAddress"`
			Networks  map[string]struct {
				IPAddress string `json:"IPAddress"`
			} `json:"Networks"`
			Ports map[string][]struct {
				HostIP   string `json:"HostIP"`
				HostPort string `json:"HostPort"`
			} `json:"Ports"`
		} `json:"NetworkSettings"`
		Mounts []struct {
			Type        string `json:"Type"`
			Name        string `json:"Name"`
			Source      string `json:"Source"`
			Destination string `json:"Destination"`
			Mode        string `json:"Mode"`
			RW          bool   `json:"RW"`
		} `json:"Mounts"`
		Name          string `json:"Name"`
		SizeRw        *int64 `json:"SizeRw"`
		SizeRootFS    *int64 `json:"SizeRootFs"`
		SizeRootfsAlt *int64 `json:"SizeRootfs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	if raw.SizeRootFS == nil && raw.SizeRootfsAlt != nil {
		raw.SizeRootFS = raw.SizeRootfsAlt
	}

	detail := &models.ContainerDetail{
		ContainerInfo: models.ContainerInfo{
			ID:          raw.ID,
			Names:       []string{strings.TrimPrefix(raw.Name, "/")},
			Image:       raw.Config.Image,
			ImageID:     raw.Image,
			Command:     strings.Join(raw.Config.Cmd, " "),
			State:       raw.State.Status,
			Status:      raw.State.Status,
			Labels:      raw.Config.Labels,
			StackName:   raw.Config.Labels["com.docker.compose.project"],
			ServiceName: raw.Config.Labels["com.docker.compose.service"],
			SizeRw:      normalizeSize(raw.SizeRw),
			SizeRootFS:  normalizeSize(raw.SizeRootFS),
		},
		StartedAt:     raw.State.StartedAt,
		FinishedAt:    raw.State.FinishedAt,
		RestartCount:  raw.RestartCount,
		RestartPolicy: raw.HostConfig.RestartPolicy.Name,
		IPAddress:     raw.NetworkSettings.IPAddress,
		Networks:      make([]string, 0),
		Mounts:        make([]models.MountDetail, 0, len(raw.Mounts)),
		Env:           raw.Config.Env,
		Args:          raw.Args,
	}

	if raw.State.Health != nil {
		detail.Health = raw.State.Health.Status
	}

	for netName := range raw.NetworkSettings.Networks {
		detail.Networks = append(detail.Networks, netName)
	}

	for _, m := range raw.Mounts {
		detail.Mounts = append(detail.Mounts, models.MountDetail{
			Type:        m.Type,
			Name:        m.Name,
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        m.Mode,
			RW:          m.RW,
		})
	}

	return detail, nil
}

func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.postEmpty(ctx, fmt.Sprintf("http://docker/containers/%s/start", id))
}

func (c *Client) StopContainer(ctx context.Context, id string) error {
	return c.postEmpty(ctx, fmt.Sprintf("http://docker/containers/%s/stop?t=10", id))
}

func (c *Client) RestartContainer(ctx context.Context, id string) error {
	return c.postEmpty(ctx, fmt.Sprintf("http://docker/containers/%s/restart?t=10", id))
}

func (c *Client) KillContainer(ctx context.Context, id string, signal string) error {
	u := fmt.Sprintf("http://docker/containers/%s/kill", id)
	if signal != "" {
		u += "?signal=" + signal
	}
	return c.postEmpty(ctx, u)
}

func (c *Client) postEmpty(ctx context.Context, rawURL string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("action failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) StreamLogs(ctx context.Context, id string, stdout, stderr, follow, timestamps bool, tail string, out io.Writer) error {
	v := url.Values{}
	if stdout {
		v.Set("stdout", "1")
	}
	if stderr {
		v.Set("stderr", "1")
	}
	if follow {
		v.Set("follow", "1")
	}
	if timestamps {
		v.Set("timestamps", "1")
	}
	if tail != "" {
		v.Set("tail", tail)
	} else {
		v.Set("tail", "100")
	}

	reqURL := fmt.Sprintf("http://docker/containers/%s/logs?%s", id, v.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker logs failed (%d): %s", resp.StatusCode, string(body))
	}

	// Demux Docker stdcopy header [8 bytes: 1 byte type, 3 bytes zeros, 4 bytes big endian length]
	header := make([]byte, 8)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, err := io.ReadFull(resp.Body, header)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			return err
		}

		size := binary.BigEndian.Uint32(header[4:8])
		if size == 0 {
			continue
		}

		frame := make([]byte, size)
		_, err = io.ReadFull(resp.Body, frame)
		if err != nil {
			return err
		}

		if _, err := out.Write(frame); err != nil {
			return err
		}
	}
}

func (c *Client) GetContainerStats(ctx context.Context, id string) (*models.ContainerStats, error) {
	reqURL := fmt.Sprintf("http://docker/containers/%s/stats?stream=false", id)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("container stats failed (%d): %s", resp.StatusCode, string(body))
	}

	var raw struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
			OnlineCPUs     uint32 `json:"online_cpus"`
		} `json:"cpu_stats"`
		PreCPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
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
			Current uint32 `json:"current"`
		} `json:"pids_stats"`
		BlkioStats struct {
			IOServiceBytesRecursive []struct {
				Op    string `json:"op"`
				Value uint64 `json:"value"`
			} `json:"io_service_bytes_recursive"`
		} `json:"blkio_stats"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	// Calculate CPU percentage
	cpuDelta := float64(raw.CPUStats.CPUUsage.TotalUsage) - float64(raw.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(raw.CPUStats.SystemCPUUsage) - float64(raw.PreCPUStats.SystemCPUUsage)
	cpuPercent := 0.0
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpus := float64(raw.CPUStats.OnlineCPUs)
		if cpus == 0 {
			cpus = 1.0
		}
		cpuPercent = (cpuDelta / systemDelta) * cpus * 100.0
	}

	memPercent := 0.0
	if raw.MemoryStats.Limit > 0 {
		memPercent = (float64(raw.MemoryStats.Usage) / float64(raw.MemoryStats.Limit)) * 100.0
	}

	var rxTotal, txTotal uint64
	for _, n := range raw.Networks {
		rxTotal += n.RxBytes
		txTotal += n.TxBytes
	}

	var readTotal, writeTotal uint64
	for _, b := range raw.BlkioStats.IOServiceBytesRecursive {
		if strings.EqualFold(b.Op, "Read") {
			readTotal += b.Value
		} else if strings.EqualFold(b.Op, "Write") {
			writeTotal += b.Value
		}
	}

	return &models.ContainerStats{
		ContainerID:   id,
		Timestamp:     time.Now(),
		CPUPercent:    cpuPercent,
		MemoryUsed:    raw.MemoryStats.Usage,
		MemoryLimit:   raw.MemoryStats.Limit,
		MemoryPercent: memPercent,
		PIDs:          raw.PidsStats.Current,
		NetRxBytes:    rxTotal,
		NetTxBytes:    txTotal,
		BlockRead:     readTotal,
		BlockWrite:    writeTotal,
	}, nil
}

func (c *Client) ListImages(ctx context.Context, all bool) ([]models.ImageInfo, error) {
	u := "http://docker/images/json"
	if all {
		u += "?all=1"
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list images failed (%d): %s", resp.StatusCode, string(body))
	}

	var rawImages []struct {
		ID          string            `json:"Id"`
		RepoTags    []string          `json:"RepoTags"`
		RepoDigests []string          `json:"RepoDigests"`
		Created     int64             `json:"Created"`
		Size        int64             `json:"Size"`
		SharedSize  int64             `json:"SharedSize"`
		Containers  int64             `json:"Containers"`
		Labels      map[string]string `json:"Labels"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawImages); err != nil {
		return nil, err
	}

	images := make([]models.ImageInfo, 0, len(rawImages))
	for _, raw := range rawImages {
		images = append(images, models.ImageInfo{
			ID:          raw.ID,
			RepoTags:    raw.RepoTags,
			RepoDigests: raw.RepoDigests,
			Created:     raw.Created,
			Size:        raw.Size,
			SharedSize:  raw.SharedSize,
			Containers:  raw.Containers,
			Labels:      raw.Labels,
		})
	}
	return images, nil
}

func (c *Client) PullImage(ctx context.Context, imageName string, logWriter io.Writer) error {
	u := fmt.Sprintf("http://docker/images/create?fromImage=%s", url.QueryEscape(imageName))
	req, err := http.NewRequestWithContext(ctx, "POST", u, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pull image failed (%d): %s", resp.StatusCode, string(body))
	}

	dec := json.NewDecoder(resp.Body)
	for dec.More() {
		var status struct {
			Status   string `json:"status"`
			Progress string `json:"progress"`
			Error    string `json:"error"`
		}
		if err := dec.Decode(&status); err != nil {
			break
		}
		if status.Error != "" {
			return errors.New(status.Error)
		}
		if logWriter != nil && status.Status != "" {
			fmt.Fprintf(logWriter, "%s %s\n", status.Status, status.Progress)
		}
	}
	return nil
}

func (c *Client) DeleteImage(ctx context.Context, id string, force bool) error {
	u := fmt.Sprintf("http://docker/images/%s", id)
	if force {
		u += "?force=1"
	}
	req, err := http.NewRequestWithContext(ctx, "DELETE", u, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete image failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) PruneImages(ctx context.Context) (*models.PruneResult, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", "http://docker/images/prune", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prune images failed (%d): %s", resp.StatusCode, string(body))
	}

	var raw struct {
		ImagesDeleted []struct {
			Untagged string `json:"Untagged"`
			Deleted  string `json:"Deleted"`
		} `json:"ImagesDeleted"`
		SpaceReclaimed uint64 `json:"SpaceReclaimed"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	deleted := make([]string, 0)
	for _, img := range raw.ImagesDeleted {
		if img.Deleted != "" {
			deleted = append(deleted, img.Deleted)
		} else if img.Untagged != "" {
			deleted = append(deleted, img.Untagged)
		}
	}

	return &models.PruneResult{
		ImagesDeleted:  deleted,
		SpaceReclaimed: raw.SpaceReclaimed,
	}, nil
}

func (c *Client) VolumeUsage(ctx context.Context) (map[string]int64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://docker/system/df", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("docker system df failed (%d): %s", resp.StatusCode, string(body))
	}
	var raw struct {
		Volumes []struct {
			Name      string `json:"Name"`
			Size      int64  `json:"Size"`
			UsageData struct {
				Size     int64 `json:"Size"`
				RefCount int64 `json:"RefCount"`
			} `json:"UsageData"`
		} `json:"Volumes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	usage := make(map[string]int64, len(raw.Volumes))
	for _, volume := range raw.Volumes {
		s := volume.UsageData.Size
		if s <= 0 && volume.Size > 0 {
			s = volume.Size
		}
		usage[volume.Name] = s
	}
	return usage, nil
}

func (c *Client) ListVolumes(ctx context.Context) ([]models.VolumeInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://docker/volumes", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list volumes failed (%d): %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Volumes []struct {
			Name       string            `json:"Name"`
			Driver     string            `json:"Driver"`
			Mountpoint string            `json:"Mountpoint"`
			CreatedAt  string            `json:"CreatedAt"`
			Status     map[string]any    `json:"Status"`
			Labels     map[string]string `json:"Labels"`
			Scope      string            `json:"Scope"`
		} `json:"Volumes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	vols := make([]models.VolumeInfo, 0, len(raw.Volumes))
	for _, rv := range raw.Volumes {
		vols = append(vols, models.VolumeInfo{
			Name:       rv.Name,
			Driver:     rv.Driver,
			Mountpoint: rv.Mountpoint,
			CreatedAt:  rv.CreatedAt,
			Status:     rv.Status,
			Labels:     rv.Labels,
			Scope:      rv.Scope,
		})
	}
	return vols, nil
}

func (c *Client) DeleteVolume(ctx context.Context, name string, force bool) error {
	u := fmt.Sprintf("http://docker/volumes/%s", name)
	if force {
		u += "?force=1"
	}
	req, err := http.NewRequestWithContext(ctx, "DELETE", u, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete volume failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) PruneVolumes(ctx context.Context) (*models.PruneVolumesResult, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", "http://docker/volumes/prune", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prune volumes failed (%d): %s", resp.StatusCode, string(body))
	}

	var raw struct {
		VolumesDeleted []string `json:"VolumesDeleted"`
		SpaceReclaimed uint64   `json:"SpaceReclaimed"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	return &models.PruneVolumesResult{
		VolumesDeleted: raw.VolumesDeleted,
		SpaceReclaimed: raw.SpaceReclaimed,
	}, nil
}

func (c *Client) ListNetworks(ctx context.Context) ([]models.NetworkInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://docker/networks", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list networks failed (%d): %s", resp.StatusCode, string(body))
	}

	var rawNets []struct {
		ID         string            `json:"Id"`
		Name       string            `json:"Name"`
		Driver     string            `json:"Driver"`
		Scope      string            `json:"Scope"`
		EnableIPv6 bool              `json:"EnableIPv6"`
		Internal   bool              `json:"Internal"`
		Attachable bool              `json:"Attachable"`
		Labels     map[string]string `json:"Labels"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawNets); err != nil {
		return nil, err
	}

	nets := make([]models.NetworkInfo, 0, len(rawNets))
	for _, rn := range rawNets {
		nets = append(nets, models.NetworkInfo{
			ID:         rn.ID,
			Name:       rn.Name,
			Driver:     rn.Driver,
			Scope:      rn.Scope,
			EnableIPv6: rn.EnableIPv6,
			Internal:   rn.Internal,
			Attachable: rn.Attachable,
			Labels:     rn.Labels,
		})
	}
	return nets, nil
}

func (c *Client) DeleteNetwork(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("http://docker/networks/%s", id), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete network failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) StreamEvents(ctx context.Context, eventChan chan<- models.DockerEvent) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://docker/events", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			return err
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var raw struct {
			Type   string `json:"Type"`
			Action string `json:"Action"`
			Actor  struct {
				ID         string            `json:"ID"`
				Attributes map[string]string `json:"Attributes"`
			} `json:"Actor"`
			Time     int64 `json:"time"`
			TimeNano int64 `json:"timeNano"`
		}

		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}

		actorName := raw.Actor.Attributes["name"]
		eventChan <- models.DockerEvent{
			Type:      raw.Type,
			Action:    raw.Action,
			ActorID:   raw.Actor.ID,
			ActorName: actorName,
			Timestamp: time.Unix(raw.Time, 0),
			Message:   fmt.Sprintf("%s %s %s", raw.Type, raw.Action, actorName),
		}
	}
}

// dialSocket connects to either a Unix domain socket or Windows named pipe
func dialSocket(ctx context.Context, socketPath string) (net.Conn, error) {
	if runtime.GOOS == "windows" && strings.HasPrefix(socketPath, `\\.\pipe\`) {
		// If on windows named pipe without specialized lib, try unix or fallback to localhost TCP if configured
		return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
	}
	return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
}
