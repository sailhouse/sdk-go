package sailhouse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type SailhouseClient struct {
	client  *http.Client
	token   string
	baseURL string
}

const BaseURL = "https://api.sailhouse.dev"

type SailhouseClientOptions struct {
	Client  *http.Client
	Token   string
	BaseURL string
}

type Map map[string]any

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

	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = BaseURL
	}

	return &SailhouseClient{
		client:  opts.Client,
		token:   opts.Token,
		baseURL: baseURL,
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

func (c *SailhouseClient) PullEvent(ctx context.Context, topic, subscription string) (*Event, error) {
	endpoint := fmt.Sprintf("%s/topics/%s/subscriptions/%s/events/pull", c.baseURL, topic, subscription)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	res, err := c.do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode == 204 {
		return nil, nil
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get events: %d", res.StatusCode)
	}

	var dest Event
	err = json.NewDecoder(res.Body).Decode(&dest)
	if err != nil {
		return nil, err
	}

	dest.client = c
	dest.topic = topic
	dest.subscription = subscription

	return &dest, nil
}

func (c *SailhouseClient) GetEvents(ctx context.Context, topic, subscription string, opts ...getOption) (GetEventsResponse, error) {
	endpoint := fmt.Sprintf("%s/topics/%s/subscriptions/%s/events", c.baseURL, topic, subscription)

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

type publishOpt struct {
	mod func(data *map[string]any)
}

func WithScheduledTime(sendAt time.Time) publishOpt {
	return publishOpt{
		mod: func(data *map[string]any) {
			timeString := sendAt.Format(time.RFC3339)
			(*data)["send_at"] = timeString
		},
	}
}

func WithMetaData(data map[string]any) publishOpt {
	return publishOpt{
		mod: func(body *map[string]any) {
			(*body)["metadata"] = data
		},
	}
}

func WithWaitGroupInstanceID(id string) publishOpt {
	return publishOpt{
		mod: func(body *map[string]any) {
			(*body)["wait_group_instance_id"] = id
		},
	}
}

type WaitEvent struct {
	Topic    string
	Body     any
	Metadata map[string]any
	SendAt   *time.Time
}

type WaitOption struct {
	mod func(data *map[string]any)
}

func WithTTL(ttl string) WaitOption {
	return WaitOption{
		mod: func(data *map[string]any) {
			(*data)["ttl"] = ttl
		},
	}
}

func (c *SailhouseClient) Publish(ctx context.Context, topic string, data any, opts ...publishOpt) (*PublishResponse, error) {
	endpoint := fmt.Sprintf("%s/topics/%s/events", c.baseURL, topic)

	body := map[string]any{
		"data": data,
	}

	for _, opt := range opts {
		opt.mod(&body)
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := c.do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 201 {
		resText := ""
		defer res.Body.Close()

		b, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		resText = string(b)
		return nil, fmt.Errorf("failed to send message: %d - %s", res.StatusCode, resText)
	}

	var response PublishResponse
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *SailhouseClient) AcknowledgeMessage(ctx context.Context, topic string, subscription string, id string) error {
	endpoint := fmt.Sprintf("%s/topics/%s/subscriptions/%s/events/%s", c.baseURL, topic, subscription, id)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, nil)
	if err != nil {
		return err
	}

	res, err := c.do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 && res.StatusCode != 204 {
		return fmt.Errorf("failed to acknowledge message: %d", res.StatusCode)
	}

	return nil
}

func (c *SailhouseClient) StreamEvents(ctx context.Context, topic string, subscription string) (<-chan Event, <-chan error) {
	done := ctx.Done()
	events := make(chan Event)
	errs := make(chan error)

	messages := make(chan []byte)

	u := url.URL{Scheme: "wss", Host: "api.sailhouse.dev", Path: "/events/stream"}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		errs <- fmt.Errorf("failed to connect to websocket: %w", err)
		return events, errs
	}

	err = conn.WriteJSON(map[string]any{
		"topic_slug":        topic,
		"subscription_slug": subscription,
		"token":             c.token,
	})
	if err != nil {
		errs <- fmt.Errorf("failed to send auth message: %w", err)
		return events, errs
	}

	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if strings.Contains(err.Error(), "use of closed network connection") {
					return
				}
				errs <- fmt.Errorf("failed to read message: %w", err)
				return
			}

			messages <- message
		}
	}()

	go func() {
		defer func() {
			conn.Close()
			close(messages)
			close(errs)
		}()

		for {
			select {
			case <-done:
				return
			case message := <-messages:
				var eventResponse EventResponse
				err = json.Unmarshal(message, &eventResponse)
				if err != nil {
					errs <- fmt.Errorf("failed to unmarshal message: %w", err)
					return
				}

				event := Event{
					ID:           eventResponse.ID,
					Data:         eventResponse.Data,
					Metadata:     eventResponse.Metadata,
					topic:        topic,
					subscription: subscription,
					client:       c,
				}

				events <- event
			}
		}
	}()

	return events, errs
}

