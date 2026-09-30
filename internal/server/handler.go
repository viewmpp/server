package server

import (
	"context"
	"io/fs"
	"net/http"
	"path"
	"server/internal/htmlutil"
	"server/internal/jsonutil"
	"server/internal/safelog"
	"server/internal/vcs"
	"server/ui"
	"strings"
	"sync"
	"time"
)

type healthcheck struct {
	Status   string `json:"status"`
	Database string `json:"database"`
	Parser   string `json:"parser"`
	Env      string `json:"env"`
	Version  string `json:"version"`
}

type healthReport struct {
	database string
	parser   string
}

type healthCache struct {
	mu       sync.Mutex
	probedAt time.Time
	report   healthReport
}

const (
	healthProbeTimeout = 2 * time.Second
	healthReportTTL    = 5 * time.Second
)

func (s *Server) serverHealthcheck(w http.ResponseWriter, r *http.Request) {
	report := s.healthReport()

	hc := healthcheck{
		Status:   healthy,
		Database: report.database,
		Parser:   report.parser,
		Env:      s.cfg.AppEnv,
		Version:  vcs.Version,
	}

	err := jsonutil.WriteJSON(w, http.StatusOK, hc, nil)
	if err != nil {
		jsonutil.ServerErrorResponse(w, r, err, s.logger)
	}
}

func (s *Server) healthReport() healthReport {
	s.health.mu.Lock()
	defer s.health.mu.Unlock()

	if time.Since(s.health.probedAt) < healthReportTTL {
		return s.health.report
	}

	s.health.report = healthReport{
		database: s.databaseHealthcheck(),
		parser:   s.parserHealthcheck(),
	}
	s.health.probedAt = time.Now()

	return s.health.report
}

func (s *Server) databaseHealthcheck() string {
	ctx, cancel := context.WithTimeout(context.Background(), healthProbeTimeout)
	defer cancel()

	if err := s.pg.Healthcheck(ctx); err != nil {
		s.logger.Warn("database healthcheck failed", "err", err)
		return unhealthy
	}

	return healthy
}

func (s *Server) parserHealthcheck() string {
	ctx, cancel := context.WithTimeout(context.Background(), healthProbeTimeout)
	defer cancel()

	if err := s.client.Healthcheck(ctx); err != nil {
		s.logger.Warn("parser healthcheck failed", "err", err)
		return unhealthy
	}

	return healthy
}

func (s *Server) static() http.Handler {
	files := http.FileServerFS(ui.Files)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if !strings.HasPrefix(name, "static/") || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}

		info, err := fs.Stat(ui.Files, name)
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}

		switch s.cfg.AppEnv {
		case "dev":
			w.Header().Set("Cache-Control", "no-store")
		default:
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}

const appHeader = "X-Requested-With"

const appHeaderValue = "viewmpp"

func (s *Server) fromApp(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(appHeader) != appHeaderValue {
			s.logger.Warn("api call without the app header", "method", r.Method, "uri", safelog.URI(r.URL.Path))
			jsonutil.ForbiddenResponse(w, "this endpoint is called by the viewer, not directly")
			return
		}

		next(w, r)
	}
}

func (s *Server) icon(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch s.cfg.AppEnv {
		case "dev":
			w.Header().Set("Cache-Control", "no-store")
		default:
			w.Header().Set("Cache-Control", "public, max-age=604800")
		}

		http.ServeFileFS(w, r, ui.Files, "static/icons/"+name)
	}
}

func (s *Server) redirect(to string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, to, http.StatusSeeOther)
	}
}

func (s *Server) moved(to string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, to, http.StatusMovedPermanently)
	}
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		jsonutil.NotFoundResponse(w)
		return
	}

	htmlutil.NotFoundPage(w, r, s.logger)
}
