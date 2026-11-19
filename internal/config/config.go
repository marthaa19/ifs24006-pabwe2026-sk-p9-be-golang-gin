// Package config memuat konfigurasi aplikasi dari berkas .env / environment.
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config menampung seluruh pengaturan yang dibutuhkan server.
type Config struct {
	AppEnv        string
	AppPort       string
	AppKey        string
	AdminUsername string
	AdminPassword string

	DBConnection string
	DBHost       string
	DBPort       string
	DBDatabase   string
	DBUsername   string
	DBPassword   string
	DBSSLMode    string

	SearchEndpoint string
	OpenAIEndpoint string
	OpenAIKey      string
	OpenAIModel    string
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// LoadConfig membaca .env (jika ada) lalu mengisi Config dengan nilai bawaan.
func LoadConfig() *Config {
	_ = godotenv.Load()
	return &Config{
		AppEnv:         getEnv("APP_ENV", "development"),
		AppPort:        getEnv("APP_PORT", "8081"),
		AppKey:         getEnv("APP_KEY", "delcom_blog_secret_key"),
		AdminUsername:  getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword:  getEnv("ADMIN_PASSWORD", "admin"),
		DBConnection:   getEnv("DB_CONNECTION", "pgsql"),
		DBHost:         getEnv("DB_HOST", "127.0.0.1"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBDatabase:     getEnv("DB_DATABASE", "pabwe_p9_blog"),
		DBUsername:     getEnv("DB_USERNAME", "postgres"),
		DBPassword:     getEnv("DB_PASSWORD", "postgres"),
		DBSSLMode:      getEnv("DB_SSLMODE", "disable"),
		SearchEndpoint: getEnv("SEARCH_ENGINE_ENDPOINT", "https://delcom.org/api/v1/search"),
		OpenAIEndpoint: getEnv("OPENAI_ENDPOINT", "https://delcom.org/api/v1"),
		OpenAIKey:      getEnv("OPENAI_KEY", ""),
		OpenAIModel:    getEnv("OPENAI_MODEL", "nama-model"),
	}
}

// DSN menyusun connection string PostgreSQL.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Jakarta",
		c.DBHost, c.DBPort, c.DBUsername, c.DBPassword, c.DBDatabase, c.DBSSLMode,
	)
}
