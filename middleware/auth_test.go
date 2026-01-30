package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/llmreq/config"
	"github.com/example/llmreq/services"
	"github.com/labstack/echo/v4"
)

func TestAuthMiddleware(t *testing.T) {
	// Mock LiteLLM
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/user/info/test@example.com") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(services.LiteLLMUser{UserID: "test@example.com"})
			return
		}
		if strings.Contains(r.URL.Path, "/user/info/new@example.com") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Path == "/user/new" {
			w.WriteHeader(http.StatusOK)
			return
		}
        w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	config.AppConfig = &config.Config{
		DefaultBudget: 1.0,
	}
	svc := services.NewLiteLLMService()
	svc.BaseURL = server.URL

	m := NewAuthMiddleware(svc)
	e := echo.New()

	// 1. Missing Header
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := m.Middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})(c)

	if err == nil {
        if rec.Code != http.StatusUnauthorized {
             t.Errorf("Expected 401, got %d", rec.Code)
        }
	}

	// 2. Existing User
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Email", "test@example.com")
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	err = m.Middleware(func(c echo.Context) error {
		if c.Get("user_id") != "test@example.com" {
			t.Error("user_id not set")
		}
		return c.String(http.StatusOK, "ok")
	})(c)
    if err != nil {
        t.Fatal(err)
    }
    if rec.Code != http.StatusOK {
        t.Errorf("Expected 200, got %d", rec.Code)
    }

	// 3. New User (JIT)
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Email", "New@example.com") // Mixed case
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	err = m.Middleware(func(c echo.Context) error {
		if c.Get("user_id") != "new@example.com" {
			t.Error("user_id not normalized")
		}
		return c.String(http.StatusOK, "ok")
	})(c)
     if err != nil {
        t.Fatal(err)
    }
     if rec.Code != http.StatusOK {
        t.Errorf("Expected 200, got %d", rec.Code)
    }
}
