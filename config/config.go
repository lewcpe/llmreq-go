package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Prefix              string
	LiteLLMAPIURL       string
	LiteLLMMasterKey    string
	DatabaseURL         string
	DefaultBudget       float64
	LongTermKeyLifetime time.Duration
	LongTermKeyLimit    int
	LongTermKeyBudget   float64
	MaxActiveKeys       int
}

var AppConfig *Config

func LoadConfig() {
	_ = godotenv.Load() // Load from .env if it exists, ignore error if not

	AppConfig = &Config{
		Prefix:              getEnv("LLMREQ_PREFIX", "/api"),
		LiteLLMAPIURL:       getEnv("LITELLM_API_URL", "http://litellm:4000"),
		LiteLLMMasterKey:    getEnv("LITELLM_MASTER_KEY", ""),
		DatabaseURL:         getEnv("LLMREQ_DATABASE_URL", "file:app.db?cache=shared&mode=rwc"),
		DefaultBudget:       getEnvFloat("LLMREQ_DEFAULT_BUDGET", 1.0),
		LongTermKeyLifetime: getEnvDuration("LLMREQ_LONGTERM_KEY_LIFETIME", 9600*time.Hour),
		LongTermKeyLimit:    getEnvInt("LLMREQ_LONGTERM_KEY_LIMIT", 1),
		LongTermKeyBudget:   getEnvFloat("LLMREQ_LONGTERM_KEY_BUDGET", 20.0),
		MaxActiveKeys:       getEnvInt("LLMREQ_MAX_ACTIVE_KEY", 10),
	}

	if AppConfig.LiteLLMMasterKey == "" {
		log.Println("Warning: LITELLM_MASTER_KEY is not set.")
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	strValue := getEnv(key, "")
	if strValue == "" {
		return fallback
	}
	if value, err := strconv.ParseFloat(strValue, 64); err == nil {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	strValue := getEnv(key, "")
	if strValue == "" {
		return fallback
	}
	if value, err := strconv.Atoi(strValue); err == nil {
		return value
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	strValue := getEnv(key, "")
	if strValue == "" {
		return fallback
	}
	if value, err := time.ParseDuration(strValue); err == nil {
		return value
	}
	return fallback
}
