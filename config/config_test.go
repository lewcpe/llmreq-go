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

func TestParseDurationExtended(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		hasError bool
	}{
		{"10s", 10 * time.Second, false},
		{"5m", 5 * time.Minute, false},
		{"2h", 2 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"60d", 60 * 24 * time.Hour, false},
		{"invalid", 0, true},
		{"10x", 0, true},
	}

	for _, test := range tests {
		result, err := parseDurationExtended(test.input)
		if test.hasError {
			if err == nil {
				t.Errorf("Expected error for input %s, got nil", test.input)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for input %s: %v", test.input, err)
			}
			if result != test.expected {
				t.Errorf("For input %s, expected %v, got %v", test.input, test.expected, result)
			}
		}
	}
}

func TestLoadConfig_StandardKeyLifetime(t *testing.T) {
	os.Setenv("LLMREQ_DEFAULT_KEY_EXPIRE", "30d")
	LoadConfig()
	expected := 30 * 24 * time.Hour
	if AppConfig.StandardKeyLifetime != expected {
		t.Errorf("Expected %v, got %v", expected, AppConfig.StandardKeyLifetime)
	}

	os.Unsetenv("LLMREQ_DEFAULT_KEY_EXPIRE")
	LoadConfig()
	expectedDefault := 60 * 24 * time.Hour
	if AppConfig.StandardKeyLifetime != expectedDefault {
		t.Errorf("Expected default %v, got %v", expectedDefault, AppConfig.StandardKeyLifetime)
	}
}
