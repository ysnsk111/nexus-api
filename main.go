package main

import (
	"log"
	"os"

	"nexus-api/core"
	"nexus-api/models"
)

func main() {
	// Initialize database
	models.InitDB()

	// Initialize default admin if not exists
	models.InitDefaultAdmin()

	// Setup Gin router
	r := core.SetupRouter()

	port := os.Getenv("PANEL_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting Nexus-API on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
