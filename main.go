package main

import (
	"log"
	"net/http"

	"github.com/example/llmreq/config"
	"github.com/example/llmreq/docs"
	"github.com/example/llmreq/handlers"
	"github.com/example/llmreq/middleware"
	"github.com/example/llmreq/models"
	"github.com/example/llmreq/services"
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
)

// @title LLM Request Manager API
// @version 1.0
// @description API for managing LiteLLM keys and user budgets.
// @host localhost:8080
// @BasePath /api

//go:generate swag init
func main() {
	// 1. Load Config
	config.LoadConfig()

	// 2. Initialize DB
	models.InitDB(config.AppConfig.DatabaseURL)

	// 3. Initialize Services
	litellmService := services.NewLiteLLMService()

	// 4. Initialize Handlers
	h := handlers.NewHandler(litellmService, models.DB)

	// 5. Setup Echo
	e := echo.New()

	// Middleware
	e.Use(echoMiddleware.RequestLoggerWithConfig(echoMiddleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogValuesFunc: func(c echo.Context, v echoMiddleware.RequestLoggerValues) error {
			log.Printf("REQUEST: uri=%s status=%v", v.URI, v.Status)
			return nil
		},
	}))
	e.Use(echoMiddleware.Recover())

	// Custom Auth Middleware
	authMiddleware := middleware.NewAuthMiddleware(litellmService)

	// 6. Routes
	// Serve OpenAPI spec
	e.GET("/openapi.json", func(c echo.Context) error {
		docs.SwaggerInfo.Host = c.Request().Host
		return c.JSON(http.StatusOK, docs.SwaggerInfo)
	})

	api := e.Group(config.AppConfig.Prefix)
	api.Use(authMiddleware.Middleware)

	api.GET("/me", h.GetMe)
	api.GET("/keys/active", h.GetActiveKeys)
	api.GET("/keys/history", h.GetKeyHistory)
	api.POST("/keys", h.CreateKey)
	api.DELETE("/keys/:key_id", h.DeleteKey)

	// 7. Start Server
	// Spec says "Environment: Dockerized". Port typically 8080 or 3000.
	// I'll use 8080 as default.
	log.Println("Starting server on :8080")
	if err := e.Start(":8080"); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
