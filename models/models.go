package models

import (
	"time"
)

type KeyHistory struct {
	ID           uint   `gorm:"primaryKey"`
	UserID       string `gorm:"index"`
	LiteLLMKeyID string `gorm:"column:litellm_key_id"`
	KeyName      string
	KeyMask      string
	KeyType      string
	CreatedAt    time.Time
	ExpiresAt    *time.Time
	RevokedAt    *time.Time
	Status       string
}
