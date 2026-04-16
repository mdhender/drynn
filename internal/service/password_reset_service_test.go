package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/mdhender/drynn/internal/email"
	"github.com/mdhender/drynn/internal/testdb"
	"github.com/mdhender/drynn/internal/testfixtures"
)

func newPasswordResetService(t testing.TB) (*PasswordResetService, *UserService, *testfixtures.Fixtures) {
	t.Helper()
	pool := testdb.New(t)
	return NewPasswordResetService(pool, email.MailgunConfig{}),
		NewUserService(pool),
		testfixtures.New(t, pool)
}

// ---------------------------------------------------------------------------
// SendReset / SendResetByEmail — require Mailgun
// ---------------------------------------------------------------------------

func TestPasswordResetService_SendReset_NoMailgun(t *testing.T) {
	svc, _, fix := newPasswordResetService(t)
	ctx := context.Background()
	user := fix.NewUser().Build(ctx)

	err := svc.SendReset(ctx, user.ID, "http://localhost")
	if !errors.Is(err, ErrMailgunNotConfigured) {
		t.Fatalf("want ErrMailgunNotConfigured, got %v", err)
	}
}

func TestPasswordResetService_SendReset_NotFound(t *testing.T) {
	svc, _, _ := newPasswordResetService(t)
	ctx := context.Background()

	err := svc.SendReset(ctx, uuid.New(), "http://localhost")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("want ErrUserNotFound, got %v", err)
	}
}

func TestPasswordResetService_SendResetByEmail_UnknownEmail(t *testing.T) {
	svc, _, _ := newPasswordResetService(t)
	ctx := context.Background()

	err := svc.SendResetByEmail(ctx, "nobody@example.com", "http://localhost")
	if err != nil {
		t.Fatalf("SendResetByEmail for unknown email should silently succeed, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ResetPassword
// ---------------------------------------------------------------------------

func TestPasswordResetService_ResetPassword(t *testing.T) {
	svc, userSvc, fix := newPasswordResetService(t)
	ctx := context.Background()

	user := fix.NewUser().Email("alice@example.com").Password("oldpassword1").Build(ctx)
	tok := fix.NewPasswordResetToken().ForUser(user).Code("resetcode123").Build(ctx)
	_ = tok

	err := svc.ResetPassword(ctx, "resetcode123", "alice@example.com", "newpassword1")
	if err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}

	// verify new password works
	_, err = userSvc.Authenticate(ctx, SignInInput{
		Email: "alice@example.com", Password: "newpassword1",
	})
	if err != nil {
		t.Fatalf("auth with new password: %v", err)
	}

	// verify old password fails
	_, err = userSvc.Authenticate(ctx, SignInInput{
		Email: "alice@example.com", Password: "oldpassword1",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password should fail, got %v", err)
	}
}

func TestPasswordResetService_ResetPassword_InvalidCode(t *testing.T) {
	svc, _, _ := newPasswordResetService(t)
	ctx := context.Background()

	err := svc.ResetPassword(ctx, "nonexistent", "alice@example.com", "newpassword1")
	if !errors.Is(err, ErrPasswordResetInvalid) {
		t.Fatalf("want ErrPasswordResetInvalid, got %v", err)
	}
}

func TestPasswordResetService_ResetPassword_Expired(t *testing.T) {
	svc, _, fix := newPasswordResetService(t)
	ctx := context.Background()

	user := fix.NewUser().Email("alice@example.com").Build(ctx)
	fix.NewPasswordResetToken().ForUser(user).Code("expiredcode").
		ExpiresAt(time.Now().Add(-1 * time.Hour)).Build(ctx)

	err := svc.ResetPassword(ctx, "expiredcode", "alice@example.com", "newpassword1")
	if !errors.Is(err, ErrPasswordResetInvalid) {
		t.Fatalf("want ErrPasswordResetInvalid, got %v", err)
	}
}

func TestPasswordResetService_ResetPassword_WrongEmail(t *testing.T) {
	svc, _, fix := newPasswordResetService(t)
	ctx := context.Background()

	user := fix.NewUser().Email("alice@example.com").Build(ctx)
	fix.NewPasswordResetToken().ForUser(user).Code("wrongemail").Build(ctx)

	err := svc.ResetPassword(ctx, "wrongemail", "wrong@example.com", "newpassword1")
	if !errors.Is(err, ErrPasswordResetInvalid) {
		t.Fatalf("want ErrPasswordResetInvalid, got %v", err)
	}
}

func TestPasswordResetService_ResetPassword_UsedToken(t *testing.T) {
	svc, _, fix := newPasswordResetService(t)
	ctx := context.Background()

	user := fix.NewUser().Email("alice@example.com").Password("password123").Build(ctx)
	fix.NewPasswordResetToken().ForUser(user).Code("usedonce").Build(ctx)

	if err := svc.ResetPassword(ctx, "usedonce", "alice@example.com", "firstnewpw1"); err != nil {
		t.Fatalf("first ResetPassword: %v", err)
	}

	err := svc.ResetPassword(ctx, "usedonce", "alice@example.com", "secondnewpw")
	if !errors.Is(err, ErrPasswordResetInvalid) {
		t.Fatalf("want ErrPasswordResetInvalid for reused token, got %v", err)
	}
}

func TestPasswordResetService_ResetPassword_ShortPassword(t *testing.T) {
	svc, _, fix := newPasswordResetService(t)
	ctx := context.Background()

	user := fix.NewUser().Email("alice@example.com").Build(ctx)
	fix.NewPasswordResetToken().ForUser(user).Code("shortpw").Build(ctx)

	err := svc.ResetPassword(ctx, "shortpw", "alice@example.com", "short")
	if !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("want ErrInvalidPassword, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// AccessRequestService
// ---------------------------------------------------------------------------

func TestAccessRequestService_Send_Disabled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped in -short mode")
	}
	svc := NewAccessRequestService(email.MailgunConfig{}, "")

	err := svc.Send(context.Background(), AccessRequestInput{
		Email:  "someone@example.com",
		Reason: "I want access",
		IP:     "127.0.0.1",
	})
	if !errors.Is(err, ErrAccessRequestsDisabled) {
		t.Fatalf("want ErrAccessRequestsDisabled, got %v", err)
	}
}

func TestAccessRequestService_Send_NoMailgun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped in -short mode")
	}
	svc := NewAccessRequestService(email.MailgunConfig{}, "admin@example.com")

	err := svc.Send(context.Background(), AccessRequestInput{
		Email:  "someone@example.com",
		Reason: "I want access",
		IP:     "127.0.0.1",
	})
	if !errors.Is(err, ErrMailgunNotConfigured) {
		t.Fatalf("want ErrMailgunNotConfigured, got %v", err)
	}
}

func TestAccessRequestService_Send_InvalidEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped in -short mode")
	}
	svc := NewAccessRequestService(
		email.MailgunConfig{APIKey: "k", SendingDomain: "d", FromAddress: "f@d.com"},
		"admin@example.com",
	)

	err := svc.Send(context.Background(), AccessRequestInput{
		Email: "not-an-email", Reason: "I want access",
	})
	if !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("want ErrInvalidEmail, got %v", err)
	}
}
