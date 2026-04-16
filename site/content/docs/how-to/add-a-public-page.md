---
title: Add a public page
weight: 40
---

Public pages are served without authentication. They use the
`public` layout and are registered as top-level routes. This guide
adds a page at `/about`.

## 1. Create the template

Create `web/templates/pages/public/about.gohtml`:

```gohtml
{{define "content"}}
<div class="container mx-auto px-4 py-8">
    <h1 class="text-2xl font-bold mb-4">About</h1>
    <p>This is a Drynn instance for running competitions.</p>
</div>
{{end}}
```

The template defines a single `content` block. The `public` layout
wraps it with the site header and footer.

## 2. Add a view-data struct

Open `internal/handler/view.go` and add a struct:

```go
type AboutViewData struct {
    BaseViewData
}
```

If the page has no dynamic data beyond the base fields, you can
reuse `HomeViewData` instead.

## 3. Add the handler method

Open `internal/handler/public.go` and add a method to
`PublicHandler`:

```go
func (h *PublicHandler) ShowAbout(c *echo.Context) error {
    return c.Render(http.StatusOK, "public/about", AboutViewData{
        BaseViewData: baseView(c, "About"),
    })
}
```

## 4. Register the template

Open `internal/server/render.go` and add an entry to the `pages`
map inside `newTemplateRenderer()`:

```go
"public/about": {layout: "web/templates/layouts/public.gohtml", page: "web/templates/pages/public/about.gohtml"},
```

The map key (`public/about`) must match the name passed to
`c.Render`.

## 5. Register the route

Open `internal/server/server.go` and add the route in
`registerRoutes()`, alongside the other public routes:

```go
e.GET("/about", publicHandler.ShowAbout)
```

## 6. Verify

```sh
go build ./...
go run ./cmd/server
```

Open `http://localhost:8080/about` in your browser. The page should
render with the public layout and no sign-in required.
