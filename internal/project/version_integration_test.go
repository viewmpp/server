package project

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"server/internal/examples"
	"server/internal/user"
	"testing"
	"time"
)

func hospitalVersion(t *testing.T, name string) []byte {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "diff", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func addVersion(h *Handler, publicID string, as *user.User, body []byte, query string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+publicID+"/versions?"+query, bytes.NewReader(body))
	r.SetPathValue("id", publicID)
	r = user.SetUserContext(r, as)

	w := httptest.NewRecorder()
	h.AddVersion(w, r)
	return w
}

func TestANewVersionKeepsTheLinkAndThePreviousPlan(t *testing.T) {
	db := openProjectQuotaTestDB(t)
	ctx := context.Background()
	ownerID, _ := createProjectQuotaUser(t, db, true)
	otherID, _ := createProjectQuotaUser(t, db, true)

	store := NewStore(db)
	h := NewHandler(store, "https://viewmpp.com", nil, 10, nil, discard{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	v1 := hospitalVersion(t, "hospital-v1.json")
	v2 := hospitalVersion(t, "hospital-v2.json")

	publicID, err := store.Save(ctx, ownerID, "hospital-v1.mpp", v1)
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	if w := addVersion(h, publicID, &user.User{ID: otherID}, v2, "name=hospital-v2.mpp"); w.Code != http.StatusNotFound {
		t.Errorf("someone else's new version got %d, want 404", w.Code)
	}

	office, err := examples.All()[0].Contract(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if w := addVersion(h, publicID, &user.User{ID: ownerID}, office, "name=office.xml"); w.Code != http.StatusConflict {
		t.Errorf("an unrelated plan got %d, want 409 asking to confirm", w.Code)
	}

	if w := addVersion(h, publicID, &user.User{ID: ownerID}, v2, "name=hospital-v2.mpp"); w.Code != http.StatusNoContent {
		t.Fatalf("the update got %d: %s", w.Code, w.Body.String())
	}

	p, err := store.GetByPublicID(ctx, publicID, true)
	if err != nil {
		t.Fatal(err)
	}
	if p.FileName != "hospital-v2.mpp" || p.PreviousFileName == nil || *p.PreviousFileName != "hospital-v1.mpp" || p.UpdatedAt == nil {
		t.Errorf("project = %q, previous %v, updated %v; want v2 current and v1 kept", p.FileName, p.PreviousFileName, p.UpdatedAt)
	}
	if !bytes.Equal(p.Contract, v2) {
		t.Error("the link does not serve the new version")
	}

	previous, err := store.PreviousContract(ctx, publicID)
	if err != nil || !bytes.Equal(previous, v1) {
		t.Errorf("previous contract err=%v, equal to v1=%v", err, bytes.Equal(previous, v1))
	}

	if w := addVersion(h, publicID, &user.User{ID: ownerID}, office, "name=office.xml&force=1"); w.Code != http.StatusNoContent {
		t.Errorf("a confirmed replacement got %d, want 204", w.Code)
	}
}
