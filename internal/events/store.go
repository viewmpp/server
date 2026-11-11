package events

import (
	"context"
	"database/sql"
	"time"
)

type Total struct {
	Day   string `json:"day"`
	Name  Name   `json:"name"`
	Total int64  `json:"total"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Add(ctx context.Context, day string, name Name, total int64) error {
	query := `
		INSERT INTO event_counts (day, name, total) VALUES ($1, $2, $3)
		ON CONFLICT (day, name) DO UPDATE SET total = event_counts.total + EXCLUDED.total`

	_, err := s.db.ExecContext(ctx, query, day, string(name), total)

	return err
}

func (s *Store) Totals(ctx context.Context, since time.Time) ([]Total, error) {
	query := `
		SELECT day::text, name, total FROM event_counts
		WHERE day >= $1
		ORDER BY day DESC, name`

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, since.UTC().Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	totals := make([]Total, 0)

	for rows.Next() {
		var t Total
		if err = rows.Scan(&t.Day, &t.Name, &t.Total); err != nil {
			return nil, err
		}
		totals = append(totals, t)
	}

	return totals, rows.Err()
}
