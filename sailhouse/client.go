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
	client *http.Client
	token  string
}

const BaseURL = "https://api.sailhouse.dev"

type SailhouseClientOptions struct {
	Client *http.Client
	Token  string
}

type Map map[string]interface{}

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

func WithMetaData(data map[string]interface{}) publishOpt {
	return publishOpt{
		mod: func(body *map[string]any) {
			(*body)["metadata"] = data
		},
	}
}

func (c *SailhouseClient) Publish(ctx context.Context, topic string, data interface{}, opts ...publishOpt) error {
	endpoint := fmt.Sprintf("%s/topics/%s/events", BaseURL, topic)

	body := map[string]interface{}{
		"data": data,
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

	err = conn.WriteJSON(map[string]interface{}{
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
