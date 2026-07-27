package main

import (
	"log"
	"net/http"
	"os"

	"organizing-app-backend/internal/db"
	"organizing-app-backend/internal/router"
	"organizing-app-backend/internal/service"
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

	// Dev convenience: opt-in credentials for the seeded demo user via env.
	if pw := os.Getenv("DEMO_USER_PASSWORD"); pw != "" {
		set, err := service.NewAuthService(database).EnsureDemoCredentials("zee@tidynest.local", pw)
		if err != nil {
			log.Fatalf("demo credentials: %v", err)
		}
		if set {
			log.Printf("demo user login enabled: zee@tidynest.local")
		}
	}

	addr := ":8080"
	log.Printf("organizing-app backend listening on %s", addr)
	if err := http.ListenAndServe(addr, router.New(database)); err != nil {
		log.Fatal(err)
	}
}
