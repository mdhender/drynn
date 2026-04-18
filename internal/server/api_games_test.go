package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mdhender/drynn/internal/service"
)

// seedAdmin registers an admin user and returns their Authorization header.
func seedAdmin(t *testing.T, ts *testServer) string {
	t.Helper()
	ctx := context.Background()
	admin, err := ts.users.CreateUser(ctx, service.CreateUserInput{
		Handle:   "admin",
		Email:    "admin@example.com",
		Password: "password123",
		IsActive: true,
		Roles:    []string{service.RoleAdmin, service.RoleUser},
	})
	if err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	return ts.authHeader(t, admin.ID)
}

func postJSON(path, auth string, body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return req
}

func getJSON(path, auth string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Accept", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return req
}

func decodeJSON(t testing.TB, resp *http.Response, dst any) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func TestAPI_Games_Create_Admin(t *testing.T) {
	ts := newTestServer(t)
	auth := seedAdmin(t, ts)

	resp := ts.do(postJSON("/api/v1/games", auth, []byte(`{"name":"Alpha"}`)))
	assertStatus(t, resp, http.StatusCreated)

	var body struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, resp, &body)
	if body.ID == 0 {
		t.Errorf("expected non-zero id, got %d", body.ID)
	}
}

func TestAPI_Games_Create_InvalidBody(t *testing.T) {
	ts := newTestServer(t)
	auth := seedAdmin(t, ts)

	resp := ts.do(postJSON("/api/v1/games", auth, []byte(`{bad json`)))
	assertStatus(t, resp, http.StatusBadRequest)

	var body struct {
		Error string `json:"error"`
	}
	decodeJSON(t, resp, &body)
	if body.Error != "invalid request body" {
		t.Errorf("error = %q, want %q", body.Error, "invalid request body")
	}
}

func TestAPI_Games_Create_BlankName(t *testing.T) {
	ts := newTestServer(t)
	auth := seedAdmin(t, ts)

	resp := ts.do(postJSON("/api/v1/games", auth, []byte(`{"name":"   "}`)))
	assertStatus(t, resp, http.StatusBadRequest)

	var body struct {
		Error string `json:"error"`
	}
	decodeJSON(t, resp, &body)
	if body.Error != "name is required" {
		t.Errorf("error = %q, want %q", body.Error, "name is required")
	}
}

func TestAPI_Games_Create_IgnoresUnknownFields(t *testing.T) {
	ts := newTestServer(t)
	auth := seedAdmin(t, ts)

	resp := ts.do(postJSON("/api/v1/games", auth, []byte(`{"name":"Alpha","seed":12345}`)))
	assertStatus(t, resp, http.StatusCreated)

	var created struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, resp, &created)

	game, err := ts.games.GetGame(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get game: %v", err)
	}
	if game.Name != "Alpha" {
		t.Errorf("stored name = %q, want %q", game.Name, "Alpha")
	}
}

func TestAPI_Games_List_Admin(t *testing.T) {
	ts := newTestServer(t)
	auth := seedAdmin(t, ts)
	ctx := context.Background()
	ts.fix.NewGame().Name("Alpha").Build(ctx)
	ts.fix.NewGame().Name("Beta").Build(ctx)

	resp := ts.do(getJSON("/api/v1/games", auth))
	assertStatus(t, resp, http.StatusOK)

	var body []map[string]any
	decodeJSON(t, resp, &body)
	if len(body) != 2 {
		t.Errorf("got %d games, want 2", len(body))
	}
}

