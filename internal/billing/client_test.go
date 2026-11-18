package billing

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func fakePaddle(t *testing.T, status int, reply string, seen *map[string]any) *Client {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer pdl_sdbx_apikey_test" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Paddle-Version") != "1" {
			t.Errorf("Paddle-Version = %q, want the pinned 1", r.Header.Get("Paddle-Version"))
		}

		got := map[string]any{"method": r.Method, "path": r.URL.Path}
		if body, _ := io.ReadAll(r.Body); len(body) > 0 {
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("content type = %q", r.Header.Get("Content-Type"))
			}
			var decoded any
			if err := json.Unmarshal(body, &decoded); err != nil {
				t.Errorf("request body is not json: %s", body)
			}
			got["body"] = decoded
		}
		if seen != nil {
			*seen = got
		}

		w.WriteHeader(status)
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)

	c := NewClient("pdl_sdbx_apikey_test", true)
	c.URL = srv.URL
	return c
}

func TestTheClientTalksToTheRightEnvironment(t *testing.T) {
	if NewClient("k", true).URL != SandboxAPI || NewClient("k", false).URL != LiveAPI {
		t.Error("sandbox and live keys only work against their own API")
	}
}

func TestThePortalLinkIsAskedForTheCustomerAndSubscription(t *testing.T) {
	var seen map[string]any
	c := fakePaddle(t, http.StatusCreated,
		`{"data":{"id":"cpls_01","urls":{"general":{"overview":"https://customer-portal.paddle.com/cpl_01?action=overview&token=t"},"subscriptions":[]}}}`, &seen)

	to, err := c.PortalURL(context.Background(), "ctm_01", "sub_01")
	if err != nil {
		t.Fatal(err)
	}
	if seen["method"] != http.MethodPost || seen["path"] != "/customers/ctm_01/portal-sessions" {
		t.Errorf("called %v %v", seen["method"], seen["path"])
	}
	ids, _ := seen["body"].(map[string]any)["subscription_ids"].([]any)
	if len(ids) != 1 || ids[0] != "sub_01" {
		t.Errorf("subscription ids = %v", ids)
	}
	if to != "https://customer-portal.paddle.com/cpl_01?action=overview&token=t" {
		t.Errorf("url = %q", to)
	}
}

func TestCancellingAsksForTheEndOfThePeriodOrNow(t *testing.T) {
	for now, want := range map[bool]string{false: "next_billing_period", true: "immediately"} {
		var seen map[string]any
		c := fakePaddle(t, http.StatusOK, `{"data":{"id":"sub_01","status":"active"}}`, &seen)

		if err := c.Cancel(context.Background(), "sub_01", now); err != nil {
			t.Fatal(err)
		}
		if seen["method"] != http.MethodPost || seen["path"] != "/subscriptions/sub_01/cancel" {
			t.Errorf("called %v %v", seen["method"], seen["path"])
		}
		if got := seen["body"].(map[string]any)["effective_from"]; got != want {
			t.Errorf("effective_from = %v, want %s", got, want)
		}
	}
}

func TestCancellingASubscriptionPaddleNoLongerHasIsDone(t *testing.T) {
	c := fakePaddle(t, http.StatusNotFound, `{"error":{"type":"request_error","code":"not_found"}}`, nil)
	if err := c.Cancel(context.Background(), "sub_01", false); err != nil {
		t.Errorf("err = %v, want nil", err)
	}

	down := fakePaddle(t, http.StatusServiceUnavailable, `oops`, nil)
	if err := down.Cancel(context.Background(), "sub_01", false); err == nil {
		t.Error("a failed cancellation was reported as done")
	}
}

func TestAnAPIRefusalIsAnError(t *testing.T) {
	c := fakePaddle(t, http.StatusForbidden, `{"error":{"type":"request_error","code":"forbidden"}}`, nil)

	_, err := c.PortalURL(context.Background(), "ctm_01", "sub_01")
	var status *StatusError
	if !errors.As(err, &status) || status.Status != http.StatusForbidden {
		t.Errorf("err = %v", err)
	}
}

func TestOnlyAnHTTPSLinkIsFollowed(t *testing.T) {
	for _, reply := range []string{
		`{"data":{"urls":{"general":{"overview":""}}}}`,
		`{"data":{"urls":{"general":{"overview":"http://customer-portal.paddle.com/x"}}}}`,
		`{"data":{"urls":{"general":{"overview":"javascript:alert(1)"}}}}`,
	} {
		c := fakePaddle(t, http.StatusCreated, reply, nil)
		if _, err := c.PortalURL(context.Background(), "ctm_01", "sub_01"); !errors.Is(err, ErrNoURL) {
			t.Errorf("reply %s: err = %v, want ErrNoURL", reply, err)
		}
	}
}
