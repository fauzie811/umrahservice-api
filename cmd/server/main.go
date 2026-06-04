package main

import (
	"log"

	"umrahservice-api/internal/broadcast"
	"umrahservice-api/internal/config"
	"umrahservice-api/internal/db"
	"umrahservice-api/internal/handlers"
	"umrahservice-api/internal/pdf"
	"umrahservice-api/internal/router"
	"umrahservice-api/internal/storage"
)

func main() {
	cfg := config.Load()

	database, err := db.Open(cfg)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	store, err := storage.New(cfg)
	if err != nil {
		log.Fatalf("storage init failed: %v", err)
	}

	b := broadcast.New(cfg)
	pdfClient := pdf.NewClient(cfg.GotenbergURL)
	h := handlers.New(database, store, cfg, b, pdfClient)
	r := router.New(database, h)

	addr := ":" + cfg.Port
	log.Printf("listening on %s (env=%s)", addr, cfg.AppEnv)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
