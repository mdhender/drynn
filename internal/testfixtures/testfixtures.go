// Package testfixtures provides data builders for inserting test rows
// into a Postgres database managed by [testdb.New].
//
// Each builder sets sensible defaults so tests only specify the fields
// they care about. Unique values (handles, emails, codes) are generated
// from a per-Fixtures sequence counter, making parallel tests safe.
//
// Builders call t.Fatal on any database error, so the calling test never
// needs to check errors from fixture setup.
package testfixtures

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/mdhender/drynn/db/sqlc"
)

// Fixtures is the entry point for creating test data. Use [New] to
// obtain an instance, then call builder methods like [Fixtures.NewUser].
type Fixtures struct {
	t   testing.TB
	q   *sqlc.Queries
	seq atomic.Uint64
}

// New returns a Fixtures backed by the given pool. Typically the pool
// comes from testdb.New(t).
func New(t testing.TB, pool *pgxpool.Pool) *Fixtures {
	t.Helper()
	return &Fixtures{t: t, q: sqlc.New(pool)}
}

func (f *Fixtures) next() uint64 { return f.seq.Add(1) }

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func randomCode() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("testfixtures: rand.Read: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// User builder
// ---------------------------------------------------------------------------

// UserBuilder creates a user row with optional role assignments.
type UserBuilder struct {
	f        *Fixtures
	handle   string
	email    string
	password string
	isActive bool
	roles    []string
}

// NewUser returns a builder pre-filled with unique defaults.
func (f *Fixtures) NewUser() *UserBuilder {
	n := f.next()
	return &UserBuilder{
		f:        f,
		handle:   fmt.Sprintf("user_%d", n),
		email:    fmt.Sprintf("user_%d@test.local", n),
		password: "password123",
		isActive: true,
	}
}

func (b *UserBuilder) Handle(h string) *UserBuilder    { b.handle = h; return b }
func (b *UserBuilder) Email(e string) *UserBuilder      { b.email = e; return b }
func (b *UserBuilder) Password(p string) *UserBuilder   { b.password = p; return b }
func (b *UserBuilder) Inactive() *UserBuilder           { b.isActive = false; return b }
func (b *UserBuilder) Roles(roles ...string) *UserBuilder { b.roles = roles; return b }
func (b *UserBuilder) Admin() *UserBuilder              { return b.Roles("admin", "user") }

// Build inserts the user (and any requested roles) and returns the row.
func (b *UserBuilder) Build(ctx context.Context) sqlc.User {
	b.f.t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(b.password), bcrypt.MinCost)
	if err != nil {
		b.f.t.Fatalf("testfixtures: hash password: %v", err)
	}

	user, err := b.f.q.CreateUser(ctx, sqlc.CreateUserParams{
		Handle:       b.handle,
		Email:        b.email,
		PasswordHash: string(hash),
		IsActive:     b.isActive,
	})
	if err != nil {
		b.f.t.Fatalf("testfixtures: create user %q: %v", b.handle, err)
	}

	for _, role := range b.roles {
		if err := b.f.q.AddRoleToUser(ctx, sqlc.AddRoleToUserParams{
			UserID: user.ID,
			Name:   role,
		}); err != nil {
			b.f.t.Fatalf("testfixtures: add role %q to %q: %v", role, b.handle, err)
		}
	}

	return user
}

// ---------------------------------------------------------------------------
// Game builder
// ---------------------------------------------------------------------------

// GameBuilder creates a game row.
type GameBuilder struct {
	f    *Fixtures
	name string
}

// NewGame returns a builder pre-filled with a unique default name.
func (f *Fixtures) NewGame() *GameBuilder {
	n := f.next()
	return &GameBuilder{f: f, name: fmt.Sprintf("game_%d", n)}
}

func (b *GameBuilder) Name(name string) *GameBuilder { b.name = name; return b }

// Build inserts the game and returns the row.
func (b *GameBuilder) Build(ctx context.Context) sqlc.Game {
	b.f.t.Helper()
	game, err := b.f.q.CreateGame(ctx, b.name)
	if err != nil {
		b.f.t.Fatalf("testfixtures: create game %q: %v", b.name, err)
	}
	return game
}

// ---------------------------------------------------------------------------
// Invitation builder
// ---------------------------------------------------------------------------

// InvitationBuilder creates an invitation row.
type InvitationBuilder struct {
	f         *Fixtures
	email     string
	code      string
	invitedBy func(context.Context) sqlc.User
	expiresAt time.Time
}

