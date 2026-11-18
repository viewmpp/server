package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	testSecret  = "pdl_ntfset_test"
	testMonthly = "pri_monthly"
	testYearly  = "pri_yearly"
)

var testNow = time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

func signAt(body string, at time.Time) string {
	ts := strconv.FormatInt(at.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(testSecret))
	mac.Write([]byte(ts + ":" + body))
	return "ts=" + ts + ";h1=" + hex.EncodeToString(mac.Sum(nil))
}

func sign(body string) string {
	return signAt(body, testNow)
}

func subscriptionBody(event, status string, fields map[string]string) string {
	values := map[string]string{
		"event_id":         `"evt_01"`,
		"occurred_at":      `"2026-11-18T10:00:00.000000Z"`,
		"user_id":          `"42"`,
		"site":             `"https://viewmpp.com"`,
		"price":            `"` + testMonthly + `"`,
		"next_billed_at":   `"2026-12-18T10:00:00Z"`,
		"period":           `{"starts_at": "2026-11-18T10:00:00Z", "ends_at": "2026-12-18T10:00:00Z"}`,
		"scheduled_change": `null`,
	}
	for k, v := range fields {
		values[k] = v
	}

	return `{
	  "event_id": ` + values["event_id"] + `,
	  "event_type": "` + event + `",
	  "occurred_at": ` + values["occurred_at"] + `,
	  "notification_id": "ntf_01",
	  "data": {
	    "id": "sub_01",
	    "status": "` + status + `",
	    "customer_id": "ctm_01",
	    "custom_data": {"user_id": ` + values["user_id"] + `, "site": ` + values["site"] + `},
	    "items": [{"status": "active", "quantity": 1, "price": {"id": ` + values["price"] + `}}],
	    "next_billed_at": ` + values["next_billed_at"] + `,
	    "current_billing_period": ` + values["period"] + `,
	    "scheduled_change": ` + values["scheduled_change"] + `,
	    "updated_at": "2026-11-18T10:00:00Z"
	  }
	}`
}

func TestOnlyABodySignedWithOurSecretAndRecentlyIsAccepted(t *testing.T) {
	body := subscriptionBody("subscription.created", "active", nil)

	if !Signed(testSecret, []byte(body), sign(body), testNow) {
		t.Fatal("a body signed with our secret was refused")
	}

	other := hmac.New(sha256.New, []byte("someone else"))
	other.Write([]byte(strconv.FormatInt(testNow.Unix(), 10) + ":" + body))

	tests := map[string]string{
		"no header":               "",
		"no timestamp":            "h1=" + strings.Split(sign(body), "h1=")[1],
		"no signature":            "ts=" + strconv.FormatInt(testNow.Unix(), 10),
		"signed by someone else":  "ts=" + strconv.FormatInt(testNow.Unix(), 10) + ";h1=" + hex.EncodeToString(other.Sum(nil)),
		"signature of a new body": sign(strings.Replace(body, `"active"`, `"canceled"`, 1)),
		"replayed an hour later":  signAt(body, testNow.Add(-time.Hour)),
		"stamped in the future":   signAt(body, testNow.Add(time.Hour)),
		"not hex":                 "ts=" + strconv.FormatInt(testNow.Unix(), 10) + ";h1=zz",
	}
	for name, header := range tests {
		t.Run(name, func(t *testing.T) {
			if Signed(testSecret, []byte(body), header, testNow) {
				t.Error("an unsigned, forged or replayed body was accepted")
			}
		})
	}
}

func TestASecondSignatureDuringSecretRotationIsAccepted(t *testing.T) {
	body := subscriptionBody("subscription.created", "active", nil)
	good := strings.Split(sign(body), ";")
	header := good[0] + ";h1=00ff;" + good[1]

	if !Signed(testSecret, []byte(body), header, testNow) {
		t.Error("a header with the old and the new signature was refused")
	}
}

