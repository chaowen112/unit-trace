package main

import "os"

// Config holds application configuration loaded from environment variables.
type Config struct {
	Addr        string
	DatabaseURL string
	ImageDir    string
}

// loadConfig reads configuration from environment variables with sensible defaults.
func loadConfig() Config {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/unittrace?sslmode=disable"
	}

	imageDir := os.Getenv("IMAGE_DIR")
	if imageDir == "" {
		imageDir = "./data/images"
	}

	return Config{
		Addr:        addr,
		DatabaseURL: dbURL,
		ImageDir:    imageDir,
	}
}
