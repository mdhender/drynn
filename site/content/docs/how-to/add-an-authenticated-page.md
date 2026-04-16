---
title: Add an authenticated page
weight: 50
---

Authenticated pages live behind the `/app` prefix and require a
valid session. They use the `app` layout, which includes the
sidebar. This guide adds a page at `/app/settings`.

## 1. Create the template

Create `web/templates/pages/app/settings.gohtml`:

```gohtml
{{define "content"}}
<div class="container mx-auto px-4 py-8">
    <h1 class="text-2xl font-bold mb-4">Settings</h1>
    <p>Hello, {{.CurrentUser.Handle}}. Manage your preferences here.</p>
</div>
{{end}}
```

Inside the `app` layout, `{{.CurrentUser}}` is always available —
the `loadCurrentViewer` middleware populates it before your handler
runs.

## 2. Add a view-data struct

Open `internal/handler/view.go`:

```go
type SettingsViewData struct {
    BaseViewData
}
```

## 3. Add the handler method

Open `internal/handler/app.go` and add a method to `AppHandler`:

```go
func (h *AppHandler) ShowSettings(c *echo.Context) error {
    return c.Render(http.StatusOK, "app/settings", SettingsViewData{
        BaseViewData: baseView(c, "Settings"),
    })
}
```

To read the current user explicitly (e.g. for data queries), call:

```go
viewer, _ := auth.CurrentViewer(c)
```

## 4. Register the template

In `internal/server/render.go`, add to the `pages` map:

```go
"app/settings": {layout: "web/templates/layouts/app.gohtml", page: "web/templates/pages/app/settings.gohtml"},
```

## 5. Register the route

In `internal/server/server.go`, add the route inside `appGroup`:

```go
appGroup.GET("/settings", appHandler.ShowSettings)
```

The `appGroup` already applies `RequireAuth` and
`loadCurrentViewer`, so no extra middleware is needed.

### Admin-only pages

If the page should be restricted to admins, register it under
`adminGroup` instead:

```go
adminGroup.GET("/settings", adminHandler.ShowSettings)
```

This adds `requireRole(service.RoleAdmin)` on top of the auth
middleware.

## 6. Verify

```sh
go build ./...
go run ./cmd/server
```

Sign in and navigate to `http://localhost:8080/app/settings`. The
page should render with the sidebar. Visiting the URL while signed
out should redirect to the sign-in page.
