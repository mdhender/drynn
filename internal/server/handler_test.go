package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"

	"github.com/mdhender/drynn/internal/auth"
	"github.com/mdhender/drynn/internal/email"
	"github.com/mdhender/drynn/internal/handler"
	drynnmiddleware "github.com/mdhender/drynn/internal/middleware"
	"github.com/mdhender/drynn/internal/service"
	"github.com/mdhender/drynn/internal/testdb"
	"github.com/mdhender/drynn/internal/testfixtures"
)

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

type testServer struct {
	echo       *echo.Echo
	pool       *pgxpool.Pool
	fix        *testfixtures.Fixtures
	jwt        *auth.Manager
	users      *service.UserService
	invitations *service.InvitationService
	recorder   *templateRecorder
}

type renderCall struct {
	Name string
	Data any
}

type templateRecorder struct {
	mu       sync.Mutex
	lastCall *renderCall
}

func (r *templateRecorder) Render(_ *echo.Context, w io.Writer, name string, data any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastCall = &renderCall{Name: name, Data: data}
	return nil
}

func (r *templateRecorder) last() *renderCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastCall
}

func (r *templateRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastCall = nil
}

func newTestServer(t testing.TB) *testServer {
	t.Helper()
	pool := testdb.New(t)
	fix := testfixtures.New(t, pool)

	userSvc := service.NewUserService(pool)
	invSvc := service.NewInvitationService(pool, email.MailgunConfig{})
	pwdSvc := service.NewPasswordResetService(pool, email.MailgunConfig{})
	accessSvc := service.NewAccessRequestService(email.MailgunConfig{}, "")

	keyStore := auth.NewKeyStore(pool)
	ctx := context.Background()
	if err := keyStore.EnsureReady(ctx); err != nil {
		t.Fatalf("seed jwt keys: %v", err)
	}

	jwtMgr := auth.NewManager(keyStore, 15*time.Minute, 24*time.Hour, false)
	rec := &templateRecorder{}

	e := echo.New()
	e.Renderer = rec

	publicH := handler.NewPublicHandler()
	authH := handler.NewAuthHandler(userSvc, invSvc, pwdSvc, accessSvc, jwtMgr, false, "http://localhost:8080")
	appH := handler.NewAppHandler(userSvc)
	adminH := handler.NewAdminHandler(userSvc, invSvc, pwdSvc, "http://localhost:8080")
	healthH := handler.NewHealthHandler(pool)

	rl := drynnmiddleware.NewRateLimiter(drynnmiddleware.DefaultAuthRate, drynnmiddleware.DefaultAuthBurst)
	registerRoutes(e, publicH, authH, appH, adminH, healthH, jwtMgr, userSvc, rl)

	return &testServer{
		echo:        e,
		pool:        pool,
		fix:         fix,
		jwt:         jwtMgr,
		users:       userSvc,
		invitations: invSvc,
		recorder:    rec,
	}
}

// do executes a request against the echo router and returns the response.
func (ts *testServer) do(req *http.Request) *http.Response {
	rec := httptest.NewRecorder()
	ts.echo.ServeHTTP(rec, req)
	return rec.Result()
}

// authCookie issues a JWT access token and returns a cookie for use in requests.
func (ts *testServer) authCookie(t testing.TB, userID uuid.UUID) *http.Cookie {
	t.Helper()
	pair, err := ts.jwt.IssueTokens(context.Background(), userID)
	if err != nil {
		t.Fatalf("issue tokens: %v", err)
	}
	return &http.Cookie{Name: auth.AccessCookieName(), Value: pair.AccessToken}
}

func get(path string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Accept", "text/html")
	return req
}

func postForm(path string, data url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	return req
}

func assertStatus(t testing.TB, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Errorf("status = %d, want %d", resp.StatusCode, want)
	}
}

