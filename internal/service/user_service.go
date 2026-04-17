package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mdhender/drynn/db/sqlc"
	"github.com/mdhender/drynn/internal/auth"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RoleUser   = "user"
	RoleAdmin  = "admin"
	RoleTester = "tester"
)

var handlePattern = regexp.MustCompile(`^[a-z0-9_]{3,32}$`)

type User struct {
	ID        uuid.UUID
	Handle    string
	Email     string
	IsActive  bool
	Roles     []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type RoleOption struct {
	Name        string
	Description string
}

type RegisterInput struct {
	Handle   string
	Email    string
	Password string
}

type SignInInput struct {
	Email    string
	Password string
}

type UpdateProfileInput struct {
	UserID uuid.UUID
	Handle string
	Email  string
}

type CreateUserInput struct {
	Handle   string
	Email    string
	Password string
	IsActive bool
	Roles    []string
}

type UpdateUserInput struct {
	ActorUserID uuid.UUID
	UserID      uuid.UUID
	Handle      string
	Email       string
	Password    string
	IsActive    bool
	Roles       []string
}

type UserService struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewUserService(pool *pgxpool.Pool) *UserService {
	return &UserService{pool: pool, queries: sqlc.New(pool)}
}

func (s *UserService) Register(ctx context.Context, input RegisterInput) (*User, error) {
	handle, err := normalizeHandle(input.Handle)
	if err != nil {
		return nil, err
	}

	email, err := normalizeEmail(input.Email)
	if err != nil {
		return nil, err
	}

	if err := validatePassword(input.Password); err != nil {
		return nil, err
	}

	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin register tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := s.queries.WithTx(tx)
	userRow, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		Handle:       handle,
		Email:        email,
		PasswordHash: hash,
		IsActive:     true,
	})
	if err != nil {
		return nil, mapConstraintError(err)
	}

	if err := setRoles(ctx, q, userRow.ID, []string{RoleUser}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit register tx: %w", err)
	}

	return s.GetUser(ctx, userRow.ID)
}

func (s *UserService) Authenticate(ctx context.Context, input SignInInput) (*User, error) {
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	row, err := s.queries.GetUserForAuthByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}

		return nil, fmt.Errorf("get user for auth: %w", err)
	}

	if !row.IsActive {
		return nil, ErrInactiveUser
	}

	if err := auth.ComparePassword(row.PasswordHash, input.Password); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.hydrateUser(ctx, s.queries, row)
}

func (s *UserService) GetUser(ctx context.Context, userID uuid.UUID) (*User, error) {
	row, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}

		return nil, fmt.Errorf("get user: %w", err)
	}

	return s.hydrateUser(ctx, s.queries, row)
}

func (s *UserService) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.queries.ListUsersWithRoles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	var users []User
	seen := make(map[uuid.UUID]int)
	for _, row := range rows {
		idx, ok := seen[row.ID]
		if !ok {
			idx = len(users)
			seen[row.ID] = idx
			users = append(users, User{
				ID:        row.ID,
				Handle:    row.Handle,
				Email:     row.Email,
				IsActive:  row.IsActive,
				CreatedAt: row.CreatedAt.Time,
				UpdatedAt: row.UpdatedAt.Time,
			})
		}
		if row.RoleName != "" {
			users[idx].Roles = append(users[idx].Roles, row.RoleName)
		}
	}

	return users, nil
}

func (s *UserService) ListRoles(ctx context.Context) ([]RoleOption, error) {
	rows, err := s.queries.ListRoles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}

	roles := make([]RoleOption, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, RoleOption{Name: row.Name, Description: row.Description})
	}

	return roles, nil
}

func (s *UserService) UpdateProfile(ctx context.Context, input UpdateProfileInput) (*User, error) {
	handle, err := normalizeHandle(input.Handle)
	if err != nil {
		return nil, err
	}

	email, err := normalizeEmail(input.Email)
	if err != nil {
		return nil, err
	}

	row, err := s.queries.UpdateUserProfile(ctx, sqlc.UpdateUserProfileParams{
		ID:     input.UserID,
		Handle: handle,
		Email:  email,
	})
	if err != nil {
		return nil, mapConstraintError(err)
	}

	return s.hydrateUser(ctx, s.queries, row)
}

func (s *UserService) CreateUser(ctx context.Context, input CreateUserInput) (*User, error) {
	handle, err := normalizeHandle(input.Handle)
	if err != nil {
		return nil, err
	}

	email, err := normalizeEmail(input.Email)
	if err != nil {
		return nil, err
	}

	if err := validatePassword(input.Password); err != nil {
		return nil, err
	}

	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create user tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := s.queries.WithTx(tx)
	row, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		Handle:       handle,
		Email:        email,
		PasswordHash: hash,
		IsActive:     input.IsActive,
	})
	if err != nil {
		return nil, mapConstraintError(err)
	}

	if err := setRoles(ctx, q, row.ID, normalizeRoles(input.Roles)); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create user tx: %w", err)
	}

	return s.GetUser(ctx, row.ID)
}

