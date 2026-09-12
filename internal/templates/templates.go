// Package templates owns the email HTML templates and their rendering.
package templates

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"time"
)

//go:embed emails/*.html
var templateFS embed.FS

// TimeLayout is how timestamps are rendered inside emails.
const TimeLayout = "Jan 2, 2006 at 3:04 PM MST"

// FormatTime renders t in TimeLayout. Use it when building a payload so the
// email shows the moment the event happened rather than the moment it rendered.
func FormatTime(t time.Time) string {
	return t.Format(TimeLayout)
}

var funcs = template.FuncMap{
	// now is the fallback when a payload carries no explicit timestamp.
	"now": func() string { return FormatTime(time.Now().UTC()) },
}

var emailTemplates = template.Must(
	template.New("emails").Funcs(funcs).ParseFS(templateFS, "emails/*.html"),
)

const (
	EmailWelcome    = "digiwallet"
	EmailLoginAlert = "login-alert"
)

// RenderEmail renders the named email template with data. An empty name falls
// back to EmailWelcome.
func RenderEmail(name string, data any) (string, error) {
	if name == "" {
		name = EmailWelcome
	}
	tmpl := emailTemplates.Lookup(name + ".html")
	if tmpl == nil {
		return "", fmt.Errorf("unknown email template %q", name)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("rendering email template %q: %w", name, err)
	}
	return buf.String(), nil
}
