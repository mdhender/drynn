package service

import (
	"context"
	"errors"
	"testing"

	"github.com/mdhender/drynn/internal/testdb"
	"github.com/mdhender/drynn/internal/testfixtures"
)

func newGameService(t testing.TB) (*GameService, *testfixtures.Fixtures) {
	t.Helper()
	pool := testdb.New(t)
	return NewGameService(pool), testfixtures.New(t, pool)
}

func TestGameService_CreateGame(t *testing.T) {
	svc, _ := newGameService(t)
	ctx := context.Background()

	game, err := svc.CreateGame(ctx, CreateGameInput{Name: "Alpha"})
	if err != nil {
		t.Fatalf("CreateGame: %v", err)
	}
	if game.Name != "Alpha" {
		t.Errorf("Name = %q, want %q", game.Name, "Alpha")
	}
	if game.Status != GameStatusSetup {
		t.Errorf("Status = %q, want %q", game.Status, GameStatusSetup)
	}
	if game.CurrentTurn != 0 {
		t.Errorf("CurrentTurn = %d, want 0", game.CurrentTurn)
	}
}

func TestGameService_CreateGame_TrimsName(t *testing.T) {
	svc, _ := newGameService(t)
	ctx := context.Background()

	game, err := svc.CreateGame(ctx, CreateGameInput{Name: "  Alpha  "})
	if err != nil {
		t.Fatalf("CreateGame: %v", err)
	}
	if game.Name != "Alpha" {
		t.Errorf("Name = %q, want trimmed %q", game.Name, "Alpha")
	}
}

func TestGameService_CreateGame_BlankName(t *testing.T) {
	svc, _ := newGameService(t)
	ctx := context.Background()

	_, err := svc.CreateGame(ctx, CreateGameInput{Name: "   "})
	if !errors.Is(err, ErrInvalidGameName) {
		t.Fatalf("want ErrInvalidGameName, got %v", err)
	}
}

func TestGameService_GetGame(t *testing.T) {
	svc, fix := newGameService(t)
	ctx := context.Background()
	seeded := fix.NewGame().Name("Alpha").Build(ctx)

	game, err := svc.GetGame(ctx, seeded.ID)
	if err != nil {
		t.Fatalf("GetGame: %v", err)
	}
	if game.ID != seeded.ID {
		t.Errorf("ID = %d, want %d", game.ID, seeded.ID)
	}
	if game.Name != "Alpha" {
		t.Errorf("Name = %q, want %q", game.Name, "Alpha")
	}
}

func TestGameService_GetGame_NotFound(t *testing.T) {
	svc, _ := newGameService(t)
	ctx := context.Background()

	_, err := svc.GetGame(ctx, 999999)
	if !errors.Is(err, ErrGameNotFound) {
		t.Fatalf("want ErrGameNotFound, got %v", err)
	}
}

func TestGameService_ListGames(t *testing.T) {
	svc, fix := newGameService(t)
	ctx := context.Background()

	first := fix.NewGame().Name("First").Build(ctx)
	second := fix.NewGame().Name("Second").Build(ctx)

	games, err := svc.ListGames(ctx)
	if err != nil {
		t.Fatalf("ListGames: %v", err)
	}
	if len(games) != 2 {
		t.Fatalf("len(games) = %d, want 2", len(games))
	}
	// ORDER BY created_at DESC, id DESC — second was inserted after first
	// and has a larger id, so it should come first.
	if games[0].ID != second.ID {
		t.Errorf("games[0].ID = %d, want %d", games[0].ID, second.ID)
	}
	if games[1].ID != first.ID {
		t.Errorf("games[1].ID = %d, want %d", games[1].ID, first.ID)
	}
}
