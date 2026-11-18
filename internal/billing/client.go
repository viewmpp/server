package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	LiveAPI     = "https://api.paddle.com"
	SandboxAPI  = "https://sandbox-api.paddle.com"
	apiVersion  = "1"
	apiTimeout  = 10 * time.Second
	maxReply    = 1 << 20
	maxExcerpt  = 300
	immediately = "immediately"
	periodEnd   = "next_billing_period"
)

type Client struct {
	URL  string
	Key  string
	HTTP *http.Client
}

func NewClient(key string, sandbox bool) *Client {
	base := LiveAPI
	if sandbox {
		base = SandboxAPI
	}
	return &Client{URL: base, Key: key, HTTP: &http.Client{Timeout: apiTimeout}}
}

func (c *Client) PortalURL(ctx context.Context, customerID, subscriptionID string) (string, error) {
	body, err := json.Marshal(map[string][]string{"subscription_ids": {subscriptionID}})
	if err != nil {
		return "", err
	}

	var reply struct {
		Data struct {
			URLs struct {
				General struct {
					Overview string `json:"overview"`
				} `json:"general"`
			} `json:"urls"`
		} `json:"data"`
	}
	path := "/customers/" + url.PathEscape(customerID) + "/portal-sessions"
	if err = c.call(ctx, http.MethodPost, path, body, &reply); err != nil {
		return "", fmt.Errorf("create portal session: %w", err)
	}

	return checkedURL(reply.Data.URLs.General.Overview)
}

func (c *Client) Cancel(ctx context.Context, subscriptionID string, now bool) error {
	when := periodEnd
	if now {
		when = immediately
	}
	body, err := json.Marshal(map[string]string{"effective_from": when})
	if err != nil {
		return err
	}

	var reply json.RawMessage
	err = c.call(ctx, http.MethodPost, "/subscriptions/"+url.PathEscape(subscriptionID)+"/cancel", body, &reply)

	var status *StatusError
	if errors.As(err, &status) && status.Status == http.StatusNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cancel subscription: %w", err)
	}
	return nil
}

var ErrNoURL = errors.New("the billing API answered without a usable link")

func checkedURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", ErrNoURL
	}
	return raw, nil
}

type StatusError struct {
	Status int
	Body   string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("billing API answered %d: %s", e.Status, e.Body)
}

func (c *Client) call(ctx context.Context, method, path string, body []byte, into any) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.URL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Paddle-Version", apiVersion)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxReply))
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		excerpt := raw
		if len(excerpt) > maxExcerpt {
			excerpt = excerpt[:maxExcerpt]
		}
		return &StatusError{Status: resp.StatusCode, Body: string(excerpt)}
	}

	return json.Unmarshal(raw, into)
}
