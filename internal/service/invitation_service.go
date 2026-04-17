package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mdhender/drynn/db/sqlc"
	"github.com/mdhender/drynn/internal/email"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const InvitationTTL = 7 * 24 * time.Hour

const (
	InvitationFilterAll     = ""
	InvitationFilterUnused  = "unused"
	InvitationFilterExpired = "expired"
	InvitationFilterUsed    = "used"
)

type Invitation struct {
	ID              uuid.UUID
	Email           string
	Code            string
	InvitedBy       uuid.UUID
	InvitedByHandle string
	UsedBy          *uuid.UUID
	UsedAt          *time.Time
	ExpiresAt       time.Time
	CreatedAt       time.Time
}

type CreateInvitationInput struct {
	Email   string
	BaseURL string
}

type InvitationService struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
	mailgun email.MailgunConfig
}

func NewInvitationService(pool *pgxpool.Pool, mailgun email.MailgunConfig) *InvitationService {
	return &InvitationService{pool: pool, queries: sqlc.New(pool), mailgun: mailgun}
}

func (s *InvitationService) CreateAndSend(ctx context.Context, inviterID uuid.UUID, input CreateInvitationInput) (*Invitation, error) {
	addr, err := normalizeEmail(input.Email)
	if err != nil {
		return nil, err
	}

	code, err := generateCode()
	if err != nil {
		return nil, fmt.Errorf("generate invitation code: %w", err)
	}

	row, err := s.queries.CreateInvitation(ctx, sqlc.CreateInvitationParams{
		Email:     addr,
		Code:      code,
		InvitedBy: inviterID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(InvitationTTL), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("create invitation: %w", err)
	}

	if err := s.sendInvitationEmail(ctx, addr, code, input.BaseURL); err != nil {
		return nil, err
	}

	return mapInvitationRow(row, ""), nil
}

func (s *InvitationService) ResendInvitation(ctx context.Context, id uuid.UUID, baseURL string) error {
	row, err := s.queries.GetInvitationByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvitationNotFound
		}
		return fmt.Errorf("get invitation: %w", err)
	}

	if row.UsedAt.Valid {
		return ErrInvitationUsed
	}

	newExpiry := time.Now().Add(InvitationTTL)
	if err := s.queries.UpdateInvitationExpiry(ctx, sqlc.UpdateInvitationExpiryParams{
		ID:        id,
		ExpiresAt: pgtype.Timestamptz{Time: newExpiry, Valid: true},
	}); err != nil {
		return fmt.Errorf("update invitation expiry: %w", err)
	}

	return s.sendInvitationEmail(ctx, row.Email, row.Code, baseURL)
}

func (s *InvitationService) ArchiveInvitation(ctx context.Context, id uuid.UUID) error {
	return s.queries.ArchiveInvitation(ctx, id)
}

func (s *InvitationService) sendInvitationEmail(ctx context.Context, to, code, baseURL string) error {
	if !s.mailgun.Configured() {
		return ErrMailgunNotConfigured
	}

	link := strings.TrimRight(baseURL, "/") + "/register?code=" + code
	body, err := email.RenderTemplate("invitation.gohtml", struct{ Link string }{Link: link})
	if err != nil {
		return fmt.Errorf("render invitation email: %w", err)
	}

	if err := email.Send(ctx, s.mailgun, to, "You're invited to Drynn", body); err != nil {
		return fmt.Errorf("send invitation email: %w", err)
	}

	return nil
}

func (s *InvitationService) ListInvitations(ctx context.Context, filter string) ([]Invitation, error) {
	switch filter {
	case InvitationFilterAll, InvitationFilterUnused, InvitationFilterExpired, InvitationFilterUsed:
	default:
		return nil, fmt.Errorf("list invitations: unknown filter %q", filter)
	}

	rows, err := s.queries.ListInvitations(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}

	invitations := make([]Invitation, 0, len(rows))
	for _, row := range rows {
		inv := mapInvitationRow(sqlc.Invitation{
			ID:         row.ID,
			Email:      row.Email,
			Code:       row.Code,
			InvitedBy:  row.InvitedBy,
			UsedBy:     row.UsedBy,
			UsedAt:     row.UsedAt,
			ExpiresAt:  row.ExpiresAt,
			ArchivedAt: row.ArchivedAt,
			CreatedAt:  row.CreatedAt,
		}, row.InvitedByHandle)
		invitations = append(invitations, *inv)
	}

	return invitations, nil
}

func (s *InvitationService) ValidateCode(ctx context.Context, code string) (*Invitation, error) {
	row, err := s.queries.GetInvitationByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvitationNotFound
		}
		return nil, fmt.Errorf("get invitation by code: %w", err)
	}

	if row.UsedAt.Valid {
		return nil, ErrInvitationUsed
	}

	if time.Now().After(row.ExpiresAt.Time) {
		return nil, ErrInvitationExpired
	}

	return mapInvitationRow(row, ""), nil
}

func (s *InvitationService) RedeemInvitation(ctx context.Context, code string, userID uuid.UUID) error {
	inv, err := s.ValidateCode(ctx, code)
	if err != nil {
		return err
	}

	return s.queries.MarkInvitationUsed(ctx, sqlc.MarkInvitationUsedParams{
		ID:     inv.ID,
		UsedBy: pgtype.UUID{Bytes: userID, Valid: true},
	})
}

func mapInvitationRow(row sqlc.Invitation, invitedByHandle string) *Invitation {
	inv := &Invitation{
		ID:              row.ID,
		Email:           row.Email,
		Code:            row.Code,
		InvitedBy:       row.InvitedBy,
		InvitedByHandle: invitedByHandle,
		ExpiresAt:       row.ExpiresAt.Time,
		CreatedAt:       row.CreatedAt.Time,
	}
	if row.UsedBy.Valid {
		id := uuid.UUID(row.UsedBy.Bytes)
		inv.UsedBy = &id
	}
	if row.UsedAt.Valid {
		t := row.UsedAt.Time
		inv.UsedAt = &t
	}
	return inv
}

func generateCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
