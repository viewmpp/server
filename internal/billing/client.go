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
	"strconv"
	"time"
)

const (
	APIURL     = "https://api.lemonsqueezy.com/v1"
	apiType    = "application/vnd.api+json"
	apiTimeout = 10 * time.Second
	maxReply   = 1 << 20
)

type Client struct {
	URL      string
	Key      string
	StoreID  string
	TestMode bool
	HTTP     *http.Client
}

func NewClient(key, storeID string, testMode bool) *Client {
	return &Client{
		URL:      APIURL,
		Key:      key,
		StoreID:  storeID,
		TestMode: testMode,
		HTTP:     &http.Client{Timeout: apiTimeout},
	}
}

type Checkout struct {
	VariantID  string
	UserID     int64
	Email      string
	Site       string
	RedirectTo string
}

type relation struct {
	Data struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	} `json:"data"`
}

func related(kind, id string) relation {
	var r relation
	r.Data.Type = kind
	r.Data.ID = id
	return r
}

type checkoutRequest struct {
	Data struct {
		Type       string `json:"type"`
		Attributes struct {
			CheckoutData struct {
				Email  string            `json:"email,omitempty"`
				Custom map[string]string `json:"custom"`
			} `json:"checkout_data"`
			ProductOptions struct {
				RedirectURL     string   `json:"redirect_url,omitempty"`
				EnabledVariants []string `json:"enabled_variants"`
			} `json:"product_options"`
			TestMode bool `json:"test_mode"`
		} `json:"attributes"`
		Relationships struct {
			Store   relation `json:"store"`
			Variant relation `json:"variant"`
		} `json:"relationships"`
	} `json:"data"`
}

func (c *Client) CreateCheckout(ctx context.Context, ch Checkout) (string, error) {
	var req checkoutRequest
	req.Data.Type = "checkouts"
	req.Data.Attributes.CheckoutData.Email = ch.Email
	req.Data.Attributes.CheckoutData.Custom = map[string]string{"user_id": strconv.FormatInt(ch.UserID, 10)}
	if ch.Site != "" {
		req.Data.Attributes.CheckoutData.Custom["site"] = ch.Site
	}
	req.Data.Attributes.ProductOptions.RedirectURL = ch.RedirectTo
	req.Data.Attributes.ProductOptions.EnabledVariants = []string{ch.VariantID}
	req.Data.Attributes.TestMode = c.TestMode
	req.Data.Relationships.Store = related("stores", c.StoreID)
	req.Data.Relationships.Variant = related("variants", ch.VariantID)

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	var reply struct {
		Data struct {
			Attributes struct {
				URL string `json:"url"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err = c.call(ctx, http.MethodPost, "/checkouts", body, &reply); err != nil {
		return "", fmt.Errorf("create checkout: %w", err)
	}

	return checkedURL(reply.Data.Attributes.URL)
}

func (c *Client) PortalURL(ctx context.Context, subscriptionID string) (string, error) {
	var reply struct {
		Data struct {
			Attributes struct {
				URLs struct {
					CustomerPortal string `json:"customer_portal"`
				} `json:"urls"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := c.call(ctx, http.MethodGet, "/subscriptions/"+url.PathEscape(subscriptionID), nil, &reply); err != nil {
		return "", fmt.Errorf("read subscription: %w", err)
	}

	return checkedURL(reply.Data.Attributes.URLs.CustomerPortal)
}

func (c *Client) Cancel(ctx context.Context, subscriptionID string) error {
	var reply struct {
		Data struct {
			Attributes struct {
				Status string `json:"status"`
			} `json:"attributes"`
		} `json:"data"`
	}
	err := c.call(ctx, http.MethodDelete, "/subscriptions/"+url.PathEscape(subscriptionID), nil, &reply)

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
	req.Header.Set("Accept", apiType)
	req.Header.Set("Authorization", "Bearer "+c.Key)
	if body != nil {
		req.Header.Set("Content-Type", apiType)
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
		if len(excerpt) > 300 {
			excerpt = excerpt[:300]
		}
		return &StatusError{Status: resp.StatusCode, Body: string(excerpt)}
	}

	return json.Unmarshal(raw, into)
}
