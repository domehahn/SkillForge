package metadata

import "time"

const (
	ArtifactKindSkill        = "skill"
	ArtifactKindAgent        = "agent"
	ArtifactKindFlow         = "flow"
	ArtifactKindPrompt       = "prompt"
	ArtifactKindTool         = "tool"
	ArtifactKindBundle       = "bundle"
	ArtifactKindMCP          = "mcp"
	ArtifactKindInstructions = "instructions"
	ArtifactKindHook         = "hook"
	ArtifactKindPlugin       = "plugin"
	ArtifactKindPolicy       = "policy"
	ArtifactKindContainer    = "container"
	ArtifactKindModelRuntime = "model-runtime"
	ArtifactKindModelProfile = "model-profile"
)

type Artifact struct {
	ID            int64     `json:"id"`
	Kind          string    `json:"kind"`
	Namespace     string    `json:"namespace"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Readme        string    `json:"readme,omitempty"`
	LatestVersion string    `json:"latest_version"`
	Visibility    string    `json:"visibility"`
	Tags          []string  `json:"tags"`
	Owners        []string  `json:"owners"`
	Downloads     int64     `json:"downloads"`
	Stars         int64     `json:"stars,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ArtifactVersion struct {
	ID               int64             `json:"id"`
	ArtifactID       int64             `json:"artifact_id"`
	Kind             string            `json:"kind"`
	Namespace        string            `json:"namespace"`
	Name             string            `json:"name"`
	Version          string            `json:"version"`
	DigestSHA256     string            `json:"digest_sha256"`
	PackageType      string            `json:"package_type"`
	SizeBytes        int64             `json:"size_bytes"`
	Entrypoint       string            `json:"entrypoint"`
	Manifest         *ArtifactManifest `json:"manifest"`
	Lockfile         *ArtifactLockfile `json:"lockfile,omitempty"`
	OCIDescriptor    *OCIDescriptor    `json:"oci_descriptor"`
	SignatureStatus  string            `json:"signature_status"`
	ScanStatus       string            `json:"scan_status"`
	ValidationStatus string            `json:"validation_status"`
	Source           string            `json:"source"`
	CreatedBy        string            `json:"created_by"`
	CreatedAt        time.Time         `json:"created_at"`
	Downloads        int64             `json:"downloads"`
	ReleaseNotes     string            `json:"release_notes,omitempty"`
}

type DayCount struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

type WebhookDelivery struct {
	ID          int64     `json:"id"`
	WebhookID   int64     `json:"webhook_id"`
	Namespace   string    `json:"namespace"`
	Event       string    `json:"event"`
	StatusCode  int       `json:"status_code"`
	DurationMs  int64     `json:"duration_ms"`
	Success     bool      `json:"success"`
	DeliveredAt time.Time `json:"delivered_at"`
}

type ArtifactManifest struct {
	APIVersion string           `json:"apiVersion" yaml:"apiVersion"`
	Kind       string           `json:"kind" yaml:"kind"`
	Metadata   ArtifactMetadata `json:"metadata" yaml:"metadata"`
	Spec       ArtifactSpec     `json:"spec" yaml:"spec"`
}

type ArtifactMetadata struct {
	Namespace   string   `json:"namespace" yaml:"namespace"`
	Name        string   `json:"name" yaml:"name"`
	Version     string   `json:"version" yaml:"version"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Visibility  string   `json:"visibility,omitempty" yaml:"visibility,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Owners      []string `json:"owners,omitempty" yaml:"owners,omitempty"`
}

type ArtifactSpec struct {
	Entrypoint    string                 `json:"entrypoint,omitempty" yaml:"entrypoint,omitempty"`
	Runtime       string                 `json:"runtime,omitempty" yaml:"runtime,omitempty"`
	Dependencies  []ArtifactDependency   `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Inputs        map[string]interface{} `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Outputs       map[string]interface{} `json:"outputs,omitempty" yaml:"outputs,omitempty"`
	Configuration map[string]interface{} `json:"configuration,omitempty" yaml:"configuration,omitempty"`
	Steps         []FlowStep             `json:"steps,omitempty" yaml:"steps,omitempty"`
	Agent         *AgentSpec             `json:"agent,omitempty" yaml:"agent,omitempty"`
	MCP           *MCPSpec               `json:"mcp,omitempty" yaml:"mcp,omitempty"`
	Instructions  *InstructionsSpec      `json:"instructions,omitempty" yaml:"instructions,omitempty"`
	Hook          *HookSpec              `json:"hook,omitempty" yaml:"hook,omitempty"`
	Plugin        *PluginSpec            `json:"plugin,omitempty" yaml:"plugin,omitempty"`
	Policy        *PolicySpec            `json:"policy,omitempty" yaml:"policy,omitempty"`
	Container     *ContainerSpec         `json:"container,omitempty" yaml:"container,omitempty"`
	ModelRuntime  *ModelRuntimeSpec      `json:"model_runtime,omitempty" yaml:"model_runtime,omitempty"`
	ModelProfile  *ModelProfileSpec      `json:"model_profile,omitempty" yaml:"model_profile,omitempty"`
}

