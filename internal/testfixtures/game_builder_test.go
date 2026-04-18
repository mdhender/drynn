package testfixtures_test

import (
	"context"
	"testing"

	"github.com/mdhender/drynn/internal/testdb"
	"github.com/mdhender/drynn/internal/testfixtures"
)

func TestFixtures_NewGame_Build(t *testing.T) {
	t.Parallel()
	pool := testdb.New(t)
	ctx := context.Background()
	fix := testfixtures.New(t, pool)

	game := fix.NewGame().Build(ctx)

	if game.ID == 0 {
		t.Fatalf("expected non-zero game id, got %d", game.ID)
	}
	if game.Name == "" {
		t.Fatalf("expected non-empty default name")
	}
	if game.Status != "setup" {
		t.Fatalf("expected status %q, got %q", "setup", game.Status)
	}
	if game.CurrentTurn != 0 {
		t.Fatalf("expected current_turn 0, got %d", game.CurrentTurn)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM games WHERE id = $1`, game.ID).Scan(&count); err != nil {
		t.Fatalf("query inserted row: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row for game %d, got %d", game.ID, count)
	}
}

func TestFixtures_NewGame_NameOverride(t *testing.T) {
	t.Parallel()
	pool := testdb.New(t)
	ctx := context.Background()
	fix := testfixtures.New(t, pool)

	game := fix.NewGame().Name("Alpha").Build(ctx)

	if game.Name != "Alpha" {
		t.Fatalf("expected name %q, got %q", "Alpha", game.Name)
	}
}
