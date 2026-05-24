package llms

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

func postJSON(ctx context.Context, client *http.Client, provider, url, apiKey string, headers map[string]string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("build provider request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := HTTPClient(client).Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("provider http request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	payload, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, resp.StatusCode, fmt.Errorf("read provider response: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, NewProviderHTTPError(provider, resp, payload)
	}
	return payload, resp.StatusCode, nil
}