type SubscriptionOptions struct {
	OnError   func(error)
	ExitOnErr bool
}

type SubscriptionHandler func(context.Context, *Event)

// Subscribe to a topic and subscription in the background, calling the handler function when new events are received.
//
// If an error is encountered, the `OnError` function within the SubscriptionOptions will be called.
func (c *SailhouseClient) Subscribe(ctx context.Context, topic string, subscription string, handler SubscriptionHandler, opts *SubscriptionOptions) {
	pollingInterval := 5 * time.Second
	doneChan := ctx.Done()
	errHandler := func(err error) {}
	exitOnErr := false

	if opts != nil {
		if opts.OnError != nil {
			errHandler = opts.OnError
		}
		exitOnErr = opts.ExitOnErr
	}

	go func() {
		for {
			event, err := c.PullEvent(ctx, topic, subscription)
			if err != nil {
				errHandler(err)
				if exitOnErr {
					return
				}
				select {
				case <-time.After(pollingInterval):
					continue
				case <-doneChan:
					return
				}
			}

			if event != nil {
				handler(ctx, event)
				continue
			}

			select {
			case <-time.After(pollingInterval):
				continue
			case <-doneChan:
				return
			}
		}
	}()
}

// Wait creates a wait group, publishes all events with the wait group ID, and marks the wait group as in progress.
// It allows you to publish multiple events across different topics and wait for all of them to be processed before proceeding.
func (c *SailhouseClient) Wait(ctx context.Context, topic string, events []WaitEvent, opts ...WaitOption) error {
	// Create wait group instance
	endpoint := fmt.Sprintf("%s/waitgroups/instances", c.baseURL)

	body := map[string]any{
		"topic": topic,
	}

	for _, opt := range opts {
		opt.mod(&body)
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

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		resText := ""
		defer res.Body.Close()

		b, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}

		resText = string(b)
		return fmt.Errorf("failed to create wait group: %d - %s", res.StatusCode, resText)
	}

	var response WaitGroupInstanceResponse
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return err
	}

	waitGroupID := response.WaitGroupInstanceID

	// Publish all events with the wait group ID
	for _, event := range events {
		var publishOpts []publishOpt

		if event.Metadata != nil {
			publishOpts = append(publishOpts, WithMetaData(event.Metadata))
		}

		if event.SendAt != nil {
			publishOpts = append(publishOpts, WithScheduledTime(*event.SendAt))
		}

		publishOpts = append(publishOpts, WithWaitGroupInstanceID(waitGroupID))

		_, err := c.Publish(ctx, event.Topic, event.Body, publishOpts...)
		if err != nil {
			return err
		}
	}

	// Mark wait group as in progress
	endpoint = fmt.Sprintf("%s/waitgroups/instances/%s/events", c.baseURL, waitGroupID)

	req, err = http.NewRequestWithContext(ctx, "PUT", endpoint, bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	res, err = c.do(req)
	if err != nil {
		return err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		resText := ""
		defer res.Body.Close()

		b, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}

		resText = string(b)
		return fmt.Errorf("failed to mark wait group as in progress: %d - %s", res.StatusCode, resText)
	}

	return nil
}
