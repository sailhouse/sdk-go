package sailhouse

import (
	"context"
	"encoding/json"
)

type GetEventsResponse struct {
	Events []*Event `json:"events"`
	Offset int      `json:"offset"`
	Limit  int      `json:"limit"`
}

type EventResponse struct {
	ID       string                 `json:"id"`
	Data     map[string]interface{} `json:"data"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type PublishResponse struct {
	ID string `json:"id"`
}

type WaitGroupInstanceResponse struct {
	WaitGroupInstanceID string `json:"wait_group_instance_id"`
}

type Event struct {
	ID           string                 `json:"id"`
	Data         map[string]interface{} `json:"data"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	topic        string
	subscription string
	client       *SailhouseClient
}

func (e *Event) As(data any) error {
	dataBytes, err := json.Marshal(e.Data)
	if err != nil {
		return err
	}

	err = json.Unmarshal(dataBytes, data)
	if err != nil {
		return err
	}

	return nil
}

func (e *Event) Ack(ctx context.Context) error {
	return e.client.AcknowledgeMessage(ctx, e.topic, e.subscription, e.ID)
}
