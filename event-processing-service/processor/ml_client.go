package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// MLRequest is the payload sent to the ML Service.
type MLRequest struct {
	DeviceID string     `json:"device_id"`
	TenantID string     `json:"tenant_id"`
	Features MLFeatures `json:"features"`
}

// MLClient posts featurized payloads to the ML Service over HTTP.
type MLClient struct {
	url    string
	client *http.Client
}

// NewMLClient creates an MLClient targeting the given endpoint URL.
// A 10-second timeout is applied to every request.
func NewMLClient(url string) *MLClient {
	return &MLClient{
		url: url,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Send marshals the MLRequest to JSON and POSTs it to the ML Service.
// The response body is logged for observability but not parsed.
func (c *MLClient) Send(ctx context.Context, deviceID, tenantID string, features MLFeatures) error {
	payload := MLRequest{
		DeviceID: deviceID,
		TenantID: tenantID,
		Features: features,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("ml_client: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ml_client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("ml_client: POST %s: %w", c.url, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ml_client: unexpected status %d: %s", resp.StatusCode, respBody)
	}

	return nil
}
