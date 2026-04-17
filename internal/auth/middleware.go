package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
)

func RequireAuth(manager *Manager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			logger := slog.Default()
			path := c.Request().URL.Path

			token, err := tokenFromRequest(c)
			if err != nil {
				logger.Debug("require auth: no access token in request", "path", path, "error", err)
				if claims, ok := tryRefreshAccess(c, manager); ok {
					c.Set("jwt_claims", claims)
					return next(c)
				}
				return unauthorized(c)
			}

			claims, err := manager.ParseAccessToken(c.Request().Context(), token)
			if err != nil {
				if errors.Is(err, jwt.ErrTokenExpired) {
					logger.Debug("require auth: access token expired", "path", path)
					if refreshed, ok := tryRefreshAccess(c, manager); ok {
						c.Set("jwt_claims", refreshed)
						return next(c)
					}
					manager.ClearAuthCookies(c)
				} else {
					logger.Info("require auth: parse access token failed", "path", path, "error", err)
				}

				return unauthorized(c)
			}

			c.Set("jwt_claims", claims)
			return next(c)
		}
	}
}

func tryRefreshAccess(c *echo.Context, manager *Manager) (*Claims, bool) {
	logger := slog.Default()
	path := c.Request().URL.Path

	cookie, err := RefreshCookie(c)
	if err != nil {
		logger.Debug("refresh access: no refresh cookie", "path", path, "error", err)
		return nil, false
	}
	if cookie.Value == "" {
		logger.Debug("refresh access: empty refresh cookie", "path", path)
		return nil, false
	}

	refreshClaims, err := manager.ParseRefreshToken(c.Request().Context(), cookie.Value)
	if err != nil {
		logger.Info("refresh access: parse refresh token failed", "path", path, "error", err)
		return nil, false
	}

	userID, err := refreshClaims.UserID()
	if err != nil {
		logger.Warn("refresh access: invalid subject uuid", "path", path, "subject", refreshClaims.Subject, "error", err)
		return nil, false
	}

	pair, err := manager.IssueTokens(c.Request().Context(), userID)
	if err != nil {
		logger.Error("refresh access: issue tokens failed", "path", path, "user_id", userID, "error", err)
		return nil, false
	}

	manager.SetAuthCookies(c, pair)

	return &Claims{
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(pair.AccessExpiry),
		},
	}, true
}

func tokenFromRequest(c *echo.Context) (string, error) {
	if cookie, err := AccessCookie(c); err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}

	header := c.Request().Header.Get(echo.HeaderAuthorization)
	if header == "" {
		return "", http.ErrNoCookie
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", echo.ErrUnauthorized
	}

	return strings.TrimSpace(parts[1]), nil
}

func unauthorized(c *echo.Context) error {
	acceptsHTML := strings.Contains(c.Request().Header.Get(echo.HeaderAccept), "text/html")
	if acceptsHTML || c.Request().Method == http.MethodGet {
		return c.Redirect(http.StatusSeeOther, "/signin")
	}

	return c.String(http.StatusUnauthorized, "authentication required")
}
