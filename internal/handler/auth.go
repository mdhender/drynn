package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/mdhender/drynn/internal/auth"
	"github.com/mdhender/drynn/internal/service"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type AuthHandler struct {
	users                *service.UserService
	invitations          *service.InvitationService
	passwordResets       *service.PasswordResetService
	accessRequests       *service.AccessRequestService
	jwtManager           *auth.Manager
	requestAccessEnabled bool
}

func NewAuthHandler(
	users *service.UserService,
	invitations *service.InvitationService,
	passwordResets *service.PasswordResetService,
	accessRequests *service.AccessRequestService,
	jwtManager *auth.Manager,
	requestAccessEnabled bool,
) *AuthHandler {
	return &AuthHandler{
		users:                users,
		invitations:          invitations,
		passwordResets:       passwordResets,
		accessRequests:       accessRequests,
		jwtManager:           jwtManager,
		requestAccessEnabled: requestAccessEnabled,
	}
}

func (h *AuthHandler) ShowRegister(c *echo.Context) error {
	code := c.QueryParam("code")
	if code == "" {
		return c.Redirect(http.StatusSeeOther, "/signin")
	}

	inv, err := h.invitations.ValidateCode(c.Request().Context(), code)
	if err != nil {
		return c.Render(http.StatusOK, "public/register", InviteRegisterViewData{
			BaseViewData: baseView(c, "Register"),
			Error:        serviceMessage(err),
		})
	}

	return c.Render(http.StatusOK, "public/register", InviteRegisterViewData{
		BaseViewData: baseView(c, "Register"),
		Code:         code,
		Email:        inv.Email,
	})
}

func (h *AuthHandler) Register(c *echo.Context) error {
	code := c.FormValue("code")
	if code == "" {
		return c.Redirect(http.StatusSeeOther, "/signin")
	}

	form := RegisterForm{
		Handle: c.FormValue("handle"),
		Email:  c.FormValue("email"),
	}

	inv, err := h.invitations.ValidateCode(c.Request().Context(), code)
	if err != nil {
		return c.Render(http.StatusUnprocessableEntity, "public/register", InviteRegisterViewData{
			BaseViewData: baseView(c, "Register"),
			Form:         form,
			Code:         code,
			Error:        serviceMessage(err),
		})
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(form.Email))
	if normalizedEmail != inv.Email {
		return c.Render(http.StatusUnprocessableEntity, "public/register", InviteRegisterViewData{
			BaseViewData: baseView(c, "Register"),
			Form:         form,
			Code:         code,
			Email:        inv.Email,
			Error:        serviceMessage(service.ErrInvitationEmail),
		})
	}

	user, err := h.users.Register(c.Request().Context(), service.RegisterInput{
		Handle:   form.Handle,
		Email:    form.Email,
		Password: c.FormValue("password"),
	})
	if err != nil {
		return c.Render(http.StatusUnprocessableEntity, "public/register", InviteRegisterViewData{
			BaseViewData: baseView(c, "Register"),
			Form:         form,
			Code:         code,
			Email:        inv.Email,
			Error:        serviceMessage(err),
		})
	}

	if err := h.invitations.RedeemInvitation(c.Request().Context(), code, user.ID); err != nil {
		slog.Default().Error("register: redeem invitation failed after successful user creation",
			"user_id", user.ID,
			"code", code,
			"error", err)
	}

	if err := h.issueSession(c, user.ID); err != nil {
		return err
	}

	return c.Redirect(http.StatusSeeOther, withFlash("/app", "Welcome aboard."))
}

func (h *AuthHandler) ShowSignIn(c *echo.Context) error {
	return c.Render(http.StatusOK, "public/signin", SignInViewData{
		BaseViewData:         baseView(c, "Sign In"),
		RequestAccessEnabled: h.requestAccessEnabled,
	})
}

func (h *AuthHandler) SignIn(c *echo.Context) error {
	form := SignInForm{Email: c.FormValue("email")}
	user, err := h.users.Authenticate(c.Request().Context(), service.SignInInput{
		Email:    form.Email,
		Password: c.FormValue("password"),
	})
	if err != nil {
		return c.Render(http.StatusUnauthorized, "public/signin", SignInViewData{
			BaseViewData:         baseView(c, "Sign In"),
			Form:                 form,
			Error:                serviceMessage(err),
			RequestAccessEnabled: h.requestAccessEnabled,
		})
	}

	if err := h.issueSession(c, user.ID); err != nil {
		return err
	}

	return c.Redirect(http.StatusSeeOther, withFlash("/app", "Welcome back."))
}

func (h *AuthHandler) ShowRequestAccess(c *echo.Context) error {
	if !h.requestAccessEnabled {
		return echo.ErrNotFound
	}
	return c.Render(http.StatusOK, "public/request-access", RequestAccessViewData{
		BaseViewData: baseView(c, "Request access"),
	})
}

