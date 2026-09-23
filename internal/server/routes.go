package server

import (
	"net/http"
)

func (s *Server) routes() http.Handler {
	top := http.NewServeMux()

	top.Handle("GET /static/", s.static())
	top.HandleFunc("GET /favicon.ico", s.icon("favicon.ico"))
	top.HandleFunc("GET /apple-touch-icon.png", s.icon("apple-touch-icon.png"))
	top.HandleFunc("GET /robots.txt", s.robots)
	top.HandleFunc("GET /sitemap.xml", s.sitemap)
	top.HandleFunc("GET /llms.txt", s.llms)

	top.Handle("/", s.noStore(s.sameOrigin(s.clientIP(s.withSession(s.authenticate(s.mux()))))))

	return s.securityHeaders(s.logRequest(s.recoverPanic(top)))
}

func (s *Server) mux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", s.handlers.Viewer.Landing)

	mux.HandleFunc("GET /api/v1/healthcheck", s.serverHealthcheck)
	mux.HandleFunc("POST /api/v1/upload", s.fromApp(s.handlers.Upload.Upload))
	mux.HandleFunc("POST /api/v1/xlsx", s.fromApp(s.handlers.Export.XLSX))
	mux.HandleFunc("POST /api/v1/projects", s.fromApp(s.requireAuthAPI(s.handlers.Project.Create)))
	mux.HandleFunc("GET /api/v1/projects/{id}", s.throttle(s.limits.Read, "read-ip:", s.handlers.Project.Contract))

	mux.HandleFunc("GET /api/v1/examples/{name}", s.throttle(s.limits.Read, "read-ip:", s.handlers.Viewer.ExampleContract))

	mux.HandleFunc("GET /mpp-to-excel", s.handlers.Project.ConvertPage)
	mux.HandleFunc("GET /examples", s.handlers.Viewer.ExamplesPage)
	mux.HandleFunc("GET /example/{name}", s.handlers.Viewer.ExamplePage)

	mux.HandleFunc("GET /open-mpp-file-without-ms-project", s.handlers.Viewer.WithoutProjectPage)
	mux.HandleFunc("GET /open-mpp-file-on-mac", s.handlers.Viewer.MacPage)
	mux.HandleFunc("GET /open-xer-file-without-primavera", s.handlers.Viewer.XERPage)
	mux.HandleFunc("GET /open-microsoft-project-xml", s.handlers.Viewer.XMLPage)
	mux.HandleFunc("GET /mpp-viewer-mac", s.moved("/open-mpp-file-on-mac"))

	mux.HandleFunc("GET /share-a-project-plan", s.handlers.Viewer.SharePage)
	mux.HandleFunc("GET /pricing", s.handlers.Viewer.PricingPage)
	mux.HandleFunc("GET /cookies", s.handlers.Viewer.CookiesPage)
	mux.HandleFunc("GET /privacy", s.handlers.Viewer.PrivacyPage)
	mux.HandleFunc("GET /terms", s.handlers.Viewer.TermsPage)

	mux.HandleFunc("GET /projects", s.requireAuthUser(s.handlers.Project.List))
	mux.HandleFunc("GET /p/{id}", s.throttle(s.limits.Read, "page-ip:", s.handlers.Project.Page))
	mux.HandleFunc("GET /p/{id}/xlsx", s.throttle(s.limits.Export, "export-ip:", s.handlers.Project.Export))
	mux.HandleFunc("POST /p/{id}/unlock", s.handlers.Project.Unlock)
	mux.HandleFunc("POST /p/{id}/access", s.requireAuthUser(s.handlers.Project.SetAccess))
	mux.HandleFunc("POST /p/{id}/delete", s.requireAuthUser(s.handlers.Project.Delete))

	mux.HandleFunc("GET /signup", s.requireAnonymousUser(s.handlers.User.SignupPage))
	mux.HandleFunc("POST /signup", s.requireAnonymousUser(s.handlers.User.Signup))

	mux.HandleFunc("GET /verify", s.requireAuthUser(s.handlers.User.VerifyPage))
	mux.HandleFunc("POST /verify", s.requireAuthUser(s.handlers.User.Verify))
	mux.HandleFunc("POST /verify/resend", s.requireAuthUser(s.handlers.User.ResendCode))

	mux.HandleFunc("GET /reset", s.handlers.User.ForgotPage)
	mux.HandleFunc("POST /reset", s.handlers.User.Forgot)
	mux.HandleFunc("GET /reset/{token}", s.handlers.User.ResetPage)
	mux.HandleFunc("POST /reset/{token}", s.handlers.User.Reset)

	mux.HandleFunc("GET /signin", s.requireAnonymousUser(s.handlers.User.SigninPage))
	mux.HandleFunc("POST /signin", s.requireAnonymousUser(s.handlers.User.Signin))
	mux.HandleFunc("POST /signout", s.handlers.User.Signout)

	mux.HandleFunc("GET /account", s.requireAuthUser(s.handlers.User.AccountPage))
	mux.HandleFunc("GET /account/password", s.redirect("/account"))
	mux.HandleFunc("POST /account/password", s.requireAuthUser(s.handlers.User.ChangePassword))
	mux.HandleFunc("GET /account/delete", s.redirect("/account"))
	mux.HandleFunc("POST /account/delete", s.requireAuthUser(s.handlers.User.DeleteAccount))

	mux.HandleFunc("POST /subscribe", s.requireAuthUser(s.handlers.User.Subscribe))

	mux.HandleFunc("/", s.notFound)

	return mux
}