func assertRedirect(t testing.TB, resp *http.Response, wantStatus int, wantPath string) {
	t.Helper()
	assertStatus(t, resp, wantStatus)
	loc := resp.Header.Get("Location")
	if loc == "" {
		t.Fatal("missing Location header")
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse Location %q: %v", loc, err)
	}
	if u.Path != wantPath {
		t.Errorf("redirect path = %q, want %q (full: %q)", u.Path, wantPath, loc)
	}
}

func assertFlash(t testing.TB, resp *http.Response, wantMsg string) {
	t.Helper()
	loc := resp.Header.Get("Location")
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	flash := u.Query().Get("flash")
	if flash != wantMsg {
		t.Errorf("flash = %q, want %q", flash, wantMsg)
	}
}

func assertTemplate(t testing.TB, rec *templateRecorder, wantName string) {
	t.Helper()
	call := rec.last()
	if call == nil {
		t.Fatal("no template rendered")
	}
	if call.Name != wantName {
		t.Errorf("template = %q, want %q", call.Name, wantName)
	}
}

// ---------------------------------------------------------------------------
// Browser session helper (carries cookies between requests)
// ---------------------------------------------------------------------------

type browserSession struct {
	ts      *testServer
	cookies []*http.Cookie
}

func (ts *testServer) newSession() *browserSession {
	return &browserSession{ts: ts}
}

func (s *browserSession) do(req *http.Request) *http.Response {
	for _, c := range s.cookies {
		req.AddCookie(c)
	}
	resp := s.ts.do(req)
	for _, c := range resp.Cookies() {
		s.setCookie(c)
	}
	return resp
}

func (s *browserSession) setCookie(c *http.Cookie) {
	for i, existing := range s.cookies {
		if existing.Name == c.Name {
			if c.MaxAge < 0 || c.Value == "" {
				s.cookies = append(s.cookies[:i], s.cookies[i+1:]...)
			} else {
				s.cookies[i] = c
			}
			return
		}
	}
	if c.MaxAge >= 0 && c.Value != "" {
		s.cookies = append(s.cookies, c)
	}
}

