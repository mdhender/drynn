package middleware

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/mdhender/drynn/internal/auth"
)

// RequestLogger returns middleware that emits one structured slog record per
// HTTP request. Each record carries method, path, status, duration, and the
// viewer id (or "guest" when the request is unauthenticated).
func RequestLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()

			err := next(c)
			if err != nil {
				c.Echo().HTTPErrorHandler(c, err)
			}

			_, status := echo.ResolveResponseStatus(c.Response(), err)

			viewerID := "guest"
			if viewer, ok := auth.CurrentViewer(c); ok {
				viewerID = viewer.ID.String()
			}

			req := c.Request()
			attrs := []slog.Attr{
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
				slog.Int("status", status),
				slog.Duration("duration", time.Since(start)),
				slog.String("viewer_id", viewerID),
			}
			if err != nil {
				attrs = append(attrs, slog.String("error", err.Error()))
			}

			level := slog.LevelInfo
			switch {
			case status >= 500:
				level = slog.LevelError
			case status >= 400:
				level = slog.LevelWarn
			}

			slog.LogAttrs(req.Context(), level, "request", attrs...)

			return err
		}
	}
}
