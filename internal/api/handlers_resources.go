package api

import (
	"encoding/json"
	"net/http"

	"docker-panel/internal/auth"
	"docker-panel/internal/rbac"
)

type PullRequest struct {
	Image string `json:"image"`
}

func (s *Server) handleListImages(w http.ResponseWriter, r *http.Request) {
	all := r.URL.Query().Get("all") == "true"
	images, err := s.agentClient.ListImages(r.Context(), all)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, images)
}

func (s *Server) handlePullImage(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if !rbac.Can(user.Role, rbac.PermManageImages) {
		writeError(w, http.StatusForbidden, "insufficient permissions to pull image")
		return
	}

	var req PullRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Image == "" {
		writeError(w, http.StatusBadRequest, "image name is required")
		return
	}

	if err := s.agentClient.PullImage(r.Context(), req.Image); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.audit.Log(user.Email, "image:pull", req.Image, r.RemoteAddr, "", "success")
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleDeleteImage(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if !rbac.Can(user.Role, rbac.PermManageImages) {
		writeError(w, http.StatusForbidden, "insufficient permissions to delete image")
		return
	}

	id := r.PathValue("id")
	force := r.URL.Query().Get("force") == "true"

	if err := s.agentClient.DeleteImage(r.Context(), id, force); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.audit.Log(user.Email, "image:delete", id, r.RemoteAddr, "", "success")
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handlePruneImages(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if !rbac.Can(user.Role, rbac.PermSystemPrune) {
		writeError(w, http.StatusForbidden, "insufficient permissions to prune images")
		return
	}

	res, err := s.agentClient.PruneImages(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.audit.Log(user.Email, "image:prune", "all", r.RemoteAddr, "", "success")
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleListVolumes(w http.ResponseWriter, r *http.Request) {
	vols, err := s.agentClient.ListVolumes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, vols)
}

func (s *Server) handleDeleteVolume(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if !rbac.Can(user.Role, rbac.PermVolumeDelete) {
		writeError(w, http.StatusForbidden, "insufficient permissions: OWNER role required to delete volumes")
		return
	}

	if !checkCriticalConfirmation(r) {
		writeError(w, http.StatusBadRequest, "critical action confirmation required (X-Critical-Confirm: 1)")
		return
	}

	name := r.PathValue("name")
	force := r.URL.Query().Get("force") == "true"

	if err := s.agentClient.DeleteVolume(r.Context(), name, force); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.audit.Log(user.Email, "volume:delete", name, r.RemoteAddr, "", "success")
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handlePruneVolumes(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if !rbac.Can(user.Role, rbac.PermVolumeDelete) {
		writeError(w, http.StatusForbidden, "insufficient permissions: OWNER role required to prune volumes")
		return
	}

	if !checkCriticalConfirmation(r) {
		writeError(w, http.StatusBadRequest, "critical action confirmation required (X-Critical-Confirm: 1)")
		return
	}

	res, err := s.agentClient.PruneVolumes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.audit.Log(user.Email, "volume:prune", "all", r.RemoteAddr, "", "success")
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleListNetworks(w http.ResponseWriter, r *http.Request) {
	nets, err := s.agentClient.ListNetworks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, nets)
}

func (s *Server) handleDeleteNetwork(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if !rbac.Can(user.Role, rbac.PermManageNetworks) {
		writeError(w, http.StatusForbidden, "insufficient permissions to delete network")
		return
	}

	id := r.PathValue("id")
	if err := s.agentClient.DeleteNetwork(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.audit.Log(user.Email, "network:delete", id, r.RemoteAddr, "", "success")
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}
