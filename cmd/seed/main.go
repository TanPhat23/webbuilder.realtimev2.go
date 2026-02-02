package main

import (
	"log"
	"os"
	"webbuilder/internal/config"
	"webbuilder/pkg/database"
	"webbuilder/seed"
)

func main() {
	// Load configuration from environment variables
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Could not load configuration: %v", err)
	}

	// Initialize database connection
	db, err := database.NewPostgresConnection(database.Config{
		URL:      cfg.Database.URL,
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.Name,
		SSLMode:  cfg.Database.SSLMode,
	})
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}

	// Get command line argument if provided
	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	seeder := seed.NewSeedData(db)

	switch command {
	case "user":
		log.Println("Seeding users...")
		if err := seed.SeedUsers(db); err != nil {
			log.Fatalf("Failed to seed users: %v", err)
		}
	case "project":
		log.Println("Seeding projects (including pages and elements)...")
		if err := seed.SeedProjects(db); err != nil {
			log.Fatalf("Failed to seed projects: %v", err)
		}
	case "workflow":
		log.Println("Seeding workflows...")
		if err := seed.SeedWorkflows(db); err != nil {
			log.Fatalf("Failed to seed workflows: %v", err)
		}
	case "all", "":
		log.Println("Initializing full database seeding...")
		if err := seeder.Run(); err != nil {
			log.Fatalf("Seeding process failed: %v", err)
		}
	default:
		log.Fatalf("Unknown command: %s. Available commands: user, project, workflow, all", command)
	}

	log.Println("Database operation completed successfully.")
}
