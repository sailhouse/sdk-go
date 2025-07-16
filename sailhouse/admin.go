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

// FilterCondition represents a single filter condition
type FilterCondition struct {
	Path      string `json:"path"`
	Condition string `json:"condition"`
	Value     string `json:"value"`
}

// ComplexFilter represents a complex filter with multiple conditions
type ComplexFilter struct {
	Filters  []FilterCondition `json:"filters"`
	Operator string            `json:"operator"`
}

// Filter represents any type of filter that can be applied
// Can be a boolean, nil, or a ComplexFilter
type Filter interface{}

// FilterOption represents a simple filter for subscription events (legacy)
// Deprecated: Use ComplexFilter for new implementations
type FilterOption struct {
	Path  string `json:"path"`
	Value string `json:"value"`
}

// RegisterPushSubscriptionOptions contains optional parameters for registering a push subscription
type RegisterPushSubscriptionOptions struct {
	// Filter can be a boolean (true/false), nil, or a ComplexFilter
	Filter Filter `json:"filter,omitempty"`

	// RateLimit specifies the rate limit for the subscription (e.g., "10/1m", "100/1h")
	RateLimit *string `json:"rate_limit,omitempty"`

	// Deduplication specifies the deduplication strategy (e.g., "5m", "1h")
	Deduplication *string `json:"deduplication,omitempty"`

	// Legacy FilterOption support for backward compatibility
	// Deprecated: Use Filter field instead
	LegacyFilter *FilterOption `json:"-"`
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
	url := fmt.Sprintf("%s/topics/%s/subscriptions/%s", c.client.baseURL, topic, subscription)

	// Build request body
	requestBody := map[string]interface{}{
		"type":     "push",
		"endpoint": endpoint,
	}

	// Add options if provided
	if options != nil {
		// Handle new Filter field
		if options.Filter != nil {
			requestBody["filter"] = options.Filter
		} else if options.LegacyFilter != nil {
			// Backward compatibility support
			requestBody["filter"] = options.LegacyFilter
		}

		// Add rate limit if provided
		if options.RateLimit != nil {
			requestBody["rate_limit"] = *options.RateLimit
		}

		// Add deduplication if provided
		if options.Deduplication != nil {
			requestBody["deduplication"] = *options.Deduplication
		}
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

// Helper functions for creating filters and options

// NewSimpleFilter creates a simple boolean filter
func NewSimpleFilter(enabled bool) Filter {
	return enabled
}

// NewComplexFilter creates a complex filter with conditions and operator
func NewComplexFilter(operator string, conditions ...FilterCondition) *ComplexFilter {
	return &ComplexFilter{
		Filters:  conditions,
		Operator: operator,
	}
}

// NewFilterCondition creates a new filter condition
func NewFilterCondition(path, condition, value string) FilterCondition {
	return FilterCondition{
		Path:      path,
		Condition: condition,
		Value:     value,
	}
}

// WithFilter sets the filter option
func WithFilter(filter Filter) func(*RegisterPushSubscriptionOptions) {
	return func(opts *RegisterPushSubscriptionOptions) {
		opts.Filter = filter
	}
}

// WithRateLimit sets the rate limit option
func WithRateLimit(rateLimit string) func(*RegisterPushSubscriptionOptions) {
	return func(opts *RegisterPushSubscriptionOptions) {
		opts.RateLimit = &rateLimit
	}
}

// WithDeduplication sets the deduplication option
func WithDeduplication(deduplication string) func(*RegisterPushSubscriptionOptions) {
	return func(opts *RegisterPushSubscriptionOptions) {
		opts.Deduplication = &deduplication
	}
}

// NewRegisterOptions creates a new RegisterPushSubscriptionOptions with the provided options
func NewRegisterOptions(optFuncs ...func(*RegisterPushSubscriptionOptions)) *RegisterPushSubscriptionOptions {
	opts := &RegisterPushSubscriptionOptions{}
	for _, fn := range optFuncs {
		fn(opts)
	}
	return opts
}
