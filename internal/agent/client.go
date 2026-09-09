package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"docker-panel/internal/compose"
	"docker-panel/internal/models"
)

type Client struct {
	httpClient   *http.Client
	streamClient *http.Client
	socketPath   string
}

func NewClient(socketURL string) *Client {
	if socketURL == "" {
		socketURL = "/run/panel-agent/agent.sock"
	}

	dialCtx := func(ctx context.Context, proto, addr string) (net.Conn, error) {
		if strings.HasPrefix(socketURL, "tcp://") {
			return (&net.Dialer{}).DialContext(ctx, "tcp", strings.TrimPrefix(socketURL, "tcp://"))
		}
		path := strings.TrimPrefix(socketURL, "unix://")
		return (&net.Dialer{}).DialContext(ctx, "unix", path)
	}

	transport := &http.Transport{
		DialContext:       dialCtx,
		DisableKeepAlives: false,
	}

	streamTransport := &http.Transport{
		DialContext:       dialCtx,
		DisableKeepAlives: false,
	}

	return &Client{
		socketPath: socketURL,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   120 * time.Second, // Long timeout for compose jobs/pulls
		},
		streamClient: &http.Client{
			Transport: streamTransport,
			Timeout:   0, // Allow continuous streaming
		},
	}
}

func (c *Client) Ping(ctx context.Context) (*PingResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://agent/ping", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent ping failed with status %d", resp.StatusCode)
	}

	var res PingResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) ListContainers(ctx context.Context, req ListContainersRequest) ([]models.ContainerInfo, error) {
	var resp ListContainersResponse
	if err := c.doJSON(ctx, "POST", "http://agent/actions/containers/list", req, &resp); err != nil {
		return nil, err
	}
	return resp.Containers, nil
}

func (c *Client) InspectContainer(ctx context.Context, id string) (*models.ContainerDetail, error) {
	var resp InspectContainerResponse
	if err := c.doJSON(ctx, "POST", "http://agent/actions/containers/inspect", InspectContainerRequest{ID: id}, &resp); err != nil {
		return nil, err
	}
	return resp.Container, nil
}

func (c *Client) StorageSnapshot(ctx context.Context) (*models.StorageSnapshot, error) {
	var resp StorageSnapshotResponse
	if err := c.doJSON(ctx, "POST", "http://agent/actions/storage/snapshot", struct{}{}, &resp); err != nil {
		return nil, err
	}
	return resp.Storage, nil
}

func (c *Client) ContainerAction(ctx context.Context, id string, action string, signal string) error {
	var resp ActionResponse
	if err := c.doJSON(ctx, "POST", "http://agent/actions/containers/action", ContainerActionRequest{
		ID:     id,
		Action: action,
		Signal: signal,
	}, &resp); err != nil {
		return err
	}
	if !resp.Success {
		return errors.New(resp.Error)
	}
	return nil
}

func (c *Client) ContainerStats(ctx context.Context, id string) (*models.ContainerStats, error) {
	var resp ContainerStatsResponse
	if err := c.doJSON(ctx, "POST", "http://agent/actions/containers/stats", ContainerStatsRequest{ID: id}, &resp); err != nil {
		return nil, err
	}
	return resp.Stats, nil
}

func (c *Client) BatchContainerStats(ctx context.Context, ids []string) (map[string]*models.ContainerStats, error) {
	var resp BatchContainerStatsResponse
	if err := c.doJSON(ctx, "POST", "http://agent/actions/containers/stats-batch", BatchContainerStatsRequest{ContainerIDs: ids}, &resp); err != nil {
		return nil, err
	}
	if resp.Stats == nil {
		return make(map[string]*models.ContainerStats), nil
	}
	return resp.Stats, nil
}

func (c *Client) StreamLogs(ctx context.Context, id string, follow bool, tail string, timestamps bool) (io.ReadCloser, error) {
	v := url.Values{}
	v.Set("id", id)
	if follow {
		v.Set("follow", "1")
	}
	if tail != "" {
		v.Set("tail", tail)
	}
	if timestamps {
		v.Set("timestamps", "1")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://agent/actions/containers/logs?%s", v.Encode()), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.streamClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("logs failed (%d): %s", resp.StatusCode, string(body))
	}
	return resp.Body, nil
}

