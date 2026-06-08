package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/skillforge/skill-registry/internal/metadata"
)

type ArtifactListResponse struct {
	Artifacts []metadata.Artifact `json:"artifacts"`
	Total     int                 `json:"total"`
	Limit     int                 `json:"limit"`
	Offset    int                 `json:"offset"`
}

type ArtifactDetailResponse struct {
	Artifact metadata.Artifact          `json:"artifact"`
	Versions []metadata.ArtifactVersion `json:"versions"`
	DistTags map[string]string          `json:"dist_tags"`
}

func (c *Client) PublishArtifact(kind, namespace, name, version string, data []byte, contentType string) (*metadata.ArtifactVersion, error) {
	path := fmt.Sprintf("/api/v1/artifacts/%s/%s/%s/versions/%s", kind, namespace, name, version)
	resp, err := c.doRequest(http.MethodPut, path, bytes.NewReader(data), map[string]string{"Content-Type": contentType})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("publish artifact failed: %s", body)
	}
	var result struct {
		Version metadata.ArtifactVersion `json:"version"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	return &result.Version, err
}

func (c *Client) ListArtifacts(kind, query string) (*ArtifactListResponse, error) {
	values := url.Values{}
	if kind != "" {
		values.Set("kind", kind)
	}
	if query != "" {
		values.Set("q", query)
	}
	resp, err := c.doRequest(http.MethodGet, "/api/v1/artifacts?"+values.Encode(), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list artifacts failed: %s", body)
	}
	var result ArtifactListResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	return &result, err
}

func (c *Client) GetArtifact(kind, namespace, name string) (*ArtifactDetailResponse, error) {
	path := fmt.Sprintf("/api/v1/artifacts/%s/%s/%s", kind, namespace, name)
	resp, err := c.doRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get artifact failed: %s", body)
	}
	var result ArtifactDetailResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	return &result, err
}

func (c *Client) ArtifactGraph(kind, namespace, name, version string) (*metadata.ArtifactGraph, error) {
	path := fmt.Sprintf("/api/v1/artifacts/%s/%s/%s/versions/%s/graph", kind, namespace, name, version)
	resp, err := c.doRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var graph metadata.ArtifactGraph
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("artifact graph failed: %s", body)
	}
	err = json.NewDecoder(resp.Body).Decode(&graph)
	return &graph, err
}

func (c *Client) ArtifactLockfile(kind, namespace, name, version string) (*metadata.ArtifactLockfile, error) {
	path := fmt.Sprintf("/api/v1/artifacts/%s/%s/%s/versions/%s/lockfile", kind, namespace, name, version)
	resp, err := c.doRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var lock metadata.ArtifactLockfile
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("artifact lockfile failed: %s", body)
	}
	err = json.NewDecoder(resp.Body).Decode(&lock)
	return &lock, err
}

func (c *Client) PromoteArtifact(kind, namespace, name, version, channel string) error {
	data, _ := json.Marshal(map[string]string{"version": version, "channel": channel})
	path := fmt.Sprintf("/api/v1/artifacts/%s/%s/%s/promotions", kind, namespace, name)
	resp, err := c.doRequest(http.MethodPost, path, bytes.NewReader(data), map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("promotion failed: %s", body)
	}
	return nil
}

func (c *Client) DownloadArtifact(kind, namespace, name, version string) ([]byte, string, string, error) {
	path := fmt.Sprintf("/api/v1/artifacts/%s/%s/%s/versions/%s/download", kind, namespace, name, version)
	resp, err := c.doRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", "", fmt.Errorf("download artifact failed: %s", body)
	}
	data, err := io.ReadAll(resp.Body)
	return data, resp.Header.Get("X-Artifact-Digest-SHA256"), resp.Header.Get("Content-Type"), err
}

func (c *Client) CreateArtifactAttestation(kind, namespace, name, version, attestationType, digest string) error {
	data, _ := json.Marshal(map[string]interface{}{"type": attestationType, "digest": digest})
	path := fmt.Sprintf("/api/v1/artifacts/%s/%s/%s/versions/%s/attestations", kind, namespace, name, version)
	resp, err := c.doRequest(http.MethodPost, path, bytes.NewReader(data), map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("attestation failed: %s", body)
	}
	return nil
}

func (c *Client) UpsertNamespaceMember(namespace, username, role string) error {
	data, _ := json.Marshal(map[string]string{"username": username, "role": role})
	resp, err := c.doRequest(http.MethodPut, "/api/v1/namespaces/"+namespace+"/members", bytes.NewReader(data), map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("namespace member failed: %s", body)
	}
	return nil
}

func (c *Client) CreateWebhook(namespace, webhookURL string, events []string) error {
	data, _ := json.Marshal(map[string]interface{}{"url": webhookURL, "events": events})
	resp, err := c.doRequest(http.MethodPost, "/api/v1/namespaces/"+namespace+"/webhooks", bytes.NewReader(data), map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook failed: %s", body)
	}
	return nil
}