func TestASubscriptionPayloadIsRead(t *testing.T) {
	e, ok, err := ParseEvent([]byte(subscriptionBody("subscription.created", "active", nil)))
	if err != nil || !ok {
		t.Fatalf("parse = %v, %v", ok, err)
	}

	next := time.Date(2026, 12, 18, 10, 0, 0, 0, time.UTC)
	if e.ID != "evt_01" || e.Name != "subscription.created" || e.UserID != 42 || e.Site != "https://viewmpp.com" ||
		e.SubscriptionID != "sub_01" || e.CustomerID != "ctm_01" || e.PriceID != testMonthly || e.Status != "active" {
		t.Errorf("event = %+v", e)
	}
	if e.NextBilledAt == nil || !e.NextBilledAt.Equal(next) || e.PeriodEndsAt == nil || !e.PeriodEndsAt.Equal(next) || e.CancelAt != nil {
		t.Errorf("dates = %v %v %v", e.NextBilledAt, e.PeriodEndsAt, e.CancelAt)
	}
	if !e.OccurredAt.Equal(time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("occurred at %v", e.OccurredAt)
	}
}

func TestAScheduledCancellationIsRead(t *testing.T) {
	e, _, err := ParseEvent([]byte(subscriptionBody("subscription.updated", "active", map[string]string{
		"next_billed_at":   `null`,
		"scheduled_change": `{"action": "cancel", "effective_at": "2026-12-18T10:00:00Z", "resume_at": null}`,
	})))
	if err != nil {
		t.Fatal(err)
	}
	if e.CancelAt == nil || !e.CancelAt.Equal(time.Date(2026, 12, 18, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("cancel at %v", e.CancelAt)
	}
}

func TestAScheduledPauseIsNotACancellation(t *testing.T) {
	e, _, err := ParseEvent([]byte(subscriptionBody("subscription.updated", "active", map[string]string{
		"scheduled_change": `{"action": "pause", "effective_at": "2026-12-18T10:00:00Z", "resume_at": null}`,
	})))
	if err != nil || e.CancelAt != nil {
		t.Errorf("cancel at %v, err %v", e.CancelAt, err)
	}
}

func TestAUserIDSentAsANumberOrMissingIsRead(t *testing.T) {
	e, _, err := ParseEvent([]byte(subscriptionBody("subscription.created", "active", map[string]string{"user_id": "42"})))
	if err != nil || e.UserID != 42 {
		t.Errorf("number: user = %d, err = %v", e.UserID, err)
	}

	e, _, err = ParseEvent([]byte(subscriptionBody("subscription.updated", "active", map[string]string{"user_id": "null"})))
	if err != nil || e.UserID != 0 {
		t.Errorf("missing: user = %d, err = %v", e.UserID, err)
	}
}

func TestEventsAboutOtherEntitiesAreSkipped(t *testing.T) {
	body := `{"event_id":"evt_2","event_type":"transaction.completed","occurred_at":"2026-11-18T10:00:00Z","data":{"id":"txn_1","status":"completed"}}`

	_, ok, err := ParseEvent([]byte(body))
	if err != nil || ok {
		t.Errorf("a transaction was taken for a subscription: ok=%v err=%v", ok, err)
	}
}

func TestABrokenPayloadIsReportedNotGuessed(t *testing.T) {
	tests := map[string]string{
		"not json":             `{"event_type":`,
		"no occurred_at":       subscriptionBody("subscription.updated", "active", map[string]string{"occurred_at": `""`}),
		"a date we can't read": subscriptionBody("subscription.updated", "active", map[string]string{"next_billed_at": `"next tuesday"`}),
		"a user id of words":   subscriptionBody("subscription.created", "active", map[string]string{"user_id": `"admin"`}),
		"a negative user id":   subscriptionBody("subscription.created", "active", map[string]string{"user_id": `"-4"`}),
		"no status":            subscriptionBody("subscription.created", "", nil),
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, err := ParseEvent([]byte(body)); !errors.Is(err, ErrMalformed) {
				t.Errorf("err = %v, want ErrMalformed", err)
			}
		})
	}
}
