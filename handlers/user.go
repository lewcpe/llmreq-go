package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (h *Handler) GetMe(c echo.Context) error {
	userID := c.Get("user_id").(string)

	user, err := h.LiteLLMService.GetUserInfo(userID)
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "LiteLLM unavailable"})
	}
	if user == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
	}

	return c.JSON(http.StatusOK, user)
}
