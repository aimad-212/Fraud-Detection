package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"fraud-detection/internal/config"
	"fraud-detection/internal/domain"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Parse command line flags
	configFile := flag.String("config", "configs/config.yaml", "Path to config file")
	migrationDir := flag.String("dir", "migrations", "Directory containing migration files")
	action := flag.String("action", "up", "Migration action: up, down, status")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadFromFile(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to database
	if cfg.Database.Type != "postgres" {
		log.Fatal("Migrations require PostgreSQL database")
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Username,
		cfg.Database.Password,
		cfg.Database.Database,
		cfg.Database.Port,
		cfg.Database.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Execute action
	switch *action {
	case "up":
		if err := runMigrations(db, *migrationDir); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		fmt.Println("Migrations completed successfully")
	case "down":
		fmt.Println("Rollback not implemented yet")
	case "status":
		fmt.Println("Status not implemented yet")
	default:
		log.Fatalf("Unknown action: %s", *action)
	}

	// Auto migrate domain models
	if err := db.AutoMigrate(
		&domain.Transaction{},
		&domain.User{},
	); err != nil {
		log.Fatalf("Auto migration failed: %v", err)
	}

	fmt.Println("Auto migration completed")
}

// runMigrations executes SQL migration files
func runMigrations(db *gorm.DB, dir string) error {
	// Read migration files
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return err
	}

	// Execute each migration file
	for _, file := range files {
		fmt.Printf("Running migration: %s\n", file)
		sql, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		if err := db.Exec(string(sql)).Error; err != nil {
			return fmt.Errorf("failed to execute %s: %w", file, err)
		}
	}

	return nil
}
