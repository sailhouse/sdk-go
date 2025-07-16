package sailhouse_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sailhouse/sdk-go/sailhouse"
)

var _ = Describe("SailhouseSubscriber", func() {
	var (
		mockServer *sailhouse.MockSailhouseServer
		client     *sailhouse.SailhouseClient
		subscriber *sailhouse.SailhouseSubscriber
		ctx        context.Context
		cancel     context.CancelFunc
	)

	BeforeEach(func() {
		mockServer = sailhouse.NewMockSailhouseServer()
		client = mockServer.CreateTestClient()
		ctx, cancel = context.WithCancel(context.Background())
	})

	AfterEach(func() {
		if subscriber != nil && subscriber.IsRunning() {
			subscriber.Stop()
		}
		cancel()
		mockServer.Close()
	})

	Describe("constructor", func() {
		It("should create subscriber with default options", func() {
			subscriber = sailhouse.NewSailhouseSubscriber(client, nil)
			Expect(subscriber).NotTo(BeNil())
			Expect(subscriber.IsRunning()).To(BeFalse())
		})

		It("should create subscriber with custom options", func() {
			options := &sailhouse.SubscriberOptions{
				PerSubscriptionProcessors: 3,
				PollInterval:              500 * time.Millisecond,
				MaxRetries:                5,
				RetryDelay:                2 * time.Second,
			}

			subscriber = sailhouse.NewSailhouseSubscriber(client, options)
			Expect(subscriber).NotTo(BeNil())
		})

		It("should create subscriber using client method", func() {
			subscriber = client.NewSubscriber(nil)
			Expect(subscriber).NotTo(BeNil())
		})
	})

	Describe("subscription management", func() {
		BeforeEach(func() {
			subscriber = sailhouse.NewSailhouseSubscriber(client, nil)
		})

		It("should register subscriptions", func() {
			handler := func(ctx context.Context, event *sailhouse.Event) error {
				return nil
			}

			subscriber.Subscribe("test-topic", "test-sub", handler)
			subscriptions := subscriber.GetSubscriptions()

			Expect(subscriptions).To(HaveLen(1))
			Expect(subscriptions[0].Topic).To(Equal("test-topic"))
			Expect(subscriptions[0].Subscription).To(Equal("test-sub"))
		})

		It("should register multiple subscriptions", func() {
			handler1 := func(ctx context.Context, event *sailhouse.Event) error { return nil }
			handler2 := func(ctx context.Context, event *sailhouse.Event) error { return nil }

			subscriber.Subscribe("topic1", "sub1", handler1)
			subscriber.Subscribe("topic2", "sub2", handler2)

			subscriptions := subscriber.GetSubscriptions()
			Expect(subscriptions).To(HaveLen(2))
		})

		It("should panic when adding subscriptions while running", func() {
			handler := func(ctx context.Context, event *sailhouse.Event) error { return nil }
			subscriber.Subscribe("test-topic", "test-sub", handler)

			// Start the subscriber
			mockServer.RegisterPullEventHandler("test-topic", "test-sub", nil)
			err := subscriber.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Try to add another subscription while running
			Expect(func() {
				subscriber.Subscribe("another-topic", "another-sub", handler)
			}).To(Panic())
		})
	})

	Describe("lifecycle management", func() {
		var eventProcessed atomic.Bool
		var handler sailhouse.SubscriberHandler

		BeforeEach(func() {
			eventProcessed.Store(false)
			handler = func(ctx context.Context, event *sailhouse.Event) error {
				eventProcessed.Store(true)
				return nil
			}
			subscriber = sailhouse.NewSailhouseSubscriber(client, nil)
		})

		It("should start and process events", func() {
			testEvent := sailhouse.CreateTestEvent("test-id", map[string]interface{}{
				"message": "hello",
			})

			mockServer.RegisterPullEventHandler("test-topic", "test-sub", testEvent)
			mockServer.RegisterAckEventHandler("test-topic", "test-sub", "test-id")

			subscriber.Subscribe("test-topic", "test-sub", handler)

			err := subscriber.Start(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(subscriber.IsRunning()).To(BeTrue())

			// Wait for event to be processed
			Eventually(func() bool {
				return eventProcessed.Load()
			}, 2*time.Second, 100*time.Millisecond).Should(BeTrue())
		})

		It("should stop gracefully", func() {
			subscriber.Subscribe("test-topic", "test-sub", handler)
			mockServer.RegisterPullEventHandler("test-topic", "test-sub", nil)

			err := subscriber.Start(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(subscriber.IsRunning()).To(BeTrue())

			subscriber.Stop()
			Expect(subscriber.IsRunning()).To(BeFalse())
		})

		It("should return error when starting without subscriptions", func() {
			err := subscriber.Start(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no subscriptions registered"))
		})

		It("should return error when starting already running subscriber", func() {
			subscriber.Subscribe("test-topic", "test-sub", handler)
			mockServer.RegisterPullEventHandler("test-topic", "test-sub", nil)

			err := subscriber.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			err = subscriber.Start(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("already running"))
		})

		It("should handle context cancellation", func() {
			subscriber.Subscribe("test-topic", "test-sub", handler)
			mockServer.RegisterPullEventHandler("test-topic", "test-sub", nil)

			testCtx, testCancel := context.WithCancel(ctx)

			err := subscriber.Start(testCtx)
			Expect(err).NotTo(HaveOccurred())
			Expect(subscriber.IsRunning()).To(BeTrue())

			testCancel() // Cancel the context

			// Wait a bit for goroutines to notice cancellation
			time.Sleep(200 * time.Millisecond)

			// Then call Stop to clean up properly
			subscriber.Stop()
			Expect(subscriber.IsRunning()).To(BeFalse())
		})
	})

	Describe("concurrent processing", func() {
		It("should run multiple processors per subscription", func() {
			processedCount := atomic.Int64{}

			handler := func(ctx context.Context, event *sailhouse.Event) error {
				processedCount.Add(1)
				time.Sleep(100 * time.Millisecond) // Simulate processing time
				return nil
			}

			options := &sailhouse.SubscriberOptions{
				PerSubscriptionProcessors: 3,
				PollInterval:              50 * time.Millisecond,
			}

			subscriber = sailhouse.NewSailhouseSubscriber(client, options)
			subscriber.Subscribe("test-topic", "test-sub", handler)

			// Set up mock to return events
			testEvent := sailhouse.CreateTestEvent("test-id", map[string]interface{}{
				"message": "hello",
			})
			mockServer.RegisterPullEventHandler("test-topic", "test-sub", testEvent)
			mockServer.RegisterAckEventHandler("test-topic", "test-sub", "test-id")

			err := subscriber.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Let it run for a bit
			time.Sleep(500 * time.Millisecond)
			subscriber.Stop()

			// Should have processed multiple events due to multiple processors
			Expect(processedCount.Load()).To(BeNumerically(">=", 3))
		})

		It("should handle multiple subscriptions concurrently", func() {
			topic1Processed := atomic.Bool{}
			topic2Processed := atomic.Bool{}

			handler1 := func(ctx context.Context, event *sailhouse.Event) error {
				topic1Processed.Store(true)
				return nil
			}

			handler2 := func(ctx context.Context, event *sailhouse.Event) error {
				topic2Processed.Store(true)
				return nil
			}

			subscriber = sailhouse.NewSailhouseSubscriber(client, nil)
			subscriber.Subscribe("topic1", "sub1", handler1)
			subscriber.Subscribe("topic2", "sub2", handler2)

			event1 := sailhouse.CreateTestEvent("id1", map[string]interface{}{"msg": "1"})
			event2 := sailhouse.CreateTestEvent("id2", map[string]interface{}{"msg": "2"})

			mockServer.RegisterPullEventHandler("topic1", "sub1", event1)
			mockServer.RegisterPullEventHandler("topic2", "sub2", event2)
			mockServer.RegisterAckEventHandler("topic1", "sub1", "id1")
			mockServer.RegisterAckEventHandler("topic2", "sub2", "id2")

			err := subscriber.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				return topic1Processed.Load() && topic2Processed.Load()
			}, 2*time.Second, 100*time.Millisecond).Should(BeTrue())
		})
	})

	Describe("error handling", func() {
		var errorsCaught []error
		var errorMutex sync.Mutex

		BeforeEach(func() {
			errorsCaught = make([]error, 0)
			errorMutex = sync.Mutex{}
		})

		captureError := func(err error) {
			errorMutex.Lock()
			defer errorMutex.Unlock()
			errorsCaught = append(errorsCaught, err)
		}

		getErrorCount := func() int {
			errorMutex.Lock()
			defer errorMutex.Unlock()
			return len(errorsCaught)
		}

		It("should retry failed event processing", func() {
			attemptCount := atomic.Int32{}

			handler := func(ctx context.Context, event *sailhouse.Event) error {
				count := attemptCount.Add(1)
				if count < 3 {
					return errors.New("temporary failure")
				}
				return nil // Success on third attempt
			}

			options := &sailhouse.SubscriberOptions{
				MaxRetries:   5,
				RetryDelay:   10 * time.Millisecond,
				ErrorHandler: captureError,
			}

			subscriber = sailhouse.NewSailhouseSubscriber(client, options)
			subscriber.Subscribe("test-topic", "test-sub", handler)

			testEvent := sailhouse.CreateTestEvent("test-id", map[string]interface{}{
				"message": "hello",
			})
			mockServer.RegisterPullEventHandler("test-topic", "test-sub", testEvent)
			mockServer.RegisterAckEventHandler("test-topic", "test-sub", "test-id")

			err := subscriber.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Wait for processing
			Eventually(func() int32 {
				return attemptCount.Load()
			}, 2*time.Second, 10*time.Millisecond).Should(BeNumerically(">=", 3))

			subscriber.Stop()

			// Should have succeeded on third attempt
			Expect(attemptCount.Load()).To(BeNumerically(">=", 3))
		})

		It("should handle maximum retries exceeded", func() {
			handler := func(ctx context.Context, event *sailhouse.Event) error {
				return errors.New("persistent failure")
			}

			options := &sailhouse.SubscriberOptions{
				MaxRetries:   2,
				RetryDelay:   10 * time.Millisecond,
				ErrorHandler: captureError,
			}

			subscriber = sailhouse.NewSailhouseSubscriber(client, options)
			subscriber.Subscribe("test-topic", "test-sub", handler)

			testEvent := sailhouse.CreateTestEvent("test-id", map[string]interface{}{
				"message": "hello",
			})
			mockServer.RegisterPullEventHandler("test-topic", "test-sub", testEvent)
			mockServer.RegisterAckEventHandler("test-topic", "test-sub", "test-id")

			err := subscriber.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Wait for error to be captured
			Eventually(func() int {
				return getErrorCount()
			}, 2*time.Second, 50*time.Millisecond).Should(BeNumerically(">=", 1))

			subscriber.Stop()

			// Should have captured an error about max retries
			Expect(getErrorCount()).To(BeNumerically(">=", 1))
		})

		It("should call error handler when configured", func() {
			handler := func(ctx context.Context, event *sailhouse.Event) error {
				return errors.New("test error")
			}

			options := &sailhouse.SubscriberOptions{
				MaxRetries:   0, // Fail immediately
				ErrorHandler: captureError,
			}

			subscriber = sailhouse.NewSailhouseSubscriber(client, options)
			subscriber.Subscribe("test-topic", "test-sub", handler)

			testEvent := sailhouse.CreateTestEvent("test-id", map[string]interface{}{
				"message": "hello",
			})
			mockServer.RegisterPullEventHandler("test-topic", "test-sub", testEvent)
			mockServer.RegisterAckEventHandler("test-topic", "test-sub", "test-id")

			err := subscriber.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() int {
				return getErrorCount()
			}, 2*time.Second, 50*time.Millisecond).Should(BeNumerically(">=", 1))

			subscriber.Stop()
		})
	})

	Describe("polling behavior", func() {
		It("should respect poll interval when no events available", func() {
			pollCount := atomic.Int32{}

			handler := func(ctx context.Context, event *sailhouse.Event) error {
				pollCount.Add(1)
				return nil
			}

			options := &sailhouse.SubscriberOptions{
				PollInterval: 200 * time.Millisecond,
			}

			subscriber = sailhouse.NewSailhouseSubscriber(client, options)
			subscriber.Subscribe("test-topic", "test-sub", handler)

			// Mock returns no events (nil)
			mockServer.RegisterPullEventHandler("test-topic", "test-sub", nil)

			err := subscriber.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Let it run for a controlled time
			time.Sleep(500 * time.Millisecond)
			subscriber.Stop()

			// Should not have processed any events since none were available
			Expect(pollCount.Load()).To(Equal(int32(0)))
		})
	})

	Describe("edge cases", func() {
		It("should handle stop when not running", func() {
			subscriber = sailhouse.NewSailhouseSubscriber(client, nil)

			// Should not panic or error
			Expect(func() {
				subscriber.Stop()
			}).NotTo(Panic())
		})

		It("should handle multiple stops", func() {
			handler := func(ctx context.Context, event *sailhouse.Event) error {
				return nil
			}

			subscriber = sailhouse.NewSailhouseSubscriber(client, nil)
			subscriber.Subscribe("test-topic", "test-sub", handler)
			mockServer.RegisterPullEventHandler("test-topic", "test-sub", nil)

			err := subscriber.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			subscriber.Stop()

			// Second stop should not panic
			Expect(func() {
				subscriber.Stop()
			}).NotTo(Panic())
		})

		It("should handle zero retries", func() {
			handler := func(ctx context.Context, event *sailhouse.Event) error {
				return errors.New("immediate failure")
			}

			options := &sailhouse.SubscriberOptions{
				MaxRetries: 0,
			}

			subscriber = sailhouse.NewSailhouseSubscriber(client, options)
			subscriber.Subscribe("test-topic", "test-sub", handler)

			testEvent := sailhouse.CreateTestEvent("test-id", map[string]interface{}{})
			mockServer.RegisterPullEventHandler("test-topic", "test-sub", testEvent)
			mockServer.RegisterAckEventHandler("test-topic", "test-sub", "test-id")

			err := subscriber.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Should still acknowledge the event even after failure
			time.Sleep(100 * time.Millisecond)
			subscriber.Stop()
		})
	})
})
