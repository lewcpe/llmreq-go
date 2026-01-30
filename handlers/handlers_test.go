package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/llmreq/config"
	"github.com/example/llmreq/models"
	"github.com/example/llmreq/services"
	"github.com/labstack/echo/v4"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, _ := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err := db.AutoMigrate(&models.KeyHistory{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	db.Exec("DELETE FROM key_histories")
	return db
}

func TestGetMe(t *testing.T) {
	// Mock LiteLLM
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/user/info/") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(services.LiteLLMUser{
				UserID: "test@example.com",
			})
			return
		}
	}))
	defer server.Close()

	config.AppConfig = &config.Config{DefaultBudget: 1.0}
	svc := services.NewLiteLLMService()
	svc.BaseURL = server.URL
	db := setupTestDB(t)
	h := NewHandler(svc, db)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("X-Forwarded-Email", "test@example.com")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "test@example.com")

	if err := h.GetMe(c); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
}

func TestCreateKey(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/key/generate" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(services.GenerateKeyResponse{
				Key: "sk-12345678",
			})
			return
		}
		if r.URL.Path == "/key/list" {
			callCount++
			w.WriteHeader(http.StatusOK)
			if callCount == 1 {
				// Limit check
				_, _ = w.Write([]byte(`{"keys": []}`))
			} else {
				// Sync check
				_, _ = w.Write([]byte(`{"keys": [{"key": "sk-1234...", "key_alias": "test-key"}]}`))
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config.AppConfig = &config.Config{
		DefaultBudget: 1.0,
		MaxActiveKeys: 10,
	}

	svc := services.NewLiteLLMService()
	svc.BaseURL = server.URL
	db := setupTestDB(t)
	h := NewHandler(svc, db)

	e := echo.New()
	body := `{"name": "test-key", "type": "standard"}`
	req := httptest.NewRequest(http.MethodPost, "/api/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "test@example.com")

	if err := h.CreateKey(c); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d, body: %s", rec.Code, rec.Body.String())
	}

	// Check DB
	var key models.KeyHistory
	db.Where("user_id = ?", "test@example.com").First(&key)
	if key.LiteLLMKeyID != "sk-1234..." {
		t.Errorf("Expected synced key ID sk-1234..., got %s", key.LiteLLMKeyID)
	}
}

func TestGetActiveKeys(t *testing.T) {
	// Mock LiteLLM
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/key/list" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"keys": [{"key": "sk-active", "key_alias": "active-key", "spend": 1.0, "user_id": "test@example.com"}]}`))
			return
		}
	}))
	defer server.Close()

	svc := services.NewLiteLLMService()
	svc.BaseURL = server.URL
	db := setupTestDB(t)
	h := NewHandler(svc, db)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/keys/active", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "test@example.com")

	if err := h.GetActiveKeys(c); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	// Check DB synced
	var key models.KeyHistory
	if err := db.Where("litellm_key_id = ?", "sk-active").First(&key).Error; err != nil {
		t.Fatal("Expected key to be synced to DB")
	}
	if key.Status != "active" {
		t.Errorf("Expected status active, got %s", key.Status)
	}
}

func TestGetActiveKeysSyncAlias(t *testing.T) {
	// Test matching by alias when ID differs
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/key/list" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"keys": [{"key": "sk-new-id", "key_alias": "alias-match", "user_id": "test@example.com"}]}`))
			return
		}
	}))
	defer server.Close()

	svc := services.NewLiteLLMService()
	svc.BaseURL = server.URL
	db := setupTestDB(t)
	h := NewHandler(svc, db)

	// Seed with old ID
	db.Create(&models.KeyHistory{
		UserID:       "test@example.com",
		LiteLLMKeyID: "sk-old-id",
		KeyName:      "alias-match",
		Status:       "active",
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/keys/active", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "test@example.com")

	if err := h.GetActiveKeys(c); err != nil {
		t.Fatal(err)
	}

	// Check DB updated
	var key models.KeyHistory
	db.Where("key_name = ?", "alias-match").First(&key)
	if key.LiteLLMKeyID != "sk-new-id" {
		t.Errorf("Expected updated ID sk-new-id, got %s", key.LiteLLMKeyID)
	}
}

func TestDeleteKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/key/delete" {
			w.WriteHeader(http.StatusOK)
			return
		}
	}))
	defer server.Close()

	svc := services.NewLiteLLMService()
	svc.BaseURL = server.URL
	db := setupTestDB(t)
	h := NewHandler(svc, db)

	// Seed DB
	db.Create(&models.KeyHistory{
		UserID:       "test@example.com",
		LiteLLMKeyID: "sk-delete",
		Status:       "active",
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/keys/sk-delete", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/keys/:key_id")
	c.SetParamNames("key_id")
	c.SetParamValues("sk-delete")
	c.Set("user_id", "test@example.com")

	if err := h.DeleteKey(c); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	var key models.KeyHistory
	db.Where("litellm_key_id = ?", "sk-delete").First(&key)
	if key.Status != "revoked" {
		t.Errorf("Expected status revoked, got %s", key.Status)
	}
}

