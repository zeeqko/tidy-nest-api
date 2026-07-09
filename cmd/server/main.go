package main

import (
	"log"
	"net/http"

	"organizing-app-backend/internal/router"
)

func main() {
	addr := ":8080"
	log.Printf("organizing-app backend listening on %s", addr)
	if err := http.ListenAndServe(addr, router.New()); err != nil {
		log.Fatal(err)
	}
}
