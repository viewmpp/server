package billing

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

const subscriptionType = "subscriptions"

type Event struct {
	Name           string
	UserID         int64
	Site           string
	SubscriptionID string
	CustomerID     string
	StoreID        string
	VariantID      string
	Status         string
	RenewsAt       *time.Time
	EndsAt         *time.Time
	TrialEndsAt    *time.Time
	UpdatedAt      time.Time
	TestMode       bool
}

func Signed(secret string, body []byte, signature string) bool {
	got, err := hex.DecodeString(signature)
	if err != nil || len(got) == 0 {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)

	return hmac.Equal(mac.Sum(nil), got)
}

type id string

func (v *id) UnmarshalJSON(raw []byte) error {
	raw = bytes.TrimSpace(raw)
	if bytes.Equal(raw, []byte("null")) {
		*v = ""
		return nil
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		*v = id(s)
		return nil
	}

	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		return fmt.Errorf("an id is neither a string nor a number: %s", raw)
	}
	*v = id(n.String())
	return nil
}

type payload struct {
	Meta struct {
		EventName  string `json:"event_name"`
		CustomData struct {
			UserID id     `json:"user_id"`
			Site   string `json:"site"`
		} `json:"custom_data"`
	} `json:"meta"`
	Data struct {
		Type       string `json:"type"`
		ID         id     `json:"id"`
		Attributes struct {
			StoreID     id      `json:"store_id"`
			CustomerID  id      `json:"customer_id"`
			VariantID   id      `json:"variant_id"`
			Status      string  `json:"status"`
			RenewsAt    *string `json:"renews_at"`
			EndsAt      *string `json:"ends_at"`
			TrialEndsAt *string `json:"trial_ends_at"`
			UpdatedAt   string  `json:"updated_at"`
			TestMode    bool    `json:"test_mode"`
		} `json:"attributes"`
	} `json:"data"`
}

var ErrMalformed = errors.New("malformed webhook payload")

func ParseEvent(body []byte) (Event, bool, error) {
	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		return Event{}, false, fmt.Errorf("%w: %v", ErrMalformed, err)
	}

	if p.Data.Type != subscriptionType {
		return Event{}, false, nil
	}

	a := p.Data.Attributes
	e := Event{
		Name:           p.Meta.EventName,
		Site:           p.Meta.CustomData.Site,
		SubscriptionID: string(p.Data.ID),
		CustomerID:     string(a.CustomerID),
		StoreID:        string(a.StoreID),
		VariantID:      string(a.VariantID),
		Status:         a.Status,
		TestMode:       a.TestMode,
	}

	if e.SubscriptionID == "" || e.Status == "" {
		return Event{}, false, fmt.Errorf("%w: a subscription without an id or a status", ErrMalformed)
	}

	if raw := string(p.Meta.CustomData.UserID); raw != "" {
		userID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || userID <= 0 {
			return Event{}, false, fmt.Errorf("%w: custom user_id %q", ErrMalformed, raw)
		}
		e.UserID = userID
	}

	updated, err := parseTime(&a.UpdatedAt)
	if err != nil || updated == nil {
		return Event{}, false, fmt.Errorf("%w: updated_at %q", ErrMalformed, a.UpdatedAt)
	}
	e.UpdatedAt = *updated

	for _, field := range []struct {
		raw *string
		to  **time.Time
	}{{a.RenewsAt, &e.RenewsAt}, {a.EndsAt, &e.EndsAt}, {a.TrialEndsAt, &e.TrialEndsAt}} {
		t, err := parseTime(field.raw)
		if err != nil {
			return Event{}, false, fmt.Errorf("%w: %v", ErrMalformed, err)
		}
		*field.to = t
	}

	return e, true, nil
}

func parseTime(raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339Nano, *raw)
	if err != nil {
		return nil, fmt.Errorf("a date that is not RFC 3339: %q", *raw)
	}
	t = t.UTC()
	return &t, nil
}
