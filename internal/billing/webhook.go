package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	subscriptionEvents = "subscription."
	SignatureTolerance = 5 * time.Minute
)

type Event struct {
	ID             string
	Name           string
	UserID         int64
	Site           string
	SubscriptionID string
	CustomerID     string
	PriceID        string
	Status         string
	NextBilledAt   *time.Time
	PeriodEndsAt   *time.Time
	CancelAt       *time.Time
	OccurredAt     time.Time
}

func Signed(secret string, body []byte, header string, now time.Time) bool {
	var ts string
	var signatures []string
	for _, part := range strings.Split(header, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch key {
		case "ts":
			ts = value
		case "h1":
			signatures = append(signatures, value)
		}
	}
	if ts == "" || len(signatures) == 0 {
		return false
	}

	seconds, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return false
	}
	age := now.Sub(time.Unix(seconds, 0))
	if age > SignatureTolerance || age < -SignatureTolerance {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + ":"))
	mac.Write(body)
	want := mac.Sum(nil)

	for _, signature := range signatures {
		got, err := hex.DecodeString(signature)
		if err == nil && hmac.Equal(want, got) {
			return true
		}
	}
	return false
}

type payload struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	OccurredAt string `json:"occurred_at"`
	Data       struct {
		ID         string `json:"id"`
		Status     string `json:"status"`
		CustomerID string `json:"customer_id"`
		CustomData struct {
			UserID json.RawMessage `json:"user_id"`
			Site   string          `json:"site"`
		} `json:"custom_data"`
		Items []struct {
			Price struct {
				ID string `json:"id"`
			} `json:"price"`
		} `json:"items"`
		NextBilledAt         *string `json:"next_billed_at"`
		CurrentBillingPeriod *struct {
			EndsAt string `json:"ends_at"`
		} `json:"current_billing_period"`
		ScheduledChange *struct {
			Action      string `json:"action"`
			EffectiveAt string `json:"effective_at"`
		} `json:"scheduled_change"`
	} `json:"data"`
}

var ErrMalformed = errors.New("malformed webhook payload")

func ParseEvent(body []byte) (Event, bool, error) {
	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		return Event{}, false, fmt.Errorf("%w: %v", ErrMalformed, err)
	}

	if !strings.HasPrefix(p.EventType, subscriptionEvents) {
		return Event{}, false, nil
	}

	d := p.Data
	if d.ID == "" || d.Status == "" {
		return Event{}, false, fmt.Errorf("%w: a subscription without an id or a status", ErrMalformed)
	}

	occurred, err := parseTime(&p.OccurredAt)
	if err != nil || occurred == nil {
		return Event{}, false, fmt.Errorf("%w: occurred_at %q", ErrMalformed, p.OccurredAt)
	}

	e := Event{
		ID:             p.EventID,
		Name:           p.EventType,
		Site:           d.CustomData.Site,
		SubscriptionID: d.ID,
		CustomerID:     d.CustomerID,
		Status:         d.Status,
		OccurredAt:     *occurred,
	}
	if len(d.Items) > 0 {
		e.PriceID = d.Items[0].Price.ID
	}

	if e.UserID, err = userID(d.CustomData.UserID); err != nil {
		return Event{}, false, err
	}

	if e.NextBilledAt, err = parseTime(d.NextBilledAt); err != nil {
		return Event{}, false, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	if d.CurrentBillingPeriod != nil {
		if e.PeriodEndsAt, err = parseTime(&d.CurrentBillingPeriod.EndsAt); err != nil {
			return Event{}, false, fmt.Errorf("%w: %v", ErrMalformed, err)
		}
	}
	if d.ScheduledChange != nil && d.ScheduledChange.Action == "cancel" {
		if e.CancelAt, err = parseTime(&d.ScheduledChange.EffectiveAt); err != nil {
			return Event{}, false, fmt.Errorf("%w: %v", ErrMalformed, err)
		}
	}

	return e, true, nil
}

func userID(raw json.RawMessage) (int64, error) {
	text := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if text == "" || text == "null" {
		return 0, nil
	}
	id, err := strconv.ParseInt(text, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%w: custom user_id %s", ErrMalformed, raw)
	}
	return id, nil
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