type ArtifactDependency struct {
	Kind      string `json:"kind" yaml:"kind"`
	Namespace string `json:"namespace" yaml:"namespace"`
	Name      string `json:"name" yaml:"name"`
	Version   string `json:"version" yaml:"version"`
	Optional  bool   `json:"optional,omitempty" yaml:"optional,omitempty"`
}

// FlowStep represents one step in a flow DAG.
type FlowStep struct {
	ID     string            `json:"id" yaml:"id"`
	Uses   string            `json:"uses" yaml:"uses"`
	Needs  []string          `json:"needs,omitempty" yaml:"needs,omitempty"`
	Inputs map[string]string `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Output string            `json:"output,omitempty" yaml:"output,omitempty"`
	If     string            `json:"if,omitempty" yaml:"if,omitempty"`
}

// AgentSpec contains typed fields for agent artifacts.
type AgentSpec struct {
	Model        string             `json:"model,omitempty" yaml:"model,omitempty"`
	ModelFamily  string             `json:"model_family,omitempty" yaml:"model_family,omitempty"`
	SystemPrompt string             `json:"system_prompt,omitempty" yaml:"system_prompt,omitempty"`
	MaxSteps     int                `json:"max_steps,omitempty" yaml:"max_steps,omitempty"`
	Memory       *AgentMemoryConfig `json:"memory,omitempty" yaml:"memory,omitempty"`
	Tools        []string           `json:"tools,omitempty" yaml:"tools,omitempty"`
	Capabilities []string           `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
}

type AgentMemoryConfig struct {
	Type     string `json:"type" yaml:"type"` // none | buffer | vector | persistent
	MaxItems int    `json:"max_items,omitempty" yaml:"max_items,omitempty"`
	Backend  string `json:"backend,omitempty" yaml:"backend,omitempty"`
}

// MCPSpec contains fields for MCP server artifacts (Model Context Protocol).
type MCPSpec struct {
	Transport    string            `json:"transport" yaml:"transport"` // stdio | sse | http
	Command      []string          `json:"command,omitempty" yaml:"command,omitempty"`
	Args         []string          `json:"args,omitempty" yaml:"args,omitempty"`
	Env          map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	SecretRefs   map[string]string `json:"secret_refs,omitempty" yaml:"secret_refs,omitempty"`
	Endpoint     string            `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Image        string            `json:"image,omitempty" yaml:"image,omitempty"`
	Policy       *MCPPolicy        `json:"policy,omitempty" yaml:"policy,omitempty"`
	Health       *HealthSpec       `json:"health,omitempty" yaml:"health,omitempty"`
	Tools        []MCPTool         `json:"tools,omitempty" yaml:"tools,omitempty"`
	Resources    []MCPResource     `json:"resources,omitempty" yaml:"resources,omitempty"`
	Prompts      []MCPPrompt       `json:"prompts,omitempty" yaml:"prompts,omitempty"`
	Capabilities MCPCapabilities   `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
}

type MCPPolicy struct {
	DefaultDeny      bool     `json:"default_deny" yaml:"default_deny"`
	AllowedTools     []string `json:"allowed_tools,omitempty" yaml:"allowed_tools,omitempty"`
	DeniedTools      []string `json:"denied_tools,omitempty" yaml:"denied_tools,omitempty"`
	RequiresApproval []string `json:"requires_approval,omitempty" yaml:"requires_approval,omitempty"`
	RateLimitPerMin  int      `json:"rate_limit_per_minute,omitempty" yaml:"rate_limit_per_minute,omitempty"`
	NetworkMode      string   `json:"network_mode,omitempty" yaml:"network_mode,omitempty"`
	ToolLockDigest   string   `json:"tool_lock_digest,omitempty" yaml:"tool_lock_digest,omitempty"`
}

type HealthSpec struct {
	Path            string `json:"path,omitempty" yaml:"path,omitempty"`
	IntervalSeconds int    `json:"interval_seconds,omitempty" yaml:"interval_seconds,omitempty"`
	TimeoutSeconds  int    `json:"timeout_seconds,omitempty" yaml:"timeout_seconds,omitempty"`
}

