package sailhouse

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// PushSubscriptionVerificationError represents an error that occurred during push subscription verification
type PushSubscriptionVerificationError struct {
	Message string
	Code    string
}

func (e *PushSubscriptionVerificationError) Error() string {
	return e.Message
}

// NewPushSubscriptionVerificationError creates a new verification error
func NewPushSubscriptionVerificationError(message, code string) *PushSubscriptionVerificationError {
	return &PushSubscriptionVerificationError{
		Message: message,
		Code:    code,
	}
}

// SignatureComponents represents the parsed components of a signature header
type SignatureComponents struct {
	Timestamp int64
	Signature string
}

// PushSubscriptionHeaders represents the headers expected from a push subscription request
type PushSubscriptionHeaders struct {
	SailhouseSignature string
	Identifier         string
	EventID            string
}

// PushSubscriptionPayload represents the structure of a push subscription payload
type PushSubscriptionPayload struct {
	Data      interface{}            `json:"data"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	ID        string                 `json:"id"`
	Timestamp string                 `json:"timestamp"`
}

// VerificationOptions contains options for signature verification
type VerificationOptions struct {
	// Tolerance for timestamp validation in seconds (default: 300)
	Tolerance int64
}

// PushSubscriptionVerifier provides functionality to verify push subscription signatures
type PushSubscriptionVerifier struct {
	secret string
}

// NewPushSubscriptionVerifier creates a new push subscription verifier
func NewPushSubscriptionVerifier(secret string) (*PushSubscriptionVerifier, error) {
	if secret == "" {
		return nil, fmt.Errorf("push subscription secret is required")
	}

	return &PushSubscriptionVerifier{
		secret: secret,
	}, nil
}

// VerifySignature verifies a push subscription signature
func (v *PushSubscriptionVerifier) VerifySignature(signature, body string, options *VerificationOptions) error {
	tolerance := int64(300) // default tolerance
	if options != nil {
		tolerance = options.Tolerance
		// If tolerance is 0 and options is provided, respect that 0 value
	}

	// Parse signature header
	components, err := v.parseSignatureHeader(signature)
	if err != nil {
		return err
	}

	// Validate timestamp
	if !v.isTimestampValid(components.Timestamp, tolerance) {
		return NewPushSubscriptionVerificationError(
			fmt.Sprintf("Request timestamp is too old. Maximum age: %d seconds", tolerance),
			"TIMESTAMP_TOO_OLD",
		)
	}

	// Calculate expected signature
	expectedSignature := v.calculateSignature(components.Timestamp, body)

	// Perform constant-time comparison
	if !v.constantTimeEqual(expectedSignature, components.Signature) {
		return NewPushSubscriptionVerificationError(
			"Signature verification failed",
			"INVALID_SIGNATURE",
		)
	}

	return nil
}

// parseSignatureHeader parses the Sailhouse-Signature header
func (v *PushSubscriptionVerifier) parseSignatureHeader(header string) (*SignatureComponents, error) {
	if header == "" {
		return nil, NewPushSubscriptionVerificationError(
			"Signature header is required",
			"MISSING_SIGNATURE_HEADER",
		)
	}

	elements := strings.Split(header, ",")
	var timestamp int64
	var signature string
	var hasTimestamp, hasSignature bool

	for _, element := range elements {
		trimmed := strings.TrimSpace(element)
		parts := strings.SplitN(trimmed, "=", 2)

		if len(parts) != 2 {
			continue
		}

		key, value := parts[0], parts[1]

		switch key {
		case "t":
			parsedTimestamp, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, NewPushSubscriptionVerificationError(
					"Invalid timestamp in signature header",
					"INVALID_TIMESTAMP",
				)
			}
			timestamp = parsedTimestamp
			hasTimestamp = true
		case "v1":
			signature = value
			hasSignature = true
		}
	}

	if !hasTimestamp || !hasSignature {
		return nil, NewPushSubscriptionVerificationError(
			"Invalid signature header format. Expected format: t=<timestamp>,v1=<signature>",
			"INVALID_SIGNATURE_FORMAT",
		)
	}

	return &SignatureComponents{
		Timestamp: timestamp,
		Signature: signature,
	}, nil
}

// isTimestampValid checks if the timestamp is within the tolerance
func (v *PushSubscriptionVerifier) isTimestampValid(timestamp, tolerance int64) bool {
	currentTime := time.Now().Unix()
	return currentTime-timestamp <= tolerance && timestamp <= currentTime
}

// calculateSignature calculates the HMAC-SHA256 signature for the payload
func (v *PushSubscriptionVerifier) calculateSignature(timestamp int64, body string) string {
	payload := fmt.Sprintf("%d.%s", timestamp, body)
	h := hmac.New(sha256.New, []byte(v.secret))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

// constantTimeEqual performs constant-time comparison to prevent timing attacks
func (v *PushSubscriptionVerifier) constantTimeEqual(expected, actual string) bool {
	expectedBytes, err := hex.DecodeString(expected)
	if err != nil {
		return false
	}

	actualBytes, err := hex.DecodeString(actual)
	if err != nil {
		return false
	}

	return subtle.ConstantTimeCompare(expectedBytes, actualBytes) == 1
}

// Convenience functions

// VerifyPushSubscriptionSignature is a convenience function for one-off signature verification
func VerifyPushSubscriptionSignature(secret, signature, body string, options *VerificationOptions) error {
	verifier, err := NewPushSubscriptionVerifier(secret)
	if err != nil {
		return err
	}

	return verifier.VerifySignature(signature, body, options)
}

// VerifyPushSubscriptionSignatureSafe is a safe verification that returns a boolean instead of an error
func VerifyPushSubscriptionSignatureSafe(secret, signature, body string, options *VerificationOptions) bool {
	err := VerifyPushSubscriptionSignature(secret, signature, body, options)
	return err == nil
}

// SailhouseClient methods for push subscription verification

// VerifyPushSubscription verifies a push subscription signature using the client
func (c *SailhouseClient) VerifyPushSubscription(signature, body, secret string, options *VerificationOptions) error {
	return VerifyPushSubscriptionSignature(secret, signature, body, options)
}

// CreatePushSubscriptionVerifier creates a push subscription verifier instance
func (c *SailhouseClient) CreatePushSubscriptionVerifier(secret string) (*PushSubscriptionVerifier, error) {
	return NewPushSubscriptionVerifier(secret)
}
