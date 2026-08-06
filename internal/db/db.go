package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

//go:embed seeders/*.sql
var seederFiles embed.FS

// Open opens the PostgreSQL connection pool and verifies the connection.
// pgxpool.New is lazy and does not connect on its own, so Open eagerly Pings
// and closes the pool on failure — callers (and tests) rely on Open failing
// fast against a dead database rather than returning a pool that will only
// fail later on first use.
// dsn example: postgres://postgres:postgres@localhost:5432/organizing_app?sslmode=disable
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// Migrate applies any pending migration files, in filename order.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	return apply(ctx, pool, migrationFiles, "migrations", "SchemaMigrations")
}

// Seed applies any pending seeder files, in filename order.
func Seed(ctx context.Context, pool *pgxpool.Pool) error {
	return apply(ctx, pool, seederFiles, "seeders", "SeedMigrations")
}

// apply runs each not-yet-applied .sql file in dir inside a transaction and
// records it in trackingTable so reruns are no-ops.
func apply(ctx context.Context, pool *pgxpool.Pool, files embed.FS, dir, trackingTable string) error {
	createTracking := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s (name TEXT PRIMARY KEY, appliedAt TIMESTAMPTZ NOT NULL DEFAULT now())`,
		trackingTable,
	)
	if _, err := pool.Exec(ctx, createTracking); err != nil {
		return fmt.Errorf("create %s: %w", trackingTable, err)
	}

	entries, err := fs.ReadDir(files, dir)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		name := entry.Name()

		var applied int
		row := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE name = $1`, trackingTable), name)
		if err := row.Scan(&applied); err != nil {
			return err
		}
		if applied > 0 {
			continue
		}

		script, err := files.ReadFile(dir + "/" + name)
		if err != nil {
			return err
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(script)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("%s/%s: %w", dir, name, err)
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s (name) VALUES ($1)`, trackingTable), name); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
