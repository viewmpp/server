package user

import (
	"log/slog"
	"server/internal/htmlutil"
	"server/internal/mailer"
	"server/internal/ratelimit"
	"server/internal/session"
	"server/internal/token"
	"sync"
	"time"
)

type Handler struct {
	store           *Store
	token           *token.Store
	sessions        *session.Store
	limiter         *ratelimit.Limiter
	mailer          *mailer.Mailer
	verificationTTL time.Duration
	verificationRC  time.Duration
	templates       *htmlutil.Templates
	wg              *sync.WaitGroup
	logger          *slog.Logger
}

func NewHandler(
	store *Store,
	token *token.Store,
	sessions *session.Store,
	limiter *ratelimit.Limiter,
	mailer *mailer.Mailer,
	vttl time.Duration,
	vrc time.Duration,
	templates *htmlutil.Templates,
	wg *sync.WaitGroup,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		store:           store,
		token:           token,
		sessions:        sessions,
		limiter:         limiter,
		mailer:          mailer,
		verificationTTL: vttl,
		verificationRC:  vrc,
		templates:       templates,
		wg:              wg,
		logger:          logger,
	}
}