func (s *UserService) UpdateUser(ctx context.Context, input UpdateUserInput) (*User, error) {
	handle, err := normalizeHandle(input.Handle)
	if err != nil {
		return nil, err
	}

	email, err := normalizeEmail(input.Email)
	if err != nil {
		return nil, err
	}

	roles := normalizeRoles(input.Roles)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin update user tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := s.queries.WithTx(tx)
	existing, err := s.getUserWithQueries(ctx, q, input.UserID)
	if err != nil {
		return nil, err
	}

	if err := ensureActiveAdminInvariant(ctx, q, existing.Roles, roles, input.IsActive); err != nil {
		return nil, err
	}

	if _, err := q.AdminUpdateUser(ctx, sqlc.AdminUpdateUserParams{
		ID:       input.UserID,
		Handle:   handle,
		Email:    email,
		IsActive: input.IsActive,
	}); err != nil {
		return nil, mapConstraintError(err)
	}

	if strings.TrimSpace(input.Password) != "" {
		if err := validatePassword(input.Password); err != nil {
			return nil, err
		}

		hash, err := auth.HashPassword(input.Password)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}

		if err := q.SetUserPassword(ctx, sqlc.SetUserPasswordParams{ID: input.UserID, PasswordHash: hash}); err != nil {
			return nil, fmt.Errorf("set password: %w", err)
		}
	}

	if err := setRoles(ctx, q, input.UserID, roles); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit update user tx: %w", err)
	}

	return s.GetUser(ctx, input.UserID)
}

func (s *UserService) DeleteUser(ctx context.Context, actorUserID, targetUserID uuid.UUID) error {
	if actorUserID == targetUserID {
		return ErrCannotDeleteSelf
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete user tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := s.queries.WithTx(tx)
	existing, err := s.getUserWithQueries(ctx, q, targetUserID)
	if err != nil {
		return err
	}

	if err := ensureDeleteAllowed(ctx, q, existing.Roles); err != nil {
		return err
	}

	if err := q.DeleteUser(ctx, targetUserID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete user tx: %w", err)
	}

	return nil
}

func (s *UserService) EnsureBootstrapAdmin(ctx context.Context, handle, email, password string) error {
	handle = strings.TrimSpace(handle)
	email = strings.TrimSpace(email)
	if strings.TrimSpace(password) == "" && handle == "" && email == "" {
		return nil
	}
	if handle == "" || email == "" || password == "" {
		return fmt.Errorf("bootstrap admin handle, email, and password must all be set together")
	}

	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return err
	}

	row, err := s.queries.GetUserByEmail(ctx, normalizedEmail)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("lookup bootstrap admin: %w", err)
	}

	roles := []string{RoleUser, RoleAdmin}
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = s.CreateUser(ctx, CreateUserInput{
			Handle:   handle,
			Email:    email,
			Password: password,
			IsActive: true,
			Roles:    roles,
		})
		return err
	}

	_, err = s.UpdateUser(ctx, UpdateUserInput{
		ActorUserID: row.ID,
		UserID:      row.ID,
		Handle:      handle,
		Email:       email,
		Password:    password,
		IsActive:    true,
		Roles:       roles,
	})
	return err
}

func (s *UserService) SeedTesters(ctx context.Context, target int, emailDomain, sentinelHash string) (created int, err error) {
	if target < 0 {
		return 0, fmt.Errorf("seed-testers count must be non-negative")
	}
	if strings.TrimSpace(emailDomain) == "" {
		return 0, fmt.Errorf("seed-testers email domain is required")
	}
	if strings.TrimSpace(sentinelHash) == "" {
		return 0, fmt.Errorf("seed-testers sentinel hash is required")
	}

	current, err := s.queries.CountUsersByRole(ctx, RoleTester)
	if err != nil {
		return 0, fmt.Errorf("count tester role: %w", err)
	}
	if int64(target) <= current {
		return 0, nil
	}

	need := target - int(current)
	next := int(current) + 1
	for created < need {
		handle := fmt.Sprintf("tester_%d", next)
		email := fmt.Sprintf("tester_%d@%s", next, emailDomain)

		err := s.createTesterRow(ctx, handle, email, sentinelHash)
		if err == nil {
			created++
			next++
			continue
		}
		if errors.Is(err, ErrHandleTaken) || errors.Is(err, ErrEmailTaken) {
			next++
			continue
		}
		return created, err
	}

	return created, nil
}

