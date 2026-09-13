package providers

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github/marveldo/eda-monolith/shared"
)

func newTestProvider(t *testing.T, handler http.HandlerFunc) *PaystackProvider {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return NewPaystackProvider(&PaystackProviderConfig{
		SecretKey: "sk_test_secret",
		BaseURL:   server.URL,
		Client:    server.Client(),
	})
}

func TestInitializeSendsMinorUnits(t *testing.T) {
	var got map[string]any
	provider := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transaction/initialize" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer sk_test_secret" {
			t.Errorf("Authorization = %q", auth)
		}
		json.NewDecoder(r.Body).Decode(&got)
		json.NewEncoder(w).Encode(map[string]any{
			"status": true,
			"data": map[string]any{
				"authorization_url": "https://checkout.paystack.com/abc",
				"access_code":       "abc",
				"reference":         "ref-1",
			},
		})
	})

	result, err := provider.Initialize(context.Background(), shared.InitializePaymentRequest{
		Reference: "ref-1",
		Amount:    shared.NewMoney(50.25),
		Currency:  "NGN",
		Email:     "a@b.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if amount := got["amount"].(float64); amount != 5025 {
		t.Fatalf("amount sent = %v, want 5025 minor units", amount)
	}
	if result.AuthorizationURL != "https://checkout.paystack.com/abc" {
		t.Fatalf("authorization url = %q", result.AuthorizationURL)
	}
}

func TestVerifyMapsStatusAndAmount(t *testing.T) {
	provider := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": true,
			"data": map[string]any{
				"reference": "ref-1",
				"status":    "success",
				"amount":    5025,
				"currency":  "NGN",
				"id":        9911,
			},
		})
	})

	result, err := provider.Verify(context.Background(), "ref-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != shared.PaymentSuccess {
		t.Fatalf("status = %q, want SUCCESS", result.Status)
	}
	if result.Amount != shared.NewMoney(50.25) {
		t.Fatalf("amount = %v, want 50.25", result.Amount)
	}
}

func TestVerifyUnknownReference(t *testing.T) {
	provider := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{"status": false, "message": "not found"})
	})

	_, err := provider.Verify(context.Background(), "nope")
	if !errors.Is(err, shared.ErrReferenceNotFound) {
		t.Fatalf("err = %v, want ErrReferenceNotFound", err)
	}
}

func TestWebhookSignature(t *testing.T) {
	provider := NewPaystackProvider(&PaystackProviderConfig{SecretKey: "sk_test_secret"})
	body := []byte(`{"event":"charge.success","data":{"reference":"ref-1","status":"success","amount":5025}}`)

	mac := hmac.New(sha512.New, []byte("sk_test_secret"))
	mac.Write(body)
	valid := hex.EncodeToString(mac.Sum(nil))

	if err := provider.VerifyWebhookSignature(valid, body); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := provider.VerifyWebhookSignature("deadbeef", body); !errors.Is(err, shared.ErrInvalidSignature) {
		t.Fatalf("forged signature accepted, err = %v", err)
	}
	if err := provider.VerifyWebhookSignature("", body); !errors.Is(err, shared.ErrInvalidSignature) {
		t.Fatalf("missing signature accepted, err = %v", err)
	}
	tampered := append([]byte{}, body...)
	tampered[0] = ' '
	if err := provider.VerifyWebhookSignature(valid, tampered); !errors.Is(err, shared.ErrInvalidSignature) {
		t.Fatalf("tampered body accepted, err = %v", err)
	}
}

func TestParseWebhook(t *testing.T) {
	provider := NewPaystackProvider(&PaystackProviderConfig{SecretKey: "sk_test_secret"})

	event, err := provider.ParseWebhook([]byte(`{"event":"charge.success","data":{"reference":"ref-1","status":"success","amount":5025,"currency":"NGN"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if event.Reference != "ref-1" || event.Status != shared.PaymentSuccess {
		t.Fatalf("event = %+v", event)
	}
	if event.Amount != shared.NewMoney(50.25) {
		t.Fatalf("amount = %v", event.Amount)
	}

	_, err = provider.ParseWebhook([]byte(`{"event":"customer.created","data":{"reference":"x"}}`))
	if !errors.Is(err, shared.ErrUnhandledWebhook) {
		t.Fatalf("err = %v, want ErrUnhandledWebhook", err)
	}
}
