package handler

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/mdhender/drynn/internal/service"
)

func TestRequestBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		tls     bool
		headers map[string]string
		want    string
	}{
		{
			name: "plain http",
			host: "localhost:8080",
			want: "http://localhost:8080",
		},
		{
			name: "tls connection",
			host: "example.com",
			tls:  true,
			want: "https://example.com",
		},
		{
			name:    "X-Forwarded-Proto overrides scheme",
			host:    "example.com",
			headers: map[string]string{"X-Forwarded-Proto": "https"},
			want:    "https://example.com",
		},
		{
			name:    "X-Forwarded-Host overrides host",
			host:    "backend:8080",
			headers: map[string]string{"X-Forwarded-Host": "example.com"},
			want:    "http://example.com",
		},
		{
			name: "both forwarded headers",
			host: "backend:8080",
			headers: map[string]string{
				"X-Forwarded-Proto": "https",
				"X-Forwarded-Host":  "example.com",
			},
			want: "https://example.com",
		},
		{
			name:    "multi-value forwarded proto picks first",
			host:    "example.com",
			headers: map[string]string{"X-Forwarded-Proto": "https, http"},
			want:    "https://example.com",
		},
		{
			name:    "multi-value forwarded host picks first",
			host:    "backend:8080",
			headers: map[string]string{"X-Forwarded-Host": "a.example.com, b.example.com"},
			want:    "http://a.example.com",
		},
	}

	e := echo.New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = tt.host
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			c := e.NewContext(req, httptest.NewRecorder())

			got := requestBaseURL(c)
			if got != tt.want {
				t.Errorf("requestBaseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFirstHeaderValue(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"  ", ""},
		{"https", "https"},
		{"https, http", "https"},
		{"  https , http ", "https"},
		{"a.example.com, b.example.com", "a.example.com"},
	}
	for _, tt := range tests {
		if got := firstHeaderValue(tt.input); got != tt.want {
			t.Errorf("firstHeaderValue(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeInvitationFilter(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{service.InvitationFilterAll, service.InvitationFilterAll},
		{service.InvitationFilterUnused, service.InvitationFilterUnused},
		{service.InvitationFilterExpired, service.InvitationFilterExpired},
		{service.InvitationFilterUsed, service.InvitationFilterUsed},
		{"bogus", service.InvitationFilterAll},
		{"", service.InvitationFilterAll},
	}
	for _, tt := range tests {
		if got := normalizeInvitationFilter(tt.input); got != tt.want {
			t.Errorf("normalizeInvitationFilter(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSelectedRoles(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]bool
		want  []string
	}{
		{"empty map", map[string]bool{}, nil},
		{"user only", map[string]bool{"user": true}, []string{"user"}},
		{"user and admin", map[string]bool{"user": true, "admin": true}, []string{"user", "admin"}},
		{"admin only", map[string]bool{"admin": true}, []string{"admin"}},
		{"false entries ignored", map[string]bool{"user": false, "admin": true}, []string{"admin"}},
		{"unknown roles ignored", map[string]bool{"superadmin": true}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectedRoles(tt.input)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("selectedRoles(%v) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("selectedRoles(%v)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
