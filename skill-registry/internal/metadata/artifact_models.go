package metadata

import "time"

const (
	ArtifactKindSkill  = "skill"
	ArtifactKindAgent  = "agent"
	ArtifactKindFlow   = "flow"
	ArtifactKindPrompt = "prompt"
	ArtifactKindTool   = "tool"
	ArtifactKindBundle = "bundle"
)

type Artifact struct {
	ID            int64     `json:"id"`
	Kind          string    `json:"kind"`
	Namespace     string    `json:"namespace"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	LatestVersion string    `json:"latest_version"`
	Visibility    string    `json:"visibility"`
	Tags          []string  `json:"tags"`
	Owners        []string  `json:"owners"`
	Downloads     int64     `json:"downloads"`
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
}

type ArtifactDependency struct {
	Kind      string `json:"kind" yaml:"kind"`
	Namespace string `json:"namespace" yaml:"namespace"`
	Name      string `json:"name" yaml:"name"`
	Version   string `json:"version" yaml:"version"`
	Optional  bool   `json:"optional,omitempty" yaml:"optional,omitempty"`
}

type FlowStep struct {
	ID    string   `json:"id" yaml:"id"`
	Uses  string   `json:"uses" yaml:"uses"`
	Needs []string `json:"needs,omitempty" yaml:"needs,omitempty"`
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
