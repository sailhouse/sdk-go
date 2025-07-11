package sailhouse_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sailhouse/sdk-go/sailhouse"
)

var _ = Describe("Event", func() {
	Describe("metadata handling", func() {
		It("should include metadata when present", func() {
			metadata := map[string]interface{}{
				"source":   "test-service",
				"version":  "1.0.0",
				"priority": "high",
			}

			event := sailhouse.CreateTestEventWithMetadata(
				"test-id",
				map[string]interface{}{"message": "hello"},
				metadata,
			)

			Expect(event.Metadata).To(Equal(metadata))
			Expect(event.Metadata["source"]).To(Equal("test-service"))
			Expect(event.Metadata["version"]).To(Equal("1.0.0"))
			Expect(event.Metadata["priority"]).To(Equal("high"))
		})

		It("should handle nil metadata gracefully", func() {
			event := sailhouse.CreateTestEvent(
				"test-id",
				map[string]interface{}{"message": "hello"},
			)

			Expect(event.Metadata).To(BeNil())
		})

		It("should marshal and unmarshal correctly with metadata", func() {
			originalEvent := sailhouse.CreateTestEventWithMetadata(
				"test-id",
				map[string]interface{}{"message": "hello"},
				map[string]interface{}{
					"source": "test-service",
					"count":  42,
				},
			)

			// Marshal to JSON
			jsonData, err := json.Marshal(originalEvent)
			Expect(err).NotTo(HaveOccurred())

			// Unmarshal back
			var unmarshaledEvent sailhouse.Event
			err = json.Unmarshal(jsonData, &unmarshaledEvent)
			Expect(err).NotTo(HaveOccurred())

			// Verify data integrity
			Expect(unmarshaledEvent.ID).To(Equal("test-id"))
			Expect(unmarshaledEvent.Data["message"]).To(Equal("hello"))
			Expect(unmarshaledEvent.Metadata["source"]).To(Equal("test-service"))
			Expect(unmarshaledEvent.Metadata["count"]).To(Equal(float64(42))) // JSON numbers become float64
		})

		It("should marshal correctly without metadata (omitempty)", func() {
			event := sailhouse.CreateTestEvent(
				"test-id",
				map[string]interface{}{"message": "hello"},
			)

			jsonData, err := json.Marshal(event)
			Expect(err).NotTo(HaveOccurred())

			// Verify metadata field is omitted
			var jsonMap map[string]interface{}
			err = json.Unmarshal(jsonData, &jsonMap)
			Expect(err).NotTo(HaveOccurred())

			_, hasMetadata := jsonMap["metadata"]
			Expect(hasMetadata).To(BeFalse())
		})
	})

	Describe("EventResponse", func() {
		It("should handle metadata in event responses", func() {
			metadata := map[string]interface{}{
				"timestamp": "2023-01-01T00:00:00Z",
				"retry":     3,
			}

			eventResponse := sailhouse.CreateTestEventResponseWithMetadata(
				"response-id",
				map[string]interface{}{"status": "success"},
				metadata,
			)

			Expect(eventResponse.Metadata).To(Equal(metadata))
			Expect(eventResponse.Metadata["timestamp"]).To(Equal("2023-01-01T00:00:00Z"))
			Expect(eventResponse.Metadata["retry"]).To(Equal(3))
		})

		It("should serialize event responses with metadata correctly", func() {
			eventResponse := sailhouse.CreateTestEventResponseWithMetadata(
				"response-id",
				map[string]interface{}{"status": "success"},
				map[string]interface{}{"source": "api"},
			)

			jsonData, err := json.Marshal(eventResponse)
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]interface{}
			err = json.Unmarshal(jsonData, &parsed)
			Expect(err).NotTo(HaveOccurred())

			Expect(parsed["id"]).To(Equal("response-id"))
			Expect(parsed["data"].(map[string]interface{})["status"]).To(Equal("success"))
			Expect(parsed["metadata"].(map[string]interface{})["source"]).To(Equal("api"))
		})

		It("should omit metadata when nil in event responses", func() {
			eventResponse := sailhouse.CreateTestEventResponse(
				"response-id",
				map[string]interface{}{"status": "success"},
			)

			jsonData, err := json.Marshal(eventResponse)
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]interface{}
			err = json.Unmarshal(jsonData, &parsed)
			Expect(err).NotTo(HaveOccurred())

			_, hasMetadata := parsed["metadata"]
			Expect(hasMetadata).To(BeFalse())
		})
	})

	Describe("complex metadata scenarios", func() {
		It("should handle nested metadata structures", func() {
			complexMetadata := map[string]interface{}{
				"service": map[string]interface{}{
					"name":    "user-service",
					"version": "2.1.0",
					"region":  "us-east-1",
				},
				"request": map[string]interface{}{
					"id":     "req-123",
					"method": "POST",
					"headers": map[string]interface{}{
						"content-type":  "application/json",
						"authorization": "Bearer token",
					},
				},
				"tags": []interface{}{"urgent", "customer-facing"},
			}

			event := sailhouse.CreateTestEventWithMetadata(
				"complex-id",
				map[string]interface{}{"action": "user_created"},
				complexMetadata,
			)

			// Verify nested structure access
			serviceInfo := event.Metadata["service"].(map[string]interface{})
			Expect(serviceInfo["name"]).To(Equal("user-service"))

			requestInfo := event.Metadata["request"].(map[string]interface{})
			headers := requestInfo["headers"].(map[string]interface{})
			Expect(headers["content-type"]).To(Equal("application/json"))

			tags := event.Metadata["tags"].([]interface{})
			Expect(tags).To(ContainElement("urgent"))
			Expect(tags).To(ContainElement("customer-facing"))
		})

		It("should preserve metadata types through JSON round-trip", func() {
			metadata := map[string]interface{}{
				"string_val": "hello",
				"int_val":    42,
				"float_val":  3.14,
				"bool_val":   true,
				"array_val":  []interface{}{1, 2, 3},
				"object_val": map[string]interface{}{"nested": "value"},
				"null_val":   nil,
			}

			event := sailhouse.CreateTestEventWithMetadata(
				"types-id",
				map[string]interface{}{"test": "data"},
				metadata,
			)

			// Marshal to JSON and back
			jsonData, err := json.Marshal(event)
			Expect(err).NotTo(HaveOccurred())

			var roundTripped sailhouse.Event
			err = json.Unmarshal(jsonData, &roundTripped)
			Expect(err).NotTo(HaveOccurred())

			// Verify types (noting JSON number conversion)
			Expect(roundTripped.Metadata["string_val"]).To(Equal("hello"))
			Expect(roundTripped.Metadata["int_val"]).To(Equal(float64(42))) // JSON converts to float64
			Expect(roundTripped.Metadata["float_val"]).To(Equal(3.14))
			Expect(roundTripped.Metadata["bool_val"]).To(Equal(true))
			Expect(roundTripped.Metadata["null_val"]).To(BeNil())

			arrayVal := roundTripped.Metadata["array_val"].([]interface{})
			Expect(arrayVal).To(HaveLen(3))
			Expect(arrayVal[0]).To(Equal(float64(1)))

			objectVal := roundTripped.Metadata["object_val"].(map[string]interface{})
			Expect(objectVal["nested"]).To(Equal("value"))
		})
	})
})
