package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// GetMe godoc
// @Summary Get current user info
// @Description Fetch current user information from LiteLLM
// @Tags user
// @Accept json
// @Produce json
// @Success 200 {object} services.LiteLLMUser
// @Failure 404 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /me [get]
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
