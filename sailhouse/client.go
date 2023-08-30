package sailhouse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type SailhouseClient struct {
	client *http.Client
	token  string
}

const BaseURL = "https://api.sailhouse.dev"

type SailhouseClientOptions struct {
	Client *http.Client
	Token  string
}

func NewSailhouseClient(token string) *SailhouseClient {
	return NewSailhouseClientWithOptions(SailhouseClientOptions{
		Token: token,
	})
}

func NewSailhouseClientWithOptions(opts SailhouseClientOptions) *SailhouseClient {
	if opts.Client == nil {
		opts.Client = &http.Client{
			Timeout: 5 * time.Second,
		}
	}

	return &SailhouseClient{
		client: opts.Client,
		token:  opts.Token,
	}
}

func (c *SailhouseClient) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", c.token)
	req.Header.Set("x-source", "sailhouse-go")

	return c.client.Do(req)
}

type Events struct {
	Events []EventResponse `json:"events"`
}

type getOption struct {
	mod (func(*http.Request))
}

func WithLimit(limit int) getOption {
	return getOption{
		mod: func(req *http.Request) {
			q := req.URL.Query()
			q.Add("limit", fmt.Sprintf("%d", limit))
			req.URL.RawQuery = q.Encode()
		},
	}
}

func WithOffset(offset int) getOption {
	return getOption{
		mod: func(req *http.Request) {
			q := req.URL.Query()
			q.Add("offset", fmt.Sprintf("%d", offset))
			req.URL.RawQuery = q.Encode()
		},
	}
}

func WithTimeWindow(dur time.Duration) getOption {
	return getOption{
		mod: func(req *http.Request) {
			q := req.URL.Query()
			q.Add("time_window", dur.String())
			req.URL.RawQuery = q.Encode()
		},
	}
}

func (c *SailhouseClient) GetEvents(ctx context.Context, topic, subscription string, opts ...getOption) (GetEventsResponse, error) {
	endpoint := fmt.Sprintf("%s/topics/%s/subscriptions/%s/events", BaseURL, topic, subscription)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return GetEventsResponse{}, err
	}

	for _, opt := range opts {
		opt.mod(req)
	}

	res, err := c.do(req)
	if err != nil {
		return GetEventsResponse{}, err
	}

	if res.StatusCode != 200 {
		return GetEventsResponse{}, fmt.Errorf("failed to get events: %d", res.StatusCode)
	}

	var dest GetEventsResponse
	err = json.NewDecoder(res.Body).Decode(&dest)
	if err != nil {
		return GetEventsResponse{}, err
	}

	for _, d := range dest.Events {
		d.client = c
		d.topic = topic
		d.subscription = subscription
	}

	return dest, nil
}

func (c *SailhouseClient) Publish(ctx context.Context, topic string, data interface{}) error {
	endpoint := fmt.Sprintf("%s/topics/%s/events", BaseURL, topic)

	body := map[string]interface{}{
		"data": data,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := c.do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != 201 {
		resText := ""
		defer res.Body.Close()

		b, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}

		resText = string(b)
		return fmt.Errorf("failed to send message: %d - %s", res.StatusCode, resText)
	}

	return nil
}

func (c *SailhouseClient) AcknowledgeMessage(ctx context.Context, topic string, subscription string, id string) error {
	endpoint := fmt.Sprintf("%s/topics/%s/subscriptions/%s/events/%s", BaseURL, topic, subscription, id)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, nil)
	if err != nil {
		return err
	}

	res, err := c.do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("failed to acknowledge message: %d", res.StatusCode)
	}

	return nil
}
