package main

import (
	"fmt"
	"log"

	"backend/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func mustInitDB() *gorm.DB {
	host := envOr("POSTGRES_HOST", "localhost")
	user := envOr("POSTGRES_USER", "admin")
	password := envOr("POSTGRES_PASSWORD", "")
	dbname := envOr("POSTGRES_DB", "postgres")
	port := envOr("POSTGRES_PORT", "5432")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to get underlying sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(5)

	// Auto-migrate tables.
	if err := db.AutoMigrate(
		&models.DockerPromotion{},
		&models.AutomationRelease{},
	); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	return db
}
