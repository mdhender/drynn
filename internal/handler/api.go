package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	drynn "github.com/mdhender/drynn"
	"github.com/mdhender/drynn/internal/auth"
	"github.com/mdhender/drynn/internal/service"

	"github.com/labstack/echo/v5"
)

type APIHandler struct {
	users      *service.UserService
	games      *service.GameService
	jwtManager *auth.Manager
	logger     *slog.Logger
}

func NewAPIHandler(
	users *service.UserService,
	games *service.GameService,
	jwtManager *auth.Manager,
	logger *slog.Logger,
) *APIHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &APIHandler{
		users:      users,
		games:      games,
		jwtManager: jwtManager,
		logger:     logger,
	}
}

type apiLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type apiLoginResponse struct {
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token"`
	AccessExpiry  string `json:"access_expiry"`
	RefreshExpiry string `json:"refresh_expiry"`
}

type apiHealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

type apiErrorResponse struct {
	Error string `json:"error"`
}

func (h *APIHandler) Login(c *echo.Context) error {
	var req apiLoginRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid request body"})
	}

	if strings.TrimSpace(req.Email) == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "email and password are required"})
	}

	user, err := h.users.Authenticate(c.Request().Context(), service.SignInInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) || errors.Is(err, service.ErrInactiveUser) {
			return c.JSON(http.StatusUnauthorized, apiErrorResponse{Error: "invalid credentials"})
		}
		h.logger.Error("api login: authenticate", "error", err)
		return c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "internal error"})
	}

	pair, err := h.jwtManager.IssueTokens(c.Request().Context(), user.ID)
	if err != nil {
		h.logger.Error("api login: issue tokens", "error", err)
		return c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "internal error"})
	}

	return c.JSON(http.StatusOK, apiLoginResponse{
		AccessToken:   pair.AccessToken,
		RefreshToken:  pair.RefreshToken,
		AccessExpiry:  pair.AccessExpiry.UTC().Format(time.RFC3339),
		RefreshExpiry: pair.RefreshExpiry.UTC().Format(time.RFC3339),
	})
}

func (h *APIHandler) Health(c *echo.Context) error {
	return c.JSON(http.StatusOK, apiHealthResponse{
		Status:  "ok",
		Version: drynn.Version().Core(),
	})
}
