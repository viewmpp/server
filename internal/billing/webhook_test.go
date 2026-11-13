package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"
)

const testSecret = "webhook-test-secret"

func sign(body string) string {
	mac := hmac.New(sha256.New, []byte(testSecret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func subscriptionBody(event, status string, fields map[string]string) string {
	values := map[string]string{
		"user_id":       `"42"`,
		"store_id":      `513638`,
		"variant_id":    `2370696`,
		"renews_at":     `"2026-12-13T10:00:00.000000Z"`,
		"ends_at":       `null`,
		"trial_ends_at": `null`,
		"updated_at":    `"2026-11-13T10:00:00.000000Z"`,
		"test_mode":     `true`,
	}
	for k, v := range fields {
		values[k] = v
	}

	return `{
	  "meta": {"event_name": "` + event + `", "custom_data": {"user_id": ` + values["user_id"] + `}},
	  "data": {
	    "type": "subscriptions",
	    "id": "901",
	    "attributes": {
	      "store_id": ` + values["store_id"] + `,
	      "customer_id": 77,
	      "order_id": 5,
	      "product_id": 3,
	      "variant_id": ` + values["variant_id"] + `,
	      "user_email": "buyer@example.com",
	      "status": "` + status + `",
	      "trial_ends_at": ` + values["trial_ends_at"] + `,
	      "renews_at": ` + values["renews_at"] + `,
	      "ends_at": ` + values["ends_at"] + `,
	      "created_at": "2026-11-13T09:59:00.000000Z",
	      "updated_at": ` + values["updated_at"] + `,
	      "test_mode": ` + values["test_mode"] + `,
	      "urls": {"customer_portal": "https://viewmpp.lemonsqueezy.com/billing?expires=1"}
	    }
	  }
	}`
}

func TestOnlyABodySignedWithOurSecretIsAccepted(t *testing.T) {
	body := subscriptionBody("subscription_created", "active", nil)

	if !Signed(testSecret, []byte(body), sign(body)) {
		t.Fatal("a body signed with our secret was refused")
	}

	tests := map[string]string{
		"no signature":              "",
		"not hex":                   "not-a-signature",
		"signed by someone else":    hex.EncodeToString(hmac.New(sha256.New, []byte("other")).Sum(nil)),
		"cut short":                 sign(body)[:20],
		"signature of another body": sign(strings.Replace(body, `"active"`, `"expired"`, 1)),
	}
	for name, signature := range tests {
		t.Run(name, func(t *testing.T) {
			if Signed(testSecret, []byte(body), signature) {
				t.Error("an unsigned or forged body was accepted; anyone could grant themselves Pro")
			}
		})
	}
}

func TestASubscriptionPayloadIsReadWithNumericIDs(t *testing.T) {
	e, ok, err := ParseEvent([]byte(subscriptionBody("subscription_created", "active", nil)))
	if err != nil || !ok {
		t.Fatalf("parse = %v, %v", ok, err)
	}

	want := Event{
		Name:           "subscription_created",
		UserID:         42,
		SubscriptionID: "901",
		CustomerID:     "77",
		StoreID:        "513638",
		VariantID:      "2370696",
		Status:         "active",
		UpdatedAt:      time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC),
		TestMode:       true,
	}
	renews := time.Date(2026, 12, 13, 10, 0, 0, 0, time.UTC)

	if e.RenewsAt == nil || !e.RenewsAt.Equal(renews) {
		t.Errorf("renews_at = %v, want %v", e.RenewsAt, renews)
	}
	if e.EndsAt != nil || e.TrialEndsAt != nil {
		t.Errorf("null dates were read as %v and %v", e.EndsAt, e.TrialEndsAt)
	}
	e.RenewsAt = nil
	if e != want {
		t.Errorf("event = %+v, want %+v", e, want)
	}
}

func TestAUserIDSentAsANumberIsReadToo(t *testing.T) {
	e, _, err := ParseEvent([]byte(subscriptionBody("subscription_created", "active", map[string]string{"user_id": "42"})))
	if err != nil || e.UserID != 42 {
		t.Fatalf("user = %d, err = %v", e.UserID, err)
	}
}

func TestARenewalWithoutCustomDataHasNoUser(t *testing.T) {
	e, ok, err := ParseEvent([]byte(subscriptionBody("subscription_updated", "active", map[string]string{"user_id": "null"})))
	if err != nil || !ok {
		t.Fatalf("parse = %v, %v", ok, err)
	}
	if e.UserID != 0 {
		t.Errorf("user = %d, want 0 so the store finds it by subscription", e.UserID)
	}
}

func TestEventsAboutOtherObjectsAreSkipped(t *testing.T) {
	body := `{"meta":{"event_name":"subscription_payment_success"},"data":{"type":"subscription-invoices","id":"3","attributes":{"status":"paid"}}}`

	_, ok, err := ParseEvent([]byte(body))
	if err != nil || ok {
		t.Errorf("an invoice was taken for a subscription: ok=%v err=%v", ok, err)
	}
}

func TestABrokenPayloadIsReportedNotGuessed(t *testing.T) {
	tests := map[string]string{
		"not json":             `{"meta":`,
		"no updated_at":        subscriptionBody("subscription_updated", "active", map[string]string{"updated_at": "null"}),
		"a date we can't read": subscriptionBody("subscription_updated", "active", map[string]string{"renews_at": `"next tuesday"`}),
		"a user id of words":   subscriptionBody("subscription_created", "active", map[string]string{"user_id": `"admin"`}),
		"a negative user id":   subscriptionBody("subscription_created", "active", map[string]string{"user_id": `"-4"`}),
		"no status":            subscriptionBody("subscription_created", "", nil),
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, err := ParseEvent([]byte(body)); !errors.Is(err, ErrMalformed) {
				t.Errorf("err = %v, want ErrMalformed", err)
			}
		})
	}
}