type InstructionsSpec struct {
	Format          string   `json:"format" yaml:"format"`
	Scope           string   `json:"scope" yaml:"scope"`
	TargetPlatforms []string `json:"target_platforms,omitempty" yaml:"target_platforms,omitempty"`
	TargetPath      string   `json:"target_path" yaml:"target_path"`
	Precedence      int      `json:"precedence,omitempty" yaml:"precedence,omitempty"`
	MergeStrategy   string   `json:"merge_strategy,omitempty" yaml:"merge_strategy,omitempty"`
}

type HookSpec struct {
	Event            string            `json:"event" yaml:"event"`
	Matcher          string            `json:"matcher,omitempty" yaml:"matcher,omitempty"`
	Command          []string          `json:"command,omitempty" yaml:"command,omitempty"`
	Image            string            `json:"image,omitempty" yaml:"image,omitempty"`
	Env              map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	SecretRefs       map[string]string `json:"secret_refs,omitempty" yaml:"secret_refs,omitempty"`
	TimeoutSeconds   int               `json:"timeout_seconds,omitempty" yaml:"timeout_seconds,omitempty"`
	NetworkMode      string            `json:"network_mode,omitempty" yaml:"network_mode,omitempty"`
	AllowedPaths     []string          `json:"allowed_paths,omitempty" yaml:"allowed_paths,omitempty"`
	FailurePolicy    string            `json:"failure_policy,omitempty" yaml:"failure_policy,omitempty"`
	RequiresApproval bool              `json:"requires_approval" yaml:"requires_approval"`
}

type PluginSpec struct {
	Protocol         string            `json:"protocol" yaml:"protocol"`
	Transport        string            `json:"transport" yaml:"transport"`
	Binary           string            `json:"binary,omitempty" yaml:"binary,omitempty"`
	Image            string            `json:"image,omitempty" yaml:"image,omitempty"`
	SHA256           string            `json:"sha256,omitempty" yaml:"sha256,omitempty"`
	Capabilities     []string          `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	Risk             string            `json:"risk,omitempty" yaml:"risk,omitempty"`
	TimeoutSeconds   int               `json:"timeout_seconds,omitempty" yaml:"timeout_seconds,omitempty"`
	NetworkMode      string            `json:"network_mode,omitempty" yaml:"network_mode,omitempty"`
	AllowedPaths     []string          `json:"allowed_paths,omitempty" yaml:"allowed_paths,omitempty"`
	SecretRefs       map[string]string `json:"secret_refs,omitempty" yaml:"secret_refs,omitempty"`
	RequiresApproval bool              `json:"requires_approval" yaml:"requires_approval"`
}

type PolicySpec struct {
	Engine     string `json:"engine" yaml:"engine"`
	Mode       string `json:"mode,omitempty" yaml:"mode,omitempty"`
	Entrypoint string `json:"entrypoint,omitempty" yaml:"entrypoint,omitempty"`
	FailClosed bool   `json:"fail_closed" yaml:"fail_closed"`
}

type ContainerSpec struct {
	Image       string            `json:"image" yaml:"image"`
	Command     []string          `json:"command,omitempty" yaml:"command,omitempty"`
	Args        []string          `json:"args,omitempty" yaml:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	SecretRefs  map[string]string `json:"secret_refs,omitempty" yaml:"secret_refs,omitempty"`
	Ports       []int             `json:"ports,omitempty" yaml:"ports,omitempty"`
	ReadOnly    bool              `json:"read_only" yaml:"read_only"`
	NetworkMode string            `json:"network_mode,omitempty" yaml:"network_mode,omitempty"`
	MemoryMB    int               `json:"memory_mb,omitempty" yaml:"memory_mb,omitempty"`
	CPUs        float64           `json:"cpus,omitempty" yaml:"cpus,omitempty"`
	GPUs        int               `json:"gpus,omitempty" yaml:"gpus,omitempty"`
	Health      *HealthSpec       `json:"health,omitempty" yaml:"health,omitempty"`
}

