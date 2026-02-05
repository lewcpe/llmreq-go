package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/example/llmreq/config"
	"github.com/example/llmreq/models"
	"github.com/example/llmreq/services"
	"github.com/labstack/echo/v4"
)

type CreateKeyRequest struct {
	Name   string  `json:"name"`
	Budget float64 `json:"budget"`
	Type   string  `json:"type"` // "standard" or "long-term"
}

type ActiveKeyResponse struct {
	Mask      string     `json:"mask"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
	Spend     float64    `json:"spend"`
	Type      string     `json:"type"`
	KeyID     string     `json:"key_id"`
}

// GetActiveKeys godoc
// @Summary Get active keys
// @Description Fetch active keys from LiteLLM and sync with local DB
// @Tags keys
// @Accept json
// @Produce json
// @Success 200 {array} ActiveKeyResponse
// @Failure 500 {object} map[string]string
// @Router /keys/active [get]
func (h *Handler) GetActiveKeys(c echo.Context) error {
	userID := c.Get("user_id").(string)

	// Fetch DB keys first
	var dbKeys []models.KeyHistory
	if err := h.DB.Where("user_id = ?", userID).Find(&dbKeys).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch local keys"})
	}
	dbKeyMap := make(map[string]*models.KeyHistory)
	for i := range dbKeys {
		dbKeyMap[dbKeys[i].LiteLLMKeyID] = &dbKeys[i]
	}

	keys, err := h.LiteLLMService.ListKeys(userID)
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Failed to fetch keys from LiteLLM"})
	}

	responseKeys := []ActiveKeyResponse{}
	processedDBIDs := make(map[uint]struct{})

	for _, k := range keys {
		// Strict user check
		if k.User != userID {
			continue
		}

		id := k.Key // Usually masked key

		// Parse expiration
		var expiresAt *time.Time
		if k.Expires != "" && k.Expires != "null" {
			if t, err := time.Parse(time.RFC3339, k.Expires); err == nil {
				expiresAt = &t
			}
		}

		isExpired := expiresAt != nil && expiresAt.Before(time.Now())

		dbKey, exists := dbKeyMap[id]
		if exists {
			// Found in DB. Ensure active.
			processedDBIDs[dbKey.ID] = struct{}{}

			dbKey.ExpiresAt = expiresAt
			if isExpired {
				if dbKey.Status != "expired" {
					dbKey.Status = "expired"
					h.DB.Save(dbKey)
				}
			} else {
				if dbKey.Status != "active" {
					dbKey.Status = "active"
					dbKey.RevokedAt = nil
					h.DB.Save(dbKey)
				}
				responseKeys = append(responseKeys, ActiveKeyResponse{
					Mask:      dbKey.KeyMask,
					Name:      dbKey.KeyName,
					CreatedAt: dbKey.CreatedAt,
					ExpiresAt: dbKey.ExpiresAt,
					Spend:     k.Spend,
					Type:      dbKey.KeyType,
					KeyID:     dbKey.LiteLLMKeyID,
				})
			}
		} else {
			// Not in DB. Try to match by alias?
			// If alias matches a dbKey, update its ID?
			// This handles re-sync if ID changed or we guessed wrong.
			var matchedByAlias *models.KeyHistory
			for i := range dbKeys {
				if dbKeys[i].KeyName == k.KeyAlias && dbKeys[i].LiteLLMKeyID != id {
					matchedByAlias = &dbKeys[i]
					break
				}
			}

			if matchedByAlias != nil {
				processedDBIDs[matchedByAlias.ID] = struct{}{}

				// Update ID
				matchedByAlias.LiteLLMKeyID = id
				matchedByAlias.ExpiresAt = expiresAt

				if isExpired {
					if matchedByAlias.Status != "expired" {
						matchedByAlias.Status = "expired"
						h.DB.Save(matchedByAlias)
					}
				} else {
					matchedByAlias.Status = "active"
					matchedByAlias.RevokedAt = nil
					h.DB.Save(matchedByAlias)

					responseKeys = append(responseKeys, ActiveKeyResponse{
						Mask:      matchedByAlias.KeyMask,
						Name:      matchedByAlias.KeyName,
						CreatedAt: matchedByAlias.CreatedAt,
						ExpiresAt: matchedByAlias.ExpiresAt,
						Spend:     k.Spend,
						Type:      matchedByAlias.KeyType,
						KeyID:     matchedByAlias.LiteLLMKeyID,
					})
				}
			} else {
				// Create new
				newKey := models.KeyHistory{
					UserID:       userID,
					LiteLLMKeyID: id,
					KeyName:      k.KeyAlias,
					KeyMask:      k.Key,
					KeyType:      "standard",
					CreatedAt:    time.Now(),
					ExpiresAt:    expiresAt,
					Status:       "active",
				}
				if isExpired {
					newKey.Status = "expired"
				}
				h.DB.Create(&newKey)

				if !isExpired {
					responseKeys = append(responseKeys, ActiveKeyResponse{
						Mask:      newKey.KeyMask,
						Name:      newKey.KeyName,
						CreatedAt: newKey.CreatedAt,
						ExpiresAt: newKey.ExpiresAt,
						Spend:     k.Spend,
						Type:      newKey.KeyType,
						KeyID:     newKey.LiteLLMKeyID,
					})
				}
			}
		}
	}

	// Revoke keys not in LiteLLM list (and not matched/processed)
	for _, dbKey := range dbKeys {
		if _, ok := processedDBIDs[dbKey.ID]; !ok {
			if dbKey.Status == "active" {
				dbKey.Status = "revoked"
				now := time.Now()
				dbKey.RevokedAt = &now
				h.DB.Save(&dbKey)
			}
		}
	}

	return c.JSON(http.StatusOK, responseKeys)
}

// GetKeyHistory godoc
// @Summary Get key history
// @Description Fetch revoked or deleted keys from local DB
// @Tags keys
// @Accept json
// @Produce json
// @Success 200 {array} models.KeyHistory
// @Router /keys/history [get]
func (h *Handler) GetKeyHistory(c echo.Context) error {
	userID := c.Get("user_id").(string)

	var history []models.KeyHistory
	h.DB.Where("user_id = ? AND (status = ? OR revoked_at IS NOT NULL)", userID, "revoked").Find(&history)

	return c.JSON(http.StatusOK, history)
}

// CreateKey godoc
// @Summary Create a new API key
// @Description Create a new API key with optional budget and type
// @Tags keys
// @Accept json
// @Produce json
// @Param request body CreateKeyRequest true "Create Key Request"
// @Success 200 {object} services.GenerateKeyResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /keys [post]
func (h *Handler) CreateKey(c echo.Context) error {
	userID := c.Get("user_id").(string)
	var req CreateKeyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.Type == "" {
		req.Type = "standard"
	}

	// Check Global Limit
	// Use LiteLLM list to count active keys
	activeKeys, err := h.LiteLLMService.ListKeys(userID)
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Failed to fetch key count"})
	}

	userActiveKeyCount := 0
	for _, k := range activeKeys {
		if k.User == userID {
			// Check expiration
			if k.Expires != "" && k.Expires != "null" {
				if t, err := time.Parse(time.RFC3339, k.Expires); err == nil {
					if t.Before(time.Now()) {
						continue // Expired
					}
				}
			}
			userActiveKeyCount++
		}
	}

	if userActiveKeyCount >= config.AppConfig.MaxActiveKeys {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Max active keys limit reached"})
	}

	// Check Long-term Limit
	if req.Type == "long-term" {
		var count int64
		h.DB.Model(&models.KeyHistory{}).Where("user_id = ? AND key_type = ? AND status = ?", userID, "long-term", "active").Count(&count)
		if int(count) >= config.AppConfig.LongTermKeyLimit {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Long-term key limit reached"})
		}
	}

	// Determine Budget and Duration
	var maxBudget float64
	var duration string

	if req.Type == "long-term" {
		maxBudget = config.AppConfig.LongTermKeyBudget
		if req.Budget > 0 && req.Budget < maxBudget {
			maxBudget = req.Budget
		}
		duration = config.AppConfig.LongTermKeyLifetime.String()
	} else {
		maxBudget = config.AppConfig.DefaultBudget
		if req.Budget > 0 && req.Budget < maxBudget {
			maxBudget = req.Budget
		}
		duration = config.AppConfig.StandardKeyLifetime.String()
	}

	// Call LiteLLM
	genReq := services.GenerateKeyRequest{
		UserID:    userID,
		KeyAlias:  req.Name,
		MaxBudget: maxBudget,
		Duration:  duration,
	}

	genResp, err := h.LiteLLMService.GenerateKey(genReq)
	if err != nil {
		log.Printf("Failed to generate key: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate key"})
	}

	// Fetch keys to find the correct ID (sync immediately)
	keys, err := h.LiteLLMService.ListKeys(userID)
	var correctID string
	var mask string

	// Default mask if ListKeys fails
	mask = genResp.Key
	if len(mask) > 8 {
		mask = mask[:4] + "..." + mask[len(mask)-4:]
	}
	correctID = mask

	if err == nil {
		for _, k := range keys {
			// Match by alias
			if k.KeyAlias == req.Name {
				correctID = k.Key
				mask = k.Key
				break
			}
		}
	}

	newKey := models.KeyHistory{
		UserID:       userID,
		LiteLLMKeyID: correctID,
		KeyName:      req.Name,
		KeyMask:      mask,
		KeyType:      req.Type,
		CreatedAt:    time.Now(),
		Status:       "active",
	}

	h.DB.Create(&newKey)

	return c.JSON(http.StatusOK, genResp)
}

// DeleteKey godoc
// @Summary Delete an API key
// @Description Revoke an API key
// @Tags keys
// @Accept json
// @Produce json
// @Param key_id path string true "Key ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /keys/{key_id} [delete]
func (h *Handler) DeleteKey(c echo.Context) error {
	keyID := c.Param("key_id")
	userID := c.Get("user_id").(string)

	// Verify ownership
	var dbKey models.KeyHistory
	if err := h.DB.Where("user_id = ? AND litellm_key_id = ?", userID, keyID).First(&dbKey).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Key not found"})
	}

	if err := h.LiteLLMService.DeleteKey(keyID); err != nil {
		log.Printf("Failed to delete key in LiteLLM: %v", err)
	}

	dbKey.Status = "revoked"
	now := time.Now()
	dbKey.RevokedAt = &now
	h.DB.Save(&dbKey)

	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}
