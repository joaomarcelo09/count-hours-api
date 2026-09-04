package config

import (
	"os"
)

type Config struct {
	Port            string
	DatabaseURL     string
	JWTSecret       string
	Environment     string
	AllowedOrigins  string
	MigrationsPath  string
}

func Load() Config {
	return Config{
		Port:           getenv("PORT", "8080"),
		DatabaseURL:    getenv("DATABASE_URL", "postgres://count:hours@localhost:5432/count_hours?sslmode=disable"),
		JWTSecret:      getenv("JWT_SECRET", "dev-secret-change-me"),
		Environment:    getenv("ENVIRONMENT", "development"),
		AllowedOrigins: getenv("ALLOWED_ORIGINS", "http://localhost:4200"),
		MigrationsPath: getenv("MIGRATIONS_PATH", "db/migrations"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}