type ModelRuntimeSpec struct {
	Engine      string            `json:"engine" yaml:"engine"`
	Image       string            `json:"image" yaml:"image"`
	API         string            `json:"api,omitempty" yaml:"api,omitempty"`
	Port        int               `json:"port,omitempty" yaml:"port,omitempty"`
	Args        []string          `json:"args,omitempty" yaml:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	SecretRefs  map[string]string `json:"secret_refs,omitempty" yaml:"secret_refs,omitempty"`
	MemoryMB    int               `json:"memory_mb,omitempty" yaml:"memory_mb,omitempty"`
	GPUs        int               `json:"gpus,omitempty" yaml:"gpus,omitempty"`
	NetworkMode string            `json:"network_mode,omitempty" yaml:"network_mode,omitempty"`
	Health      *HealthSpec       `json:"health,omitempty" yaml:"health,omitempty"`
}

type ModelProfileSpec struct {
	Runtime       ArtifactDependency `json:"runtime" yaml:"runtime"`
	ModelID       string             `json:"model_id" yaml:"model_id"`
	Family        string             `json:"family,omitempty" yaml:"family,omitempty"`
	Quantization  string             `json:"quantization,omitempty" yaml:"quantization,omitempty"`
	ContextWindow int                `json:"context_window,omitempty" yaml:"context_window,omitempty"`
	License       string             `json:"license,omitempty" yaml:"license,omitempty"`
	Parameters    map[string]any     `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

type MCPTool struct {
	Name        string      `json:"name" yaml:"name"`
	Description string      `json:"description,omitempty" yaml:"description,omitempty"`
	InputSchema interface{} `json:"input_schema,omitempty" yaml:"input_schema,omitempty"`
}

type MCPResource struct {
	URI         string `json:"uri" yaml:"uri"`
	Name        string `json:"name,omitempty" yaml:"name,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	MimeType    string `json:"mime_type,omitempty" yaml:"mime_type,omitempty"`
}

type MCPPrompt struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type MCPCapabilities struct {
	Tools     bool `json:"tools,omitempty" yaml:"tools,omitempty"`
	Resources bool `json:"resources,omitempty" yaml:"resources,omitempty"`
	Prompts   bool `json:"prompts,omitempty" yaml:"prompts,omitempty"`
	Logging   bool `json:"logging,omitempty" yaml:"logging,omitempty"`
}

type ArtifactLockfile struct {
	APIVersion string             `json:"apiVersion" yaml:"apiVersion"`
	Root       string             `json:"root" yaml:"root"`
	Resolved   []ResolvedArtifact `json:"resolved" yaml:"resolved"`
	CreatedAt  time.Time          `json:"created_at" yaml:"created_at"`
}

type ResolvedArtifact struct {
	Kind         string `json:"kind" yaml:"kind"`
	Namespace    string `json:"namespace" yaml:"namespace"`
	Name         string `json:"name" yaml:"name"`
	Version      string `json:"version" yaml:"version"`
	DigestSHA256 string `json:"digest_sha256" yaml:"digest_sha256"`
}

type ArtifactGraph struct {
	Nodes []ResolvedArtifact `json:"nodes"`
	Edges []ArtifactEdge     `json:"edges"`
}

type ArtifactEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type OCIDescriptor struct {
	MediaType    string            `json:"mediaType"`
	Digest       string            `json:"digest"`
	Size         int64             `json:"size"`
	ArtifactType string            `json:"artifactType"`
	Annotations  map[string]string `json:"annotations,omitempty"`
}

type Promotion struct {
	ID        int64     `json:"id"`
	Kind      string    `json:"kind"`
	Namespace string    `json:"namespace"`
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	Channel   string    `json:"channel"`
	Actor     string    `json:"actor"`
	CreatedAt time.Time `json:"created_at"`
}

type NamespaceMember struct {
	Namespace string    `json:"namespace"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type Webhook struct {
	ID        int64     `json:"id"`
	Namespace string    `json:"namespace"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

type Attestation struct {
	ID                int64                  `json:"id"`
	ArtifactVersionID int64                  `json:"artifact_version_id"`
	Type              string                 `json:"type"`
	Digest            string                 `json:"digest"`
	Predicate         map[string]interface{} `json:"predicate,omitempty"`
	CreatedBy         string                 `json:"created_by"`
	CreatedAt         time.Time              `json:"created_at"`
}

const (
	ActivationStateDiscovered = "discovered"
	ActivationStateInstalled  = "installed"
	ActivationStateApproved   = "approved"
	ActivationStateActive     = "active"
	ActivationStateDisabled   = "disabled"
	ActivationStateFailed     = "failed"
	ActivationStateRollback   = "rollback"
)

// Activation is desired state only. A separate runtime controller owns any
// side effect and reports observed state back to the registry.
type Activation struct {
	ID              int64      `json:"id"`
	Namespace       string     `json:"namespace"`
	Kind            string     `json:"kind"`
	Name            string     `json:"name"`
	Version         string     `json:"version"`
	PreviousVersion string     `json:"previous_version,omitempty"`
	DesiredState    string     `json:"desired_state"`
	ObservedState   string     `json:"observed_state"`
	Generation      int64      `json:"generation"`
	ApprovedBy      string     `json:"approved_by,omitempty"`
	ApprovedAt      *time.Time `json:"approved_at,omitempty"`
	Reason          string     `json:"reason,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	CreatedBy       string     `json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type ActivationEvent struct {
	ID           int64     `json:"id"`
	ActivationID int64     `json:"activation_id"`
	Generation   int64     `json:"generation"`
	Action       string    `json:"action"`
	Actor        string    `json:"actor"`
	FromState    string    `json:"from_state"`
	ToState      string    `json:"to_state"`
	Reason       string    `json:"reason,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