func TestAPI_Games_Show_Admin(t *testing.T) {
	ts := newTestServer(t)
	auth := seedAdmin(t, ts)
	ctx := context.Background()
	game := ts.fix.NewGame().Name("Alpha").Build(ctx)

	resp := ts.do(getJSON(fmt.Sprintf("/api/v1/games/%d", game.ID), auth))
	assertStatus(t, resp, http.StatusOK)

	var body struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Status      string `json:"status"`
		CurrentTurn int32  `json:"current_turn"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}
	decodeJSON(t, resp, &body)

	if body.ID != game.ID {
		t.Errorf("id = %d, want %d", body.ID, game.ID)
	}
	if body.Name != "Alpha" {
		t.Errorf("name = %q, want %q", body.Name, "Alpha")
	}
	if body.Status != service.GameStatusSetup {
		t.Errorf("status = %q, want %q", body.Status, service.GameStatusSetup)
	}
	if body.CurrentTurn != 0 {
		t.Errorf("current_turn = %d, want 0", body.CurrentTurn)
	}
	if body.CreatedAt == "" || body.UpdatedAt == "" {
		t.Errorf("expected non-empty timestamps, got created_at=%q updated_at=%q", body.CreatedAt, body.UpdatedAt)
	}
}

func TestAPI_Games_Show_NotFound(t *testing.T) {
	ts := newTestServer(t)
	auth := seedAdmin(t, ts)

	resp := ts.do(getJSON("/api/v1/games/999999", auth))
	assertStatus(t, resp, http.StatusNotFound)

	var body struct {
		Error string `json:"error"`
	}
	decodeJSON(t, resp, &body)
	if body.Error != "game not found" {
		t.Errorf("error = %q, want %q", body.Error, "game not found")
	}
}

func TestAPI_Games_Show_InvalidID(t *testing.T) {
	ts := newTestServer(t)
	auth := seedAdmin(t, ts)

	resp := ts.do(getJSON("/api/v1/games/not-a-number", auth))
	assertStatus(t, resp, http.StatusBadRequest)

	var body struct {
		Error string `json:"error"`
	}
	decodeJSON(t, resp, &body)
	if body.Error != "invalid game id" {
		t.Errorf("error = %q, want %q", body.Error, "invalid game id")
	}
}

func deleteJSON(path, auth string) *http.Request {
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	req.Header.Set("Accept", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return req
}

func putJSON(path, auth string, body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return req
}

func TestAPI_Games_Delete_Admin(t *testing.T) {
	ts := newTestServer(t)
	auth := seedAdmin(t, ts)
	ctx := context.Background()
	game := ts.fix.NewGame().Name("Alpha").Build(ctx)

	resp := ts.do(deleteJSON(fmt.Sprintf("/api/v1/games/%d", game.ID), auth))
	assertStatus(t, resp, http.StatusNoContent)

	resp = ts.do(getJSON(fmt.Sprintf("/api/v1/games/%d", game.ID), auth))
	assertStatus(t, resp, http.StatusNotFound)
}

func TestAPI_Games_Delete_NotFound(t *testing.T) {
	ts := newTestServer(t)
	auth := seedAdmin(t, ts)

	resp := ts.do(deleteJSON("/api/v1/games/999999", auth))
	assertStatus(t, resp, http.StatusNotFound)

	var body struct {
		Error string `json:"error"`
	}
	decodeJSON(t, resp, &body)
	if body.Error != "game not found" {
		t.Errorf("error = %q, want %q", body.Error, "game not found")
	}
}

func TestAPI_Games_Delete_InvalidID(t *testing.T) {
	ts := newTestServer(t)
	auth := seedAdmin(t, ts)

	resp := ts.do(deleteJSON("/api/v1/games/not-a-number", auth))
	assertStatus(t, resp, http.StatusBadRequest)

	var body struct {
		Error string `json:"error"`
	}
	decodeJSON(t, resp, &body)
	if body.Error != "invalid game id" {
		t.Errorf("error = %q, want %q", body.Error, "invalid game id")
	}
}

func TestAPI_Games_Update_NotImplemented(t *testing.T) {
	ts := newTestServer(t)
	auth := seedAdmin(t, ts)
	ctx := context.Background()
	game := ts.fix.NewGame().Name("Alpha").Build(ctx)

	resp := ts.do(putJSON(fmt.Sprintf("/api/v1/games/%d", game.ID), auth, []byte(`{}`)))
	assertStatus(t, resp, http.StatusNotImplemented)

	var body struct {
		Error string `json:"error"`
	}
	decodeJSON(t, resp, &body)
	if body.Error != "not yet implemented" {
		t.Errorf("error = %q, want %q", body.Error, "not yet implemented")
	}
}

func TestAPI_Games_AuthRequired(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.do(getJSON("/api/v1/games", ""))
	assertStatus(t, resp, http.StatusUnauthorized)

	var body struct {
		Error string `json:"error"`
	}
	decodeJSON(t, resp, &body)
	if body.Error != "authentication required" {
		t.Errorf("error = %q, want %q", body.Error, "authentication required")
	}
}

func TestAPI_Games_AdminRequired(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	user, err := ts.users.Register(ctx, service.RegisterInput{
		Handle:   "regular",
		Email:    "regular@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("register user: %v", err)
	}

	resp := ts.do(getJSON("/api/v1/games", ts.authHeader(t, user.ID)))
	assertStatus(t, resp, http.StatusForbidden)

	var body struct {
		Error string `json:"error"`
	}
	decodeJSON(t, resp, &body)
	if body.Error != "forbidden" {
		t.Errorf("error = %q, want %q", body.Error, "forbidden")
	}
}

