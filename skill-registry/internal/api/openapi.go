package api

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var openapiSpec []byte

func (h *Handler) GetOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	if h.config != nil && h.config.Security.OpenAPICORSOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", h.config.Security.OpenAPICORSOrigin)
	}
	w.WriteHeader(200)
	_, _ = w.Write(openapiSpec)
}

func (h *Handler) GetAPIDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	_, _ = w.Write([]byte(redocHTML))
}

const redocHTML = `<!DOCTYPE html>
<html>
<head>
  <title>SkillForge Registry — API Reference</title>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link href="https://fonts.googleapis.com/css?family=Montserrat:300,400,700|Roboto:300,400,700" rel="stylesheet">
  <style>body { margin: 0; padding: 0; }</style>
</head>
<body>
  <redoc spec-url="/api/v1/openapi.yaml"
         expand-responses="200,201"
         hide-download-button
         theme='{"colors":{"primary":{"main":"#6366f1"}},"typography":{"fontFamily":"Roboto,sans-serif","headings":{"fontFamily":"Montserrat,sans-serif"}}}'
  ></redoc>
  <script src="https://cdn.jsdelivr.net/npm/redoc@2.1.3/bundles/redoc.standalone.js"></script>
</body>
</html>`
