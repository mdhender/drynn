package handler

import (
	"net/http"
	"strings"

	"github.com/mdhender/drynn/internal/auth"
	"github.com/mdhender/drynn/internal/service"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type AdminHandler struct {
	users          *service.UserService
	invitations    *service.InvitationService
	passwordResets *service.PasswordResetService
}

func NewAdminHandler(users *service.UserService, invitations *service.InvitationService, passwordResets *service.PasswordResetService) *AdminHandler {
	return &AdminHandler{users: users, invitations: invitations, passwordResets: passwordResets}
}

func (h *AdminHandler) ListUsers(c *echo.Context) error {
	users, err := h.users.ListUsers(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, "could not load users")
	}

	return c.Render(http.StatusOK, "admin/users", AdminUsersViewData{
		BaseViewData: baseView(c, "User Management"),
		Users:        users,
	})
}

func (h *AdminHandler) ShowCreateUser(c *echo.Context) error {
	roles, err := h.users.ListRoles(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, "could not load roles")
	}

	return c.Render(http.StatusOK, "admin/user-form", AdminUserFormViewData{
		BaseViewData: baseView(c, "Create User"),
		Heading:      "Create User",
		SubmitLabel:  "Create user",
		Action:       "/app/admin/users",
		CancelURL:    "/app/admin/users",
		Form: AdminUserForm{
			IsActive: true,
			Roles:    map[string]bool{service.RoleUser: true},
		},
		RoleOptions: roles,
	})
}

func (h *AdminHandler) CreateUser(c *echo.Context) error {
	roles, err := h.users.ListRoles(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, "could not load roles")
	}
	form := adminFormFromRequest(c)
	_, err = h.users.CreateUser(c.Request().Context(), service.CreateUserInput{
		Handle:   form.Handle,
		Email:    form.Email,
		Password: form.Password,
		IsActive: form.IsActive,
		Roles:    selectedRoles(form.Roles),
	})
	if err != nil {
		return c.Render(http.StatusUnprocessableEntity, "admin/user-form", AdminUserFormViewData{
			BaseViewData: baseView(c, "Create User"),
			Heading:      "Create User",
			SubmitLabel:  "Create user",
			Action:       "/app/admin/users",
			CancelURL:    "/app/admin/users",
			Form:         form,
			RoleOptions:  roles,
			Error:        serviceMessage(err),
		})
	}

	return c.Redirect(http.StatusSeeOther, withFlash("/app/admin/users", "User created."))
}

func (h *AdminHandler) ShowEditUser(c *echo.Context) error {
	roles, err := h.users.ListRoles(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, "could not load roles")
	}

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid user id")
	}

	user, err := h.users.GetUser(c.Request().Context(), userID)
	if err != nil {
		return c.String(http.StatusNotFound, "user not found")
	}

	return c.Render(http.StatusOK, "admin/user-form", AdminUserFormViewData{
		BaseViewData: baseView(c, "Edit User"),
		Heading:      "Edit User",
		SubmitLabel:  "Save changes",
		Action:       "/app/admin/users/" + user.ID.String(),
		CancelURL:    "/app/admin/users",
		Form:         adminFormFromUser(user),
		RoleOptions:  roles,
	})
}

func (h *AdminHandler) UpdateUser(c *echo.Context) error {
	roles, err := h.users.ListRoles(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, "could not load roles")
	}
	viewer, _ := auth.CurrentViewer(c)
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid user id")
	}

	form := adminFormFromRequest(c)
	form.ID = userID
	_, err = h.users.UpdateUser(c.Request().Context(), service.UpdateUserInput{
		ActorUserID: viewer.ID,
		UserID:      userID,
		Handle:      form.Handle,
		Email:       form.Email,
		Password:    form.Password,
		IsActive:    form.IsActive,
		Roles:       selectedRoles(form.Roles),
	})
	if err != nil {
		return c.Render(http.StatusUnprocessableEntity, "admin/user-form", AdminUserFormViewData{
			BaseViewData: baseView(c, "Edit User"),
			Heading:      "Edit User",
			SubmitLabel:  "Save changes",
			Action:       "/app/admin/users/" + userID.String(),
			CancelURL:    "/app/admin/users",
			Form:         form,
			RoleOptions:  roles,
			Error:        serviceMessage(err),
		})
	}

	return c.Redirect(http.StatusSeeOther, withFlash("/app/admin/users", "User updated."))
}

func (h *AdminHandler) SendPasswordReset(c *echo.Context) error {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid user id")
	}

	if err := h.passwordResets.SendReset(c.Request().Context(), userID, requestBaseURL(c)); err != nil {
		return c.Redirect(http.StatusSeeOther, withFlash("/app/admin/users", serviceMessage(err)))
	}

	return c.Redirect(http.StatusSeeOther, withFlash("/app/admin/users", "Password reset email sent."))
}

