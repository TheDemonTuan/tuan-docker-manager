package docker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_VolumeUsage_UsageData(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/system/df" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		raw := map[string]any{
			"Volumes": []map[string]any{
				{
					"Name": "db_data",
					"UsageData": map[string]any{
						"Size":     int64(2147483648), // 2 GB
						"RefCount": 1,
					},
				},
				{
					"Name": "cache_vol",
					"Size": int64(104857600), // 100 MB fallback
				},
			},
		}
		_ = json.NewEncoder(w).Encode(raw)
	}))
	defer ts.Close()

	cli := NewClient("tcp://" + ts.Listener.Addr().String())
	usage, err := cli.VolumeUsage(context.Background())
	if err != nil {
		t.Fatalf("VolumeUsage failed: %v", err)
	}

	if usage["db_data"] != 2147483648 {
		t.Errorf("expected 2GB for db_data, got %d", usage["db_data"])
	}
	if usage["cache_vol"] != 104857600 {
		t.Errorf("expected 100MB for cache_vol, got %d", usage["cache_vol"])
	}
}

func TestClient_ListContainers_SizeRootFs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/containers/json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("size") != "true" {
			t.Errorf("expected size=true query parameter")
		}
		raw := []map[string]any{
			{
				"Id":         "c1",
				"Names":      []string{"/web"},
				"Image":      "nginx:alpine",
				"ImageID":    "sha256:12345",
				"State":      "running",
				"Status":     "Up 10 hours",
				"SizeRw":     int64(10485760),   // 10 MB
				"SizeRootFs": int64(1258291200), // 1.2 GB
			},
		}
		_ = json.NewEncoder(w).Encode(raw)
	}))
	defer ts.Close()

	cli := NewClient("tcp://" + ts.Listener.Addr().String())
	containers, err := cli.ListContainers(context.Background(), true)
	if err != nil {
		t.Fatalf("ListContainers failed: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if containers[0].SizeRw == nil || *containers[0].SizeRw != 10485760 {
		t.Errorf("expected 10MB SizeRw, got %v", containers[0].SizeRw)
	}
	if containers[0].SizeRootFS == nil || *containers[0].SizeRootFS != 1258291200 {
		t.Errorf("expected 1.2GB SizeRootFS, got %v", containers[0].SizeRootFS)
	}
}
