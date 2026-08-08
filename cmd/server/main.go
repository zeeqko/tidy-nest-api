package main

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"organizing-app-backend/internal/ai"
	"organizing-app-backend/internal/db"
	"organizing-app-backend/internal/router"
	"organizing-app-backend/internal/storage"
)

// loadDotEnv reads KEY=VALUE pairs from a local .env file (dev convenience
// only) and applies them via os.Setenv, skipping any key already set in the
// real environment so k8s/docker env vars always win. Missing file is not an
// error — .env is optional and never present in deployed environments.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if _, alreadySet := os.LookupEnv(key); alreadySet {
			continue
		}
		os.Setenv(key, strings.TrimSpace(value))
	}
}

func main() {
	loadDotEnv(".env")

	ctx := context.Background()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/organizing_app?sslmode=disable"
	}

	pool, err := db.Open(ctx, dsn)
	if err != nil {
		log.Fatalf("connect to postgres: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if err := db.Seed(ctx, pool); err != nil {
		log.Fatalf("seed: %v", err)
	}

	// Where item photos live: the R2 bucket when configured, else local disk.
	photos, err := storage.FromEnv(ctx)
	if err != nil {
		log.Fatalf("photo storage: %v", err)
	}
	log.Printf("item photos stored in %s", photos.Describe())

	aiClient := ai.NewClientFromEnv()
	if aiClient == nil {
		log.Printf("GEMINI_API_KEY not set — AI item recognition disabled")
	}

	addr := ":8080"
	log.Printf("organizing-app backend listening on %s", addr)
	if err := http.ListenAndServe(addr, router.New(pool, photos, aiClient)); err != nil {
		log.Fatal(err)
	}
}
