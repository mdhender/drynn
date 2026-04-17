package email

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"sync"
)

var (
	emailTemplates     *template.Template
	emailTemplatesOnce sync.Once
	emailTemplatesErr  error
)

func loadTemplates() (*template.Template, error) {
	emailTemplatesOnce.Do(func() {
		emailTemplates, emailTemplatesErr = template.ParseFS(os.DirFS("."), "web/templates/emails/*.gohtml")
	})
	return emailTemplates, emailTemplatesErr
}

// RenderTemplate executes the named email template with the given data and
// returns the resulting HTML string.
func RenderTemplate(name string, data any) (string, error) {
	tmpl, err := loadTemplates()
	if err != nil {
		return "", fmt.Errorf("load email templates: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("render email template %s: %w", name, err)
	}
	return buf.String(), nil
}
