package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mdhender/drynn/internal/email"
	"github.com/mdhender/drynn/internal/testdb"
	"github.com/mdhender/drynn/internal/testfixtures"
)

func newInvitationService(t testing.TB) (*InvitationService, *testfixtures.Fixtures, *pgxpool.Pool) {
	t.Helper()
	pool := testdb.New(t)
	return NewInvitationService(pool, email.MailgunConfig{}), testfixtures.New(t, pool), pool
}

// ---------------------------------------------------------------------------
// CreateAndSend — requires Mailgun
// ---------------------------------------------------------------------------

func TestInvitationService_CreateAndSend_NoMailgun(t *testing.T) {
	svc, fix, _ := newInvitationService(t)
	ctx := context.Background()
	admin := fix.NewUser().Admin().Build(ctx)

	_, err := svc.CreateAndSend(ctx, admin.ID, CreateInvitationInput{
		Email:   "invite@example.com",
		BaseURL: "http://localhost",
	})
	if !errors.Is(err, ErrMailgunNotConfigured) {
		t.Fatalf("want ErrMailgunNotConfigured, got %v", err)
	}
}

func TestInvitationService_CreateAndSend_InvalidEmail(t *testing.T) {
	svc, fix, _ := newInvitationService(t)
	ctx := context.Background()
	admin := fix.NewUser().Admin().Build(ctx)

	_, err := svc.CreateAndSend(ctx, admin.ID, CreateInvitationInput{
		Email:   "not-an-email",
		BaseURL: "http://localhost",
	})
	if !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("want ErrInvalidEmail, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ValidateCode
// ---------------------------------------------------------------------------

func TestInvitationService_ValidateCode(t *testing.T) {
	svc, fix, _ := newInvitationService(t)
	ctx := context.Background()
	inv := fix.NewInvitation().Code("valid123").Build(ctx)

	result, err := svc.ValidateCode(ctx, "valid123")
	if err != nil {
		t.Fatalf("ValidateCode: %v", err)
	}
	if result.ID != inv.ID {
		t.Errorf("ID = %v, want %v", result.ID, inv.ID)
	}
}

func TestInvitationService_ValidateCode_NotFound(t *testing.T) {
	svc, _, _ := newInvitationService(t)
	ctx := context.Background()

	_, err := svc.ValidateCode(ctx, "nonexistent")
	if !errors.Is(err, ErrInvitationNotFound) {
		t.Fatalf("want ErrInvitationNotFound, got %v", err)
	}
}

func TestInvitationService_ValidateCode_Expired(t *testing.T) {
	svc, fix, _ := newInvitationService(t)
	ctx := context.Background()
	fix.NewInvitation().Code("expired123").ExpiresAt(time.Now().Add(-1 * time.Hour)).Build(ctx)

	_, err := svc.ValidateCode(ctx, "expired123")
	if !errors.Is(err, ErrInvitationExpired) {
		t.Fatalf("want ErrInvitationExpired, got %v", err)
	}
}

func TestInvitationService_ValidateCode_Used(t *testing.T) {
	svc, fix, pool := newInvitationService(t)
	ctx := context.Background()
	admin := fix.NewUser().Admin().Build(ctx)
	inv := fix.NewInvitation().Code("used123").InvitedByUser(admin).Build(ctx)

	// redeem first
	user := fix.NewUser().Build(ctx)
	if err := svc.RedeemInvitation(ctx, "used123", user.ID); err != nil {
		t.Fatalf("RedeemInvitation: %v", err)
	}
	_ = inv
	_ = pool

	_, err := svc.ValidateCode(ctx, "used123")
	if !errors.Is(err, ErrInvitationUsed) {
		t.Fatalf("want ErrInvitationUsed, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// RedeemInvitation
// ---------------------------------------------------------------------------

func TestInvitationService_RedeemInvitation(t *testing.T) {
	svc, fix, _ := newInvitationService(t)
	ctx := context.Background()
	fix.NewInvitation().Code("redeem123").Build(ctx)
	user := fix.NewUser().Build(ctx)

	if err := svc.RedeemInvitation(ctx, "redeem123", user.ID); err != nil {
		t.Fatalf("RedeemInvitation: %v", err)
	}

	// second redemption should fail
	_, err := svc.ValidateCode(ctx, "redeem123")
	if !errors.Is(err, ErrInvitationUsed) {
		t.Fatalf("redeemed invitation should be used, got %v", err)
	}
}

func TestInvitationService_RedeemInvitation_Expired(t *testing.T) {
	svc, fix, _ := newInvitationService(t)
	ctx := context.Background()
	fix.NewInvitation().Code("expired_redeem").Expired().Build(ctx)
	user := fix.NewUser().Build(ctx)

	err := svc.RedeemInvitation(ctx, "expired_redeem", user.ID)
	if !errors.Is(err, ErrInvitationExpired) {
		t.Fatalf("want ErrInvitationExpired, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ListInvitations
// ---------------------------------------------------------------------------

func TestInvitationService_ListInvitations(t *testing.T) {
	svc, fix, _ := newInvitationService(t)
	ctx := context.Background()
	admin := fix.NewUser().Admin().Build(ctx)

	fix.NewInvitation().InvitedByUser(admin).Build(ctx)
	fix.NewInvitation().InvitedByUser(admin).Expired().Build(ctx)

	all, err := svc.ListInvitations(ctx, InvitationFilterAll)
	if err != nil {
		t.Fatalf("ListInvitations(all): %v", err)
	}
	if len(all) != 2 {
		t.Errorf("ListInvitations(all) = %d, want 2", len(all))
	}

	unused, err := svc.ListInvitations(ctx, InvitationFilterUnused)
	if err != nil {
		t.Fatalf("ListInvitations(unused): %v", err)
	}
	if len(unused) != 1 {
		t.Errorf("ListInvitations(unused) = %d, want 1", len(unused))
	}

	expired, err := svc.ListInvitations(ctx, InvitationFilterExpired)
	if err != nil {
		t.Fatalf("ListInvitations(expired): %v", err)
	}
	if len(expired) != 1 {
		t.Errorf("ListInvitations(expired) = %d, want 1", len(expired))
	}
}

func TestInvitationService_ListInvitations_InvalidFilter(t *testing.T) {
	svc, _, _ := newInvitationService(t)
	ctx := context.Background()

	_, err := svc.ListInvitations(ctx, "bogus")
	if err == nil {
		t.Fatal("want error for invalid filter")
	}
}

// ---------------------------------------------------------------------------
// ArchiveInvitation
// ---------------------------------------------------------------------------

func TestInvitationService_ArchiveInvitation(t *testing.T) {
	svc, fix, _ := newInvitationService(t)
	ctx := context.Background()
	inv := fix.NewInvitation().Build(ctx)

	if err := svc.ArchiveInvitation(ctx, inv.ID); err != nil {
		t.Fatalf("ArchiveInvitation: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ResendInvitation
// ---------------------------------------------------------------------------

func TestInvitationService_ResendInvitation_NotFound(t *testing.T) {
	svc, _, _ := newInvitationService(t)
	ctx := context.Background()

	err := svc.ResendInvitation(ctx, [16]byte{1}, "http://localhost")
	if !errors.Is(err, ErrInvitationNotFound) {
		t.Fatalf("want ErrInvitationNotFound, got %v", err)
	}
}

func TestInvitationService_ResendInvitation_Used(t *testing.T) {
	svc, fix, _ := newInvitationService(t)
	ctx := context.Background()
	inv := fix.NewInvitation().Code("resend_used").Build(ctx)
	user := fix.NewUser().Build(ctx)
	svc.RedeemInvitation(ctx, "resend_used", user.ID)

	err := svc.ResendInvitation(ctx, inv.ID, "http://localhost")
	if !errors.Is(err, ErrInvitationUsed) {
		t.Fatalf("want ErrInvitationUsed, got %v", err)
	}
}
