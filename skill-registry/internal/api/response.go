package api

import (
	"encoding/json"
	"net/http"
)

// errorDetail is the legacy nested error object kept for backward compatibility.
type errorDetail struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

// errorResponse is the wire format for all error responses.
// Top-level "error" (string) and "code" follow registryapi.ErrorResponse.
// The nested "detail" object preserves the previous format for existing clients.
type errorResponse struct {
	Error  string      `json:"error"`            // registryapi-compatible flat message
	Code   string      `json:"code,omitempty"`   // registryapi-compatible flat code
	Detail errorDetail `json:"detail"`           // legacy nested object
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

// WriteError writes an error response in the merged format:
//
//	{"error":"<message>","code":"<code>","detail":{"code":"<code>","message":"<message>"}}
func WriteError(w http.ResponseWriter, status int, code, message string, details ...string) {
	WriteJSON(w, status, errorResponse{
		Error: message,
		Code:  code,
		Detail: errorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

// RegistryMetadata represents registry metadata
type RegistryMetadata struct {
	Name                  string   `json:"name"`
	Version               string   `json:"version"`
	SupportedPackageTypes []string `json:"supported_package_types"`
	AuthMode              string   `json:"auth_mode"`
	Capabilities          []string `json:"capabilities"`
}
