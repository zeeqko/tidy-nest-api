package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

//go:embed seeders/*.sql
var seederFiles embed.FS

// Open opens the PostgreSQL database and verifies the connection.
// dsn example: postgres://postgres:postgres@localhost:5432/organizing_app?sslmode=disable
func Open(dsn string) (*sql.DB, error) {
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := database.Ping(); err != nil {
		database.Close()
		return nil, err
	}
	return database, nil
}

// Migrate applies any pending migration files, in filename order.
func Migrate(database *sql.DB) error {
	return apply(database, migrationFiles, "migrations", "SchemaMigrations")
}

// Seed applies any pending seeder files, in filename order.
func Seed(database *sql.DB) error {
	return apply(database, seederFiles, "seeders", "SeedMigrations")
}

// apply runs each not-yet-applied .sql file in dir inside a transaction and
// records it in trackingTable so reruns are no-ops.
func apply(database *sql.DB, files embed.FS, dir, trackingTable string) error {
	createTracking := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s (name TEXT PRIMARY KEY, appliedAt TIMESTAMPTZ NOT NULL DEFAULT now())`,
		trackingTable,
	)
	if _, err := database.Exec(createTracking); err != nil {
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
		row := database.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE name = $1`, trackingTable), name)
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

		tx, err := database.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(script)); err != nil {
			tx.Rollback()
			return fmt.Errorf("%s/%s: %w", dir, name, err)
		}
		if _, err := tx.Exec(fmt.Sprintf(`INSERT INTO %s (name) VALUES ($1)`, trackingTable), name); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
