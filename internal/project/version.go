package project

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"server/internal/contract"
	"server/internal/diff"
	"server/internal/events"
	"server/internal/jsonutil"
	"server/internal/user"
	"time"
)

const (
	MsgDifferentPlan = "this file shares few tasks with the current plan"

	minSharedTasks = 1.0 / 3
)

func (s *Store) ReplaceContract(ctx context.Context, publicID string, userID int64, fileName string, contract []byte) error {
	packed, err := compress(contract)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	res, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET previous_contract  = contract,
		    previous_file_name = file_name,
		    contract           = $3,
		    file_name          = $4,
		    updated_at         = now()
		WHERE public_id = $1 AND user_id = $2`,
		publicID, userID, packed, sanitizeFileName(fileName))
	if err != nil {
		return err
	}

	changed, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return ErrNotFound
	}

	return nil
}

func (s *Store) PreviousContract(ctx context.Context, publicID string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var packed []byte
	err := s.db.QueryRowContext(ctx, `SELECT previous_contract FROM projects WHERE public_id = $1`, publicID).Scan(&packed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if packed == nil {
		return nil, ErrNotFound
	}

	return decompress(packed)
}

func (h *Handler) AddVersion(w http.ResponseWriter, r *http.Request) {
	p, ok := h.findWithContract(w, r, true)
	if !ok {
		return
	}

	u := user.GetUserContext(r)
	if !owns(u, p) {
		jsonutil.NotFoundResponse(w)
		return
	}

	if r.ContentLength > contract.MaxBytes {
		jsonutil.ContentTooLargeError(w)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, contract.MaxBytes))
	if err != nil {
		jsonutil.BadRequestResponse(w, "plan is too large or could not be read")
		return
	}

	next, err := contract.Decode(body)
	if err != nil {
		h.logger.Warn("version rejected", "err", err)
		if errors.Is(err, contract.ErrLimit) {
			h.counts.Count(events.SaveTooLarge)
			jsonutil.BadRequestResponse(w, MsgTooLargeToSave)
			return
		}
		jsonutil.BadRequestResponse(w, "that is not a valid plan")
		return
	}

	if r.URL.Query().Get("force") != "1" {
		current, err := contract.Decode(p.Contract)
		if err != nil {
			jsonutil.ServerErrorResponse(w, r, err, h.logger)
			return
		}
		if diff.Overlap(current, next) < minSharedTasks {
			jsonutil.ConflictResponse(w, MsgDifferentPlan)
			return
		}
	}

	if err = h.store.ReplaceContract(r.Context(), p.PublicID, u.ID, r.URL.Query().Get("name"), body); err != nil {
		if errors.Is(err, ErrNotFound) {
			jsonutil.NotFoundResponse(w)
			return
		}
		jsonutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	h.counts.Count(events.VersionUploaded)

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Previous(w http.ResponseWriter, r *http.Request) {
	p, ok := h.find(w, r, true)
	if !ok {
		return
	}

	previous, err := h.store.PreviousContract(r.Context(), p.PublicID)
	if errors.Is(err, ErrNotFound) {
		jsonutil.NotFoundResponse(w)
		return
	}
	if err != nil {
		jsonutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(previous)
}
