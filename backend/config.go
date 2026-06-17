package main

import (
	"fmt"
	"os"
)

type Config struct {
	Addr        string
	DatabaseURL string
	ImageDir    string
}

func loadConfig() Config {
	return Config{
		Addr:        getEnv("ADDR", ":8080"),
		DatabaseURL: buildDatabaseURL(),
		ImageDir:    getEnv("IMAGE_DIR", "./data/images"),
	}
}

// buildDatabaseURL prefers DATABASE_URL if set, otherwise constructs one from
// the individual DB_* variables (DB_HOST, DB_PORT, DB_NAME, DB_USER, DB_PASSWORD, DB_SSLMODE).
func buildDatabaseURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	name := getEnv("DB_NAME", "unittrace")
	user := getEnv("DB_USER", "postgres")
	pass := getEnv("DB_PASSWORD", "postgres")
	ssl := getEnv("DB_SSLMODE", "disable")
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, port, name, ssl)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
