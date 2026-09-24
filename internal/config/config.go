package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddress     string
	DatabaseURL     string
	KafkaBrokers    []string
	KafkaTopic      string
	OTLPEndpoint    string
	ShutdownTimeout time.Duration
}

func Load() Config {
	return Config{
		HTTPAddress:     env("HTTP_ADDRESS", ":8080"),
		DatabaseURL:     env("DATABASE_URL", "postgres://insights:insights@localhost:5432/insights?sslmode=disable"),
		KafkaBrokers:    strings.Split(env("KAFKA_BROKERS", "localhost:9092"), ","),
		KafkaTopic:      env("KAFKA_TOPIC", "transaction-ingested"),
		OTLPEndpoint:    env("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		ShutdownTimeout: time.Duration(envInt("SHUTDOWN_TIMEOUT_SECONDS", 15)) * time.Second,
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}
