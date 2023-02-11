package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	stripe "github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/webhook"
)

func createStripeWebhookHandler(webhookSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const MaxBodyBytes = int64(65536)
		r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading request body: %v\n", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		// Pass the request body and Stripe-Signature header to ConstructEvent, along
		// with the webhook signing key.
		event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"),
			webhookSecret)
		if err != nil {
			log.Printf("Error verifying webhook signature: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Unmarshal the event data into an appropriate struct depending on its Type
		switch event.Type {
		case "payment_intent.succeeded":
			var paymentIntent stripe.PaymentIntent
			err := json.Unmarshal(event.Data.Raw, &paymentIntent)
			if err != nil {
				log.Printf("Error parsing webhook JSON: %v\n", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			handlePaymentIntentSucceeded(paymentIntent)
		default:
			log.Printf("Unhandled event type: %v\n", event.Type)
		}

		w.WriteHeader(http.StatusOK)
	}
}

func handlePaymentIntentSucceeded(paymentIntent stripe.PaymentIntent) {
	email := paymentIntent.ReceiptEmail
	if email == "" {
		log.Printf("No email address on payment intent: %+v\n", paymentIntent)
		return
	}

	if err := sendEmail(email); err != nil {
		log.Printf("Error sending email: %v\n", err)
	}
}

func sendEmail(email string) error {
	log.Printf("Sending email to %s\n", email)
	return nil
}
