package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// newTestPool connects to TEST_DATABASE_URL and returns a pool whose
// search_path is a throwaway schema, dropped on cleanup. The test is skipped
// when TEST_DATABASE_URL is unset.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()

	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	schema := "test_" + hex.EncodeToString(buf)

	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatalf("create schema: %v", err)
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}

	t.Cleanup(func() {
		pool.Close()
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop schema: %v", err)
		}
		admin.Close()
	})
	return pool
}

// execMigration runs one embedded migration file the way apply does.
func execMigration(t *testing.T, pool *pgxpool.Pool, name string) {
	t.Helper()
	ctx := context.Background()
	script, err := migrationFiles.ReadFile("migrations/" + name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(script)); err != nil {
		tx.Rollback(ctx)
		t.Fatalf("%s: %v", name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestMigration0016ShiftsOnlyY(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries { // embed.FS ReadDir is sorted by filename
		if e.Name() >= "0016" {
			break
		}
		execMigration(t, pool, e.Name())
	}

	var userID, invID, lookID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO Users (name, createdAt, updatedAt) VALUES ('u', now(), now()) RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO Inventories (userId, name, createdAt, updatedAt) VALUES ($1, 'shirt', now(), now()) RETURNING id`,
		userID).Scan(&invID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO Looks (userId, name, createdAt, updatedAt) VALUES ($1, 'look', now(), now()) RETURNING id`,
		userID).Scan(&lookID); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		x, y, w, h, r float64
		z             int
	}{
		{10.5, 0, 120.25, 150.75, 15, 3},
		{200, 250.25, 80, 90, -30.5, 7},
	} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO LookItems (lookId, inventoryId, x, y, width, height, rotation, zIndex)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			lookID, invID, row.x, row.y, row.w, row.h, row.r, row.z); err != nil {
			t.Fatal(err)
		}
	}

	execMigration(t, pool, "0016_recentre_look_items.sql")

	rows, err := pool.Query(ctx,
		`SELECT x, y, width, height, rotation, zIndex FROM LookItems ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	want := [][6]float64{
		{10.5, 105.5, 120.25, 150.75, 15, 3},
		{200, 355.75, 80, 90, -30.5, 7},
	}
	i := 0
	for rows.Next() {
		var x, y, w, h, r float64
		var z int
		if err := rows.Scan(&x, &y, &w, &h, &r, &z); err != nil {
			t.Fatal(err)
		}
		got := [6]float64{x, y, w, h, r, float64(z)}
		if i >= len(want) || got != want[i] {
			t.Fatalf("row %d: got %v, want %v", i, got, fmt.Sprint(want))
		}
		i++
	}
	if i != len(want) {
		t.Fatalf("got %d rows, want %d", i, len(want))
	}
}
