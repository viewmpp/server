package project

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"server/internal/user"
	"testing"
)

func TestTheListHoldsOnlyTheUsersOwnPlans(t *testing.T) {
	db := openProjectQuotaTestDB(t)

	ownerID, ownerSuffix := createProjectQuotaUser(t, db, true)
	otherID, otherSuffix := createProjectQuotaUser(t, db, true)

	insertQuotaProject(t, db, ownerID, "mine-a-"+ownerSuffix, AccessPrivate)
	insertQuotaProject(t, db, ownerID, "mine-b-"+ownerSuffix, AccessPublic)
	insertQuotaProject(t, db, otherID, "theirs-"+otherSuffix, AccessPublic)

	h := NewHandler(NewStore(db), "https://viewmpp.com", nil, 10, nil, discard{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	r = user.SetUserContext(r, &user.User{ID: ownerID})
	w := httptest.NewRecorder()

	h.Mine(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}

	var plans []savedPlan
	if err := json.Unmarshal(w.Body.Bytes(), &plans); err != nil {
		t.Fatalf("list is not JSON: %v", err)
	}

	seen := map[string]bool{}
	for _, p := range plans {
		seen[p.ID] = true
	}

	if !seen["mine-a-"+ownerSuffix] || !seen["mine-b-"+ownerSuffix] {
		t.Errorf("list %v is missing the user's own plans", plans)
	}
	if seen["theirs-"+otherSuffix] {
		t.Error("the list holds another user's plan")
	}
}
