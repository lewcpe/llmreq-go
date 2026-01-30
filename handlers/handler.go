package handlers

import (
	"github.com/example/llmreq/services"
	"gorm.io/gorm"
)

type Handler struct {
	LiteLLMService *services.LiteLLMService
	DB             *gorm.DB
}

func NewHandler(service *services.LiteLLMService, db *gorm.DB) *Handler {
	return &Handler{
		LiteLLMService: service,
		DB:             db,
	}
}
