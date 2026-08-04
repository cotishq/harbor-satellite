package server

import (
	"log"
	"net/http"

	"github.com/container-registry/harbor-satellite/pkg/version"
)

const (
	invalidNameMessage = "Invalid %s name: must be 1-255 chars, start with letter/number, and contain only lowercase letters, numbers, and ._-"
)

func (s *Server) Ping(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte("pong")); err != nil {
		log.Printf("failed to write ping response: %v", err)
	}
}

func (s *Server) Health(w http.ResponseWriter, _ *http.Request) {
	err := s.db.Ping()
	if err != nil {
		log.Printf("error pinging db: %v", err)
		WriteJSONResponse(w, http.StatusServiceUnavailable, healthResponse(string(Unhealthy)))
		return
	}

	WriteJSONResponse(w, http.StatusOK, healthResponse(string(Healthy)))
}

func healthResponse(status string) HealthResponse {
	info := version.BuildInfo()
	return HealthResponse{
		Status:    HealthResponseStatus(status),
		Version:   info.Version,
		GitCommit: info.GitCommit,
	}
}
