package handler

import (
	"errors"
	"net/url"

	"github.com/mdhender/drynn/internal/auth"
	"github.com/mdhender/drynn/internal/service"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type BaseViewData struct {
	Title       string
	CurrentPath string
	Flash       string
	CurrentUser *auth.Viewer
}

type HomeViewData struct {
	BaseViewData
}

type RegisterForm struct {
	Handle   string
	Email    string
	Password string
}

type RegisterViewData struct {
	BaseViewData
	Form  RegisterForm
	Error string
}

type SignInForm struct {
	Email    string
	Password string
}

type SignInViewData struct {
	BaseViewData
	Form                 SignInForm
	Error                string
	RequestAccessEnabled bool
}

type ProfileForm struct {
	Handle string
	Email  string
}

type ProfileViewData struct {
	BaseViewData
	Form  ProfileForm
	Error string
}

type AdminUsersViewData struct {
	BaseViewData
	Users []service.User
}

type AdminUserForm struct {
	ID       uuid.UUID
	Handle   string
	Email    string
	Password string
	IsActive bool
	Roles    map[string]bool
}

type AdminUserFormViewData struct {
	BaseViewData
	Heading     string
	SubmitLabel string
	Action      string
	CancelURL   string
	Form        AdminUserForm
	RoleOptions []service.RoleOption
	Error       string
}

type AdminInvitationRow struct {
	service.Invitation
	RegisterURL string
}

type InvitationFilterOption struct {
	Value string
	Label string
}

type AdminInvitationsViewData struct {
	BaseViewData
	Invitations   []AdminInvitationRow
	Filter        string
	FilterOptions []InvitationFilterOption
}

type AdminInviteFormViewData struct {
	BaseViewData
	Email string
	Error string
}

type InviteRegisterViewData struct {
	BaseViewData
	Form  RegisterForm
	Code  string
	Email string
	Error string
}

type ResetPasswordViewData struct {
	BaseViewData
	Code  string
	Email string
	Error string
}

type ForgotPasswordViewData struct {
	BaseViewData
	Email     string
	Submitted bool
}

type RequestAccessForm struct {
	Email  string
	Reason string
}

type RequestAccessViewData struct {
	BaseViewData
	Form      RequestAccessForm
	Error     string
	Submitted bool
}

func baseView(c *echo.Context, title string) BaseViewData {
	viewer, _ := auth.CurrentViewer(c)
	return BaseViewData{
		Title:       title,
		CurrentPath: c.Request().URL.Path,
		Flash:       c.QueryParam("flash"),
		CurrentUser: viewer,
	}
}

func serviceMessage(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, service.ErrInvalidCredentials):
		return "The email address or password was incorrect."
	case errors.Is(err, service.ErrEmailTaken):
		return "That email address is already in use."
	case errors.Is(err, service.ErrHandleTaken):
		return "That handle is already in use."
	case errors.Is(err, service.ErrInvalidHandle):
		return service.ErrInvalidHandle.Error()
	case errors.Is(err, service.ErrInvalidEmail):
		return "Enter a valid email address."
	case errors.Is(err, service.ErrInvalidPassword):
		return service.ErrInvalidPassword.Error()
	case errors.Is(err, service.ErrInactiveUser):
		return "This account has been deactivated."
	case errors.Is(err, service.ErrLastAdmin):
		return service.ErrLastAdmin.Error()
	case errors.Is(err, service.ErrCannotDeleteSelf):
		return service.ErrCannotDeleteSelf.Error()
	case errors.Is(err, service.ErrUserNotFound):
		return "That user could not be found."
	case errors.Is(err, service.ErrInvitationNotFound):
		return "That invitation code is invalid."
	case errors.Is(err, service.ErrInvitationUsed):
		return "That invitation has already been used."
	case errors.Is(err, service.ErrInvitationExpired):
		return "That invitation has expired."
	case errors.Is(err, service.ErrInvitationEmail):
		return "The email address does not match the invitation."
	case errors.Is(err, service.ErrMailgunNotConfigured):
		return "Mailgun settings must be configured before sending invitations."
	case errors.Is(err, service.ErrPasswordResetInvalid):
		return "That password reset link is invalid, expired, or the email does not match."
	default:
		return "Something went wrong. Please try again."
	}
}

func withFlash(path, message string) string {
	if message == "" {
		return path
	}

	return path + "?flash=" + url.QueryEscape(message)
}

func adminFormFromUser(user *service.User) AdminUserForm {
	roles := map[string]bool{}
	for _, role := range user.Roles {
		roles[role] = true
	}

	return AdminUserForm{
		ID:       user.ID,
		Handle:   user.Handle,
		Email:    user.Email,
		IsActive: user.IsActive,
		Roles:    roles,
	}
}
