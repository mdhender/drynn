package handler

import (
	"testing"

	"github.com/mdhender/drynn/internal/service"
)

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
