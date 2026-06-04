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
