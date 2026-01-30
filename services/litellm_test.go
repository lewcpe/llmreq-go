package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/llmreq/config"
)

func TestLiteLLMService_GetUserInfo(t *testing.T) {
	// Setup Mock Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/info/test@example.com" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(LiteLLMUser{
				UserID:    "test@example.com",
				UserEmail: "test@example.com",
				Spend:     10.0,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Setup Service
	config.AppConfig = &config.Config{
		LiteLLMAPIURL:    server.URL,
		LiteLLMMasterKey: "test-key",
	}
	service := NewLiteLLMService()
	service.BaseURL = server.URL // Override with mock URL

	// Test Success
	user, err := service.GetUserInfo("test@example.com")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if user == nil {
		t.Fatal("Expected user, got nil")
	}
	if user.UserID != "test@example.com" {
		t.Errorf("Expected user ID test@example.com, got %s", user.UserID)
	}

	// Test Not Found
	user, err = service.GetUserInfo("nonexistent")
	if err != nil {
		t.Fatalf("Expected no error for 404, got %v", err)
	}
	if user != nil {
		t.Fatal("Expected nil user, got object")
	}
}

func TestLiteLLMService_CreateUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/new" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	service := NewLiteLLMService()
	service.BaseURL = server.URL

	err := service.CreateUser("new@example.com", "new@example.com", 1.0)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestLiteLLMService_ListKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/key/list" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"keys": [{"key": "sk-123", "spend": 0.5}]}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	service := NewLiteLLMService()
	service.BaseURL = server.URL

	keys, err := service.ListKeys("user")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Errorf("Expected 1 key, got %d", len(keys))
	}
	if keys[0].Key != "sk-123" {
		t.Errorf("Expected key sk-123, got %s", keys[0].Key)
	}
}

func TestLiteLLMService_GenerateKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/key/generate" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(GenerateKeyResponse{
				Key: "sk-generated",
			})
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	service := NewLiteLLMService()
	service.BaseURL = server.URL

	resp, err := service.GenerateKey(GenerateKeyRequest{UserID: "u"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Key != "sk-generated" {
		t.Errorf("Expected sk-generated, got %s", resp.Key)
	}
}

func TestLiteLLMService_DeleteKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/key/delete" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	service := NewLiteLLMService()
	service.BaseURL = server.URL

	err := service.DeleteKey("sk-123")
	if err != nil {
		t.Fatal(err)
	}
}
