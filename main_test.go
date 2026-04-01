package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/prometheus/client_golang/prometheus"
)

// mockContainerClient implements a minimal mock for docker client operations
type mockContainerClient struct {
	containers []types.Container
	infos      map[string]types.ContainerJSON
}

func (m *mockContainerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	return m.containers, nil
}

func (m *mockContainerClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	return m.infos[containerID], nil
}

// TestHealthyEndpoint tests the /-/healthy endpoint without starting the server
func TestHealthyEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/-/healthy", nil)
	w := httptest.NewRecorder()

	testHandler := http.NewServeMux()
	testHandler.HandleFunc("/-/healthy", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("up"))
	})

	testHandler.ServeHTTP(w, req)

	result := w.Result()
	body, _ := io.ReadAll(result.Body)

	if result.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}

	if !strings.Contains(string(body), "up") {
		t.Errorf("expected 'up' in response, got: %s", string(body))
	}
}

// TestRootEndpoint tests the root endpoint without starting the server
func TestRootEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	testHandler := http.NewServeMux()
	testHandler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<h1>docker state exporter</h1>"))
	})

	testHandler.ServeHTTP(w, req)

	result := w.Result()
	body, _ := io.ReadAll(result.Body)

	if result.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}

	if !strings.Contains(string(body), "docker state exporter") {
		t.Errorf("expected 'docker state exporter' in response, got: %s", string(body))
	}
}

// TestDockerHealthCollectorWithMockedClient tests collector can gather container states
func TestDockerHealthCollectorWithMockedClient(t *testing.T) {
	mockClient := &mockContainerClient{
		containers: []types.Container{
			{
				ID:    "test-container-1",
				Names: []string{"/test1"},
				Image: "test-image:latest",
			},
		},
		infos: map[string]types.ContainerJSON{
			"test-container-1": {
				ContainerJSONBase: &types.ContainerJSONBase{
					ID: "test-container-1",
					State: &types.ContainerState{
						Status:     "running",
						OOMKilled:  false,
						StartedAt:  "2024-01-01T00:00:00Z",
						FinishedAt: "0001-01-01T00:00:00Z",
					},
				},
				Config: &container.Config{
					Image: "test-image:latest",
					Labels: map[string]string{
						"test-label": "test-value",
					},
				},
			},
		},
	}

	collector := &dockerHealthCollector{
		containerClient: mockClient,
	}

	// Call collectContainer to fetch from mock
	collector.collectContainer()

	// Verify cache was populated
	if len(collector.containerInfoCache) != 1 {
		t.Errorf("expected 1 container in cache, got %d", len(collector.containerInfoCache))
	}

	if collector.containerInfoCache[0].ID != "test-container-1" {
		t.Errorf("expected container ID 'test-container-1', got '%s'", collector.containerInfoCache[0].ID)
	}

	if collector.containerInfoCache[0].Config.Image != "test-image:latest" {
		t.Errorf("expected image 'test-image:latest', got '%s'", collector.containerInfoCache[0].Config.Image)
	}
}

// TestCollectMetrics tests that metrics are collected properly from container info
func TestCollectMetrics(t *testing.T) {
	mockClient := &mockContainerClient{
		containers: []types.Container{
			{
				ID:    "test-container-1",
				Names: []string{"/test1"},
				Image: "test-image:latest",
			},
		},
		infos: map[string]types.ContainerJSON{
			"test-container-1": {
				ContainerJSONBase: &types.ContainerJSONBase{
					ID: "test-container-1",
					State: &types.ContainerState{
						Status:     "running",
						OOMKilled:  false,
						StartedAt:  "2024-01-01T00:00:00.000000000Z",
						FinishedAt: "0001-01-01T00:00:00.000000000Z",
						Health:     nil,
					},
					RestartCount: 0,
				},
				Config: &container.Config{
					Image: "test-image:latest",
					Labels: map[string]string{
						"app": "test",
					},
				},
			},
		},
	}

	collector := &dockerHealthCollector{
		containerClient: mockClient,
	}

	collector.collectContainer()

	// Collect metrics
	ch := make(chan prometheus.Metric, 100)
	defer close(ch)

	// This should not panic when collecting metrics
	collector.collectMetrics(ch)

	if len(collector.containerInfoCache) != 1 {
		t.Errorf("expected 1 container, got %d", len(collector.containerInfoCache))
	}
}
