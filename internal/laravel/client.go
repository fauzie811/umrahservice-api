// Package laravel is a thin server-to-server client for the upstream Laravel
// app, used for work the Go API delegates rather than reproduces.
package laravel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client calls the Laravel app's internal (shared-secret) API.
type Client struct {
	baseURL string
	secret  string
	http    *http.Client
}

func NewClient(baseURL, secret string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		secret:  secret,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// PIF fetches the generated PIF PDF for a group, returning its filename and
// base64-encoded contents (mirrors GeneratePIF + base64_encode in Laravel).
func (c *Client) PIF(ctx context.Context, groupID uint64) (name string, base64Data string, err error) {
	url := fmt.Sprintf("%s/api/internal/groups/%d/pif", c.baseURL, groupID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("X-Internal-Secret", c.secret)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("laravel pif returned %d: %s", resp.StatusCode, string(body))
	}

	var out struct {
		PDFName string `json:"pdf_name"`
		PDFData string `json:"pdf_data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", "", err
	}
	return out.PDFName, out.PDFData, nil
}
