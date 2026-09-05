package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"docker-panel/internal/compose"
	"docker-panel/internal/models"
)

func TestAgentClient_Ping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ping" {
			t.Errorf("expected path /ping, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(PingResponse{
			Status:        "ok",
			Version:       "0.1.0-mvp",
			DockerVersion: "27.2.0",
			OS:            "linux",
			Hostname:      "test-vps",
		})
	}))
	defer server.Close()

	client := NewClient("tcp://" + server.Listener.Addr().String())
	resp, err := client.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	if resp.Status != "ok" || resp.DockerVersion != "27.2.0" || resp.Hostname != "test-vps" {
		t.Errorf("unexpected ping response: %+v", resp)
	}
}

func TestAgentClient_ListContainers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/actions/containers/list" {
			t.Errorf("expected /actions/containers/list, got %s", r.URL.Path)
		}
		var req ListContainersRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if !req.All {
			t.Errorf("expected All=true")
		}

		resp := []models.ContainerInfo{
			{
				ID:        "c-1",
				Names:     []string{"/nginx-web"},
				Image:     "nginx:latest",
				State:     "running",
				Status:    "Up 2 hours",
				StackName: "production",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ListContainersResponse{Containers: resp})
	}))
	defer server.Close()

	client := NewClient("tcp://" + server.Listener.Addr().String())
	containers, err := client.ListContainers(context.Background(), ListContainersRequest{All: true})
	if err != nil {
		t.Fatalf("ListContainers failed: %v", err)
	}

	if len(containers) != 1 || len(containers[0].Names) == 0 || containers[0].Names[0] != "/nginx-web" {
		t.Errorf("unexpected containers result: %+v", containers)
	}
}

func TestAgentClient_ComposeAction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/actions/compose/action" {
			t.Errorf("expected /actions/compose/action, got %s", r.URL.Path)
		}
		var req ComposeActionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req.StackName != "my-stack" || req.Action != "up" {
			t.Errorf("unexpected request params: %+v", req)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ComposeActionResponse{
			Success: true,
			Logs:    "Container web Started\nContainer db Started\n",
		})
	}))
	defer server.Close()

	client := NewClient("tcp://" + server.Listener.Addr().String())
	resp, err := client.ComposeAction(context.Background(), ComposeActionRequest{
		StackName:      "my-stack",
		Action:         "up",
		ComposeContent: "version: '3.8'",
		EnvContent:     "PORT=80",
	})
	if err != nil {
		t.Fatalf("ComposeAction failed: %v", err)
	}

	if !resp.Success || resp.Logs == "" {
		t.Errorf("unexpected compose action response: %+v", resp)
	}
}

func TestAgentClient_PruneResources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/actions/images/prune":
			_ = json.NewEncoder(w).Encode(models.PruneResult{
				ImagesDeleted:  []string{"img1", "img2"},
				SpaceReclaimed: 104857600,
			})
		case "/actions/volumes/prune":
			_ = json.NewEncoder(w).Encode(models.PruneVolumesResult{
				VolumesDeleted: []string{"vol1"},
				SpaceReclaimed: 52428800,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient("tcp://" + server.Listener.Addr().String())

	imgPrune, err := client.PruneImages(context.Background())
	if err != nil {
		t.Fatalf("PruneImages failed: %v", err)
	}
	if len(imgPrune.ImagesDeleted) != 2 || imgPrune.SpaceReclaimed != 104857600 {
		t.Errorf("unexpected imgPrune: %+v", imgPrune)
	}

	volPrune, err := client.PruneVolumes(context.Background())
	if err != nil {
		t.Fatalf("PruneVolumes failed: %v", err)
	}
	if len(volPrune.VolumesDeleted) != 1 || volPrune.SpaceReclaimed != 52428800 {
		t.Errorf("unexpected volPrune: %+v", volPrune)
	}
}

func TestAgentClient_DiscoverStacks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/actions/compose/discover" {
			t.Errorf("expected /actions/compose/discover, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DiscoverStacksResponse{
			Stacks: []compose.DiscoveredStack{
				{
					Name:           "demo-app",
					Path:           "/srv/docker-panel/stacks/demo-app",
					ComposePath:    "/srv/docker-panel/stacks/demo-app/compose.yaml",
					ComposeContent: "services:\n  app:\n    image: nginx\n",
					EnvContent:     "PORT=80\n",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient("tcp://" + server.Listener.Addr().String())
	stacks, err := client.DiscoverStacks(context.Background())
	if err != nil {
		t.Fatalf("DiscoverStacks failed: %v", err)
	}
	if len(stacks) != 1 || stacks[0].Name != "demo-app" {
		t.Errorf("unexpected stacks: %+v", stacks)
	}
}