func TestGetKeyHistory(t *testing.T) {
	db := setupTestDB(t)
	h := NewHandler(nil, db)

	// Seed
	db.Create(&models.KeyHistory{
		UserID:       "test@example.com",
		LiteLLMKeyID: "sk-revoked",
		Status:       "revoked",
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/keys/history", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "test@example.com")

	if err := h.GetKeyHistory(c); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	var resp []models.KeyHistory
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp) != 1 {
		t.Errorf("Expected 1 history item, got %d", len(resp))
	}
}

func TestCreateKeyLimits(t *testing.T) {
	// Mock
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/key/list" {
			w.WriteHeader(http.StatusOK)
			// Return max active keys
			keys := make([]map[string]interface{}, 10)
			for i := 0; i < 10; i++ {
				keys[i] = map[string]interface{}{"key": "k", "user_id": "test@example.com"}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"keys": keys})
			return
		}
	}))
	defer server.Close()

	config.AppConfig = &config.Config{MaxActiveKeys: 10}
	svc := services.NewLiteLLMService()
	svc.BaseURL = server.URL
	db := setupTestDB(t)
	h := NewHandler(svc, db)

	e := echo.New()
	body := `{"name": "test-key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "test@example.com")

	_ = h.CreateKey(c)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for limit reached, got %d", rec.Code)
	}
}

func TestGetActiveKeysError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	svc := services.NewLiteLLMService()
	svc.BaseURL = server.URL
	db := setupTestDB(t)
	h := NewHandler(svc, db)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/keys/active", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "test@example.com")

	_ = h.GetActiveKeys(c)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected 503, got %d", rec.Code)
	}
}

func TestCreateLongTermKeyLimit(t *testing.T) {
	// Mock
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/key/list" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"keys": []}`))
			return
		}
	}))
	defer server.Close()

	config.AppConfig = &config.Config{
		MaxActiveKeys:    10,
		LongTermKeyLimit: 1,
	}
	svc := services.NewLiteLLMService()
	svc.BaseURL = server.URL
	db := setupTestDB(t)
	h := NewHandler(svc, db)

	// Seed 1 active long term key
	db.Create(&models.KeyHistory{
		UserID:  "test@example.com",
		KeyType: "long-term",
		Status:  "active",
	})

	e := echo.New()
	body := `{"name": "test-key", "type": "long-term"}`
	req := httptest.NewRequest(http.MethodPost, "/api/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "test@example.com")

	_ = h.CreateKey(c)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for long-term limit reached, got %d", rec.Code)
	}
}

func TestSyncRevokedKeys(t *testing.T) {
	// Mock LiteLLM returning empty list
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/key/list" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"keys": []}`))
			return
		}
	}))
	defer server.Close()

	svc := services.NewLiteLLMService()
	svc.BaseURL = server.URL
	db := setupTestDB(t)
	h := NewHandler(svc, db)

	// Seed DB with active key that is NOT in LiteLLM
	db.Create(&models.KeyHistory{
		UserID:       "test@example.com",
		LiteLLMKeyID: "sk-missing",
		Status:       "active",
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/keys/active", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "test@example.com")

	if err := h.GetActiveKeys(c); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	// Check DB updated to revoked
	var key models.KeyHistory
	db.Where("litellm_key_id = ?", "sk-missing").First(&key)
	if key.Status != "revoked" {
		t.Errorf("Expected status revoked, got %s", key.Status)
	}
}
