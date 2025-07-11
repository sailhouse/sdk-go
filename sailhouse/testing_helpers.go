package sailhouse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"
)

// MockSailhouseServer provides a mock HTTP server for testing
type MockSailhouseServer struct {
	*httptest.Server
	handlers map[string]http.HandlerFunc
}

// NewMockSailhouseServer creates a new mock server for testing
func NewMockSailhouseServer() *MockSailhouseServer {
	mock := &MockSailhouseServer{
		handlers: make(map[string]http.HandlerFunc),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", mock.routeHandler)
	
	mock.Server = httptest.NewServer(mux)
	return mock
}

// routeHandler routes requests to registered handlers
func (m *MockSailhouseServer) routeHandler(w http.ResponseWriter, r *http.Request) {
	key := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
	
	if handler, exists := m.handlers[key]; exists {
		handler(w, r)
		return
	}
	
	// Default 404 response
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{
		"error": fmt.Sprintf("No handler registered for %s", key),
	})
}

// RegisterHandler registers a handler for a specific method and path
func (m *MockSailhouseServer) RegisterHandler(method, path string, handler http.HandlerFunc) {
	key := fmt.Sprintf("%s %s", method, path)
	m.handlers[key] = handler
}

// RegisterPullEventHandler registers a handler for pull event requests
func (m *MockSailhouseServer) RegisterPullEventHandler(topic, subscription string, event *Event) {
	path := fmt.Sprintf("/topics/%s/subscriptions/%s/events/pull", topic, subscription)
	m.RegisterHandler("GET", path, func(w http.ResponseWriter, r *http.Request) {
		if event == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(event)
	})
}

// RegisterPublishEventHandler registers a handler for publish event requests
func (m *MockSailhouseServer) RegisterPublishEventHandler(topic string, response *PublishResponse) {
	path := fmt.Sprintf("/topics/%s/events", topic)
	m.RegisterHandler("POST", path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	})
}

// RegisterAckEventHandler registers a handler for acknowledge event requests
func (m *MockSailhouseServer) RegisterAckEventHandler(topic, subscription, eventID string) {
	path := fmt.Sprintf("/topics/%s/subscriptions/%s/events/%s", topic, subscription, eventID)
	m.RegisterHandler("POST", path, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// RegisterAdminHandler registers a handler for admin API requests
func (m *MockSailhouseServer) RegisterAdminHandler(topic, subscription string, result *RegisterResult) {
	path := fmt.Sprintf("/topics/%s/subscriptions/%s", topic, subscription)
	m.RegisterHandler("PUT", path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})
}

// CreateTestClient creates a client configured to use the mock server
func (m *MockSailhouseServer) CreateTestClient() *SailhouseClient {
	client := NewSailhouseClient("test-token")
	// Override the base URL to point to our mock server
	// We'll need to modify the client to support this
	return client
}

// TestEvent creates a test event with data
func CreateTestEvent(id string, data map[string]interface{}) *Event {
	return &Event{
		ID:   id,
		Data: data,
	}
}

// CreateTestEventResponse creates a test event response
func CreateTestEventResponse(id string, data map[string]interface{}) *EventResponse {
	return &EventResponse{
		ID:   id,
		Data: data,
	}
}

// CreateTestPublishResponse creates a test publish response
func CreateTestPublishResponse(id string) *PublishResponse {
	return &PublishResponse{
		ID: id,
	}
}

// CreateTestRegisterResult creates a test admin register result
func CreateTestRegisterResult(outcome string) *RegisterResult {
	return &RegisterResult{
		Outcome: outcome,
	}
}

// WaitForCondition waits for a condition to be true with timeout
func WaitForCondition(condition func() bool, timeout time.Duration, interval time.Duration) bool {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(interval)
	}
	
	return false
}

// Close closes the mock server
func (m *MockSailhouseServer) Close() {
	if m.Server != nil {
		m.Server.Close()
	}
}