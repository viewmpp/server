package server

import (
	"net/http"
)

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", s.static())

	mux.HandleFunc("GET /{$}", s.viewerHandler.Landing)

	mux.HandleFunc("GET /robots.txt", s.robots)
	mux.HandleFunc("GET /sitemap.xml", s.sitemap)

	mux.HandleFunc("GET /api/v1/healthcheck", s.healthcheck)
	mux.HandleFunc("POST /api/v1/upload", s.uploadHandler.Upload)
	mux.HandleFunc("POST /api/v1/xlsx", s.exportHandler.XLSX)
	mux.HandleFunc("POST /api/v1/projects", s.projectHandler.Create)
	mux.HandleFunc("GET /api/v1/projects/{id}", s.projectHandler.Contract)

	mux.HandleFunc("GET /api/v1/examples/{name}", s.viewerHandler.ExampleContract)

	mux.HandleFunc("GET /mpp-to-excel", s.projectHandler.ConvertPage)
	mux.HandleFunc("GET /examples", s.viewerHandler.ExamplesPage)
	mux.HandleFunc("GET /example/{name}", s.viewerHandler.ExamplePage)

	mux.HandleFunc("GET /open-mpp-file-without-ms-project", s.viewerHandler.WithoutProjectPage)
	mux.HandleFunc("GET /mpp-viewer-mac", s.viewerHandler.MacPage)

	mux.HandleFunc("GET /share-a-project-plan", s.viewerHandler.SharePage)
	mux.HandleFunc("GET /pricing", s.viewerHandler.PricingPage)
	mux.HandleFunc("GET /privacy", s.viewerHandler.PrivacyPage)
	mux.HandleFunc("GET /terms", s.viewerHandler.TermsPage)

	mux.HandleFunc("GET /projects", s.projectHandler.List)
	mux.HandleFunc("GET /p/{id}", s.projectHandler.Page)
	mux.HandleFunc("GET /p/{id}/xlsx", s.projectHandler.Export)
	mux.HandleFunc("POST /p/{id}/unlock", s.projectHandler.Unlock)
	mux.HandleFunc("POST /p/{id}/access", s.projectHandler.SetAccess)
	mux.HandleFunc("POST /p/{id}/delete", s.projectHandler.Delete)

	mux.HandleFunc("GET /signup", s.userHandler.SignupPage)
	mux.HandleFunc("POST /signup", s.userHandler.Signup)

	mux.HandleFunc("GET /verify", s.userHandler.VerifyPage)
	mux.HandleFunc("POST /verify", s.userHandler.Verify)
	mux.HandleFunc("POST /verify/resend", s.userHandler.ResendCode)

	mux.HandleFunc("GET /reset", s.userHandler.ForgotPage)
	mux.HandleFunc("POST /reset", s.userHandler.Forgot)
	mux.HandleFunc("GET /reset/{token}", s.userHandler.ResetPage)
	mux.HandleFunc("POST /reset/{token}", s.userHandler.Reset)

	mux.HandleFunc("GET /signin", s.userHandler.SigninPage)
	mux.HandleFunc("POST /signin", s.userHandler.Signin)
	mux.HandleFunc("POST /signout", s.userHandler.Signout)

	mux.HandleFunc("GET /account", s.userHandler.AccountPage)
	mux.HandleFunc("POST /account/password", s.userHandler.ChangePassword)
	mux.HandleFunc("POST /account/delete", s.userHandler.DeleteAccount)

	mux.HandleFunc("POST /subscribe", s.userHandler.Subscribe)

	mux.HandleFunc("/", s.notFound)

	return s.recoverPanic(s.logRequest(s.noStore(s.clientIP(s.withSession(s.authenticate(mux))))))
}
