package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"server/internal/contract"
	"strings"
	"time"
)

type ParseError struct {
	Status  int
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parser returned %d: %s", e.Status, e.Code)
}

type Client struct {
	URL  string
	HTTP *http.Client
}

func (c *Client) endpoint(path string) string {
	return strings.TrimSuffix(c.URL, "/") + path
}

const healthcheckURL = "/healthcheck"

const parseURL = "/parse"

const maxHealthcheckBytes = 4 << 10

const healthyStatus = "OK"

func (c *Client) Healthcheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint(healthcheckURL), nil)
	if err != nil {
		return err
	}

	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}

	payload := res.Body
	defer func() {
		_ = payload.Close()
	}()

	data, err := io.ReadAll(io.LimitReader(payload, maxHealthcheckBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxHealthcheckBytes {
		return fmt.Errorf("parser healthcheck response exceeds %d bytes", maxHealthcheckBytes)
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("parser healthcheck returned %d", res.StatusCode)
	}

	phc := struct {
		Status  string `json:"status"`
		Env     string `json:"env"`
		Version string `json:"version"`
	}{}

	if err = json.Unmarshal(data, &phc); err != nil {
		return fmt.Errorf("parser healthcheck is not the expected json: %w", err)
	}

	if phc.Status != healthyStatus {
		return fmt.Errorf("parser reports status %q", phc.Status)
	}

	return nil
}

func (c *Client) Parse(ctx context.Context, body io.Reader, size int64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	res, err := c.request(ctx, body, size)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = res.Body.Close()
	}()

	data, err := io.ReadAll(io.LimitReader(res.Body, contract.MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > contract.MaxBytes {
		return nil, fmt.Errorf("parser response exceeds %d bytes", contract.MaxBytes)
	}
	if res.StatusCode != http.StatusOK {
		pe := &ParseError{Status: res.StatusCode}
		if err = json.Unmarshal(data, pe); err != nil {
			pe.Message = string(data)
		}
		return nil, pe
	}
	return data, nil
}

func (c *Client) request(ctx context.Context, body io.Reader, size int64) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(parseURL), body)
	if err != nil {
		return nil, err
	}

	req.ContentLength = size
	req.Header.Set("Content-Type", "application/octet-stream")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}
