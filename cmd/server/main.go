package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"organizing-app-backend/internal/db"
	"organizing-app-backend/internal/router"
	"organizing-app-backend/internal/storage"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/organizing_app?sslmode=disable"
	}

	database, err := db.Open(dsn)
	if err != nil {
		log.Fatalf("connect to postgres: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if err := db.Seed(database); err != nil {
		log.Fatalf("seed: %v", err)
	}

	// Where item photos live: the R2 bucket when configured, else local disk.
	photos, err := storage.FromEnv(context.Background())
	if err != nil {
		log.Fatalf("photo storage: %v", err)
	}
	log.Printf("item photos stored in %s", photos.Describe())

	addr := ":8080"
	log.Printf("organizing-app backend listening on %s", addr)
	if err := http.ListenAndServe(addr, router.New(database, photos)); err != nil {
		log.Fatal(err)
	}
}
