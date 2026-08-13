package server

import (
	"net/http"
	"server/internal/htmlutil"
	"server/internal/jsonutil"
	"server/ui"
	"strings"
)

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", s.static())

	mux.HandleFunc("GET /{$}", s.viewerHandler.Landing)

	mux.HandleFunc("GET /api/v1/healthcheck", s.healthcheck)
	mux.HandleFunc("POST /api/v1/upload", s.uploadHandler.Upload)
	mux.HandleFunc("GET /api/v1/projects/{id}", s.projectHandler.Contract)

	mux.HandleFunc("GET /api/v1/examples/{name}", s.viewerHandler.ExampleContract)

	mux.HandleFunc("GET /example/{name}", s.viewerHandler.ExamplePage)

	mux.HandleFunc("GET /projects", s.projectHandler.List)
	mux.HandleFunc("GET /p/{id}", s.projectHandler.Page)
	mux.HandleFunc("POST /p/{id}/access", s.projectHandler.SetAccess)
	mux.HandleFunc("POST /p/{id}/delete", s.projectHandler.Delete)

	mux.HandleFunc("GET /signup", s.userHandler.SignupPage)
	mux.HandleFunc("POST /signup", s.userHandler.Signup)

	mux.HandleFunc("GET /verify", s.userHandler.VerifyPage)
	mux.HandleFunc("POST /verify", s.userHandler.Verify)
	mux.HandleFunc("POST /verify/resend", s.userHandler.ResendCode)

	mux.HandleFunc("GET /signin", s.userHandler.SigninPage)
	mux.HandleFunc("POST /signin", s.userHandler.Signin)
	mux.HandleFunc("POST /signout", s.userHandler.Signout)

	mux.HandleFunc("/", s.notFound)

	withSession := s.store.Sessions.Middleware(s.authenticate(mux), func(w http.ResponseWriter, r *http.Request, err error) {
		htmlutil.ServerErrorResponse(w, r, err, s.logger)
	})

	return s.recoverPanic(s.logRequest(withSession))
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		jsonutil.NotFoundResponse(w)
		return
	}

	htmlutil.NotFoundPage(w, r, s.logger)
}

func (s *Server) static() http.Handler {
	files := http.FileServerFS(ui.Files)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch s.cfg.Env {
		case "dev":
			w.Header().Set("Cache-Control", "no-store")
		default:
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}
