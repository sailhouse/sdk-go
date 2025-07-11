package sailhouse_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sailhouse/sdk-go/sailhouse"
)

var _ = Describe("SailhouseClient", func() {
	var (
		mockServer *sailhouse.MockSailhouseServer
		client     *sailhouse.SailhouseClient
	)

	BeforeEach(func() {
		mockServer = sailhouse.NewMockSailhouseServer()
		client = sailhouse.NewSailhouseClient("test-token")
	})

	AfterEach(func() {
		mockServer.Close()
	})

	Describe("basic functionality", func() {
		It("should create a client", func() {
			Expect(client).NotTo(BeNil())
		})

		It("should have test helpers working", func() {
			event := sailhouse.CreateTestEvent("test-id", map[string]interface{}{
				"message": "hello",
			})
			Expect(event.ID).To(Equal("test-id"))
			Expect(event.Data["message"]).To(Equal("hello"))
		})
	})

	Describe("mock server", func() {
		It("should handle registered routes", func() {
			mockServer.RegisterPullEventHandler("test-topic", "test-sub", nil)
			Expect(mockServer.URL).To(ContainSubstring("http://"))
		})
	})
})