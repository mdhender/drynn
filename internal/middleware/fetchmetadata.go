package middleware

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

// FetchMetadata returns middleware that rejects cross-origin state-changing
// requests. It delegates the Sec-Fetch-Site / Origin+Host checks to Go's
// net/http.CrossOriginProtection (introduced in Go 1.25), which allows
// requests whose Sec-Fetch-Site is same-origin/same-site/none, or whose
// Origin hostname matches Host, and rejects everything else.
//
// An earlier revision of this middleware layered a strict top-up that
// rejected any state-changing request missing Sec-Fetch-Site outright. Some
// browsers (notably over plain HTTP to custom .test TLDs) omit the header on
// same-origin form POSTs, which caused the sign-out form to 403. The
// stdlib's Origin/Host fallback already protects against cross-origin
// forgery from a real browser, so the top-up was removed.
func FetchMetadata() echo.MiddlewareFunc {
	protection := http.NewCrossOriginProtection()
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if err := protection.Check(c.Request()); err != nil {
				return c.String(http.StatusForbidden, "forbidden: cross-origin request rejected")
			}

			return next(c)
		}
	}
}
