package handler

import (
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/mdhender/drynn/internal/service"
)

func TestWithFlash(t *testing.T) {
	tests := []struct {
		path    string
		message string
		want    string
	}{
		{"/app", "", "/app"},
		{"/app", "Hello!", "/app?flash=Hello%21"},
		{"/app/admin", "has spaces", "/app/admin?flash=has+spaces"},
		{"/", "a&b=c", "/?flash=a%26b%3Dc"},
	}
	for _, tt := range tests {
		if got := withFlash(tt.path, tt.message); got != tt.want {
			t.Errorf("withFlash(%q, %q) = %q, want %q", tt.path, tt.message, got, tt.want)
		}
	}
}

func TestServiceMessage(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{service.ErrInvalidCredentials, "The email address or password was incorrect."},
		{service.ErrEmailTaken, "That email address is already in use."},
		{service.ErrHandleTaken, "That handle is already in use."},
		{service.ErrInvalidHandle, service.ErrInvalidHandle.Error()},
		{service.ErrInvalidEmail, "Enter a valid email address."},
		{service.ErrInvalidPassword, service.ErrInvalidPassword.Error()},
		{service.ErrInactiveUser, "This account has been deactivated."},
		{service.ErrLastAdmin, service.ErrLastAdmin.Error()},
		{service.ErrCannotDeleteSelf, service.ErrCannotDeleteSelf.Error()},
		{service.ErrUserNotFound, "That user could not be found."},
		{service.ErrInvitationNotFound, "That invitation code is invalid."},
		{service.ErrInvitationUsed, "That invitation has already been used."},
		{service.ErrInvitationExpired, "That invitation has expired."},
		{service.ErrInvitationEmail, "The email address does not match the invitation."},
		{service.ErrMailgunNotConfigured, "Mailgun settings must be configured before sending invitations."},
		{service.ErrPasswordResetInvalid, "That password reset link is invalid, expired, or the email does not match."},
	}
	for _, tt := range tests {
		name := "nil"
		if tt.err != nil {
			name = tt.err.Error()
		}
		t.Run(name, func(t *testing.T) {
			if got := serviceMessage(tt.err); got != tt.want {
				t.Errorf("serviceMessage(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

func TestServiceMessage_UnknownError(t *testing.T) {
	got := serviceMessage(fmt.Errorf("something unexpected"))
	if got != "Something went wrong. Please try again." {
		t.Errorf("unknown error: got %q", got)
	}
}

func TestServiceMessage_WrappedError(t *testing.T) {
	wrapped := fmt.Errorf("outer: %w", service.ErrEmailTaken)
	got := serviceMessage(wrapped)
	if got != "That email address is already in use." {
		t.Errorf("wrapped ErrEmailTaken: got %q", got)
	}
}

func TestAdminFormFromUser(t *testing.T) {
	id := uuid.New()
	user := &service.User{
		ID:       id,
		Handle:   "alice",
		Email:    "alice@example.com",
		IsActive: true,
		Roles:    []string{"user", "admin"},
	}

	form := adminFormFromUser(user)

	if form.ID != id {
		t.Errorf("ID = %v, want %v", form.ID, id)
	}
	if form.Handle != "alice" {
		t.Errorf("Handle = %q, want %q", form.Handle, "alice")
	}
	if form.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", form.Email, "alice@example.com")
	}
	if !form.IsActive {
		t.Error("IsActive should be true")
	}
	if !form.Roles["user"] || !form.Roles["admin"] {
		t.Errorf("Roles = %v, want user+admin", form.Roles)
	}
	if form.Roles["tester"] {
		t.Error("Roles should not include tester")
	}
}
