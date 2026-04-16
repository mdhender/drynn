package auth

import (
	"testing"
)

func TestViewer_HasRole(t *testing.T) {
	v := &Viewer{Roles: []string{"user", "admin"}}

	tests := []struct {
		role string
		want bool
	}{
		{"user", true},
		{"admin", true},
		{"tester", false},
		{"guest", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := v.HasRole(tt.role); got != tt.want {
			t.Errorf("HasRole(%q) = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestViewer_HasRole_NilReceiver(t *testing.T) {
	var v *Viewer
	if v.HasRole("user") {
		t.Error("nil Viewer.HasRole should return false")
	}
}

func TestViewer_HasRole_NoRoles(t *testing.T) {
	v := &Viewer{}
	if v.HasRole("user") {
		t.Error("empty roles should return false for any role")
	}
}

func TestGuestViewer(t *testing.T) {
	g := guestViewer()
	if g == nil {
		t.Fatal("guestViewer returned nil")
	}
	if !g.HasRole("guest") {
		t.Error("guest viewer should have guest role")
	}
	if g.HasRole("user") {
		t.Error("guest viewer should not have user role")
	}
	if g.ID.String() != "00000000-0000-0000-0000-000000000000" {
		t.Error("guest viewer should have zero UUID")
	}
}