func (h *AdminHandler) DeleteUser(c *echo.Context) error {
	viewer, _ := auth.CurrentViewer(c)
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid user id")
	}

	if err := h.users.DeleteUser(c.Request().Context(), viewer.ID, userID); err != nil {
		return c.Redirect(http.StatusSeeOther, withFlash("/app/admin/users", serviceMessage(err)))
	}

	return c.Redirect(http.StatusSeeOther, withFlash("/app/admin/users", "User deleted."))
}

func adminFormFromRequest(c *echo.Context) AdminUserForm {
	_ = c.Request().ParseForm()
	params := c.Request().Form
	roles := make(map[string]bool)
	for _, role := range params["roles"] {
		roles[role] = true
	}

	return AdminUserForm{
		Handle:   c.FormValue("handle"),
		Email:    c.FormValue("email"),
		Password: c.FormValue("password"),
		IsActive: c.FormValue("is_active") == "on",
		Roles:    roles,
	}
}

func (h *AdminHandler) ListInvitations(c *echo.Context) error {
	filter := normalizeInvitationFilter(c.QueryParam("filter"))
	invitations, err := h.invitations.ListInvitations(c.Request().Context(), filter)
	if err != nil {
		return c.String(http.StatusInternalServerError, "could not load invitations")
	}

	baseURL := requestBaseURL(c)
	rows := make([]AdminInvitationRow, 0, len(invitations))
	for _, inv := range invitations {
		rows = append(rows, AdminInvitationRow{
			Invitation:  inv,
			RegisterURL: baseURL + "/register?code=" + inv.Code,
		})
	}

	return c.Render(http.StatusOK, "admin/invitations", AdminInvitationsViewData{
		BaseViewData:  baseView(c, "Invitations"),
		Invitations:   rows,
		Filter:        filter,
		FilterOptions: invitationFilterOptions,
	})
}

func (h *AdminHandler) ShowInviteForm(c *echo.Context) error {
	return c.Render(http.StatusOK, "admin/invite-form", AdminInviteFormViewData{
		BaseViewData: baseView(c, "Send Invitation"),
	})
}

func (h *AdminHandler) SendInvitation(c *echo.Context) error {
	viewer, _ := auth.CurrentViewer(c)
	email := c.FormValue("email")

	_, err := h.invitations.CreateAndSend(c.Request().Context(), viewer.ID, service.CreateInvitationInput{
		Email:   email,
		BaseURL: requestBaseURL(c),
	})
	if err != nil {
		return c.Render(http.StatusUnprocessableEntity, "admin/invite-form", AdminInviteFormViewData{
			BaseViewData: baseView(c, "Send Invitation"),
			Email:        email,
			Error:        serviceMessage(err),
		})
	}

	return c.Redirect(http.StatusSeeOther, withFlash("/app/admin/invitations", "Invitation sent."))
}

func (h *AdminHandler) ResendInvitation(c *echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid invitation id")
	}

	if err := h.invitations.ResendInvitation(c.Request().Context(), id, requestBaseURL(c)); err != nil {
		return c.Redirect(http.StatusSeeOther, withFlash("/app/admin/invitations", serviceMessage(err)))
	}

	return c.Redirect(http.StatusSeeOther, withFlash("/app/admin/invitations", "Invitation resent."))
}

func (h *AdminHandler) ArchiveInvitation(c *echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid invitation id")
	}

	if err := h.invitations.ArchiveInvitation(c.Request().Context(), id); err != nil {
		return c.Redirect(http.StatusSeeOther, withFlash("/app/admin/invitations", "Could not archive invitation."))
	}

	return c.Redirect(http.StatusSeeOther, withFlash("/app/admin/invitations", "Invitation archived."))
}

func requestBaseURL(c *echo.Context) string {
	r := c.Request()

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := firstHeaderValue(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		scheme = forwarded
	}

	host := r.Host
	if forwarded := firstHeaderValue(r.Header.Get("X-Forwarded-Host")); forwarded != "" {
		host = forwarded
	}

	return scheme + "://" + host
}

func firstHeaderValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if i := strings.Index(value, ","); i >= 0 {
		value = value[:i]
	}
	return strings.TrimSpace(value)
}

var invitationFilterOptions = []InvitationFilterOption{
	{Value: service.InvitationFilterAll, Label: "All"},
	{Value: service.InvitationFilterUnused, Label: "Unused"},
	{Value: service.InvitationFilterExpired, Label: "Expired"},
	{Value: service.InvitationFilterUsed, Label: "Used"},
}

func normalizeInvitationFilter(value string) string {
	switch value {
	case service.InvitationFilterUnused, service.InvitationFilterExpired, service.InvitationFilterUsed:
		return value
	default:
		return service.InvitationFilterAll
	}
}

func selectedRoles(roleMap map[string]bool) []string {
	roles := make([]string, 0, len(roleMap))
	for _, role := range []string{service.RoleUser, service.RoleAdmin} {
		if roleMap[role] {
			roles = append(roles, role)
		}
	}

	return roles
}
