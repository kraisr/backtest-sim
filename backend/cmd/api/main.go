package main

import (
	"log"
	"net/http"

	"backtest-sim/backend/internal/api"
)

func main() {
	// Build the API router and attach all registered routes
	router := api.NewRouter()

	log.Println("api listening on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}
