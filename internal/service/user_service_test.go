package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/mdhender/drynn/internal/auth"
	"github.com/mdhender/drynn/internal/testdb"
	"github.com/mdhender/drynn/internal/testfixtures"
)

func newUserService(t testing.TB) (*UserService, *testfixtures.Fixtures) {
	t.Helper()
	pool := testdb.New(t)
	return NewUserService(pool), testfixtures.New(t, pool)
}

// ---------------------------------------------------------------------------
// Register
// ---------------------------------------------------------------------------

func TestUserService_Register(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()

	user, err := svc.Register(ctx, RegisterInput{
		Handle:   "alice",
		Email:    "alice@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if user.Handle != "alice" {
		t.Errorf("Handle = %q, want %q", user.Handle, "alice")
	}
	if user.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", user.Email, "alice@example.com")
	}
	if !user.IsActive {
		t.Error("new user should be active")
	}
	if len(user.Roles) != 1 || user.Roles[0] != RoleUser {
		t.Errorf("Roles = %v, want [user]", user.Roles)
	}
}

func TestUserService_Register_NormalizesInput(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()

	user, err := svc.Register(ctx, RegisterInput{
		Handle:   "  Alice  ",
		Email:    "  ALICE@Example.COM  ",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if user.Handle != "alice" {
		t.Errorf("Handle = %q, want normalized %q", user.Handle, "alice")
	}
	if user.Email != "alice@example.com" {
		t.Errorf("Email = %q, want normalized %q", user.Email, "alice@example.com")
	}
}

func TestUserService_Register_DuplicateEmail(t *testing.T) {
	svc, fix := newUserService(t)
	ctx := context.Background()
	fix.NewUser().Email("taken@example.com").Build(ctx)

	_, err := svc.Register(ctx, RegisterInput{
		Handle: "other", Email: "taken@example.com", Password: "password123",
	})
	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("want ErrEmailTaken, got %v", err)
	}
}

func TestUserService_Register_DuplicateHandle(t *testing.T) {
	svc, fix := newUserService(t)
	ctx := context.Background()
	fix.NewUser().Handle("taken").Build(ctx)

	_, err := svc.Register(ctx, RegisterInput{
		Handle: "taken", Email: "new@example.com", Password: "password123",
	})
	if !errors.Is(err, ErrHandleTaken) {
		t.Fatalf("want ErrHandleTaken, got %v", err)
	}
}

func TestUserService_Register_InvalidPassword(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()

	_, err := svc.Register(ctx, RegisterInput{
		Handle: "alice", Email: "alice@example.com", Password: "short",
	})
	if !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("want ErrInvalidPassword, got %v", err)
	}
}

func TestUserService_Register_InvalidHandle(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()

	_, err := svc.Register(ctx, RegisterInput{
		Handle: "ab", Email: "alice@example.com", Password: "password123",
	})
	if !errors.Is(err, ErrInvalidHandle) {
		t.Fatalf("want ErrInvalidHandle, got %v", err)
	}
}

