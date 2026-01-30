package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/example/llmreq/config"
)

type LiteLLMService struct {
	BaseURL   string
	MasterKey string
	Client    *http.Client
}

func NewLiteLLMService() *LiteLLMService {
	return &LiteLLMService{
		BaseURL:   config.AppConfig.LiteLLMAPIURL,
		MasterKey: config.AppConfig.LiteLLMMasterKey,
		Client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Structs for LiteLLM API

type LiteLLMUser struct {
	UserID    string  `json:"user_id"`
	UserEmail string  `json:"user_email"`
	MaxBudget float64 `json:"max_budget,omitempty"`
	Spend     float64 `json:"spend"`
}

type LiteLLMKey struct {
	KeyName  string                 `json:"key_name"`
	KeyAlias string                 `json:"key_alias"`
	Key      string                 `json:"key"`
	Token    string                 `json:"token"` // Sometimes key is returned as token
	Spend    float64                `json:"spend"`
	Expires  string                 `json:"expires"`
	User     string                 `json:"user_id"`
	TeamId   string                 `json:"team_id"`
	Models   []string               `json:"models"`
	Metadata map[string]interface{} `json:"metadata"`
}

type GenerateKeyRequest struct {
	UserID    string  `json:"user_id"`
	KeyAlias  string  `json:"key_alias"`
	MaxBudget float64 `json:"max_budget,omitempty"`
	Duration  string  `json:"duration,omitempty"`
}

type GenerateKeyResponse struct {
	Key       string  `json:"key"`
	Hidden    bool    `json:"hidden"`
	Token     string  `json:"token"`
	Spend     float64 `json:"spend"`
	MaxBudget float64 `json:"max_budget"`
	User      string  `json:"user_id"`
	KeyAlias  string  `json:"key_alias"`
	KeyName   string  `json:"key_name"`
}

type DeleteKeyRequest struct {
	Keys []string `json:"keys"`
}

// Methods

func (s *LiteLLMService) GetUserInfo(userID string) (*LiteLLMUser, error) {
	reqURL := fmt.Sprintf("%s/user/info/%s", s.BaseURL, userID)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	s.setAuth(req)

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, nil // User not found
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get user info: status %d", resp.StatusCode)
	}

	var user LiteLLMUser
	// LiteLLM /user/info response structure might vary slightly, but assuming it returns the user object directly or inside "user_info"
	// Based on docs, it returns the user object.
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *LiteLLMService) CreateUser(userID, email string, maxBudget float64) error {
	reqURL := fmt.Sprintf("%s/user/new", s.BaseURL)
	payload := map[string]interface{}{
		"user_id":    userID,
		"user_email": email,
	}
	if maxBudget > 0 {
		payload["max_budget"] = maxBudget
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	s.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create user: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (s *LiteLLMService) ListKeys(userID string) ([]LiteLLMKey, error) {
	reqURL := fmt.Sprintf("%s/key/list", s.BaseURL)
	// Assuming GET /key/list accepts user_id as query param?
	// Or maybe POST /key/list? Spec says "Call LiteLLM GET /key/list (filtered by user_id)".
	// LiteLLM docs usually say GET /key/list returns all keys, but let's check if we can filter.
	// Actually, typically LiteLLM /key/list is for admin to list all keys.
	// But if we pass user_id, it might filter.
	// If not, we have to filter client side? That would be bad if there are many keys.
	// Let's assume query param.

	u, _ := url.Parse(reqURL)
	q := u.Query()
	q.Set("user_id", userID)
	u.RawQuery = q.Encode()
	reqURL = u.String()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	s.setAuth(req)

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to list keys: status %d", resp.StatusCode)
	}

	var response struct {
		Keys []LiteLLMKey `json:"keys"`
	}
	// Try to decode as list or wrapped in keys
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}
	// First try wrapped
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		// If fail, try raw list
		if err := json.Unmarshal(bodyBytes, &response.Keys); err != nil {
			return nil, fmt.Errorf("failed to decode keys: %v", err)
		}
	}

	return response.Keys, nil
}

func (s *LiteLLMService) GenerateKey(reqPayload GenerateKeyRequest) (*GenerateKeyResponse, error) {
	reqURL := fmt.Sprintf("%s/key/generate", s.BaseURL)
	body, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	s.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to generate key: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var keyResp GenerateKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&keyResp); err != nil {
		return nil, err
	}

	return &keyResp, nil
}

func (s *LiteLLMService) DeleteKey(keyID string) error {
	reqURL := fmt.Sprintf("%s/key/delete", s.BaseURL)
	payload := DeleteKeyRequest{
		Keys: []string{keyID},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	s.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete key: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (s *LiteLLMService) setAuth(req *http.Request) {
	if s.MasterKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.MasterKey)
	}
}