func (s *browserSession) hasCookie(name string) bool {
	for _, c := range s.cookies {
		if c.Name == name && c.Value != "" {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Integration smoke test
// ---------------------------------------------------------------------------

func TestSmokeTest_InvitationToSignOut(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()

	// --- Setup: seed an admin and create an invitation ---
	admin, _ := ts.users.CreateUser(ctx, service.CreateUserInput{
		Handle: "admin", Email: "admin@example.com", Password: "adminpass1",
		IsActive: true, Roles: []string{"admin", "user"},
	})
	_ = admin
	ts.fix.NewInvitation().Code("smoke_invite").Email("newbie@example.com").Build(ctx)

	// --- Step 1: Register via invitation ---
	sess := ts.newSession()
	resp := sess.do(postForm("/register", url.Values{
		"code":     {"smoke_invite"},
		"handle":   {"newbie"},
		"email":    {"newbie@example.com"},
		"password": {"newbiepass1"},
	}))
	assertRedirect(t, resp, http.StatusSeeOther, "/app")
	assertFlash(t, resp, "Welcome aboard.")
	if !sess.hasCookie(auth.AccessCookieName()) {
		t.Fatal("step 1: no access cookie after registration")
	}

	// --- Step 2: Access profile while authenticated ---
	ts.recorder.reset()
	resp = sess.do(get("/app/profile"))
	assertStatus(t, resp, http.StatusOK)
	assertTemplate(t, ts.recorder, "app/profile")

	// --- Step 3: Sign out ---
	resp = sess.do(postForm("/logout", nil))
	assertRedirect(t, resp, http.StatusSeeOther, "/")
	assertFlash(t, resp, "Signed out.")

	// --- Step 4: Confirm unauthenticated after sign-out ---
	resp = sess.do(get("/app/profile"))
	assertRedirect(t, resp, http.StatusSeeOther, "/signin")

	// --- Step 5: Sign in as admin ---
	adminSess := ts.newSession()
	resp = adminSess.do(postForm("/signin", url.Values{
		"email": {"admin@example.com"}, "password": {"adminpass1"},
	}))
	assertRedirect(t, resp, http.StatusSeeOther, "/app")
	assertFlash(t, resp, "Welcome back.")
	if !adminSess.hasCookie(auth.AccessCookieName()) {
		t.Fatal("step 5: no access cookie after admin sign-in")
	}

	// --- Step 6: Admin creates a new user ---
	resp = adminSess.do(postForm("/app/admin/users", url.Values{
		"handle":   {"created_by_admin"},
		"email":    {"created@example.com"},
		"password": {"password123"},
		"is_active": {"on"},
		"roles[]":  {"user"},
	}))
	assertRedirect(t, resp, http.StatusSeeOther, "/app/admin/users")
	assertFlash(t, resp, "User created.")

	// --- Step 7: Verify the created user appears in the user list ---
	ts.recorder.reset()
	resp = adminSess.do(get("/app/admin/users"))
	assertStatus(t, resp, http.StatusOK)
	assertTemplate(t, ts.recorder, "admin/users")

	// --- Step 8: Admin signs out ---
	resp = adminSess.do(postForm("/logout", nil))
	assertRedirect(t, resp, http.StatusSeeOther, "/")
	assertFlash(t, resp, "Signed out.")

	// --- Step 9: Verify admin is no longer authenticated ---
	resp = adminSess.do(get("/app/admin/users"))
	assertRedirect(t, resp, http.StatusSeeOther, "/signin")

	// --- Step 10: Verify the admin-created user can sign in ---
	createdSess := ts.newSession()
	resp = createdSess.do(postForm("/signin", url.Values{
		"email": {"created@example.com"}, "password": {"password123"},
	}))
	assertRedirect(t, resp, http.StatusSeeOther, "/app")
	assertFlash(t, resp, "Welcome back.")
}

// ---------------------------------------------------------------------------
// Health endpoints
// ---------------------------------------------------------------------------

func TestHandler_Healthz(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.do(get("/healthz"))
	assertStatus(t, resp, http.StatusOK)
}

func TestHandler_Readyz(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.do(get("/readyz"))
	assertStatus(t, resp, http.StatusOK)
}

// ---------------------------------------------------------------------------
// Public routes
// ---------------------------------------------------------------------------

func TestHandler_ShowHome(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.do(get("/"))
	assertStatus(t, resp, http.StatusOK)
	assertTemplate(t, ts.recorder, "public/home")
}

func TestHandler_ShowSignIn(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.do(get("/signin"))
	assertStatus(t, resp, http.StatusOK)
	assertTemplate(t, ts.recorder, "public/signin")
}

func TestHandler_ShowForgotPassword(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.do(get("/forgot-password"))
	assertStatus(t, resp, http.StatusOK)
	assertTemplate(t, ts.recorder, "public/forgot-password")
}

func TestHandler_ShowResetPassword(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.do(get("/reset-password?code=abc123"))
	assertStatus(t, resp, http.StatusOK)
	assertTemplate(t, ts.recorder, "public/reset-password")
}

// ---------------------------------------------------------------------------
// Register
// ---------------------------------------------------------------------------

func TestHandler_ShowRegister_NoCode(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.do(get("/register"))
	assertRedirect(t, resp, http.StatusSeeOther, "/signin")
}

func TestHandler_ShowRegister_ValidCode(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	inv := ts.fix.NewInvitation().Code("testcode123").Build(ctx)
	_ = inv

	resp := ts.do(get("/register?code=testcode123"))
	assertStatus(t, resp, http.StatusOK)
	assertTemplate(t, ts.recorder, "public/register")
}

func TestHandler_ShowRegister_InvalidCode(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.do(get("/register?code=bogus"))
	assertStatus(t, resp, http.StatusOK)
	assertTemplate(t, ts.recorder, "public/register")
}

// ---------------------------------------------------------------------------
// Sign in / Sign out
// ---------------------------------------------------------------------------

func TestHandler_SignIn_Success(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	ts.users.Register(ctx, service.RegisterInput{
		Handle: "alice", Email: "alice@example.com", Password: "password123",
	})

	resp := ts.do(postForm("/signin", url.Values{
		"email": {"alice@example.com"}, "password": {"password123"},
	}))
	assertRedirect(t, resp, http.StatusSeeOther, "/app")
	assertFlash(t, resp, "Welcome back.")

	// check auth cookies were set
	cookies := resp.Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == auth.AccessCookieName() && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Error("access cookie not set after sign in")
	}
}

func TestHandler_SignIn_WrongPassword(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	ts.users.Register(ctx, service.RegisterInput{
		Handle: "alice", Email: "alice@example.com", Password: "password123",
	})

	resp := ts.do(postForm("/signin", url.Values{
		"email": {"alice@example.com"}, "password": {"wrong"},
	}))
	assertStatus(t, resp, http.StatusUnauthorized)
	assertTemplate(t, ts.recorder, "public/signin")
}

func TestHandler_SignOut(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.do(postForm("/logout", nil))
	assertRedirect(t, resp, http.StatusSeeOther, "/")
	assertFlash(t, resp, "Signed out.")
}

// ---------------------------------------------------------------------------
// Register flow (POST)
// ---------------------------------------------------------------------------

func TestHandler_Register_Success(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	ts.fix.NewInvitation().Code("regcode").Email("newuser@example.com").Build(ctx)

	resp := ts.do(postForm("/register", url.Values{
		"code":     {"regcode"},
		"handle":   {"newuser"},
		"email":    {"newuser@example.com"},
		"password": {"password123"},
	}))
	assertRedirect(t, resp, http.StatusSeeOther, "/app")
	assertFlash(t, resp, "Welcome aboard.")
}

func TestHandler_Register_NoCode(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.do(postForm("/register", url.Values{
		"handle": {"x"}, "email": {"x@x.com"}, "password": {"password123"},
	}))
	assertRedirect(t, resp, http.StatusSeeOther, "/signin")
}

func TestHandler_Register_EmailMismatch(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	ts.fix.NewInvitation().Code("mismatch").Email("invited@example.com").Build(ctx)

	resp := ts.do(postForm("/register", url.Values{
		"code":     {"mismatch"},
		"handle":   {"alice"},
		"email":    {"different@example.com"},
		"password": {"password123"},
	}))
	assertStatus(t, resp, http.StatusUnprocessableEntity)
	assertTemplate(t, ts.recorder, "public/register")
}

func TestHandler_Register_InvalidHandle(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	ts.fix.NewInvitation().Code("badhandle").Email("alice@example.com").Build(ctx)

	resp := ts.do(postForm("/register", url.Values{
		"code":     {"badhandle"},
		"handle":   {"ab"},
		"email":    {"alice@example.com"},
		"password": {"password123"},
	}))
	assertStatus(t, resp, http.StatusUnprocessableEntity)
	assertTemplate(t, ts.recorder, "public/register")
}

// ---------------------------------------------------------------------------
// Password reset flow
// ---------------------------------------------------------------------------

func TestHandler_ForgotPassword_Post(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.do(postForm("/forgot-password", url.Values{
		"email": {"anyone@example.com"},
	}))
	assertStatus(t, resp, http.StatusOK)
	assertTemplate(t, ts.recorder, "public/forgot-password")
}

func TestHandler_ResetPassword_InvalidCode(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.do(postForm("/reset-password", url.Values{
		"code": {"bogus"}, "email": {"a@b.com"}, "password": {"newpassword1"},
	}))
	assertStatus(t, resp, http.StatusUnprocessableEntity)
	assertTemplate(t, ts.recorder, "public/reset-password")
}

func TestHandler_ResetPassword_Success(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	user := ts.fix.NewUser().Email("reset@example.com").Build(ctx)
	ts.fix.NewPasswordResetToken().ForUser(user).Code("resetok").Build(ctx)

	resp := ts.do(postForm("/reset-password", url.Values{
		"code": {"resetok"}, "email": {"reset@example.com"}, "password": {"newpassword1"},
	}))
	assertRedirect(t, resp, http.StatusSeeOther, "/signin")
	assertFlash(t, resp, "Password reset. Please sign in with your new password.")
}

// ---------------------------------------------------------------------------
// Auth middleware: unauthenticated access to /app
// ---------------------------------------------------------------------------

func TestHandler_App_Unauthenticated(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.do(get("/app/profile"))
	assertRedirect(t, resp, http.StatusSeeOther, "/signin")
}

// ---------------------------------------------------------------------------
// App routes (authenticated)
// ---------------------------------------------------------------------------

func TestHandler_ShowProfile(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	user, _ := ts.users.Register(ctx, service.RegisterInput{
		Handle: "alice", Email: "alice@example.com", Password: "password123",
	})

	req := get("/app/profile")
	req.AddCookie(ts.authCookie(t, user.ID))
	resp := ts.do(req)
	assertStatus(t, resp, http.StatusOK)
	assertTemplate(t, ts.recorder, "app/profile")
}

func TestHandler_UpdateProfile_Success(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	user, _ := ts.users.Register(ctx, service.RegisterInput{
		Handle: "alice", Email: "alice@example.com", Password: "password123",
	})

	req := postForm("/app/profile", url.Values{
		"handle": {"alice_new"}, "email": {"alice_new@example.com"},
	})
	req.AddCookie(ts.authCookie(t, user.ID))
	resp := ts.do(req)
	assertRedirect(t, resp, http.StatusSeeOther, "/app/profile")
	assertFlash(t, resp, "Profile updated.")
}

func TestHandler_UpdateProfile_DuplicateHandle(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	ts.users.Register(ctx, service.RegisterInput{Handle: "bob", Email: "bob@example.com", Password: "password123"})
	alice, _ := ts.users.Register(ctx, service.RegisterInput{Handle: "alice", Email: "alice@example.com", Password: "password123"})

	req := postForm("/app/profile", url.Values{
		"handle": {"bob"}, "email": {"alice@example.com"},
	})
	req.AddCookie(ts.authCookie(t, alice.ID))
	resp := ts.do(req)
	assertStatus(t, resp, http.StatusUnprocessableEntity)
	assertTemplate(t, ts.recorder, "app/profile")
}

// ---------------------------------------------------------------------------
// Admin routes — role enforcement
// ---------------------------------------------------------------------------

func TestHandler_Admin_NonAdmin_Forbidden(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	user, _ := ts.users.Register(ctx, service.RegisterInput{
		Handle: "regular", Email: "regular@example.com", Password: "password123",
	})

	req := get("/app/admin/users")
	req.AddCookie(ts.authCookie(t, user.ID))
	resp := ts.do(req)
	assertStatus(t, resp, http.StatusForbidden)
}

func TestHandler_Admin_ListUsers(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	admin, _ := ts.users.CreateUser(ctx, service.CreateUserInput{
		Handle: "admin", Email: "admin@example.com", Password: "password123",
		IsActive: true, Roles: []string{"admin", "user"},
	})

	req := get("/app/admin/users")
	req.AddCookie(ts.authCookie(t, admin.ID))
	resp := ts.do(req)
	assertStatus(t, resp, http.StatusOK)
	assertTemplate(t, ts.recorder, "admin/users")
}

func TestHandler_Admin_ShowCreateUser(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	admin, _ := ts.users.CreateUser(ctx, service.CreateUserInput{
		Handle: "admin", Email: "admin@example.com", Password: "password123",
		IsActive: true, Roles: []string{"admin", "user"},
	})

	req := get("/app/admin/users/new")
	req.AddCookie(ts.authCookie(t, admin.ID))
	resp := ts.do(req)
	assertStatus(t, resp, http.StatusOK)
	assertTemplate(t, ts.recorder, "admin/user-form")
}

func TestHandler_Admin_CreateUser_Success(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	admin, _ := ts.users.CreateUser(ctx, service.CreateUserInput{
		Handle: "admin", Email: "admin@example.com", Password: "password123",
		IsActive: true, Roles: []string{"admin", "user"},
	})

	req := postForm("/app/admin/users", url.Values{
		"handle": {"newbie"}, "email": {"newbie@example.com"},
		"password": {"password123"}, "is_active": {"on"}, "roles[]": {"user"},
	})
	req.AddCookie(ts.authCookie(t, admin.ID))
	resp := ts.do(req)
	assertRedirect(t, resp, http.StatusSeeOther, "/app/admin/users")
	assertFlash(t, resp, "User created.")
}

func TestHandler_Admin_CreateUser_ValidationError(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	admin, _ := ts.users.CreateUser(ctx, service.CreateUserInput{
		Handle: "admin", Email: "admin@example.com", Password: "password123",
		IsActive: true, Roles: []string{"admin", "user"},
	})

	req := postForm("/app/admin/users", url.Values{
		"handle": {"ab"}, "email": {"newbie@example.com"},
		"password": {"password123"}, "is_active": {"on"},
	})
	req.AddCookie(ts.authCookie(t, admin.ID))
	resp := ts.do(req)
	assertStatus(t, resp, http.StatusUnprocessableEntity)
	assertTemplate(t, ts.recorder, "admin/user-form")
}

func TestHandler_Admin_DeleteUser_Success(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	admin, _ := ts.users.CreateUser(ctx, service.CreateUserInput{
		Handle: "admin", Email: "admin@example.com", Password: "password123",
		IsActive: true, Roles: []string{"admin", "user"},
	})
	target, _ := ts.users.Register(ctx, service.RegisterInput{Handle: "target", Email: "target@example.com", Password: "password123"})

	req := postForm("/app/admin/users/"+target.ID.String()+"/delete", nil)
	req.AddCookie(ts.authCookie(t, admin.ID))
	resp := ts.do(req)
	assertRedirect(t, resp, http.StatusSeeOther, "/app/admin/users")
	assertFlash(t, resp, "User deleted.")
}

func TestHandler_Admin_ListInvitations(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	admin, _ := ts.users.CreateUser(ctx, service.CreateUserInput{
		Handle: "admin", Email: "admin@example.com", Password: "password123",
		IsActive: true, Roles: []string{"admin", "user"},
	})

	req := get("/app/admin/invitations")
	req.AddCookie(ts.authCookie(t, admin.ID))
	resp := ts.do(req)
	assertStatus(t, resp, http.StatusOK)
	assertTemplate(t, ts.recorder, "admin/invitations")
}

func TestHandler_Admin_ShowInviteForm(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	admin, _ := ts.users.CreateUser(ctx, service.CreateUserInput{
		Handle: "admin", Email: "admin@example.com", Password: "password123",
		IsActive: true, Roles: []string{"admin", "user"},
	})

	req := get("/app/admin/invitations/new")
	req.AddCookie(ts.authCookie(t, admin.ID))
	resp := ts.do(req)
	assertStatus(t, resp, http.StatusOK)
	assertTemplate(t, ts.recorder, "admin/invite-form")
}

// ---------------------------------------------------------------------------
// App redirect: /app → /app/profile
// ---------------------------------------------------------------------------

func TestHandler_App_Redirect(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	user, _ := ts.users.Register(ctx, service.RegisterInput{
		Handle: "alice", Email: "alice@example.com", Password: "password123",
	})

	req := get("/app")
	req.AddCookie(ts.authCookie(t, user.ID))
	resp := ts.do(req)
	assertRedirect(t, resp, http.StatusSeeOther, "/app/profile")
}

// ---------------------------------------------------------------------------
// Request access (disabled)
// ---------------------------------------------------------------------------

func TestHandler_RequestAccess_Disabled(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.do(get("/request-access"))
	assertStatus(t, resp, http.StatusNotFound)
}
