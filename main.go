// Delcom Blog API - titik masuk server.
package main

import (
	"log"

	"app/internal/config"
	"app/internal/database"
	"app/internal/features/ai"
	"app/internal/server"
)

func main() {
	cfg := config.LoadConfig()

	db, err := database.Setup(cfg)
	if err != nil {
		log.Fatalf("gagal menyiapkan database: %v", err)
	}

	router := server.NewRouter(cfg, db,
		ai.NewHTTPSearcher(cfg.SearchEndpoint),
		ai.NewOpenAIClient(cfg.OpenAIEndpoint, cfg.OpenAIKey, cfg.OpenAIModel),
	)

	log.Printf("Delcom Blog API aktif di http://localhost:%s (Swagger: /swagger/index.html)", cfg.AppPort)
	if err := router.Run(":" + cfg.AppPort); err != nil {
		log.Fatalf("server berhenti: %v", err)
	}
}
