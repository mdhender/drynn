package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mdhender/drynn/db/sqlc"
	"github.com/mdhender/drynn/internal/auth"
	"github.com/mdhender/drynn/internal/email"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const PasswordResetTTL = 30 * time.Minute

type PasswordResetService struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
	mailgun email.MailgunConfig
}

func NewPasswordResetService(pool *pgxpool.Pool, mailgun email.MailgunConfig) *PasswordResetService {
	return &PasswordResetService{pool: pool, queries: sqlc.New(pool), mailgun: mailgun}
}

func (s *PasswordResetService) SendReset(ctx context.Context, userID uuid.UUID, baseURL string) error {
	userRow, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("get user: %w", err)
	}

	return s.issueReset(ctx, userRow.ID, userRow.Email, baseURL)
}

// SendResetByEmail starts a self-service password reset for the supplied
// email. To avoid leaking which addresses are registered, unknown or
// malformed emails return nil without sending mail. A misconfigured mailer
// is still surfaced so operators notice.
func (s *PasswordResetService) SendResetByEmail(ctx context.Context, submittedEmail, baseURL string) error {
	normalizedEmail, err := normalizeEmail(submittedEmail)
	if err != nil {
		return nil
	}

	userRow, err := s.queries.GetUserByEmail(ctx, normalizedEmail)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("get user by email: %w", err)
	}

	return s.issueReset(ctx, userRow.ID, userRow.Email, baseURL)
}

func (s *PasswordResetService) issueReset(ctx context.Context, userID uuid.UUID, email, baseURL string) error {
	code, err := generateCode()
	if err != nil {
		return fmt.Errorf("generate reset code: %w", err)
	}

	if _, err := s.queries.CreatePasswordResetToken(ctx, sqlc.CreatePasswordResetTokenParams{
		UserID:    userID,
		Code:      code,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(PasswordResetTTL), Valid: true},
	}); err != nil {
		return fmt.Errorf("create password reset token: %w", err)
	}

	return s.sendResetEmail(ctx, email, code, baseURL)
}

func (s *PasswordResetService) sendResetEmail(ctx context.Context, to, code, baseURL string) error {
	if !mailgunConfigured(s.mailgun) {
		return ErrMailgunNotConfigured
	}

	link := strings.TrimRight(baseURL, "/") + "/reset-password?code=" + code
	body := fmt.Sprintf(
		`<p>A password reset was requested for your Hobo account.</p>`+
			`<p>Click the link below to choose a new password. This link expires in 30 minutes and can only be used once.</p>`+
			`<p><a href="%s">%s</a></p>`+
			`<p>If you didn't request this, you can ignore this email.</p>`,
		link, link,
	)

	if err := email.Send(ctx, s.mailgun, to, "Reset your Hobo password", body); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}

	return nil
}

func (s *PasswordResetService) ResetPassword(ctx context.Context, code, submittedEmail, newPassword string) error {
	normalizedEmail, emailErr := normalizeEmail(submittedEmail)
	pwErr := validatePassword(newPassword)

	row, err := s.queries.GetPasswordResetTokenByCode(ctx, code)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("get password reset token: %w", err)
	}

	tokenFound := err == nil
	codeMatches := false
	if tokenFound {
		codeMatches = subtle.ConstantTimeCompare([]byte(row.Code), []byte(code)) == 1
	}

	var userEmail string
	var userID uuid.UUID
	if tokenFound {
		userRow, uErr := s.queries.GetUserByID(ctx, row.UserID)
		if uErr == nil {
			userEmail = userRow.Email
			userID = userRow.ID
		}
	}

	emailMatches := false
	if emailErr == nil && userEmail != "" {
		emailMatches = subtle.ConstantTimeCompare([]byte(userEmail), []byte(normalizedEmail)) == 1
	}

	if pwErr != nil {
		return pwErr
	}

	tokenValid := tokenFound && codeMatches && !row.UsedAt.Valid && time.Now().Before(row.ExpiresAt.Time)
	if !tokenValid || !emailMatches || emailErr != nil {
		return ErrPasswordResetInvalid
	}

	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin reset tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := s.queries.WithTx(tx)
	if err := q.SetUserPassword(ctx, sqlc.SetUserPasswordParams{ID: userID, PasswordHash: hash}); err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	if err := q.MarkPasswordResetTokenUsed(ctx, row.ID); err != nil {
		return fmt.Errorf("mark token used: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit reset tx: %w", err)
	}

	return nil
}
