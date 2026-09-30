package postgres

import (
	"context"
	"database/sql"
	"server/internal/config"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Postgres struct {
	DB *sql.DB
}

func Open(cfg config.DB) (*Postgres, error) {
	db, err := sql.Open("pgx", cfg.DBDSN)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxIdleTime(cfg.DBMaxIdleTime)

	pg := &Postgres{DB: db}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = pg.Healthcheck(ctx)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return pg, nil
}

func (pg *Postgres) Healthcheck(ctx context.Context) error {
	err := pg.DB.PingContext(ctx)
	if err != nil {
		return err
	}

	return nil
}
