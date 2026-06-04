package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/skillforge/skill-registry/internal/metadata"
	"github.com/skillforge/skill-registry/internal/validation"
)

// Client is an HTTP client for the skill registry
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a new registry client
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) doRequest(method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	url := c.baseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return c.httpClient.Do(req)
}

// Publish publishes a skill package
func (c *Client) Publish(namespace, name, version string, data []byte, contentType string) (*metadata.SkillVersion, error) {
	path := fmt.Sprintf("/api/v1/skills/%s/%s/versions/%s", namespace, name, version)
	headers := map[string]string{
		"Content-Type": contentType,
	}

	resp, err := c.doRequest("PUT", path, bytes.NewReader(data), headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("publish failed: %s (status %d)", string(body), resp.StatusCode)
	}

	var skillVersion metadata.SkillVersion
	if err := json.NewDecoder(resp.Body).Decode(&skillVersion); err != nil {
		return nil, err
	}

	return &skillVersion, nil
}

// ListSkills lists skills
func (c *Client) ListSkills(query string) (*metadata.SkillList, error) {
	path := "/api/v1/skills"
	if query != "" {
		path += "?q=" + query
	}

	resp, err := c.doRequest("GET", path, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list failed: %s", string(body))
	}

	var skillList metadata.SkillList
	if err := json.NewDecoder(resp.Body).Decode(&skillList); err != nil {
		return nil, err
	}

	return &skillList, nil
}

// GetSkill retrieves a skill with its versions
func (c *Client) GetSkill(namespace, name string) (*metadata.Skill, []metadata.SkillVersion, error) {
	path := fmt.Sprintf("/api/v1/skills/%s/%s", namespace, name)

	resp, err := c.doRequest("GET", path, nil, nil)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("get skill failed: %s", string(body))
	}

	var result struct {
		Skill    metadata.Skill          `json:"skill"`
		Versions []metadata.SkillVersion `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, err
	}

	return &result.Skill, result.Versions, nil
}

// Download downloads a skill package
func (c *Client) Download(namespace, name, version string) ([]byte, string, error) {
	path := fmt.Sprintf("/api/v1/skills/%s/%s/versions/%s/download", namespace, name, version)

	resp, err := c.doRequest("GET", path, nil, nil)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("download failed: %s", string(body))
	}

	digest := resp.Header.Get("X-Skill-Digest-SHA256")
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	return data, digest, nil
}

// Validate validates a skill package
func (c *Client) Validate(data []byte, contentType string) (*validation.ValidationResult, error) {
	path := "/api/v1/validate"
	headers := map[string]string{
		"Content-Type": contentType,
	}

	resp, err := c.doRequest("POST", path, bytes.NewReader(data), headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result validation.ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// LoginResponse represents the login API response
type LoginResponse struct {
	Token     string    `json:"token"`
	User      string    `json:"user"`
	Role      string    `json:"role"`
	Scopes    []string  `json:"scopes"`
	ExpiresAt *string   `json:"expires_at,omitempty"`
}

// TokenResponse represents a token
type TokenResponse struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Name      string    `json:"name"`
	Token     string    `json:"token,omitempty"`
	Scopes    []string  `json:"scopes"`
	CreatedAt string    `json:"created_at"`
	ExpiresAt *string   `json:"expires_at,omitempty"`
	RevokedAt *string   `json:"revoked_at,omitempty"`
}

// Login authenticates with username and password
func (c *Client) Login(username, password string) (*LoginResponse, error) {
	reqBody := map[string]string{
		"username": username,
		"password": password,
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	resp, err := c.doRequest("POST", "/api/v1/auth/login", bytes.NewReader(jsonData), headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("login failed: %s (status %d)", string(body), resp.StatusCode)
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return nil, err
	}

	// Update client token
	c.token = loginResp.Token

	return &loginResp, nil
}

// CreateToken creates a new API token
func (c *Client) CreateToken(name string, scopes []string, expiresIn *int64) (*TokenResponse, error) {
	reqBody := map[string]interface{}{
		"name":   name,
		"scopes": scopes,
	}
	if expiresIn != nil {
		reqBody["expires_in"] = *expiresIn
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	resp, err := c.doRequest("POST", "/api/v1/tokens", bytes.NewReader(jsonData), headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create token failed: %s (status %d)", string(body), resp.StatusCode)
	}

	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	return &token, nil
}

// ListTokens lists all tokens for the authenticated user
func (c *Client) ListTokens() ([]TokenResponse, error) {
	resp, err := c.doRequest("GET", "/api/v1/tokens", nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list tokens failed: %s", string(body))
	}

	var result struct {
		Tokens []TokenResponse `json:"tokens"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Tokens, nil
}

// RevokeToken revokes a token by ID
func (c *Client) RevokeToken(tokenID int64) error {
	path := fmt.Sprintf("/api/v1/tokens/%d", tokenID)
	
	resp, err := c.doRequest("DELETE", path, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("revoke token failed: %s", string(body))
	}

	return nil
}
