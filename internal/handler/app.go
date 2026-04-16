package handler

import (
	"net/http"

	"github.com/mdhender/drynn/internal/auth"
	"github.com/mdhender/drynn/internal/service"

	"github.com/labstack/echo/v5"
)

type AppHandler struct {
	users *service.UserService
}

func NewAppHandler(users *service.UserService) *AppHandler {
	return &AppHandler{users: users}
}

func (h *AppHandler) ShowProfile(c *echo.Context) error {
	viewer, _ := auth.CurrentViewer(c)
	return c.Render(http.StatusOK, "app/profile", ProfileViewData{
		BaseViewData: baseView(c, "Profile"),
		Form: ProfileForm{
			Handle: viewer.Handle,
			Email:  viewer.Email,
		},
	})
}

func (h *AppHandler) UpdateProfile(c *echo.Context) error {
	viewer, _ := auth.CurrentViewer(c)
	form := ProfileForm{
		Handle: c.FormValue("handle"),
		Email:  c.FormValue("email"),
	}

	_, err := h.users.UpdateProfile(c.Request().Context(), service.UpdateProfileInput{
		UserID: viewer.ID,
		Handle: form.Handle,
		Email:  form.Email,
	})
	if err != nil {
		return c.Render(http.StatusUnprocessableEntity, "app/profile", ProfileViewData{
			BaseViewData: baseView(c, "Profile"),
			Form:         form,
			Error:        serviceMessage(err),
		})
	}

	return c.Redirect(http.StatusSeeOther, withFlash("/app/profile", "Profile updated."))
}
