package main

import (
	"database/sql"
	"flag"
	"log"
	"net/http"

	_ "github.com/lib/pq"
)

func main() {
	listenAddr := flag.String("listen", ":9090", "Address to serve the dashboard on")
	dbDSN := flag.String("db", "", "PostgreSQL DSN (required)")
	flag.Parse()

	if *dbDSN == "" {
		log.Fatal("missing required -db flag")
	}

	db, err := sql.Open("postgres", *dbDSN)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("db ping: %v", err)
	}
	log.Println("Database connected")

	mux := http.NewServeMux()
	registerRoutes(mux, db)

	log.Printf("Dashboard listening on %s", *listenAddr)
	if err := http.ListenAndServe(*listenAddr, corsMiddleware(mux)); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
