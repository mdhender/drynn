package handler

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

type PublicHandler struct{}

func NewPublicHandler() *PublicHandler {
	return &PublicHandler{}
}

func (h *PublicHandler) ShowHome(c *echo.Context) error {
	return c.Render(http.StatusOK, "public/home", HomeViewData{
		BaseViewData: baseView(c, "Hobo"),
	})
}
