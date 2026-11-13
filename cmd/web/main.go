package main

import (
	"expvar"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"server/internal/background"
	"server/internal/billing"
	"server/internal/clientip"
	"server/internal/config"
	"server/internal/diagnostics"
	"server/internal/events"
	"server/internal/export"
	"server/internal/htmlutil"
	"server/internal/mailer"
	"server/internal/parser"
	"server/internal/postgres"
	"server/internal/project"
	"server/internal/ratelimit"
	"server/internal/server"
	"server/internal/store"
	"server/internal/upload"
	"server/internal/user"
	"server/internal/vcs"
	"server/internal/viewer"
	"sync"
	"time"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()

	if err := cfg.Validate(); err != nil {
		return err
	}

	var wg sync.WaitGroup

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.Level()}))

	expvar.NewString("version").Set(vcs.Version)

	client := &parser.Client{
		URL:  cfg.ParserURL,
		HTTP: &http.Client{Timeout: cfg.ClientTimeout},
	}

	templates, err := htmlutil.NewTemplates()
	if err != nil {
		return err
	}

	pg, err := postgres.Open(cfg.DB)
	if err != nil {
		return err
	}
	defer pg.DB.Close()

	expvar.Publish("database_stats", expvar.Func(func() any {
		return pg.DB.Stats()
	}))

	eventStore := events.NewStore(pg.DB)

	diag := diagnostics.New(cfg.DiagAddr, diagnostics.NewHandler(eventStore, logger), logger)

	go diag.ListenAndServe()

	htmlutil.SetBaseURL(cfg.AppBaseURL)

	resolver, err := clientip.NewResolver(cfg.AppProxies)
	if err != nil {
		return err
	}

	uploadLimiter := ratelimit.New(cfg.UploadLimit, cfg.UploadWindow)
	defer uploadLimiter.Close()

	projectLimiter := ratelimit.New(cfg.ProjectLimit, cfg.ProjectWindow)
	defer projectLimiter.Close()

	readLimiter := ratelimit.New(cfg.ReadLimit, cfg.ReadWindow)
	defer readLimiter.Close()

	exportLimiter := ratelimit.New(cfg.ExportLimit, cfg.ExportWindow)
	defer exportLimiter.Close()

	addressLimiter := ratelimit.New(cfg.AddressLimit, cfg.AddressWindow)
	defer addressLimiter.Close()

	userLimiter := ratelimit.New(cfg.UserLimit, cfg.UserWindow)
	defer userLimiter.Close()

	str := store.New(pg.DB, cfg, logger)

	stop := make(chan struct{})
	defer close(stop)

	counts := events.NewRecorder(eventStore, logger)
	counts.Start(stop)
	defer counts.Close()

	background.Sweep(stop, logger, "sessions", cfg.BGSweepRepetition, cfg.BGSweepTimeout, str.Sessions.DeleteExpired)
	background.Sweep(stop, logger, "tokens", cfg.BGSweepRepetition, cfg.BGSweepTimeout, str.Tokens.DeleteExpired)
	background.Sweep(stop, logger, "protected links", cfg.BGSweepRepetition, cfg.BGSweepTimeout, str.Projects.DemoteExpiredProtected)

	mail := mailer.New(cfg.Resend, cfg.AppBaseURL, templates, logger, cfg.AppEnv == "prod")

	var billingHandler *billing.Handler
	var accounts user.Billing
	if cfg.Billing.Enabled() {
		billingHandler = billing.NewHandler(
			billing.NewStore(pg.DB),
			billing.NewClient(cfg.BillingAPIKey, cfg.BillingStoreID, cfg.BillingTestMode),
			billing.Settings{
				Secret:    cfg.BillingWebhookSecret,
				StoreID:   cfg.BillingStoreID,
				Monthly:   cfg.BillingVariantMonthly,
				Yearly:    cfg.BillingVariantYearly,
				TestMode:  cfg.BillingTestMode,
				TestUsers: cfg.BillingTestUsers,
				BaseURL:   cfg.AppBaseURL,
			},
			counts, logger)
		accounts = billingHandler

		user.OfferBilling(billingHandler.Offered, cfg.BillingTrialPeriod)

		only := []int64(nil)
		if cfg.BillingTestMode {
			only = append([]int64{}, cfg.BillingTestUsers...)
		}
		reminders := user.NewReminders(str.Users, mail, only, counts, logger)
		background.Sweep(stop, logger, "trial reminders", time.Hour, cfg.BGSweepTimeout, reminders.Send)

		logger.Info("billing on", "test_mode", cfg.BillingTestMode, "test_users", len(cfg.BillingTestUsers))
	}

	viewerHandler := viewer.NewHandler(templates, cfg.AppBaseURL, str.Users, counts, logger)
	uploadHandler := upload.NewHandler(client, str.Projects, uploadLimiter, counts, logger)
	exportHandler := export.NewHandler(exportLimiter, counts, logger)
	projectHandler := project.NewHandler(str.Projects, cfg.AppBaseURL, projectLimiter, cfg.ProjLenListLimit, templates, counts, logger)
	userHandler := user.NewHandler(str.Users, str.Projects, str.Tokens, str.Sessions, userLimiter, mail,
		cfg.MailerVerificationTTL, cfg.MailerVerificationRC, cfg.ResetTTL, cfg.AppBaseURL, cfg.AppEarlyAccessSeats, cfg.AppEarlyAccessPeriod,
		cfg.BillingTrialPeriod, accounts, counts, templates, &wg, logger)

	noticeLimiter := ratelimit.New(1, server.ThrottleNoticeWindow)
	defer noticeLimiter.Close()

	services := server.NewServices(cfg, client, pg, str, diag, logger)

	limits := server.NewLimits(resolver, readLimiter, exportLimiter, addressLimiter, noticeLimiter)

	handlers := server.NewHandlers(viewerHandler, uploadHandler, exportHandler, projectHandler, userHandler, billingHandler)

	return server.New(services, limits, handlers, &wg).Serve()
}