func (s *UserService) createTesterRow(ctx context.Context, handle, email, sentinelHash string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin seed tester tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := s.queries.WithTx(tx)
	row, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		Handle:       handle,
		Email:        email,
		PasswordHash: sentinelHash,
		IsActive:     true,
	})
	if err != nil {
		return mapConstraintError(err)
	}

	if err := setRoles(ctx, q, row.ID, []string{RoleUser, RoleTester}); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit seed tester tx: %w", err)
	}

	return nil
}

func (s *UserService) getUserWithQueries(ctx context.Context, q *sqlc.Queries, userID uuid.UUID) (*User, error) {
	row, err := q.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}

		return nil, fmt.Errorf("get user: %w", err)
	}

	return s.hydrateUser(ctx, q, row)
}

func (s *UserService) hydrateUser(ctx context.Context, q *sqlc.Queries, row sqlc.User) (*User, error) {
	roles, err := q.ListRoleNamesByUser(ctx, row.ID)
	if err != nil {
		return nil, fmt.Errorf("list user roles: %w", err)
	}

	return &User{
		ID:        row.ID,
		Handle:    row.Handle,
		Email:     row.Email,
		IsActive:  row.IsActive,
		Roles:     roles,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func normalizeHandle(handle string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(handle))
	if !handlePattern.MatchString(normalized) {
		return "", ErrInvalidHandle
	}

	return normalized, nil
}

func normalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	parsed, err := mail.ParseAddress(normalized)
	if err != nil || parsed.Address != normalized {
		return "", ErrInvalidEmail
	}

	return normalized, nil
}

func (s *UserService) SetPasswordByEmail(ctx context.Context, email, password string) error {
	email, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	if err := validatePassword(password); err != nil {
		return err
	}

	row, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("get user by email: %w", err)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.queries.SetUserPassword(ctx, sqlc.SetUserPasswordParams{ID: row.ID, PasswordHash: hash}); err != nil {
		return fmt.Errorf("set password: %w", err)
	}

	return nil
}

func validatePassword(password string) error {
	if len(strings.TrimSpace(password)) < 8 {
		return ErrInvalidPassword
	}

	return nil
}

func normalizeRoles(roles []string) []string {
	seen := map[string]struct{}{RoleUser: {}}
	for _, role := range roles {
		role = strings.ToLower(strings.TrimSpace(role))
		switch role {
		case RoleUser, RoleAdmin, RoleTester:
			seen[role] = struct{}{}
		}
	}

	normalized := make([]string, 0, len(seen))
	for _, role := range []string{RoleUser, RoleAdmin, RoleTester} {
		if _, ok := seen[role]; ok {
			normalized = append(normalized, role)
		}
	}

	return normalized
}

func setRoles(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, roles []string) error {
	if err := q.RemoveAllRolesFromUser(ctx, userID); err != nil {
		return fmt.Errorf("clear user roles: %w", err)
	}

	for _, role := range normalizeRoles(roles) {
		if err := q.AddRoleToUser(ctx, sqlc.AddRoleToUserParams{UserID: userID, Name: role}); err != nil {
			return fmt.Errorf("assign role %q: %w", role, err)
		}
	}

	return nil
}

func ensureDeleteAllowed(ctx context.Context, q *sqlc.Queries, existingRoles []string) error {
	if !slices.Contains(existingRoles, RoleAdmin) {
		return nil
	}

	count, err := q.CountUsersByRole(ctx, RoleAdmin)
	if err != nil {
		return fmt.Errorf("count admins before delete: %w", err)
	}
	if count <= 1 {
		return ErrLastAdmin
	}

	return nil
}

func ensureActiveAdminInvariant(ctx context.Context, q *sqlc.Queries, existingRoles, newRoles []string, isActive bool) error {
	if !slices.Contains(existingRoles, RoleAdmin) {
		return nil
	}
	if slices.Contains(newRoles, RoleAdmin) && isActive {
		return nil
	}

	count, err := q.CountUsersByRole(ctx, RoleAdmin)
	if err != nil {
		return fmt.Errorf("count admins before update: %w", err)
	}
	if count <= 1 {
		return ErrLastAdmin
	}

	return nil
}

func mapConstraintError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return fmt.Errorf("database operation: %w", err)
	}

	switch pgErr.ConstraintName {
	case "users_email_key":
		return ErrEmailTaken
	case "users_handle_key":
		return ErrHandleTaken
	case "users_handle_format", "users_handle_length", "users_handle_lowercase":
		return ErrInvalidHandle
	case "users_email_lowercase", "users_email_length":
		return ErrInvalidEmail
	default:
		return fmt.Errorf("database constraint %s: %w", pgErr.ConstraintName, err)
	}
}