func TestUserService_Register_InvalidEmail(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()

	_, err := svc.Register(ctx, RegisterInput{
		Handle: "alice", Email: "not-an-email", Password: "password123",
	})
	if !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("want ErrInvalidEmail, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Authenticate
// ---------------------------------------------------------------------------

func TestUserService_Authenticate(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()

	svc.Register(ctx, RegisterInput{Handle: "alice", Email: "alice@example.com", Password: "password123"})

	user, err := svc.Authenticate(ctx, SignInInput{Email: "alice@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if user.Handle != "alice" {
		t.Errorf("Handle = %q, want %q", user.Handle, "alice")
	}
}

func TestUserService_Authenticate_WrongPassword(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()
	svc.Register(ctx, RegisterInput{Handle: "alice", Email: "alice@example.com", Password: "password123"})

	_, err := svc.Authenticate(ctx, SignInInput{Email: "alice@example.com", Password: "wrong"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestUserService_Authenticate_UnknownEmail(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()

	_, err := svc.Authenticate(ctx, SignInInput{Email: "nobody@example.com", Password: "password123"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestUserService_Authenticate_InactiveUser(t *testing.T) {
	svc, fix := newUserService(t)
	ctx := context.Background()
	fix.NewUser().Handle("inactive").Email("inactive@example.com").Password("password123").Inactive().Build(ctx)

	_, err := svc.Authenticate(ctx, SignInInput{Email: "inactive@example.com", Password: "password123"})
	if !errors.Is(err, ErrInactiveUser) {
		t.Fatalf("want ErrInactiveUser, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetUser / ListUsers
// ---------------------------------------------------------------------------

func TestUserService_GetUser(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()
	created, _ := svc.Register(ctx, RegisterInput{Handle: "alice", Email: "alice@example.com", Password: "password123"})

	found, err := svc.GetUser(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if found.Handle != "alice" {
		t.Errorf("Handle = %q, want %q", found.Handle, "alice")
	}
}

func TestUserService_GetUser_NotFound(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()

	_, err := svc.GetUser(ctx, uuid.New())
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("want ErrUserNotFound, got %v", err)
	}
}

func TestUserService_ListUsers(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()
	svc.Register(ctx, RegisterInput{Handle: "alice", Email: "alice@example.com", Password: "password123"})
	svc.Register(ctx, RegisterInput{Handle: "bob", Email: "bob@example.com", Password: "password123"})

	users, err := svc.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("ListUsers returned %d users, want 2", len(users))
	}
}

// ---------------------------------------------------------------------------
// UpdateProfile
// ---------------------------------------------------------------------------

func TestUserService_UpdateProfile(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()
	created, _ := svc.Register(ctx, RegisterInput{Handle: "alice", Email: "alice@example.com", Password: "password123"})

	updated, err := svc.UpdateProfile(ctx, UpdateProfileInput{
		UserID: created.ID, Handle: "alice_new", Email: "alice_new@example.com",
	})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if updated.Handle != "alice_new" {
		t.Errorf("Handle = %q, want %q", updated.Handle, "alice_new")
	}
	if updated.Email != "alice_new@example.com" {
		t.Errorf("Email = %q, want %q", updated.Email, "alice_new@example.com")
	}
}

func TestUserService_UpdateProfile_DuplicateHandle(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()
	svc.Register(ctx, RegisterInput{Handle: "alice", Email: "alice@example.com", Password: "password123"})
	bob, _ := svc.Register(ctx, RegisterInput{Handle: "bob", Email: "bob@example.com", Password: "password123"})

	_, err := svc.UpdateProfile(ctx, UpdateProfileInput{
		UserID: bob.ID, Handle: "alice", Email: "bob@example.com",
	})
	if !errors.Is(err, ErrHandleTaken) {
		t.Fatalf("want ErrHandleTaken, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// CreateUser (admin)
// ---------------------------------------------------------------------------

func TestUserService_CreateUser(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()

	user, err := svc.CreateUser(ctx, CreateUserInput{
		Handle: "admin", Email: "admin@example.com", Password: "password123",
		IsActive: true, Roles: []string{"admin", "user"},
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.Handle != "admin" {
		t.Errorf("Handle = %q, want %q", user.Handle, "admin")
	}
	hasAdmin := false
	for _, r := range user.Roles {
		if r == RoleAdmin {
			hasAdmin = true
		}
	}
	if !hasAdmin {
		t.Errorf("Roles = %v, want admin included", user.Roles)
	}
}

// ---------------------------------------------------------------------------
// UpdateUser (admin)
// ---------------------------------------------------------------------------

func TestUserService_UpdateUser(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()
	created, _ := svc.CreateUser(ctx, CreateUserInput{
		Handle: "alice", Email: "alice@example.com", Password: "password123",
		IsActive: true, Roles: []string{"user"},
	})

	updated, err := svc.UpdateUser(ctx, UpdateUserInput{
		ActorUserID: uuid.New(),
		UserID:      created.ID,
		Handle:      "alice_edited",
		Email:       "alice_edited@example.com",
		Password:    "newpassword123",
		IsActive:    true,
		Roles:       []string{"user", "admin"},
	})
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if updated.Handle != "alice_edited" {
		t.Errorf("Handle = %q, want %q", updated.Handle, "alice_edited")
	}

	// verify new password works
	_, err = svc.Authenticate(ctx, SignInInput{Email: "alice_edited@example.com", Password: "newpassword123"})
	if err != nil {
		t.Fatalf("auth with new password: %v", err)
	}
}

func TestUserService_UpdateUser_LastAdminRoleRemoval(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()
	admin, _ := svc.CreateUser(ctx, CreateUserInput{
		Handle: "admin", Email: "admin@example.com", Password: "password123",
		IsActive: true, Roles: []string{"admin", "user"},
	})

	_, err := svc.UpdateUser(ctx, UpdateUserInput{
		ActorUserID: uuid.New(), UserID: admin.ID,
		Handle: "admin", Email: "admin@example.com",
		IsActive: true, Roles: []string{"user"},
	})
	if !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("want ErrLastAdmin, got %v", err)
	}
}

func TestUserService_UpdateUser_LastAdminDeactivation(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()
	admin, _ := svc.CreateUser(ctx, CreateUserInput{
		Handle: "admin", Email: "admin@example.com", Password: "password123",
		IsActive: true, Roles: []string{"admin", "user"},
	})

	_, err := svc.UpdateUser(ctx, UpdateUserInput{
		ActorUserID: uuid.New(), UserID: admin.ID,
		Handle: "admin", Email: "admin@example.com",
		IsActive: false, Roles: []string{"admin", "user"},
	})
	if !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("want ErrLastAdmin, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// DeleteUser
// ---------------------------------------------------------------------------

func TestUserService_DeleteUser(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()
	admin, _ := svc.CreateUser(ctx, CreateUserInput{
		Handle: "admin", Email: "admin@example.com", Password: "password123",
		IsActive: true, Roles: []string{"admin", "user"},
	})
	target, _ := svc.Register(ctx, RegisterInput{Handle: "target", Email: "target@example.com", Password: "password123"})

	if err := svc.DeleteUser(ctx, admin.ID, target.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	_, err := svc.GetUser(ctx, target.ID)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("deleted user should not be found, got %v", err)
	}
}

func TestUserService_DeleteUser_Self(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()
	admin, _ := svc.CreateUser(ctx, CreateUserInput{
		Handle: "admin", Email: "admin@example.com", Password: "password123",
		IsActive: true, Roles: []string{"admin", "user"},
	})

	err := svc.DeleteUser(ctx, admin.ID, admin.ID)
	if !errors.Is(err, ErrCannotDeleteSelf) {
		t.Fatalf("want ErrCannotDeleteSelf, got %v", err)
	}
}

func TestUserService_DeleteUser_LastAdmin(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()
	admin, _ := svc.CreateUser(ctx, CreateUserInput{
		Handle: "admin", Email: "admin@example.com", Password: "password123",
		IsActive: true, Roles: []string{"admin", "user"},
	})
	other, _ := svc.CreateUser(ctx, CreateUserInput{
		Handle: "other", Email: "other@example.com", Password: "password123",
		IsActive: true, Roles: []string{"user"},
	})

	err := svc.DeleteUser(ctx, other.ID, admin.ID)
	if !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("want ErrLastAdmin, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// SetPasswordByEmail
// ---------------------------------------------------------------------------

func TestUserService_SetPasswordByEmail(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()
	svc.Register(ctx, RegisterInput{Handle: "alice", Email: "alice@example.com", Password: "oldpassword1"})

	if err := svc.SetPasswordByEmail(ctx, "alice@example.com", "newpassword1"); err != nil {
		t.Fatalf("SetPasswordByEmail: %v", err)
	}

	_, err := svc.Authenticate(ctx, SignInInput{Email: "alice@example.com", Password: "newpassword1"})
	if err != nil {
		t.Fatalf("auth with new password: %v", err)
	}
}

func TestUserService_SetPasswordByEmail_NotFound(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()

	err := svc.SetPasswordByEmail(ctx, "nobody@example.com", "password123")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("want ErrUserNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// SeedTesters
// ---------------------------------------------------------------------------

func TestUserService_SeedTesters(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()

	hash, _ := auth.HashPassword("tester")
	created, err := svc.SeedTesters(ctx, 3, "test.local", hash)
	if err != nil {
		t.Fatalf("SeedTesters: %v", err)
	}
	if created != 3 {
		t.Errorf("created = %d, want 3", created)
	}

	users, _ := svc.ListUsers(ctx)
	if len(users) != 3 {
		t.Errorf("ListUsers = %d, want 3", len(users))
	}
}

func TestUserService_SeedTesters_Idempotent(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()

	hash, _ := auth.HashPassword("tester")
	svc.SeedTesters(ctx, 2, "test.local", hash)

	created, err := svc.SeedTesters(ctx, 2, "test.local", hash)
	if err != nil {
		t.Fatalf("SeedTesters: %v", err)
	}
	if created != 0 {
		t.Errorf("second call should create 0, got %d", created)
	}
}

// ---------------------------------------------------------------------------
// ListRoles
// ---------------------------------------------------------------------------

func TestUserService_ListRoles(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()

	roles, err := svc.ListRoles(ctx)
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	if len(roles) == 0 {
		t.Fatal("ListRoles returned no roles; migration should seed roles")
	}

	names := map[string]bool{}
	for _, r := range roles {
		names[r.Name] = true
	}
	for _, want := range []string{"user", "admin", "tester"} {
		if !names[want] {
			t.Errorf("missing role %q in %v", want, roles)
		}
	}
}
