package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	os.Setenv("LLMREQ_PREFIX", "/test")
	os.Setenv("LLMREQ_DEFAULT_BUDGET", "5.0")
	os.Setenv("LLMREQ_MAX_ACTIVE_KEY", "20")

	LoadConfig()

	if AppConfig.Prefix != "/test" {
		t.Errorf("Expected /test, got %s", AppConfig.Prefix)
	}
	if AppConfig.DefaultBudget != 5.0 {
		t.Errorf("Expected 5.0, got %f", AppConfig.DefaultBudget)
	}
	if AppConfig.MaxActiveKeys != 20 {
		t.Errorf("Expected 20, got %d", AppConfig.MaxActiveKeys)
	}

	// Default fallback
	os.Unsetenv("LLMREQ_LONGTERM_KEY_LIFETIME")
	LoadConfig()
	if AppConfig.LongTermKeyLifetime != 9600*time.Hour {
		t.Errorf("Expected 9600h, got %v", AppConfig.LongTermKeyLifetime)
	}
}
