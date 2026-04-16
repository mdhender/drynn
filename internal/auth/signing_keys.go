package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mdhender/drynn/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const signingAlgorithmHS256 = "HS256"

const (
	keyStateActive  = "active"
	keyStateRetired = "retired"
)

var (
	ErrInvalidTokenType          = errors.New("token type must be access or refresh")
	ErrNoActiveSigningKey        = errors.New("no active signing key configured")
	ErrSigningKeyNotFound        = errors.New("signing key not found")
	ErrSigningKeyUnavailable     = errors.New("signing key is not valid for verification")
	ErrCannotDeleteActiveKey     = errors.New("active signing keys must be rotated or expired before deletion")
	ErrNegativeVerificationGrace = errors.New("verification grace period must be non-negative")
)

type SigningKey struct {
	ID          uuid.UUID
	TokenType   string
	Algorithm   string
	Secret      []byte
	State       string
	VerifyUntil *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type KeyStore struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewKeyStore(pool *pgxpool.Pool) *KeyStore {
	return &KeyStore{pool: pool, queries: sqlc.New(pool)}
}

func (s *KeyStore) EnsureReady(ctx context.Context) error {
	for _, tokenType := range []string{TokenTypeAccess, TokenTypeRefresh} {
		if _, err := s.ActiveSigningKey(ctx, tokenType); err != nil {
			return fmt.Errorf("%s signing key: %w", tokenType, err)
		}
	}

	return nil
}

func (s *KeyStore) ActiveSigningKey(ctx context.Context, tokenType string) (*SigningKey, error) {
	normalizedType, err := normalizeTokenType(tokenType)
	if err != nil {
		return nil, err
	}

	row, err := s.queries.GetActiveJWTSigningKey(ctx, normalizedType)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoActiveSigningKey
		}

		return nil, fmt.Errorf("get active signing key: %w", err)
	}

	key := hydrateSigningKey(row)
	return &key, nil
}

func (s *KeyStore) VerificationKey(ctx context.Context, keyID uuid.UUID, tokenType string, now time.Time) (*SigningKey, error) {
	normalizedType, err := normalizeTokenType(tokenType)
	if err != nil {
		return nil, err
	}

	row, err := s.queries.GetJWTSigningKeyByID(ctx, keyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSigningKeyNotFound
		}

		return nil, fmt.Errorf("get signing key: %w", err)
	}

	key := hydrateSigningKey(row)
	if key.TokenType != normalizedType {
		return nil, ErrSigningKeyUnavailable
	}

	switch key.State {
	case keyStateActive:
		return &key, nil
	case keyStateRetired:
		if key.VerifyUntil != nil && now.Before(key.VerifyUntil.UTC()) {
			return &key, nil
		}
	}

	return nil, ErrSigningKeyUnavailable
}

func (s *KeyStore) CreateSigningKey(ctx context.Context, tokenType string, verifyOldFor time.Duration) (*SigningKey, *SigningKey, error) {
	normalizedType, err := normalizeTokenType(tokenType)
	if err != nil {
		return nil, nil, err
	}
	if verifyOldFor < 0 {
		return nil, nil, ErrNegativeVerificationGrace
	}

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, nil, fmt.Errorf("generate signing key secret: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin create signing key tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := s.queries.WithTx(tx)
	var retiredKey *SigningKey
	activeRow, err := q.GetActiveJWTSigningKeyForUpdate(ctx, normalizedType)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, fmt.Errorf("lock active signing key: %w", err)
	}
	if err == nil {
		retiredRow, err := q.RetireJWTSigningKey(ctx, sqlc.RetireJWTSigningKeyParams{
			ID:          activeRow.ID,
			VerifyUntil: timestamptzFromTime(time.Now().UTC().Add(verifyOldFor)),
		})
		if err != nil {
			return nil, nil, fmt.Errorf("retire active signing key: %w", err)
		}

		retired := hydrateSigningKey(retiredRow)
		retiredKey = &retired
	}

	createdRow, err := q.CreateJWTSigningKey(ctx, sqlc.CreateJWTSigningKeyParams{
		TokenType:   normalizedType,
		Algorithm:   signingAlgorithmHS256,
		Secret:      secret,
		State:       keyStateActive,
		VerifyUntil: pgtype.Timestamptz{},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create signing key: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit create signing key tx: %w", err)
	}

	created := hydrateSigningKey(createdRow)
	return &created, retiredKey, nil
}

func (s *KeyStore) ExpireSigningKey(ctx context.Context, keyID uuid.UUID, verifyFor time.Duration) (*SigningKey, error) {
	if verifyFor < 0 {
		return nil, ErrNegativeVerificationGrace
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin expire signing key tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := s.queries.WithTx(tx)
	row, err := q.GetJWTSigningKeyByIDForUpdate(ctx, keyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSigningKeyNotFound
		}

		return nil, fmt.Errorf("lock signing key: %w", err)
	}

	retiredRow, err := q.RetireJWTSigningKey(ctx, sqlc.RetireJWTSigningKeyParams{
		ID:          row.ID,
		VerifyUntil: timestamptzFromTime(time.Now().UTC().Add(verifyFor)),
	})
	if err != nil {
		return nil, fmt.Errorf("expire signing key: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit expire signing key tx: %w", err)
	}

	retired := hydrateSigningKey(retiredRow)
	return &retired, nil
}

func (s *KeyStore) DeleteSigningKey(ctx context.Context, keyID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete signing key tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := s.queries.WithTx(tx)
	row, err := q.GetJWTSigningKeyByIDForUpdate(ctx, keyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSigningKeyNotFound
		}

		return fmt.Errorf("lock signing key: %w", err)
	}
	if row.State == keyStateActive {
		return ErrCannotDeleteActiveKey
	}

	if err := q.DeleteJWTSigningKey(ctx, keyID); err != nil {
		return fmt.Errorf("delete signing key: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete signing key tx: %w", err)
	}

	return nil
}

func hydrateSigningKey(row sqlc.JwtSigningKey) SigningKey {
	var verifyUntil *time.Time
	if row.VerifyUntil.Valid {
		timeValue := row.VerifyUntil.Time.UTC()
		verifyUntil = &timeValue
	}

	return SigningKey{
		ID:          row.ID,
		TokenType:   row.TokenType,
		Algorithm:   row.Algorithm,
		Secret:      row.Secret,
		State:       row.State,
		VerifyUntil: verifyUntil,
		CreatedAt:   row.CreatedAt.Time.UTC(),
		UpdatedAt:   row.UpdatedAt.Time.UTC(),
	}
}

func normalizeTokenType(tokenType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(tokenType)) {
	case TokenTypeAccess:
		return TokenTypeAccess, nil
	case TokenTypeRefresh:
		return TokenTypeRefresh, nil
	default:
		return "", ErrInvalidTokenType
	}
}

func timestamptzFromTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}