func (c *Client) ComposeAction(ctx context.Context, req ComposeActionRequest) (*ComposeActionResponse, error) {
	var resp ComposeActionResponse
	if err := c.doJSON(ctx, "POST", "http://agent/actions/compose/action", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) DiscoverStacks(ctx context.Context) ([]compose.DiscoveredStack, error) {
	var resp DiscoverStacksResponse
	if err := c.doJSON(ctx, "POST", "http://agent/actions/compose/discover", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Stacks, nil
}

func (c *Client) ReadFile(ctx context.Context, path string) (string, bool, error) {
	var resp ReadFileResponse
	if err := c.doJSON(ctx, "POST", "http://agent/actions/compose/read-file", ReadFileRequest{Path: path}, &resp); err != nil {
		return "", false, err
	}
	return resp.Content, resp.Exists, nil
}

func (c *Client) ListImages(ctx context.Context, all bool) ([]models.ImageInfo, error) {
	var resp ListImagesResponse
	if err := c.doJSON(ctx, "POST", "http://agent/actions/images/list", ListImagesRequest{All: all}, &resp); err != nil {
		return nil, err
	}
	return resp.Images, nil
}

func (c *Client) PullImage(ctx context.Context, image string) error {
	var resp ActionResponse
	if err := c.doJSON(ctx, "POST", "http://agent/actions/images/pull", PullImageRequest{Image: image}, &resp); err != nil {
		return err
	}
	return nil
}

func (c *Client) DeleteImage(ctx context.Context, id string, force bool) error {
	var resp ActionResponse
	if err := c.doJSON(ctx, "POST", "http://agent/actions/images/delete", DeleteImageRequest{ID: id, Force: force}, &resp); err != nil {
		return err
	}
	return nil
}

func (c *Client) PruneImages(ctx context.Context) (*models.PruneResult, error) {
	var resp models.PruneResult
	if err := c.doJSON(ctx, "POST", "http://agent/actions/images/prune", PruneImagesRequest{All: true}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListVolumes(ctx context.Context) ([]models.VolumeInfo, error) {
	var resp ListVolumesResponse
	if err := c.doJSON(ctx, "POST", "http://agent/actions/volumes/list", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Volumes, nil
}

func (c *Client) DeleteVolume(ctx context.Context, name string, force bool) error {
	var resp ActionResponse
	if err := c.doJSON(ctx, "POST", "http://agent/actions/volumes/delete", DeleteVolumeRequest{Name: name, Force: force}, &resp); err != nil {
		return err
	}
	return nil
}

func (c *Client) PruneVolumes(ctx context.Context) (*models.PruneVolumesResult, error) {
	var resp models.PruneVolumesResult
	if err := c.doJSON(ctx, "POST", "http://agent/actions/volumes/prune", PruneVolumesRequest{All: true}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListNetworks(ctx context.Context) ([]models.NetworkInfo, error) {
	var resp ListNetworksResponse
	if err := c.doJSON(ctx, "POST", "http://agent/actions/networks/list", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Networks, nil
}

func (c *Client) DeleteNetwork(ctx context.Context, id string) error {
	var resp ActionResponse
	if err := c.doJSON(ctx, "POST", "http://agent/actions/networks/delete", DeleteNetworkRequest{ID: id}, &resp); err != nil {
		return err
	}
	return nil
}

func (c *Client) GetHostMetrics(ctx context.Context) (*models.HostMetrics, error) {
	var m models.HostMetrics
	if err := c.doJSON(ctx, "GET", "http://agent/metrics/host", nil, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (c *Client) GetGPUMetrics(ctx context.Context) (*models.GPUMetrics, error) {
	var m models.GPUMetrics
	if err := c.doJSON(ctx, "GET", "http://agent/metrics/gpu", nil, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (c *Client) StreamEvents(ctx context.Context, eventChan chan<- models.DockerEvent) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://agent/events", nil)
	if err != nil {
		return err
	}
	resp, err := c.streamClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var event models.DockerEvent
			if err := json.Unmarshal([]byte(data), &event); err == nil {
				select {
				case eventChan <- event:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}
	return scanner.Err()
}

func (c *Client) doJSON(ctx context.Context, method, targetURL string, reqBody any, dest any) error {
	var r io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, r)
	if err != nil {
		return err
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp.Error != "" {
			return errors.New(errResp.Error)
		}
		return fmt.Errorf("agent returned HTTP %d", resp.StatusCode)
	}

	if dest != nil {
		if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}
	return nil
}
