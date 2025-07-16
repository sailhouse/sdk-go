package sailhouse_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sailhouse/sdk-go/sailhouse"
)

var _ = Describe("PushSubscriptionVerifier", func() {
	var (
		secret   string
		verifier *sailhouse.PushSubscriptionVerifier
	)

	BeforeEach(func() {
		secret = "test-secret-key"
		var err error
		verifier, err = sailhouse.NewPushSubscriptionVerifier(secret)
		Expect(err).NotTo(HaveOccurred())
		Expect(verifier).NotTo(BeNil())
	})

	Describe("constructor", func() {
		It("should create verifier with secret", func() {
			v, err := sailhouse.NewPushSubscriptionVerifier("valid-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(v).NotTo(BeNil())
		})

		It("should return error for empty secret", func() {
			v, err := sailhouse.NewPushSubscriptionVerifier("")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("secret is required"))
			Expect(v).To(BeNil())
		})
	})

	Describe("signature verification", func() {
		// Helper function to create a valid signature
		createValidSignature := func(timestamp int64, body string) string {
			payload := fmt.Sprintf("%d.%s", timestamp, body)
			h := hmac.New(sha256.New, []byte(secret))
			h.Write([]byte(payload))
			signature := hex.EncodeToString(h.Sum(nil))
			return fmt.Sprintf("t=%d,v1=%s", timestamp, signature)
		}

		It("should verify valid signature", func() {
			timestamp := time.Now().Unix()
			body := `{"event": "test", "data": {"message": "hello"}}`
			signature := createValidSignature(timestamp, body)

			err := verifier.VerifySignature(signature, body, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should verify signature with custom tolerance", func() {
			timestamp := time.Now().Unix() - 500 // 500 seconds ago
			body := `{"event": "test"}`
			signature := createValidSignature(timestamp, body)

			options := &sailhouse.VerificationOptions{Tolerance: 600}
			err := verifier.VerifySignature(signature, body, options)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reject invalid signature", func() {
			timestamp := time.Now().Unix()
			body := `{"event": "test"}`
			signature := fmt.Sprintf("t=%d,v1=invalidsignature", timestamp)

			err := verifier.VerifySignature(signature, body, nil)
			Expect(err).To(HaveOccurred())

			verificationErr, ok := err.(*sailhouse.PushSubscriptionVerificationError)
			Expect(ok).To(BeTrue())
			Expect(verificationErr.Code).To(Equal("INVALID_SIGNATURE"))
		})

		It("should reject expired timestamp", func() {
			timestamp := time.Now().Unix() - 400 // 400 seconds ago
			body := `{"event": "test"}`
			signature := createValidSignature(timestamp, body)

			err := verifier.VerifySignature(signature, body, nil)
			Expect(err).To(HaveOccurred())

			verificationErr, ok := err.(*sailhouse.PushSubscriptionVerificationError)
			Expect(ok).To(BeTrue())
			Expect(verificationErr.Code).To(Equal("TIMESTAMP_TOO_OLD"))
		})

		It("should handle empty body", func() {
			timestamp := time.Now().Unix()
			body := ""
			signature := createValidSignature(timestamp, body)

			err := verifier.VerifySignature(signature, body, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle complex JSON body", func() {
			timestamp := time.Now().Unix()
			body := `{
				"id": "event-123",
				"data": {
					"user": {"name": "John Doe", "email": "john@example.com"},
					"action": "purchase",
					"items": [
						{"id": 1, "name": "Product A"},
						{"id": 2, "name": "Product B"}
					]
				},
				"metadata": {"source": "api", "version": "1.0"}
			}`
			signature := createValidSignature(timestamp, body)

			err := verifier.VerifySignature(signature, body, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should be sensitive to body changes", func() {
			timestamp := time.Now().Unix()
			originalBody := `{"event": "test", "data": {"message": "hello"}}`
			modifiedBody := `{"event": "test", "data": {"message": "hello!"}}`
			signature := createValidSignature(timestamp, originalBody)

			// Should pass with original body
			err := verifier.VerifySignature(signature, originalBody, nil)
			Expect(err).NotTo(HaveOccurred())

			// Should fail with modified body
			err = verifier.VerifySignature(signature, modifiedBody, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should handle unicode characters in body", func() {
			timestamp := time.Now().Unix()
			body := `{"message": "Hello 🌍 World! 你好"}`
			signature := createValidSignature(timestamp, body)

			err := verifier.VerifySignature(signature, body, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("signature header parsing", func() {
		It("should parse valid signature header", func() {
			// We'll test this through the public VerifySignature method
			timestamp := time.Now().Unix()
			body := `{"test": "data"}`
			signature := fmt.Sprintf("t=%d,v1=abc123", timestamp)

			err := verifier.VerifySignature(signature, body, nil)
			Expect(err).To(HaveOccurred()) // Will fail due to invalid signature, but header parsing should work

			// Check that it's not a header parsing error
			verificationErr, ok := err.(*sailhouse.PushSubscriptionVerificationError)
			Expect(ok).To(BeTrue())
			Expect(verificationErr.Code).NotTo(Equal("INVALID_SIGNATURE_FORMAT"))
		})

		It("should parse header with spaces", func() {
			timestamp := time.Now().Unix()
			body := `{"test": "data"}`
			// Note the space after the comma
			signature := fmt.Sprintf("t=%d, v1=abc123", timestamp)

			err := verifier.VerifySignature(signature, body, nil)
			Expect(err).To(HaveOccurred())

			verificationErr, ok := err.(*sailhouse.PushSubscriptionVerificationError)
			Expect(ok).To(BeTrue())
			Expect(verificationErr.Code).NotTo(Equal("INVALID_SIGNATURE_FORMAT"))
		})

		It("should reject missing header", func() {
			err := verifier.VerifySignature("", "body", nil)
			Expect(err).To(HaveOccurred())

			verificationErr, ok := err.(*sailhouse.PushSubscriptionVerificationError)
			Expect(ok).To(BeTrue())
			Expect(verificationErr.Code).To(Equal("MISSING_SIGNATURE_HEADER"))
		})

		It("should reject missing timestamp", func() {
			signature := "v1=abc123"
			err := verifier.VerifySignature(signature, "body", nil)
			Expect(err).To(HaveOccurred())

			verificationErr, ok := err.(*sailhouse.PushSubscriptionVerificationError)
			Expect(ok).To(BeTrue())
			Expect(verificationErr.Code).To(Equal("INVALID_SIGNATURE_FORMAT"))
		})

		It("should reject missing signature", func() {
			timestamp := time.Now().Unix()
			signature := fmt.Sprintf("t=%d", timestamp)
			err := verifier.VerifySignature(signature, "body", nil)
			Expect(err).To(HaveOccurred())

			verificationErr, ok := err.(*sailhouse.PushSubscriptionVerificationError)
			Expect(ok).To(BeTrue())
			Expect(verificationErr.Code).To(Equal("INVALID_SIGNATURE_FORMAT"))
		})

		It("should reject invalid timestamp", func() {
			signature := "t=invalid,v1=abc123"
			err := verifier.VerifySignature(signature, "body", nil)
			Expect(err).To(HaveOccurred())

			verificationErr, ok := err.(*sailhouse.PushSubscriptionVerificationError)
			Expect(ok).To(BeTrue())
			Expect(verificationErr.Code).To(Equal("INVALID_TIMESTAMP"))
		})

		It("should handle extra elements in header", func() {
			timestamp := time.Now().Unix()
			body := `{"test": "data"}`
			signature := fmt.Sprintf("t=%d,v1=abc123,extra=value", timestamp)

			err := verifier.VerifySignature(signature, body, nil)
			Expect(err).To(HaveOccurred())

			verificationErr, ok := err.(*sailhouse.PushSubscriptionVerificationError)
			Expect(ok).To(BeTrue())
			Expect(verificationErr.Code).NotTo(Equal("INVALID_SIGNATURE_FORMAT"))
		})
	})

	Describe("timestamp validation", func() {
		It("should accept current timestamp", func() {
			timestamp := time.Now().Unix()
			body := `{"test": "data"}`

			// Create valid signature for current time
			payload := fmt.Sprintf("%d.%s", timestamp, body)
			h := hmac.New(sha256.New, []byte(secret))
			h.Write([]byte(payload))
			sig := hex.EncodeToString(h.Sum(nil))
			signature := fmt.Sprintf("t=%d,v1=%s", timestamp, sig)

			err := verifier.VerifySignature(signature, body, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reject future timestamp", func() {
			timestamp := time.Now().Unix() + 100 // 100 seconds in future
			body := `{"test": "data"}`

			payload := fmt.Sprintf("%d.%s", timestamp, body)
			h := hmac.New(sha256.New, []byte(secret))
			h.Write([]byte(payload))
			sig := hex.EncodeToString(h.Sum(nil))
			signature := fmt.Sprintf("t=%d,v1=%s", timestamp, sig)

			err := verifier.VerifySignature(signature, body, nil)
			Expect(err).To(HaveOccurred())

			verificationErr, ok := err.(*sailhouse.PushSubscriptionVerificationError)
			Expect(ok).To(BeTrue())
			Expect(verificationErr.Code).To(Equal("TIMESTAMP_TOO_OLD"))
		})

		It("should handle zero tolerance", func() {
			timestamp := time.Now().Unix() - 1 // 1 second ago
			body := `{"test": "data"}`

			payload := fmt.Sprintf("%d.%s", timestamp, body)
			h := hmac.New(sha256.New, []byte(secret))
			h.Write([]byte(payload))
			sig := hex.EncodeToString(h.Sum(nil))
			signature := fmt.Sprintf("t=%d,v1=%s", timestamp, sig)

			options := &sailhouse.VerificationOptions{Tolerance: 0}
			err := verifier.VerifySignature(signature, body, options)
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("Convenience Functions", func() {
	var secret string

	BeforeEach(func() {
		secret = "test-secret-key"
	})

	createValidSignature := func(timestamp int64, body string) string {
		payload := fmt.Sprintf("%d.%s", timestamp, body)
		h := hmac.New(sha256.New, []byte(secret))
		h.Write([]byte(payload))
		signature := hex.EncodeToString(h.Sum(nil))
		return fmt.Sprintf("t=%d,v1=%s", timestamp, signature)
	}

	Describe("VerifyPushSubscriptionSignature", func() {
		It("should verify valid signature", func() {
			timestamp := time.Now().Unix()
			body := `{"event": "test"}`
			signature := createValidSignature(timestamp, body)

			err := sailhouse.VerifyPushSubscriptionSignature(secret, signature, body, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error for invalid signature", func() {
			timestamp := time.Now().Unix()
			body := `{"event": "test"}`
			signature := fmt.Sprintf("t=%d,v1=invalidsignature", timestamp)

			err := sailhouse.VerifyPushSubscriptionSignature(secret, signature, body, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should respect custom tolerance", func() {
			timestamp := time.Now().Unix() - 500 // 500 seconds ago
			body := `{"event": "test"}`
			signature := createValidSignature(timestamp, body)

			options := &sailhouse.VerificationOptions{Tolerance: 600}
			err := sailhouse.VerifyPushSubscriptionSignature(secret, signature, body, options)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("VerifyPushSubscriptionSignatureSafe", func() {
		It("should return true for valid signature", func() {
			timestamp := time.Now().Unix()
			body := `{"event": "test"}`
			signature := createValidSignature(timestamp, body)

			result := sailhouse.VerifyPushSubscriptionSignatureSafe(secret, signature, body, nil)
			Expect(result).To(BeTrue())
		})

		It("should return false for invalid signature instead of error", func() {
			timestamp := time.Now().Unix()
			body := `{"event": "test"}`
			signature := fmt.Sprintf("t=%d,v1=invalidsignature", timestamp)

			result := sailhouse.VerifyPushSubscriptionSignatureSafe(secret, signature, body, nil)
			Expect(result).To(BeFalse())
		})

		It("should return false for expired timestamp instead of error", func() {
			timestamp := time.Now().Unix() - 400 // 400 seconds ago
			body := `{"event": "test"}`
			signature := createValidSignature(timestamp, body)

			result := sailhouse.VerifyPushSubscriptionSignatureSafe(secret, signature, body, nil)
			Expect(result).To(BeFalse())
		})

		It("should return false for malformed signature instead of error", func() {
			body := `{"event": "test"}`
			signature := "invalid-header"

			result := sailhouse.VerifyPushSubscriptionSignatureSafe(secret, signature, body, nil)
			Expect(result).To(BeFalse())
		})
	})
})

var _ = Describe("SailhouseClient HMAC Methods", func() {
	var (
		client *sailhouse.SailhouseClient
		secret string
	)

	BeforeEach(func() {
		client = sailhouse.NewSailhouseClient("test-token")
		secret = "test-secret-key"
	})

	createValidSignature := func(timestamp int64, body string) string {
		payload := fmt.Sprintf("%d.%s", timestamp, body)
		h := hmac.New(sha256.New, []byte(secret))
		h.Write([]byte(payload))
		signature := hex.EncodeToString(h.Sum(nil))
		return fmt.Sprintf("t=%d,v1=%s", timestamp, signature)
	}

	Describe("VerifyPushSubscription", func() {
		It("should verify valid signature", func() {
			timestamp := time.Now().Unix()
			body := `{"event": "test"}`
			signature := createValidSignature(timestamp, body)

			err := client.VerifyPushSubscription(signature, body, secret, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error for invalid signature", func() {
			timestamp := time.Now().Unix()
			body := `{"event": "test"}`
			signature := fmt.Sprintf("t=%d,v1=invalidsignature", timestamp)

			err := client.VerifyPushSubscription(signature, body, secret, nil)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("CreatePushSubscriptionVerifier", func() {
		It("should create verifier instance", func() {
			verifier, err := client.CreatePushSubscriptionVerifier(secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(verifier).NotTo(BeNil())

			// Test that the verifier works
			timestamp := time.Now().Unix()
			body := `{"event": "test"}`
			signature := createValidSignature(timestamp, body)

			err = verifier.VerifySignature(signature, body, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error for empty secret", func() {
			verifier, err := client.CreatePushSubscriptionVerifier("")
			Expect(err).To(HaveOccurred())
			Expect(verifier).To(BeNil())
		})
	})
})

var _ = Describe("Real-world Examples", func() {
	var secret string

	BeforeEach(func() {
		secret = "whsec_test_secret"
	})

	createValidSignature := func(timestamp int64, body string) string {
		payload := fmt.Sprintf("%d.%s", timestamp, body)
		h := hmac.New(sha256.New, []byte(secret))
		h.Write([]byte(payload))
		signature := hex.EncodeToString(h.Sum(nil))
		return fmt.Sprintf("t=%d,v1=%s", timestamp, signature)
	}

	It("should handle example from documentation", func() {
		timestamp := time.Now().Unix() - 100 // 100 seconds ago
		body := `{"id":"evt_123","data":{"user":"john@example.com","action":"signup"},"timestamp":"2023-11-09T16:00:00Z"}`

		signature := createValidSignature(timestamp, body)
		verifier, err := sailhouse.NewPushSubscriptionVerifier(secret)
		Expect(err).NotTo(HaveOccurred())

		err = verifier.VerifySignature(signature, body, nil)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should handle different content types", func() {
		timestamp := time.Now().Unix()
		verifier, err := sailhouse.NewPushSubscriptionVerifier(secret)
		Expect(err).NotTo(HaveOccurred())

		testCases := []string{
			`{"simple": "object"}`,
			`{"nested": {"deep": {"value": 123}}}`,
			`{"array": [1, 2, 3, "string", {"nested": true}]}`,
			`{"unicode": "测试 🚀 émoji"}`,
			`{"number": 123.456, "boolean": true, "null": null}`,
			`[]`, // Empty array
			`{}`, // Empty object
		}

		for _, body := range testCases {
			signature := createValidSignature(timestamp, body)
			err := verifier.VerifySignature(signature, body, nil)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("should demonstrate Express.js raw body scenario", func() {
		// Simulate Express with express.raw({ type: 'application/json' })
		timestamp := time.Now().Unix()
		jsonData := map[string]interface{}{
			"event": "user.created",
			"data":  map[string]interface{}{"id": 123, "email": "user@example.com"},
		}

		// In real Express app, you'd get the raw body as Buffer
		body := `{"event":"user.created","data":{"id":123,"email":"user@example.com"}}`

		signature := createValidSignature(timestamp, body)
		verifier, err := sailhouse.NewPushSubscriptionVerifier(secret)
		Expect(err).NotTo(HaveOccurred())

		err = verifier.VerifySignature(signature, body, nil)
		Expect(err).NotTo(HaveOccurred())

		// Just to use jsonData to avoid compiler warning
		Expect(jsonData).NotTo(BeNil())
	})
})
