package email

import (
	"bytes"
	"fmt"
	"html/template"

	drynn "github.com/mdhender/drynn"
)

var emailTemplates = template.Must(template.ParseFS(drynn.EmailTemplateFS(), "web/templates/emails/*.gohtml"))

// RenderTemplate executes the named email template with the given data and
// returns the resulting HTML string.
func RenderTemplate(name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := emailTemplates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("render email template %s: %w", name, err)
	}
	return buf.String(), nil
}
