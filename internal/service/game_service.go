package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mdhender/drynn/db/sqlc"
)

const (
	GameStatusSetup     = "setup"
	GameStatusActive    = "active"
	GameStatusCompleted = "completed"
)

type Game struct {
	ID          int64
	Name        string
	Status      string
	CurrentTurn int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CreateGameInput struct {
	Name string
}

type GameService struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewGameService(pool *pgxpool.Pool) *GameService {
	return &GameService{pool: pool, queries: sqlc.New(pool)}
}

func (s *GameService) CreateGame(ctx context.Context, input CreateGameInput) (*Game, error) {
	name, err := normalizeGameName(input.Name)
	if err != nil {
		return nil, err
	}

	row, err := s.queries.CreateGame(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("create game: %w", err)
	}

	return mapGame(row), nil
}

func (s *GameService) GetGame(ctx context.Context, gameID int64) (*Game, error) {
	row, err := s.queries.GetGameByID(ctx, gameID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrGameNotFound
		}
		return nil, fmt.Errorf("get game: %w", err)
	}

	return mapGame(row), nil
}

func (s *GameService) ListGames(ctx context.Context) ([]Game, error) {
	rows, err := s.queries.ListGames(ctx)
	if err != nil {
		return nil, fmt.Errorf("list games: %w", err)
	}

	games := make([]Game, 0, len(rows))
	for _, row := range rows {
		games = append(games, *mapGame(row))
	}
	return games, nil
}

func normalizeGameName(name string) (string, error) {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return "", ErrInvalidGameName
	}
	return normalized, nil
}

func mapGame(row sqlc.Game) *Game {
	return &Game{
		ID:          row.ID,
		Name:        row.Name,
		Status:      row.Status,
		CurrentTurn: row.CurrentTurn,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
}
