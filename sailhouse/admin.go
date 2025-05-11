package sailhouse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// RegisterResult represents the result of registering a push subscription
type RegisterResult struct {
	Outcome string `json:"outcome"` // "created", "updated", or "none"
}

// FilterOption represents a filter for subscription events
type FilterOption struct {
	Path  string `json:"path"`
	Value string `json:"value"`
}

// RegisterPushSubscriptionOptions contains optional parameters for registering a push subscription
type RegisterPushSubscriptionOptions struct {
	Filter *FilterOption
}

// AdminClient provides administrative functionality for Sailhouse
type AdminClient struct {
	client *SailhouseClient
}

// NewAdminClient creates a new AdminClient using the provided SailhouseClient
func NewAdminClient(client *SailhouseClient) *AdminClient {
	return &AdminClient{
		client: client,
	}
}

// RegisterPushSubscription registers a push subscription for a topic
func (c *AdminClient) RegisterPushSubscription(
	ctx context.Context,
	topic string,
	subscription string,
	endpoint string,
	options *RegisterPushSubscriptionOptions,
) (*RegisterResult, error) {
	url := fmt.Sprintf("%s/topics/%s/subscriptions/%s", BaseURL, topic, subscription)

	// Build request body
	requestBody := map[string]interface{}{
		"type":     "push",
		"endpoint": endpoint,
	}

	// Add filter if provided
	if options != nil && options.Filter != nil {
		requestBody["filter"] = options.Filter
	}

	// Marshal request body to JSON
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.client.do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	var result RegisterResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}