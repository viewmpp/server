package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func (c *Client) Parse(ctx context.Context, body io.Reader) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	res, err := c.request(ctx, body)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = res.Body.Close()
	}()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		pe := &ParseError{Status: res.StatusCode}
		if err := json.Unmarshal(data, pe); err != nil {
			pe.Message = string(data)
		}
		return nil, pe
	}
	return data, nil
}

func (c *Client) request(ctx context.Context, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}
