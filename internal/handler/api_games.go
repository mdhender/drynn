package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/mdhender/drynn/internal/service"
)

type apiCreateGameRequest struct {
	Name string `json:"name"`
}

type apiCreateGameResponse struct {
	ID int64 `json:"id"`
}

type apiGameResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	CurrentTurn int32  `json:"current_turn"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (h *APIHandler) CreateGame(c *echo.Context) error {
	var req apiCreateGameRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid request body"})
	}

	game, err := h.games.CreateGame(c.Request().Context(), service.CreateGameInput{Name: req.Name})
	if err != nil {
		if errors.Is(err, service.ErrInvalidGameName) {
			return c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "name is required"})
		}
		h.logger.Error("api create game", "error", err)
		return c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "internal error"})
	}

	return c.JSON(http.StatusCreated, apiCreateGameResponse{ID: game.ID})
}

func (h *APIHandler) ListGames(c *echo.Context) error {
	games, err := h.games.ListGames(c.Request().Context())
	if err != nil {
		h.logger.Error("api list games", "error", err)
		return c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "internal error"})
	}

	out := make([]apiGameResponse, 0, len(games))
	for i := range games {
		out = append(out, apiGameFromService(&games[i]))
	}
	return c.JSON(http.StatusOK, out)
}

func (h *APIHandler) GetGame(c *echo.Context) error {
	id, err := parseGameID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid game id"})
	}

	game, err := h.games.GetGame(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrGameNotFound) {
			return c.JSON(http.StatusNotFound, apiErrorResponse{Error: "game not found"})
		}
		h.logger.Error("api get game", "error", err)
		return c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "internal error"})
	}

	return c.JSON(http.StatusOK, apiGameFromService(game))
}

func (h *APIHandler) UpdateGame(c *echo.Context) error {
	if _, err := parseGameID(c); err != nil {
		return c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid game id"})
	}
	return c.JSON(http.StatusNotImplemented, apiErrorResponse{Error: "not yet implemented"})
}

func (h *APIHandler) DeleteGame(c *echo.Context) error {
	id, err := parseGameID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid game id"})
	}

	if err := h.games.DeleteGame(c.Request().Context(), id); err != nil {
		if errors.Is(err, service.ErrGameNotFound) {
			return c.JSON(http.StatusNotFound, apiErrorResponse{Error: "game not found"})
		}
		h.logger.Error("api delete game", "error", err)
		return c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "internal error"})
	}

	return c.NoContent(http.StatusNoContent)
}

func parseGameID(c *echo.Context) (int64, error) {
	return strconv.ParseInt(c.Param("id"), 10, 64)
}

func apiGameFromService(g *service.Game) apiGameResponse {
	return apiGameResponse{
		ID:          g.ID,
		Name:        g.Name,
		Status:      g.Status,
		CurrentTurn: g.CurrentTurn,
		CreatedAt:   g.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   g.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
