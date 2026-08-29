package store

import (
	"database/sql"
	"log/slog"
	"server/internal/config"
	"server/internal/project"
	"server/internal/session"
	"server/internal/token"
	"server/internal/user"
)

type Store struct {
	Users    *user.Store
	Tokens   *token.Store
	Sessions *session.Store
	Projects *project.Store
}

func New(db *sql.DB, cfg config.Config, logger *slog.Logger) *Store {
	return &Store{
		Users:    user.NewStore(db),
		Tokens:   token.NewStore(db),
		Sessions: session.NewStore(db, cfg.SessionLifetime, cfg.Env != "dev", cfg.SecretKey, logger),
		Projects: project.NewStore(db),
	}
}