func (h *AuthHandler) RequestAccess(c *echo.Context) error {
	if !h.requestAccessEnabled {
		return echo.ErrNotFound
	}

	form := RequestAccessForm{
		Email:  c.FormValue("email"),
		Reason: c.FormValue("reason"),
	}

	if c.FormValue("website") != "" {
		slog.Default().Info("request-access: honeypot triggered", "ip", c.RealIP())
		return c.Render(http.StatusOK, "public/request-access", RequestAccessViewData{
			BaseViewData: baseView(c, "Request access"),
			Submitted:    true,
		})
	}

	if strings.TrimSpace(form.Email) == "" {
		return c.Render(http.StatusUnprocessableEntity, "public/request-access", RequestAccessViewData{
			BaseViewData: baseView(c, "Request access"),
			Form:         form,
			Error:        "Enter a valid email address.",
		})
	}

	err := h.accessRequests.Send(c.Request().Context(), service.AccessRequestInput{
		Email:  form.Email,
		Reason: form.Reason,
		IP:     c.RealIP(),
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidEmail) {
			return c.Render(http.StatusUnprocessableEntity, "public/request-access", RequestAccessViewData{
				BaseViewData: baseView(c, "Request access"),
				Form:         form,
				Error:        "Enter a valid email address.",
			})
		}
		slog.Default().Error("request-access: send failed", "error", err)
	}

	return c.Render(http.StatusOK, "public/request-access", RequestAccessViewData{
		BaseViewData: baseView(c, "Request access"),
		Submitted:    true,
	})
}

func (h *AuthHandler) ShowForgotPassword(c *echo.Context) error {
	return c.Render(http.StatusOK, "public/forgot-password", ForgotPasswordViewData{
		BaseViewData: baseView(c, "Forgot password"),
	})
}

func (h *AuthHandler) ForgotPassword(c *echo.Context) error {
	email := c.FormValue("email")

	if err := h.passwordResets.SendResetByEmail(c.Request().Context(), email, requestBaseURL(c)); err != nil {
		slog.Default().Error("forgot-password: send reset failed", "error", err)
	}

	return c.Render(http.StatusOK, "public/forgot-password", ForgotPasswordViewData{
		BaseViewData: baseView(c, "Forgot password"),
		Email:        email,
		Submitted:    true,
	})
}

func (h *AuthHandler) ShowResetPassword(c *echo.Context) error {
	code := c.QueryParam("code")
	return c.Render(http.StatusOK, "public/reset-password", ResetPasswordViewData{
		BaseViewData: baseView(c, "Reset password"),
		Code:         code,
	})
}

func (h *AuthHandler) ResetPassword(c *echo.Context) error {
	code := c.FormValue("code")
	email := c.FormValue("email")
	password := c.FormValue("password")

	err := h.passwordResets.ResetPassword(c.Request().Context(), code, email, password)
	if err != nil {
		return c.Render(http.StatusUnprocessableEntity, "public/reset-password", ResetPasswordViewData{
			BaseViewData: baseView(c, "Reset password"),
			Code:         code,
			Email:        email,
			Error:        serviceMessage(err),
		})
	}

	return c.Redirect(http.StatusSeeOther, withFlash("/signin", "Password reset. Please sign in with your new password."))
}

func (h *AuthHandler) SignOut(c *echo.Context) error {
	h.jwtManager.ClearAuthCookies(c)
	return c.Redirect(http.StatusSeeOther, withFlash("/", "Signed out."))
}

func (h *AuthHandler) Refresh(c *echo.Context) error {
	cookie, err := auth.RefreshCookie(c)
	if err != nil || cookie.Value == "" {
		h.jwtManager.ClearAuthCookies(c)
		return c.Redirect(http.StatusSeeOther, "/signin")
	}

	claims, err := h.jwtManager.ParseRefreshToken(c.Request().Context(), cookie.Value)
	if err != nil {
		h.jwtManager.ClearAuthCookies(c)
		return c.Redirect(http.StatusSeeOther, "/signin")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		h.jwtManager.ClearAuthCookies(c)
		return c.Redirect(http.StatusSeeOther, "/signin")
	}

	user, err := h.users.GetUser(c.Request().Context(), userID)
	if err != nil || !user.IsActive {
		h.jwtManager.ClearAuthCookies(c)
		return c.Redirect(http.StatusSeeOther, "/signin")
	}

	if err := h.issueSession(c, user.ID); err != nil {
		return err
	}

	return c.Redirect(http.StatusSeeOther, "/app")
}

func (h *AuthHandler) issueSession(c *echo.Context, userID uuid.UUID) error {
	pair, err := h.jwtManager.IssueTokens(c.Request().Context(), userID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "could not create session")
	}

	h.jwtManager.SetAuthCookies(c, pair)
	return nil
}
