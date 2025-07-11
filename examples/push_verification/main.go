package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/sailhouse/sdk-go/sailhouse"
)

// WebhookPayload represents the structure of incoming webhook data
type WebhookPayload struct {
	ID        string                 `json:"id"`
	Data      map[string]interface{} `json:"data"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp string                 `json:"timestamp"`
}

func main() {
	// Get the webhook secret from environment variable
	secret := os.Getenv("SAILHOUSE_WEBHOOK_SECRET")
	if secret == "" {
		log.Fatal("SAILHOUSE_WEBHOOK_SECRET environment variable is required")
	}

	// Create a Sailhouse client
	client := sailhouse.NewSailhouseClient("not-needed-for-verification")

	// Create a push subscription verifier instance (reusable)
	verifier, err := client.CreatePushSubscriptionVerifier(secret)
	if err != nil {
		log.Fatalf("Failed to create verifier: %v", err)
	}

	// Set up HTTP handlers
	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		handleWebhook(w, r, verifier)
	})

	http.HandleFunc("/webhook-simple", func(w http.ResponseWriter, r *http.Request) {
		handleWebhookSimple(w, r, secret)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Webhook server is running\n")
	})

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Starting webhook server on port %s...\n", port)
	fmt.Println("Endpoints:")
	fmt.Println("  POST /webhook         - Webhook handler with verifier instance")
	fmt.Println("  POST /webhook-simple  - Webhook handler with one-off verification")
	fmt.Println("  GET  /health          - Health check")
	fmt.Println()
	fmt.Println("Set SAILHOUSE_WEBHOOK_SECRET environment variable to your webhook secret")

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// handleWebhook demonstrates using a reusable verifier instance
func handleWebhook(w http.ResponseWriter, r *http.Request, verifier *sailhouse.PushSubscriptionVerifier) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the raw body (important: don't parse as JSON first!)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	bodyString := string(body)

	// Get the signature from headers
	signature := r.Header.Get("Sailhouse-Signature")
	if signature == "" {
		http.Error(w, "Missing Sailhouse-Signature header", http.StatusBadRequest)
		return
	}

	// Verify the signature
	options := &sailhouse.VerificationOptions{
		Tolerance: 300, // 5 minutes tolerance
	}

	err = verifier.VerifySignature(signature, bodyString, options)
	if err != nil {
		log.Printf("Signature verification failed: %v", err)
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse the webhook payload
	var payload WebhookPayload
	err = json.Unmarshal(body, &payload)
	if err != nil {
		log.Printf("Failed to parse webhook payload: %v", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Process the webhook
	processWebhookPayload(payload)

	// Respond with success
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Webhook processed successfully\n")
}

// handleWebhookSimple demonstrates using the one-off verification function
func handleWebhookSimple(w http.ResponseWriter, r *http.Request, secret string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the raw body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	bodyString := string(body)

	// Get the signature from headers
	signature := r.Header.Get("Sailhouse-Signature")
	if signature == "" {
		http.Error(w, "Missing Sailhouse-Signature header", http.StatusBadRequest)
		return
	}

	// Verify using the convenience function
	options := &sailhouse.VerificationOptions{Tolerance: 300}
	err = sailhouse.VerifyPushSubscriptionSignature(secret, signature, bodyString, options)
	if err != nil {
		log.Printf("Signature verification failed: %v", err)
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse and process the webhook
	var payload WebhookPayload
	err = json.Unmarshal(body, &payload)
	if err != nil {
		log.Printf("Failed to parse webhook payload: %v", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	processWebhookPayload(payload)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Webhook processed successfully (simple)\n")
}

// processWebhookPayload demonstrates how to handle the verified webhook data
func processWebhookPayload(payload WebhookPayload) {
	fmt.Printf("Processing webhook event:\n")
	fmt.Printf("  ID: %s\n", payload.ID)
	fmt.Printf("  Timestamp: %s\n", payload.Timestamp)

	// Process the data
	if payload.Data != nil {
		fmt.Printf("  Data: %+v\n", payload.Data)

		// Example: Handle different event types
		if eventType, ok := payload.Data["event_type"].(string); ok {
			switch eventType {
			case "user.created":
				handleUserCreated(payload.Data)
			case "payment.completed":
				handlePaymentCompleted(payload.Data)
			case "order.shipped":
				handleOrderShipped(payload.Data)
			default:
				fmt.Printf("  Unknown event type: %s\n", eventType)
			}
		}
	}

	// Process metadata if present
	if len(payload.Metadata) > 0 {
		fmt.Printf("  Metadata: %+v\n", payload.Metadata)
	}

	fmt.Println("  ✓ Event processed successfully")
}

func handleUserCreated(data map[string]interface{}) {
	fmt.Println("  → Handling user creation event")
	if userID, ok := data["user_id"]; ok {
		fmt.Printf("    New user ID: %v\n", userID)
	}
	if email, ok := data["email"]; ok {
		fmt.Printf("    User email: %v\n", email)
	}
	// Add your user creation logic here
}

func handlePaymentCompleted(data map[string]interface{}) {
	fmt.Println("  → Handling payment completion event")
	if amount, ok := data["amount"]; ok {
		fmt.Printf("    Payment amount: %v\n", amount)
	}
	if currency, ok := data["currency"]; ok {
		fmt.Printf("    Currency: %v\n", currency)
	}
	// Add your payment processing logic here
}

func handleOrderShipped(data map[string]interface{}) {
	fmt.Println("  → Handling order shipped event")
	if orderID, ok := data["order_id"]; ok {
		fmt.Printf("    Order ID: %v\n", orderID)
	}
	if trackingNumber, ok := data["tracking_number"]; ok {
		fmt.Printf("    Tracking: %v\n", trackingNumber)
	}
	// Add your order tracking logic here
}

/*
Example usage:

1. Set your webhook secret:
   export SAILHOUSE_WEBHOOK_SECRET="whsec_your_secret_here"

2. Run the server:
   go run main.go

3. Test with curl (you'll need a valid signature):
   curl -X POST http://localhost:8080/webhook \
     -H "Content-Type: application/json" \
     -H "Sailhouse-Signature: t=1699564800,v1=your_signature_here" \
     -d '{"id":"evt_123","data":{"event_type":"user.created","user_id":"user_456","email":"test@example.com"},"timestamp":"2023-11-09T16:00:00Z"}'

4. The server will:
   - Verify the HMAC signature
   - Parse the JSON payload
   - Process the event based on event_type
   - Return success or error response

Important notes:
- Always read the raw body before parsing JSON
- Use the Sailhouse-Signature header for verification
- Set appropriate tolerance for timestamp validation
- Handle verification errors gracefully
- Log verification failures for debugging
*/
