package server

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
)

type templateRenderer struct {
	templates map[string]*template.Template
}

type templateConfig struct {
	layout string
	page   string
}

func newTemplateRenderer() (*templateRenderer, error) {
	pages := map[string]templateConfig{
		"public/home":     {layout: "web/templates/layouts/public.gohtml", page: "web/templates/pages/public/home.gohtml"},
		"public/register": {layout: "web/templates/layouts/public.gohtml", page: "web/templates/pages/public/register.gohtml"},
		"public/signin":   {layout: "web/templates/layouts/public.gohtml", page: "web/templates/pages/public/signin.gohtml"},
		"public/reset-password":  {layout: "web/templates/layouts/public.gohtml", page: "web/templates/pages/public/reset_password.gohtml"},
		"public/forgot-password": {layout: "web/templates/layouts/public.gohtml", page: "web/templates/pages/public/forgot_password.gohtml"},
		"public/request-access":  {layout: "web/templates/layouts/public.gohtml", page: "web/templates/pages/public/request_access.gohtml"},
		"app/profile":     {layout: "web/templates/layouts/app.gohtml", page: "web/templates/pages/app/profile.gohtml"},
		"admin/users":         {layout: "web/templates/layouts/app.gohtml", page: "web/templates/pages/admin/users.gohtml"},
		"admin/user-form":     {layout: "web/templates/layouts/app.gohtml", page: "web/templates/pages/admin/user_form.gohtml"},
		"admin/invitations":   {layout: "web/templates/layouts/app.gohtml", page: "web/templates/pages/admin/invitations.gohtml"},
		"admin/invite-form":   {layout: "web/templates/layouts/app.gohtml", page: "web/templates/pages/admin/invite_form.gohtml"},
	}

	funcs := template.FuncMap{
		"hasRole": func(roles []string, role string) bool {
			for _, current := range roles {
				if current == role {
					return true
				}
			}

			return false
		},
		"hasPrefix": strings.HasPrefix,
		"join":      strings.Join,
		"expired": func(t time.Time) bool {
			return time.Now().After(t)
		},
		"formatTime": func(t time.Time) string {
			return t.Format("2 Jan 2006 15:04")
		},
	}

	renderer := &templateRenderer{templates: make(map[string]*template.Template, len(pages))}
	fsys := os.DirFS(".")
	for name, cfg := range pages {
		tmpl, err := template.New(name).Funcs(funcs).ParseFS(
			fsys,
			cfg.layout,
			"web/components/*.gohtml",
			cfg.page,
		)
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", name, err)
		}

		renderer.templates[name] = tmpl
	}

	return renderer, nil
}

func (r *templateRenderer) Render(c *echo.Context, w io.Writer, name string, data any) error {
	tmpl, ok := r.templates[name]
	if !ok {
		return fmt.Errorf("template %q not found", name)
	}

	return tmpl.ExecuteTemplate(w, "base", data)
}
