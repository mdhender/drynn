package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

const (
	accessCookieName  = "drynn_access"
	refreshCookieName = "drynn_refresh"

	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type Claims struct {
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

// UserID parses the Subject claim as a UUID.
func (c *Claims) UserID() (uuid.UUID, error) {
	return uuid.Parse(c.Subject)
}

type TokenPair struct {
	AccessToken   string
	AccessExpiry  time.Time
	RefreshToken  string
	RefreshExpiry time.Time
}

type Viewer struct {
	ID       uuid.UUID
	Handle   string
	Email    string
	IsActive bool
	Roles    []string
}

func (v *Viewer) HasRole(role string) bool {
	if v == nil {
		return false
	}

	for _, current := range v.Roles {
		if current == role {
			return true
		}
	}

	return false
}

// guestViewer returns a fresh sentinel Viewer representing an unauthenticated
// session. Callers receive their own copy so appending to Roles cannot leak
// across requests.
func guestViewer() *Viewer {
	return &Viewer{Roles: []string{"guest"}}
}

type Manager struct {
	keys         *KeyStore
	accessTTL    time.Duration
	refreshTTL   time.Duration
	cookieSecure bool
}

func NewManager(keys *KeyStore, accessTTL, refreshTTL time.Duration, cookieSecure bool) *Manager {
	return &Manager{
		keys:         keys,
		accessTTL:    accessTTL,
		refreshTTL:   refreshTTL,
		cookieSecure: cookieSecure,
	}
}

func (m *Manager) IssueTokens(ctx context.Context, userID uuid.UUID) (TokenPair, error) {
	now := time.Now().UTC()
	accessExpiry := now.Add(m.accessTTL)
	refreshExpiry := now.Add(m.refreshTTL)

	accessToken, err := m.signToken(ctx, userID, TokenTypeAccess, now, accessExpiry)
	if err != nil {
		return TokenPair{}, err
	}

	refreshToken, err := m.signToken(ctx, userID, TokenTypeRefresh, now, refreshExpiry)
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:   accessToken,
		AccessExpiry:  accessExpiry,
		RefreshToken:  refreshToken,
		RefreshExpiry: refreshExpiry,
	}, nil
}

func (m *Manager) ParseAccessToken(ctx context.Context, token string) (*Claims, error) {
	return m.parseToken(ctx, token, TokenTypeAccess)
}

func (m *Manager) ParseRefreshToken(ctx context.Context, token string) (*Claims, error) {
	return m.parseToken(ctx, token, TokenTypeRefresh)
}

func (m *Manager) SetAuthCookies(c *echo.Context, pair TokenPair) {
	c.SetCookie(m.newCookie(accessCookieName, pair.AccessToken, pair.AccessExpiry))
	c.SetCookie(m.newCookie(refreshCookieName, pair.RefreshToken, pair.RefreshExpiry))
}

func (m *Manager) ClearAuthCookies(c *echo.Context) {
	expired := time.Unix(0, 0).UTC()
	c.SetCookie(m.newCookie(accessCookieName, "", expired))
	c.SetCookie(m.newCookie(refreshCookieName, "", expired))
}

func AccessCookie(c *echo.Context) (*http.Cookie, error) {
	return c.Cookie(accessCookieName)
}

func RefreshCookie(c *echo.Context) (*http.Cookie, error) {
	return c.Cookie(refreshCookieName)
}

// AccessCookieName returns the name used for the access-token cookie. Exposed
// so tests and other packages can reference the single source of truth.
func AccessCookieName() string { return accessCookieName }

// RefreshCookieName returns the name used for the refresh-token cookie.
func RefreshCookieName() string { return refreshCookieName }

func SetViewer(c *echo.Context, viewer *Viewer) {
	c.Set("current_viewer", viewer)
}

// CurrentViewer returns the authenticated viewer for the current request, or a
// synthetic guest viewer when no session is present. The boolean reports
// whether an authenticated viewer was found — a false result always pairs with
// the guest sentinel, so callers can safely call HasRole or read Roles without
// a nil check.
func CurrentViewer(c *echo.Context) (*Viewer, bool) {
	viewer, ok := c.Get("current_viewer").(*Viewer)
	if !ok || viewer == nil {
		return guestViewer(), false
	}
	return viewer, true
}

func ClaimsFromContext(c *echo.Context) (*Claims, bool) {
	claims, ok := c.Get("jwt_claims").(*Claims)
	return claims, ok
}

func (m *Manager) signToken(ctx context.Context, userID uuid.UUID, tokenType string, issuedAt, expiresAt time.Time) (string, error) {
	key, err := m.keys.ActiveSigningKey(ctx, tokenType)
	if err != nil {
		return "", fmt.Errorf("load %s signing key: %w", tokenType, err)
	}

	claims := Claims{
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = key.ID.String()

	signed, err := token.SignedString(key.Secret)
	if err != nil {
		return "", fmt.Errorf("sign %s token: %w", tokenType, err)
	}

	return signed, nil
}

func (m *Manager) parseToken(ctx context.Context, rawToken, expectedType string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(rawToken, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %q", token.Method.Alg())
		}

		kidValue, ok := token.Header["kid"].(string)
		if !ok || strings.TrimSpace(kidValue) == "" {
			return nil, fmt.Errorf("missing signing key identifier")
		}

		keyID, err := uuid.Parse(strings.TrimSpace(kidValue))
		if err != nil {
			return nil, fmt.Errorf("parse signing key identifier: %w", err)
		}

		key, err := m.keys.VerificationKey(ctx, keyID, expectedType, time.Now().UTC())
		if err != nil {
			return nil, err
		}
		if key.Algorithm != signingAlgorithmHS256 {
			return nil, fmt.Errorf("unexpected signing algorithm %q", key.Algorithm)
		}

		return key.Secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, fmt.Errorf("parse %s token: %w", expectedType, err)
	}

	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("invalid %s token", expectedType)
	}

	if claims.TokenType != expectedType {
		return nil, fmt.Errorf("unexpected token type %q", claims.TokenType)
	}

	return claims, nil
}

func (m *Manager) newCookie(name, value string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
	}
}
