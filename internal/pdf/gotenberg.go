// Package pdf renders the group PIF and converts HTML to PDF via Gotenberg.
package pdf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// Client talks to a Gotenberg server's Chromium HTML conversion route.
type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{baseURL: baseURL, http: &http.Client{Timeout: 30 * time.Second}}
}

// ConvertHTML mirrors GotenbergPdfService: POST the HTML as index.html to
// /forms/chromium/convert/html with preferCssPageSize, returning PDF bytes.
func (c *Client) ConvertHTML(ctx context.Context, html, filename string) ([]byte, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	part, err := w.CreateFormFile("files", "index.html")
	if err != nil {
		return nil, err
	}
	if _, err := part.Write([]byte(html)); err != nil {
		return nil, err
	}
	_ = w.WriteField("preferCssPageSize", "true")
	if filename != "" {
		_ = w.WriteField("outputFilename", filename)
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	url := c.baseURL + "/forms/chromium/convert/html"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gotenberg returned %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}
