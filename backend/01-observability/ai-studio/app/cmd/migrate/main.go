
package main

import (
	"fmt"
	"log"
	"os"
	
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		getEnv("DB_USER", "user"),
		getEnv("DB_PASSWORD", "password"),
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_NAME", "appdb"),
	)
	
	m, err := migrate.New("file:///app/migrations", dbURL)
	if err != nil {
		log.Fatalf("could not create migrate instance: %v", err)
	}
	
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("could not run up migrations: %v", err)
	}
	
	log.Println("Migrations applied successfully!")
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
