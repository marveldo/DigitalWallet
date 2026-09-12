package templates_test

import (
	"strings"
	"testing"
	"time"

	"github/marveldo/eda-monolith/internal/queues"
	"github/marveldo/eda-monolith/internal/templates"
)

func TestRenderDigiwallet(t *testing.T) {
	body, err := templates.RenderEmail(templates.EmailWelcome, queues.EmailPayload{
		To:        []string{"ada@example.com"},
		Subject:   "Welcome to DigiWallet",
		FirstName: "Ada",
		OTP:       "482913",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"482913", "Ada", "ada@example.com", "DigiWallet"} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered body missing %q", want)
		}
	}
}

func TestRenderLoginAlert(t *testing.T) {
	body, err := templates.RenderEmail(templates.EmailLoginAlert, queues.EmailPayload{
		To:        []string{"ada@example.com"},
		Subject:   "New sign-in to DigiWallet",
		FirstName: "Ada",
		Data:      map[string]string{"time": templates.FormatTime(time.Date(2026, 9, 12, 14, 30, 0, 0, time.UTC))},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Ada", "ada@example.com", "Sep 12, 2026 at 2:30 PM UTC"} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered body missing %q", want)
		}
	}
}

func TestRenderLoginAlertFallsBackToNow(t *testing.T) {
	body, err := templates.RenderEmail(templates.EmailLoginAlert, queues.EmailPayload{
		To: []string{"ada@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := templates.FormatTime(time.Now().UTC()); !strings.Contains(body, want) {
		t.Errorf("rendered body missing fallback time %q", want)
	}
}

func TestRenderUnknownTemplate(t *testing.T) {
	if _, err := templates.RenderEmail("nope", queues.EmailPayload{}); err == nil {
		t.Fatal("expected error for unknown template")
	}
}
