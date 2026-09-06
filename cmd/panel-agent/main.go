package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
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
	healthcheck := flag.Bool("healthcheck", false, "Check panel agent readiness")
	socketURL := flag.String("socket", getEnv("AGENT_SOCKET_URL", getEnv("SOCKET_PATH", "/run/panel-agent/agent.sock")), "Agent listen socket URL (unix://path or tcp://addr)")
	dockerHost := getEnv("DOCKER_SOCKET_PATH", getEnv("DOCKER_HOST", "/var/run/docker.sock"))
	dockerHost = strings.TrimPrefix(dockerHost, "unix://")
	dockerSock := flag.String("docker-sock", dockerHost, "Path to Docker daemon socket")
	socketGID := flag.Int("socket-gid", getEnvInt("AGENT_SOCKET_GID", 10001), "GID for Unix socket group ownership")
	stacksRoot := flag.String("stacks-root", getEnv("STACKS_ROOT", "/srv/docker-panel/stacks"), "Base directory for Compose stacks")
	procPath := flag.String("host-proc", getEnv("HOST_PROC", "/host/proc"), "Host proc filesystem path")
	sysPath := flag.String("host-sys", getEnv("HOST_SYS", "/host/sys"), "Host sys filesystem path")
	rootPath := flag.String("host-root", getEnv("HOST_ROOT", "/host/root"), "Host root filesystem path")
	flag.Parse()

	if *healthcheck {
		if err := checkAgentHealth(*socketURL); err != nil {
			log.Printf("[panel-agent] Healthcheck failed: %v", err)
			os.Exit(1)
		}
		return
	}

	log.Printf("[panel-agent] Starting version %s...", Version)
	log.Printf("[panel-agent] Socket: %s (GID: %d)", *socketURL, *socketGID)
	log.Printf("[panel-agent] Stacks root: %s", *stacksRoot)

	// Ensure stacks root exists
	_ = os.MkdirAll(*stacksRoot, 0755)

	dockerClient := docker.NewClient(*dockerSock)
	hostColl := metrics.NewHostCollector(*procPath, *sysPath, *rootPath)
	gpuColl := gpu.NewCollector()
	composeRun := compose.NewRunner(*stacksRoot)

	handler := agent.NewAgentHandler(dockerClient, hostColl, gpuColl, composeRun, Version)

	var listener net.Listener
	var err error

	isUnix := !strings.HasPrefix(*socketURL, "tcp://")
	cleanPath := strings.TrimPrefix(*socketURL, "unix://")

	if isUnix {
		// Ensure socket parent directory exists with secure group permissions
		socketDir := filepath.Dir(cleanPath)
		if err := os.MkdirAll(socketDir, 0770); err != nil {
			log.Fatalf("[panel-agent] Failed to create socket directory %s: %v", socketDir, err)
		}
		if *socketGID >= 0 {
			_ = os.Chown(socketDir, -1, *socketGID)
		}
		_ = os.Chmod(socketDir, 0770)

		// Remove stale socket if exists
		if _, err := os.Stat(cleanPath); err == nil {
			_ = os.Remove(cleanPath)
		}

		listener, err = net.Listen("unix", cleanPath)
		if err != nil {
			log.Fatalf("[panel-agent] Failed to bind unix socket %s: %v", cleanPath, err)
		}
		// Set permissions and group so panel-core (running as panel:10001) can access socket
		if *socketGID >= 0 {
			if err := os.Chown(cleanPath, -1, *socketGID); err != nil {
				log.Printf("[panel-agent] Notice: could not chown socket to gid %d: %v", *socketGID, err)
			}
		}
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

func checkAgentHealth(socketURL string) error {
	transport := &http.Transport{}
	if strings.HasPrefix(socketURL, "tcp://") {
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", strings.TrimPrefix(socketURL, "tcp://"))
		}
	} else {
		path := strings.TrimPrefix(socketURL, "unix://")
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", path)
		}
	}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	resp, err := client.Get("http://agent/ping")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

func getEnv(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}

func getEnvInt(key string, def int) int {
	if val := os.Getenv(key); val != "" {
		if v, err := strconv.Atoi(val); err == nil {
			return v
		}
	}
	return def
}