// NewInvitation returns a builder with a unique email, random code, and
// 7-day expiry. If no inviter is set via [InvitationBuilder.InvitedBy],
// a throwaway admin user is created automatically.
func (f *Fixtures) NewInvitation() *InvitationBuilder {
	n := f.next()
	return &InvitationBuilder{
		f:         f,
		email:     fmt.Sprintf("invite_%d@test.local", n),
		code:      randomCode(),
		expiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
}

func (b *InvitationBuilder) Email(e string) *InvitationBuilder     { b.email = e; return b }
func (b *InvitationBuilder) Code(c string) *InvitationBuilder      { b.code = c; return b }
func (b *InvitationBuilder) ExpiresAt(t time.Time) *InvitationBuilder { b.expiresAt = t; return b }
func (b *InvitationBuilder) Expired() *InvitationBuilder {
	b.expiresAt = time.Now().Add(-1 * time.Hour)
	return b
}

// InvitedByUser sets a specific user as the inviter.
func (b *InvitationBuilder) InvitedByUser(u sqlc.User) *InvitationBuilder {
	b.invitedBy = func(context.Context) sqlc.User { return u }
	return b
}

// Build inserts the invitation and returns the row.
func (b *InvitationBuilder) Build(ctx context.Context) sqlc.Invitation {
	b.f.t.Helper()

	var inviter sqlc.User
	if b.invitedBy != nil {
		inviter = b.invitedBy(ctx)
	} else {
		inviter = b.f.NewUser().Admin().Build(ctx)
	}

	inv, err := b.f.q.CreateInvitation(ctx, sqlc.CreateInvitationParams{
		Email:     b.email,
		Code:      b.code,
		InvitedBy: inviter.ID,
		ExpiresAt: timestamptz(b.expiresAt),
	})
	if err != nil {
		b.f.t.Fatalf("testfixtures: create invitation for %q: %v", b.email, err)
	}
	return inv
}

// ---------------------------------------------------------------------------
// Password reset token builder
// ---------------------------------------------------------------------------

// PasswordResetTokenBuilder creates a password_reset_tokens row.
type PasswordResetTokenBuilder struct {
	f         *Fixtures
	userFn    func(context.Context) sqlc.User
	code      string
	expiresAt time.Time
}

// NewPasswordResetToken returns a builder with a random code and 30-minute
// expiry. If no user is set via [PasswordResetTokenBuilder.ForUser], a
// throwaway user is created automatically.
func (f *Fixtures) NewPasswordResetToken() *PasswordResetTokenBuilder {
	return &PasswordResetTokenBuilder{
		f:         f,
		code:      randomCode(),
		expiresAt: time.Now().Add(30 * time.Minute),
	}
}

func (b *PasswordResetTokenBuilder) Code(c string) *PasswordResetTokenBuilder { b.code = c; return b }
func (b *PasswordResetTokenBuilder) ExpiresAt(t time.Time) *PasswordResetTokenBuilder {
	b.expiresAt = t
	return b
}
func (b *PasswordResetTokenBuilder) Expired() *PasswordResetTokenBuilder {
	b.expiresAt = time.Now().Add(-1 * time.Hour)
	return b
}

// ForUser ties the token to a specific user.
func (b *PasswordResetTokenBuilder) ForUser(u sqlc.User) *PasswordResetTokenBuilder {
	b.userFn = func(context.Context) sqlc.User { return u }
	return b
}

// Build inserts the token and returns the row.
func (b *PasswordResetTokenBuilder) Build(ctx context.Context) sqlc.PasswordResetToken {
	b.f.t.Helper()

	var user sqlc.User
	if b.userFn != nil {
		user = b.userFn(ctx)
	} else {
		user = b.f.NewUser().Build(ctx)
	}

	tok, err := b.f.q.CreatePasswordResetToken(ctx, sqlc.CreatePasswordResetTokenParams{
		UserID:    user.ID,
		Code:      b.code,
		ExpiresAt: timestamptz(b.expiresAt),
	})
	if err != nil {
		b.f.t.Fatalf("testfixtures: create password reset token: %v", err)
	}
	return tok
}

// ---------------------------------------------------------------------------
// JWT signing key builder
// ---------------------------------------------------------------------------

// SigningKeyBuilder creates a jwt_signing_keys row.
type SigningKeyBuilder struct {
	f         *Fixtures
	tokenType string
	algorithm string
	secret    []byte
	state     string
}

// NewSigningKey returns a builder for an active access signing key
// with a random 32-byte secret.
func (f *Fixtures) NewSigningKey() *SigningKeyBuilder {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		f.t.Fatalf("testfixtures: generate secret: %v", err)
	}
	return &SigningKeyBuilder{
		f:         f,
		tokenType: "access",
		algorithm: "HS256",
		secret:    secret,
		state:     "active",
	}
}

func (b *SigningKeyBuilder) TokenType(tt string) *SigningKeyBuilder { b.tokenType = tt; return b }
func (b *SigningKeyBuilder) Refresh() *SigningKeyBuilder            { b.tokenType = "refresh"; return b }
func (b *SigningKeyBuilder) Secret(s []byte) *SigningKeyBuilder     { b.secret = s; return b }
func (b *SigningKeyBuilder) State(s string) *SigningKeyBuilder      { b.state = s; return b }
func (b *SigningKeyBuilder) Retired() *SigningKeyBuilder            { b.state = "retired"; return b }

// Build inserts the signing key and returns the row.
func (b *SigningKeyBuilder) Build(ctx context.Context) sqlc.JwtSigningKey {
	b.f.t.Helper()

	key, err := b.f.q.CreateJWTSigningKey(ctx, sqlc.CreateJWTSigningKeyParams{
		TokenType: b.tokenType,
		Algorithm: b.algorithm,
		Secret:    b.secret,
		State:     b.state,
	})
	if err != nil {
		b.f.t.Fatalf("testfixtures: create signing key (%s/%s): %v", b.tokenType, b.state, err)
	}
	return key
}
