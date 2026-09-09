package api

import "net/http"

func (s *Server) handleStorageSnapshot(w http.ResponseWriter, r *http.Request) {
	storage, err := s.agentClient.StorageSnapshot(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, storage)
}
