package user

import (
	"context"
	"log/slog"
	"server/internal/htmlutil"
	"server/internal/mailer"
	"server/internal/ratelimit"
	"server/internal/session"
	"server/internal/token"
	"sync"
	"time"
)

type projectCounts interface {
	CountByUserID(ctx context.Context, userID int64) (int, error)
	CountShared(ctx context.Context, userID int64) (int, error)
}

type Handler struct {
	store            *Store
	projects         projectCounts
	token            *token.Store
	sessions         *session.Store
	limiter          *ratelimit.Limiter
	mailer           *mailer.Mailer
	verificationTTL  time.Duration
	verificationRC   time.Duration
	resetTTL         time.Duration
	baseURL          string
	earlyAccessSeats int
	templates        *htmlutil.Templates
	wg               *sync.WaitGroup
	logger           *slog.Logger
}

func NewHandler(
	store *Store,
	projects projectCounts,
	token *token.Store,
	sessions *session.Store,
	limiter *ratelimit.Limiter,
	mailer *mailer.Mailer,
	vttl time.Duration,
	vrc time.Duration,
	resetTTL time.Duration,
	baseURL string,
	earlyAccessSeats int,
	templates *htmlutil.Templates,
	wg *sync.WaitGroup,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		store:            store,
		projects:         projects,
		token:            token,
		sessions:         sessions,
		limiter:          limiter,
		mailer:           mailer,
		verificationTTL:  vttl,
		verificationRC:   vrc,
		resetTTL:         resetTTL,
		baseURL:          baseURL,
		earlyAccessSeats: earlyAccessSeats,
		templates:        templates,
		wg:               wg,
		logger:           logger,
	}
}
