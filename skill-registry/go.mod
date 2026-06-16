module github.com/skillforge/skill-registry

go 1.25.0

require (
	github.com/domehahn/sklib v0.0.0
	github.com/go-chi/chi/v5 v5.3.0
	github.com/lib/pq v1.12.3
	github.com/mattn/go-sqlite3 v1.14.45
	gopkg.in/yaml.v3 v3.0.1
)

require golang.org/x/crypto v0.52.0 // indirect

// In workspace mode go.work takes precedence; this fallback enables Docker builds with GOWORK=off.
replace github.com/domehahn/sklib v0.0.0 => ../sklib
