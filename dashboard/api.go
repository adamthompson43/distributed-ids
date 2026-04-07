package main

import (
	"database/sql"
	"net/http"
)

func registerRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("/api/health", handleHealth)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}