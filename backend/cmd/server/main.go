// Package main starts the calendar HTTP server.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/godovdm/ai-for-developers-project-386/backend/internal/server"
	"github.com/godovdm/ai-for-developers-project-386/backend/internal/web"
)

func main() {
	access := server.OwnerAccess{
		Password:      os.Getenv("OWNER_PASSWORD"),
		SessionSecret: os.Getenv("OWNER_SESSION_SECRET"),
	}
	if !access.Enabled() {
		log.Print("WARNING: OWNER_PASSWORD is not set; owner operations are unprotected (demonstration mode)")
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", server.NewWithOwnerAccess(access))
	dir := os.Getenv("FRONTEND_DIR")
	if dir == "" {
		dir = "../frontend/dist"
	}
	mux.Handle("/", web.SPA(dir))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Print("starting server")
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
