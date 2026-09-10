package queues

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
)

//go:embed templates/*.html
var templateFS embed.FS

var emailTemplates = template.Must(template.ParseFS(templateFS, "templates/*.html"))

const TemplateWelcome = "digiwallet"

func RenderEmail(name string, payload EmailPayload) (string, error) {
	if name == "" {
		name = TemplateWelcome
	}
	tmpl := emailTemplates.Lookup(name + ".html")
	if tmpl == nil {
		return "", fmt.Errorf("unknown email template %q", name)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, payload); err != nil {
		return "", fmt.Errorf("rendering email template %q: %w", name, err)
	}
	return buf.String(), nil
}
