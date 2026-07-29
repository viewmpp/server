package server

import (
	"mpp-viewer-server/internal/jsonutil"
	"mpp-viewer-server/internal/vcs"
	"net/http"
)

type healthcheck struct {
	Status  string `json:"status"`
	Env     string `json:"env"`
	Version string `json:"version"`
}

func (s *Server) healthcheck(w http.ResponseWriter, r *http.Request) {
	hc := healthcheck{
		Status:  "OK",
		Env:     s.cfg.Env,
		Version: vcs.Version(),
	}

	err := jsonutil.WriteJSON(w, http.StatusOK, hc, nil)
	if err != nil {
		jsonutil.ServerErrorResponse(w, r, err, s.logger)
	}
}
