package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"docker-panel/internal/agent"
	"docker-panel/internal/compose"
	"docker-panel/internal/docker"
	"docker-panel/internal/gpu"
	"docker-panel/internal/metrics"
)

var Version = "0.1.0-mvp"

func main() {
	socketURL := flag.String("socket", getEnv("AGENT_SOCKET_URL", "/run/panel-agent/agent.sock"), "Agent listen socket URL (unix://path or tcp://addr)")
	dockerSock := flag.String("docker-sock", getEnv("DOCKER_SOCKET_PATH", "/var/run/docker.sock"), "Path to Docker daemon socket")
	stacksRoot := flag.String("stacks-root", getEnv("STACKS_ROOT", "/srv/docker-panel/stacks"), "Base directory for Compose stacks")
	procPath := flag.String("host-proc", getEnv("HOST_PROC", "/host/proc"), "Host proc filesystem path")
	sysPath := flag.String("host-sys", getEnv("HOST_SYS", "/host/sys"), "Host sys filesystem path")
	flag.Parse()

	log.Printf("[panel-agent] Starting version %s...", Version)
	log.Printf("[panel-agent] Socket: %s", *socketURL)
	log.Printf("[panel-agent] Stacks root: %s", *stacksRoot)

	// Ensure stacks root exists
	_ = os.MkdirAll(*stacksRoot, 0755)

	dockerClient := docker.NewClient(*dockerSock)
	hostColl := metrics.NewHostCollector(*procPath, *sysPath)
	gpuColl := gpu.NewCollector()
	composeRun := compose.NewRunner(*stacksRoot)

	handler := agent.NewAgentHandler(dockerClient, hostColl, gpuColl, composeRun, Version)

	var listener net.Listener
	var err error

	isUnix := !strings.HasPrefix(*socketURL, "tcp://")
	cleanPath := strings.TrimPrefix(*socketURL, "unix://")

	if isUnix {
		// Ensure socket parent directory exists
		socketDir := filepath.Dir(cleanPath)
		if err := os.MkdirAll(socketDir, 0755); err != nil {
			log.Fatalf("[panel-agent] Failed to create socket directory %s: %v", socketDir, err)
		}

		// Remove stale socket if exists
		if _, err := os.Stat(cleanPath); err == nil {
			_ = os.Remove(cleanPath)
		}

		listener, err = net.Listen("unix", cleanPath)
		if err != nil {
			log.Fatalf("[panel-agent] Failed to bind unix socket %s: %v", cleanPath, err)
		}
		// Set permissions so panel-core can read/write
		_ = os.Chmod(cleanPath, 0660)
		defer os.Remove(cleanPath)
	} else {
		tcpAddr := strings.TrimPrefix(*socketURL, "tcp://")
		listener, err = net.Listen("tcp", tcpAddr)
		if err != nil {
			log.Fatalf("[panel-agent] Failed to bind tcp address %s: %v", tcpAddr, err)
		}
	}
	defer listener.Close()

	srv := &http.Server{
		Handler:      handler.Router(),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 0, // Streaming logs / events
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[panel-agent] Listening on %s", listener.Addr().String())
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[panel-agent] Server error: %v", err)
		}
	}()

	<-stop
	log.Printf("[panel-agent] Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Printf("[panel-agent] Stopped.")
}

func getEnv(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}
