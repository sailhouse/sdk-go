package sailhouse_test

import (
	"context"
	"encoding/json"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sailhouse/sdk-go/sailhouse"
)

var _ = Describe("AdminClient", func() {
	var (
		mockServer  *sailhouse.MockSailhouseServer
		client      *sailhouse.SailhouseClient
		adminClient *sailhouse.AdminClient
		ctx         context.Context
	)

	BeforeEach(func() {
		mockServer = sailhouse.NewMockSailhouseServer()
		client = mockServer.CreateTestClient()
		adminClient = sailhouse.NewAdminClient(client)
		ctx = context.Background()
	})

	AfterEach(func() {
		mockServer.Close()
	})

	Describe("RegisterPushSubscription", func() {
		It("should register a push subscription without options", func() {
			result := sailhouse.CreateTestRegisterResult("created")
			mockServer.RegisterAdminHandler("test-topic", "test-sub", result)

			response, err := adminClient.RegisterPushSubscription(
				ctx,
				"test-topic",
				"test-sub",
				"https://example.com/webhook",
				nil,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(response.Outcome).To(Equal("created"))
		})

		It("should register with simple boolean filter", func() {
			result := sailhouse.CreateTestRegisterResult("created")
			
			// Custom handler to verify request body
			mockServer.RegisterHandler("PUT", "/topics/test-topic/subscriptions/test-sub", func(w http.ResponseWriter, r *http.Request) {
				var requestBody map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&requestBody)
				Expect(err).NotTo(HaveOccurred())
				
				Expect(requestBody["type"]).To(Equal("push"))
				Expect(requestBody["endpoint"]).To(Equal("https://example.com/webhook"))
				Expect(requestBody["filter"]).To(Equal(true))
				
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(result)
			})

			options := sailhouse.NewRegisterOptions(
				sailhouse.WithFilter(sailhouse.NewSimpleFilter(true)),
			)

			response, err := adminClient.RegisterPushSubscription(
				ctx,
				"test-topic",
				"test-sub",
				"https://example.com/webhook",
				options,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(response.Outcome).To(Equal("created"))
		})

		It("should register with complex filter", func() {
			result := sailhouse.CreateTestRegisterResult("updated")
			
			mockServer.RegisterHandler("PUT", "/topics/test-topic/subscriptions/test-sub", func(w http.ResponseWriter, r *http.Request) {
				var requestBody map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&requestBody)
				Expect(err).NotTo(HaveOccurred())
				
				filter := requestBody["filter"].(map[string]interface{})
				Expect(filter["operator"]).To(Equal("AND"))
				
				filters := filter["filters"].([]interface{})
				Expect(filters).To(HaveLen(2))
				
				firstFilter := filters[0].(map[string]interface{})
				Expect(firstFilter["path"]).To(Equal("data.user.type"))
				Expect(firstFilter["condition"]).To(Equal("equals"))
				Expect(firstFilter["value"]).To(Equal("premium"))
				
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(result)
			})

			complexFilter := sailhouse.NewComplexFilter("AND",
				sailhouse.NewFilterCondition("data.user.type", "equals", "premium"),
				sailhouse.NewFilterCondition("data.amount", "greater_than", "100"),
			)

			options := sailhouse.NewRegisterOptions(
				sailhouse.WithFilter(complexFilter),
			)

			response, err := adminClient.RegisterPushSubscription(
				ctx,
				"test-topic",
				"test-sub",
				"https://example.com/webhook",
				options,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(response.Outcome).To(Equal("updated"))
		})

		It("should register with rate limit", func() {
			result := sailhouse.CreateTestRegisterResult("created")
			
			mockServer.RegisterHandler("PUT", "/topics/test-topic/subscriptions/test-sub", func(w http.ResponseWriter, r *http.Request) {
				var requestBody map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&requestBody)
				Expect(err).NotTo(HaveOccurred())
				
				Expect(requestBody["rate_limit"]).To(Equal("10/1m"))
				
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(result)
			})

			options := sailhouse.NewRegisterOptions(
				sailhouse.WithRateLimit("10/1m"),
			)

			response, err := adminClient.RegisterPushSubscription(
				ctx,
				"test-topic",
				"test-sub",
				"https://example.com/webhook",
				options,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(response.Outcome).To(Equal("created"))
		})

		It("should register with deduplication", func() {
			result := sailhouse.CreateTestRegisterResult("created")
			
			mockServer.RegisterHandler("PUT", "/topics/test-topic/subscriptions/test-sub", func(w http.ResponseWriter, r *http.Request) {
				var requestBody map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&requestBody)
				Expect(err).NotTo(HaveOccurred())
				
				Expect(requestBody["deduplication"]).To(Equal("5m"))
				
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(result)
			})

			options := sailhouse.NewRegisterOptions(
				sailhouse.WithDeduplication("5m"),
			)

			response, err := adminClient.RegisterPushSubscription(
				ctx,
				"test-topic",
				"test-sub",
				"https://example.com/webhook",
				options,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(response.Outcome).To(Equal("created"))
		})

		It("should register with all options combined", func() {
			result := sailhouse.CreateTestRegisterResult("updated")
			
			mockServer.RegisterHandler("PUT", "/topics/test-topic/subscriptions/test-sub", func(w http.ResponseWriter, r *http.Request) {
				var requestBody map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&requestBody)
				Expect(err).NotTo(HaveOccurred())
				
				// Verify all fields are present
				Expect(requestBody["type"]).To(Equal("push"))
				Expect(requestBody["endpoint"]).To(Equal("https://example.com/webhook"))
				Expect(requestBody["rate_limit"]).To(Equal("50/1h"))
				Expect(requestBody["deduplication"]).To(Equal("10m"))
				
				filter := requestBody["filter"].(map[string]interface{})
				Expect(filter["operator"]).To(Equal("OR"))
				
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(result)
			})

			complexFilter := sailhouse.NewComplexFilter("OR",
				sailhouse.NewFilterCondition("data.priority", "equals", "high"),
				sailhouse.NewFilterCondition("data.urgent", "equals", "true"),
			)

			options := sailhouse.NewRegisterOptions(
				sailhouse.WithFilter(complexFilter),
				sailhouse.WithRateLimit("50/1h"),
				sailhouse.WithDeduplication("10m"),
			)

			response, err := adminClient.RegisterPushSubscription(
				ctx,
				"test-topic",
				"test-sub",
				"https://example.com/webhook",
				options,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(response.Outcome).To(Equal("updated"))
		})

		It("should handle nil filter correctly", func() {
			result := sailhouse.CreateTestRegisterResult("created")
			
			mockServer.RegisterHandler("PUT", "/topics/test-topic/subscriptions/test-sub", func(w http.ResponseWriter, r *http.Request) {
				var requestBody map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&requestBody)
				Expect(err).NotTo(HaveOccurred())
				
				// Filter should not be present in request when nil
				_, hasFilter := requestBody["filter"]
				Expect(hasFilter).To(BeFalse())
				
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(result)
			})

			options := &sailhouse.RegisterPushSubscriptionOptions{
				Filter: nil,
			}

			response, err := adminClient.RegisterPushSubscription(
				ctx,
				"test-topic",
				"test-sub",
				"https://example.com/webhook",
				options,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(response.Outcome).To(Equal("created"))
		})
	})

	Describe("filter helper functions", func() {
		It("should create simple boolean filters", func() {
			trueFilter := sailhouse.NewSimpleFilter(true)
			falseFilter := sailhouse.NewSimpleFilter(false)
			
			Expect(trueFilter).To(Equal(true))
			Expect(falseFilter).To(Equal(false))
		})

		It("should create complex filters with conditions", func() {
			condition1 := sailhouse.NewFilterCondition("data.user.id", "equals", "123")
			condition2 := sailhouse.NewFilterCondition("data.amount", "greater_than", "50")
			
			complexFilter := sailhouse.NewComplexFilter("AND", condition1, condition2)
			
			Expect(complexFilter.Operator).To(Equal("AND"))
			Expect(complexFilter.Filters).To(HaveLen(2))
			Expect(complexFilter.Filters[0].Path).To(Equal("data.user.id"))
			Expect(complexFilter.Filters[0].Condition).To(Equal("equals"))
			Expect(complexFilter.Filters[0].Value).To(Equal("123"))
		})

		It("should create filter conditions correctly", func() {
			condition := sailhouse.NewFilterCondition("path.to.field", "not_equals", "value")
			
			Expect(condition.Path).To(Equal("path.to.field"))
			Expect(condition.Condition).To(Equal("not_equals"))
			Expect(condition.Value).To(Equal("value"))
		})
	})

	Describe("options builder pattern", func() {
		It("should build options using functional pattern", func() {
			complexFilter := sailhouse.NewComplexFilter("AND",
				sailhouse.NewFilterCondition("data.type", "equals", "important"),
			)
			
			options := sailhouse.NewRegisterOptions(
				sailhouse.WithFilter(complexFilter),
				sailhouse.WithRateLimit("25/30m"),
				sailhouse.WithDeduplication("15m"),
			)
			
			Expect(options.Filter).To(Equal(complexFilter))
			Expect(*options.RateLimit).To(Equal("25/30m"))
			Expect(*options.Deduplication).To(Equal("15m"))
		})

		It("should handle empty options", func() {
			options := sailhouse.NewRegisterOptions()
			
			Expect(options.Filter).To(BeNil())
			Expect(options.RateLimit).To(BeNil())
			Expect(options.Deduplication).To(BeNil())
		})
	})
})