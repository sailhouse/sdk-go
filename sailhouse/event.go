package sailhouse

import (
	"encoding/json"
)

type GetEventsResponse struct {
	Events []*Event `json:"events"`
	Offset int      `json:"offset"`
	Limit  int      `json:"limit"`
}

type EventResponse struct {
	ID   string                 `json:"id"`
	Data map[string]interface{} `json:"data"`
}

type Event struct {
	ID           string                 `json:"id"`
	Data         map[string]interface{} `json:"data"`
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

func (e *Event) Ack() {
	e.client.AcknowledgeMessage(e.topic, e.subscription, e.ID)
}
