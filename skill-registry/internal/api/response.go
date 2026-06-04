package api

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error information
type ErrorDetail struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

// WriteJSON writes a JSON response
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// ParseJSON parses JSON from request body
func ParseJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// WriteError writes an error response
func WriteError(w http.ResponseWriter, status int, code, message string, details ...string) {
	resp := ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
	WriteJSON(w, status, resp)
}

// RegistryMetadata represents registry metadata
type RegistryMetadata struct {
	Name                 string   `json:"name"`
	Version              string   `json:"version"`
	SupportedPackageTypes []string `json:"supported_package_types"`
	AuthMode             string   `json:"auth_mode"`
	Capabilities         []string `json:"capabilities"`
}
