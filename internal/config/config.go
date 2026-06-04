package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration, loaded from the environment (and an
// optional .env file). Field defaults mirror the Laravel app's .env so the Go
// service can run drop-in against the same infrastructure.
type Config struct {
	AppEnv string
	AppURL string
	Port   string

	DB DBConfig

	AWS AWSConfig

	// Laravel is the upstream Laravel app the API delegates to for work it does
	// not reproduce (e.g. PIF PDF generation).
	LaravelURL     string
	InternalSecret string

	Reverb ReverbConfig
}

type DBConfig struct {
	Host     string
	Port     string
	Database string
	Username string
	Password string
}

type AWSConfig struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	Bucket          string
	URL             string
	Endpoint        string
	UsePathStyle    bool
}

type ReverbConfig struct {
	AppID  string
	Key    string
	Secret string
	Host   string
	Port   string
	Scheme string
}

// Load reads configuration from a .env file (if present) and the process
// environment. Environment variables always win over the .env file.
func Load() *Config {
	// Best-effort: ignore missing .env so production env-only setups work.
	_ = godotenv.Load()

	return &Config{
		AppEnv: env("APP_ENV", "production"),
		AppURL: env("APP_URL", "http://localhost"),
		Port:   env("PORT", "8000"),
		DB: DBConfig{
			Host:     env("DB_HOST", "127.0.0.1"),
			Port:     env("DB_PORT", "3306"),
			Database: env("DB_DATABASE", "umrahservice_app"),
			Username: env("DB_USERNAME", "root"),
			Password: env("DB_PASSWORD", ""),
		},
		AWS: AWSConfig{
			AccessKeyID:     env("AWS_ACCESS_KEY_ID", ""),
			SecretAccessKey: env("AWS_SECRET_ACCESS_KEY", ""),
			Region:          env("AWS_DEFAULT_REGION", "us-east-1"),
			Bucket:          env("AWS_BUCKET", ""),
			URL:             env("AWS_URL", ""),
			Endpoint:        env("AWS_ENDPOINT", ""),
			UsePathStyle:    envBool("AWS_USE_PATH_STYLE_ENDPOINT", false),
		},
		LaravelURL:     env("LARAVEL_URL", "http://localhost"),
		InternalSecret: env("INTERNAL_API_SECRET", ""),
		Reverb: ReverbConfig{
			AppID:  env("REVERB_APP_ID", ""),
			Key:    env("REVERB_APP_KEY", ""),
			Secret: env("REVERB_APP_SECRET", ""),
			Host:   env("REVERB_HOST", "localhost"),
			Port:   env("REVERB_PORT", "8080"),
			Scheme: env("REVERB_SCHEME", "http"),
		},
	}
}

// DSN builds the GORM MySQL/MariaDB connection string.
func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.Username, c.Password, c.Host, c.Port, c.Database,
	)
}

func (c *Config) IsLocal() bool { return c.AppEnv == "local" }

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
