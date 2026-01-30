package middleware

import (
	"log"
	"net/http"
	"strings"

	"github.com/example/llmreq/config"
	"github.com/example/llmreq/services"
	"github.com/labstack/echo/v4"
)

type AuthMiddleware struct {
	LiteLLMService *services.LiteLLMService
}

func NewAuthMiddleware(service *services.LiteLLMService) *AuthMiddleware {
	return &AuthMiddleware{
		LiteLLMService: service,
	}
}

func (m *AuthMiddleware) Middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		email := c.Request().Header.Get("X-Forwarded-Email")
		if email == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized: Missing X-Forwarded-Email header"})
		}

		// Normalize email
		userID := strings.ToLower(email)
		c.Set("user_id", userID)

		// JIT Provisioning
		// Note: Doing this on *every* request might be slow if LiteLLM is slow.
		// But spec says "On every authenticated request".
		// We could cache this locally to improve performance, but sticking to spec first.

		user, err := m.LiteLLMService.GetUserInfo(userID)
		if err != nil {
			// If error, it might be that LiteLLM is down or returned error.
			// But if it's 404 (user not found), GetUserInfo returns nil, nil.
            // If it returns error, it's a real error.
            log.Printf("Error checking user info: %v", err)
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "LiteLLM service unavailable"})
		}

		if user == nil {
			// User does not exist, create it
			err := m.LiteLLMService.CreateUser(userID, userID, config.AppConfig.DefaultBudget)
			if err != nil {
                log.Printf("Error creating user: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to provision user"})
			}
		}

		return next(c)
	}
}
