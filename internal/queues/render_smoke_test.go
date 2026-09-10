package queues

import (
	"strings"
	"testing"
)

func TestRenderDigiwallet(t *testing.T) {
	body, err := RenderEmail(TemplateWelcome, EmailPayload{
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
