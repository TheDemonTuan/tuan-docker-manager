package api

import (
	"bufio"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"

	"docker-panel/internal/agent"
	"docker-panel/internal/auth"
	"docker-panel/internal/rbac"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Origin checked in auth middleware
	},
}

func (s *Server) handleListContainers(w http.ResponseWriter, r *http.Request) {
	all := r.URL.Query().Get("all") != "false"
	stackName := r.URL.Query().Get("stack")

	containers, err := s.agentClient.ListContainers(r.Context(), agent.ListContainersRequest{
		All:       all,
		StackName: stackName,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, containers)
}

func (s *Server) handleGetContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	container, err := s.agentClient.InspectContainer(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, container)
}

func (s *Server) handleContainerStart(w http.ResponseWriter, r *http.Request) {
	s.execContainerAction(w, r, "start")
}

func (s *Server) handleContainerStop(w http.ResponseWriter, r *http.Request) {
	s.execContainerAction(w, r, "stop")
}

func (s *Server) handleContainerRestart(w http.ResponseWriter, r *http.Request) {
	s.execContainerAction(w, r, "restart")
}

func (s *Server) handleContainerKill(w http.ResponseWriter, r *http.Request) {
	s.execContainerAction(w, r, "kill")
}

func (s *Server) execContainerAction(w http.ResponseWriter, r *http.Request, action string) {
	user := auth.GetUser(r.Context())
	perm := rbac.PermContainerAction
	if action == "kill" {
		perm = rbac.PermContainerKill
	}

	if !rbac.Can(user.Role, perm) {
		writeError(w, http.StatusForbidden, "insufficient permissions for container action")
		return
	}

	id := r.PathValue("id")
	if err := s.agentClient.ContainerAction(r.Context(), id, action, ""); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.audit.Log(user.Email, "container:"+action, id, r.RemoteAddr, "", "success")
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleContainerStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stats, err := s.agentClient.ContainerStats(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleContainerLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	follow := r.URL.Query().Get("follow") == "true"
	tail := r.URL.Query().Get("tail")
	timestamps := r.URL.Query().Get("timestamps") == "true"

	stream, err := s.agentClient.StreamLogs(r.Context(), id, follow, tail, timestamps)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintf(w, "data: %s\n\n", line)
		flusher.Flush()
	}
}

func (s *Server) handleContainerTerminal(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if !rbac.Can(user.Role, rbac.PermViewTerminal) {
		writeError(w, http.StatusForbidden, "terminal access forbidden")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Initial message
	_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n--- Connected to container terminal session ---\r\n$ "))

	// Echo/Shell message loop
	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if mt == websocket.TextMessage || mt == websocket.BinaryMessage {
			// Echo input or handle enter
			if string(msg) == "\r" || string(msg) == "\n" {
				_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n$ "))
			} else {
				_ = conn.WriteMessage(websocket.TextMessage, msg)
			}
		}
	}
}
