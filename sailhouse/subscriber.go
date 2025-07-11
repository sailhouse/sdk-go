package sailhouse

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SubscriberHandler is a function type for handling subscription events in the subscriber
type SubscriberHandler func(ctx context.Context, event *Event) error

// SubscriberOptions contains configuration options for the subscriber
type SubscriberOptions struct {
	// PerSubscriptionProcessors is the number of concurrent processors per subscription (default: 1)
	PerSubscriptionProcessors int

	// PollInterval is the interval between polls when no events are available (default: 1 second)
	PollInterval time.Duration

	// ErrorHandler is called when an error occurs during event processing
	ErrorHandler func(error)

	// MaxRetries is the maximum number of retries for failed event processing (default: 3)
	MaxRetries int

	// RetryDelay is the delay between retries (default: 1 second)
	RetryDelay time.Duration
}

// Subscription represents a registered subscription handler
type Subscription struct {
	Topic        string
	Subscription string
	Handler      SubscriberHandler
}

// SailhouseSubscriber manages multiple long-running subscription processors
type SailhouseSubscriber struct {
	client        *SailhouseClient
	subscriptions []Subscription
	options       SubscriberOptions
	running       bool
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	mu            sync.RWMutex
}

// NewSailhouseSubscriber creates a new subscriber instance
func NewSailhouseSubscriber(client *SailhouseClient, options *SubscriberOptions) *SailhouseSubscriber {
	opts := SubscriberOptions{
		PerSubscriptionProcessors: 1,
		PollInterval:              time.Second,
		MaxRetries:                3,
		RetryDelay:                time.Second,
	}

	if options != nil {
		if options.PerSubscriptionProcessors > 0 {
			opts.PerSubscriptionProcessors = options.PerSubscriptionProcessors
		}
		if options.PollInterval > 0 {
			opts.PollInterval = options.PollInterval
		}
		if options.ErrorHandler != nil {
			opts.ErrorHandler = options.ErrorHandler
		}
		if options.MaxRetries >= 0 {
			opts.MaxRetries = options.MaxRetries
		}
		if options.RetryDelay > 0 {
			opts.RetryDelay = options.RetryDelay
		}
	}

	return &SailhouseSubscriber{
		client:        client,
		subscriptions: make([]Subscription, 0),
		options:       opts,
		running:       false,
	}
}

// Subscribe registers a subscription handler
func (s *SailhouseSubscriber) Subscribe(topic, subscription string, handler SubscriberHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		panic("cannot add subscriptions while subscriber is running")
	}

	s.subscriptions = append(s.subscriptions, Subscription{
		Topic:        topic,
		Subscription: subscription,
		Handler:      handler,
	})
}

// Start begins processing all registered subscriptions
func (s *SailhouseSubscriber) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("subscriber is already running")
	}

	if len(s.subscriptions) == 0 {
		return fmt.Errorf("no subscriptions registered")
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.running = true

	// Start processors for each subscription
	for _, subscription := range s.subscriptions {
		for i := 0; i < s.options.PerSubscriptionProcessors; i++ {
			s.wg.Add(1)
			go s.runProcessor(subscription, i)
		}
	}

	return nil
}

// Stop gracefully stops all subscription processors
func (s *SailhouseSubscriber) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}

	s.running = false
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()

	// Wait for all processors to finish
	s.wg.Wait()
}

// IsRunning returns whether the subscriber is currently running
func (s *SailhouseSubscriber) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetSubscriptions returns a copy of the registered subscriptions
func (s *SailhouseSubscriber) GetSubscriptions() []Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Subscription, len(s.subscriptions))
	copy(result, s.subscriptions)
	return result
}

// runProcessor runs a single processor for a subscription
func (s *SailhouseSubscriber) runProcessor(subscription Subscription, processorID int) {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			s.processNextEvent(subscription, processorID)
		}
	}
}

// processNextEvent processes the next available event for a subscription
func (s *SailhouseSubscriber) processNextEvent(subscription Subscription, processorID int) {
	// Pull an event from the subscription
	event, err := s.client.PullEvent(s.ctx, subscription.Topic, subscription.Subscription)
	if err != nil {
		s.handleError(fmt.Errorf("processor %d failed to pull event from %s/%s: %w",
			processorID, subscription.Topic, subscription.Subscription, err))
		s.sleep(s.options.PollInterval)
		return
	}

	// If no event is available, wait before trying again
	if event == nil {
		s.sleep(s.options.PollInterval)
		return
	}

	// Process the event with retries
	s.processEventWithRetries(subscription, event, processorID)
}

// processEventWithRetries processes an event with retry logic
func (s *SailhouseSubscriber) processEventWithRetries(subscription Subscription, event *Event, processorID int) {
	var lastErr error

	for attempt := 0; attempt <= s.options.MaxRetries; attempt++ {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Call the user's handler
		err := subscription.Handler(s.ctx, event)
		if err == nil {
			// Success - acknowledge the event
			ackErr := event.Ack(s.ctx)
			if ackErr != nil {
				s.handleError(fmt.Errorf("processor %d failed to acknowledge event %s: %w",
					processorID, event.ID, ackErr))
			}
			return
		}

		lastErr = err

		// If this was the last attempt, handle the final error
		if attempt == s.options.MaxRetries {
			s.handleError(fmt.Errorf("processor %d failed to process event %s after %d attempts: %w",
				processorID, event.ID, s.options.MaxRetries+1, lastErr))

			// Still acknowledge the event to prevent infinite reprocessing
			// In a production system, you might want to send failed events to a dead letter queue
			ackErr := event.Ack(s.ctx)
			if ackErr != nil {
				s.handleError(fmt.Errorf("processor %d failed to acknowledge failed event %s: %w",
					processorID, event.ID, ackErr))
			}
			return
		}

		// Wait before retrying
		s.sleep(s.options.RetryDelay)
	}
}

// handleError handles errors using the configured error handler
func (s *SailhouseSubscriber) handleError(err error) {
	if s.options.ErrorHandler != nil {
		s.options.ErrorHandler(err)
	}
	// If no error handler is configured, errors are silently ignored
	// In a real application, you might want to log to stderr or a logger
}

// sleep sleeps for the specified duration, but can be interrupted by context cancellation
func (s *SailhouseSubscriber) sleep(duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-s.ctx.Done():
		return
	case <-timer.C:
		return
	}
}

// SailhouseClient extension for creating subscribers
func (c *SailhouseClient) NewSubscriber(options *SubscriberOptions) *SailhouseSubscriber {
	return NewSailhouseSubscriber(c, options)
}
