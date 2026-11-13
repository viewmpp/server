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

func fakeLemonSqueezy(t *testing.T, status int, reply string, seen *map[string]any) *Client {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Accept") != apiType {
			t.Errorf("accept = %q", r.Header.Get("Accept"))
		}
		if r.Method == http.MethodPost && r.Header.Get("Content-Type") != apiType {
			t.Errorf("content type = %q", r.Header.Get("Content-Type"))
		}

		got := map[string]any{"method": r.Method, "path": r.URL.Path}
		if body, _ := io.ReadAll(r.Body); len(body) > 0 {
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

	c := NewClient("test-key", "513638", true)
	c.URL = srv.URL
	return c
}

func dig(v any, path ...string) any {
	for _, key := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[key]
	}
	return v
}

func TestACheckoutIsCreatedForOurStoreVariantAndUser(t *testing.T) {
	var seen map[string]any
	c := fakeLemonSqueezy(t, http.StatusCreated,
		`{"data":{"type":"checkouts","id":"x","attributes":{"url":"https://viewmpp.lemonsqueezy.com/checkout/custom/x?signature=s"}}}`, &seen)

	to, err := c.CreateCheckout(context.Background(), Checkout{
		VariantID: "2370717", UserID: 42, Email: "buyer@example.com", Site: "https://viewmpp.com", RedirectTo: "https://viewmpp.com/account",
	})
	if err != nil {
		t.Fatal(err)
	}
	if to != "https://viewmpp.lemonsqueezy.com/checkout/custom/x?signature=s" {
		t.Errorf("url = %q", to)
	}

	body := seen["body"]
	checks := []struct {
		path []string
		want any
	}{
		{[]string{"data", "type"}, "checkouts"},
		{[]string{"data", "attributes", "checkout_data", "email"}, "buyer@example.com"},
		{[]string{"data", "attributes", "checkout_data", "custom", "user_id"}, "42"},
		{[]string{"data", "attributes", "checkout_data", "custom", "site"}, "https://viewmpp.com"},
		{[]string{"data", "attributes", "product_options", "redirect_url"}, "https://viewmpp.com/account"},
		{[]string{"data", "attributes", "test_mode"}, true},
		{[]string{"data", "relationships", "store", "data", "type"}, "stores"},
		{[]string{"data", "relationships", "store", "data", "id"}, "513638"},
		{[]string{"data", "relationships", "variant", "data", "type"}, "variants"},
		{[]string{"data", "relationships", "variant", "data", "id"}, "2370717"},
	}
	if seen["method"] != http.MethodPost || seen["path"] != "/checkouts" {
		t.Errorf("called %v %v", seen["method"], seen["path"])
	}
	for _, check := range checks {
		if got := dig(body, check.path...); got != check.want {
			t.Errorf("%v = %v, want %v", check.path, got, check.want)
		}
	}
	variants, _ := dig(body, "data", "attributes", "product_options", "enabled_variants").([]any)
	if len(variants) != 1 || variants[0] != "2370717" {
		t.Errorf("enabled variants = %v: the buyer could switch to a plan we didn't offer", variants)
	}
}

func TestALiveClientAsksForALiveCheckout(t *testing.T) {
	var seen map[string]any
	c := fakeLemonSqueezy(t, http.StatusCreated, `{"data":{"attributes":{"url":"https://viewmpp.lemonsqueezy.com/checkout/custom/x"}}}`, &seen)
	c.TestMode = false

	if _, err := c.CreateCheckout(context.Background(), Checkout{VariantID: "1", UserID: 1}); err != nil {
		t.Fatal(err)
	}
	if got := dig(seen["body"], "data", "attributes", "test_mode"); got != false {
		t.Errorf("test_mode = %v, want an explicit false", got)
	}
}

func TestThePortalLinkIsReadFreshFromTheSubscription(t *testing.T) {
	var seen map[string]any
	c := fakeLemonSqueezy(t, http.StatusOK,
		`{"data":{"type":"subscriptions","id":"901","attributes":{"urls":{"customer_portal":"https://viewmpp.lemonsqueezy.com/billing?expires=9&signature=s"}}}}`, &seen)

	to, err := c.PortalURL(context.Background(), "901")
	if err != nil {
		t.Fatal(err)
	}
	if seen["method"] != http.MethodGet || seen["path"] != "/subscriptions/901" {
		t.Errorf("called %v %v", seen["method"], seen["path"])
	}
	if to != "https://viewmpp.lemonsqueezy.com/billing?expires=9&signature=s" {
		t.Errorf("url = %q", to)
	}
}

func TestAnAPIRefusalIsAnError(t *testing.T) {
	c := fakeLemonSqueezy(t, http.StatusUnprocessableEntity, `{"errors":[{"detail":"The variant does not belong to the store"}]}`, nil)

	_, err := c.CreateCheckout(context.Background(), Checkout{VariantID: "1", UserID: 1})
	var status *StatusError
	if !errors.As(err, &status) || status.Status != http.StatusUnprocessableEntity {
		t.Errorf("err = %v", err)
	}
}

func TestOnlyAnHTTPSLinkIsFollowed(t *testing.T) {
	for _, reply := range []string{
		`{"data":{"attributes":{"url":""}}}`,
		`{"data":{"attributes":{"url":"http://viewmpp.lemonsqueezy.com/checkout"}}}`,
		`{"data":{"attributes":{"url":"javascript:alert(1)"}}}`,
	} {
		c := fakeLemonSqueezy(t, http.StatusCreated, reply, nil)
		if _, err := c.CreateCheckout(context.Background(), Checkout{VariantID: "1", UserID: 1}); !errors.Is(err, ErrNoURL) {
			t.Errorf("reply %s: err = %v, want ErrNoURL", reply, err)
		}
	}
}

func TestCancellingDeletesTheSubscription(t *testing.T) {
	var seen map[string]any
	c := fakeLemonSqueezy(t, http.StatusOK, `{"data":{"type":"subscriptions","id":"901","attributes":{"status":"cancelled"}}}`, &seen)

	if err := c.Cancel(context.Background(), "901"); err != nil {
		t.Fatal(err)
	}
	if seen["method"] != http.MethodDelete || seen["path"] != "/subscriptions/901" {
		t.Errorf("called %v %v", seen["method"], seen["path"])
	}
}

func TestCancellingASubscriptionLemonSqueezyNoLongerHasIsDone(t *testing.T) {
	c := fakeLemonSqueezy(t, http.StatusNotFound, `{"errors":[{"status":"404"}]}`, nil)
	if err := c.Cancel(context.Background(), "901"); err != nil {
		t.Errorf("err = %v, want nil", err)
	}

	down := fakeLemonSqueezy(t, http.StatusServiceUnavailable, `oops`, nil)
	if err := down.Cancel(context.Background(), "901"); err == nil {
		t.Error("a failed cancellation was reported as done")
	}
}
