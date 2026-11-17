package validation

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/skillforge/skill-registry/internal/metadata"
	"gopkg.in/yaml.v3"
)

var artifactKinds = map[string]bool{
	metadata.ArtifactKindSkill:        true,
	metadata.ArtifactKindAgent:        true,
	metadata.ArtifactKindFlow:         true,
	metadata.ArtifactKindPrompt:       true,
	metadata.ArtifactKindTool:         true,
	metadata.ArtifactKindBundle:       true,
	metadata.ArtifactKindMCP:          true,
	metadata.ArtifactKindInstructions: true,
	metadata.ArtifactKindHook:         true,
	metadata.ArtifactKindPlugin:       true,
	metadata.ArtifactKindPolicy:       true,
	metadata.ArtifactKindContainer:    true,
	metadata.ArtifactKindModelRuntime: true,
	metadata.ArtifactKindModelProfile: true,
}

var artifactEntrypoints = map[string]string{
	metadata.ArtifactKindSkill:        "SKILL.md",
	metadata.ArtifactKindAgent:        "AGENT.md",
	metadata.ArtifactKindPrompt:       "PROMPT.md",
	metadata.ArtifactKindTool:         "TOOL.md",
	metadata.ArtifactKindBundle:       "BUNDLE.md",
	metadata.ArtifactKindFlow:         "FLOW.yaml",
	metadata.ArtifactKindMCP:          "mcp.yaml",
	metadata.ArtifactKindInstructions: "AGENTS.md",
	metadata.ArtifactKindHook:         "hook.yaml",
	metadata.ArtifactKindPlugin:       "plugin.yaml",
	metadata.ArtifactKindPolicy:       "policy.rego",
	metadata.ArtifactKindContainer:    "container.yaml",
	metadata.ArtifactKindModelRuntime: "runtime.yaml",
	metadata.ArtifactKindModelProfile: "model.yaml",
}

func ValidateArtifactKind(kind string) error {
	if !artifactKinds[strings.ToLower(kind)] {
		return fmt.Errorf("unsupported artifact kind %q", kind)
	}
	return nil
}

func (v *Validator) ValidateArtifactPackage(data []byte, packageType, expectedKind string) (*metadata.ArtifactManifest, []string, error) {
	if int64(len(data)) > v.maxSizeBytes {
		return nil, nil, fmt.Errorf("package exceeds maximum size")
	}
	if err := ValidateArtifactKind(expectedKind); err != nil {
		return nil, nil, err
	}
	files, err := v.artifactFiles(data, packageType)
	if err != nil {
		return nil, nil, err
	}
	var manifest metadata.ArtifactManifest
	if content, ok := files["artifact.yaml"]; ok {
		if err := yaml.Unmarshal(content, &manifest); err != nil {
			return nil, nil, fmt.Errorf("invalid artifact.yaml: %w", err)
		}
	} else if content, ok := files["artifact.yml"]; ok {
		if err := yaml.Unmarshal(content, &manifest); err != nil {
			return nil, nil, fmt.Errorf("invalid artifact.yml: %w", err)
		}
	} else {
		entrypoint := artifactEntrypoints[expectedKind]
		content, ok := findArtifactFile(files, entrypoint)
		if !ok {
			return nil, nil, fmt.Errorf("artifact.yaml or %s not found", entrypoint)
		}
		if expectedKind == metadata.ArtifactKindFlow || expectedKind == metadata.ArtifactKindMCP {
			if err := yaml.Unmarshal(content, &manifest); err != nil {
				return nil, nil, fmt.Errorf("invalid %s: %w", entrypoint, err)
			}
		} else {
			legacy := v.extractFrontmatterManifest(content)
			manifest = metadata.ArtifactManifest{
				APIVersion: "skillforge.dev/v1",
				Kind:       expectedKind,
				Metadata: metadata.ArtifactMetadata{
					Name: legacy.Name, Version: legacy.Version, Description: legacy.Description,
					Tags: legacy.Tags, Owners: legacy.Owners,
				},
				Spec: metadata.ArtifactSpec{Entrypoint: entrypoint},
			}
		}
	}
	manifest.Kind = strings.ToLower(manifest.Kind)
	if manifest.APIVersion == "" {
		manifest.APIVersion = "skillforge.dev/v1"
	}
	if manifest.APIVersion != "skillforge.dev/v1" && manifest.APIVersion != "skillforge.dev/v2" {
		return nil, nil, fmt.Errorf("unsupported apiVersion %q", manifest.APIVersion)
	}
	if isExecutableArtifact(expectedKind) && manifest.APIVersion != "skillforge.dev/v2" {
		return nil, nil, fmt.Errorf("executable artifact kind %q requires apiVersion skillforge.dev/v2", expectedKind)
	}
	if manifest.Kind == "" {
		manifest.Kind = expectedKind
	}
	if manifest.Kind != expectedKind {
		return nil, nil, fmt.Errorf("manifest kind %q does not match %q", manifest.Kind, expectedKind)
	}
	if manifest.Metadata.Visibility == "" {
		manifest.Metadata.Visibility = "public"
	}
	if manifest.Spec.Entrypoint == "" {
		manifest.Spec.Entrypoint = artifactEntrypoints[expectedKind]
	}
	if _, ok := findArtifactFile(files, manifest.Spec.Entrypoint); !ok {
		return nil, nil, fmt.Errorf("entrypoint %q not found", manifest.Spec.Entrypoint)
	}

	var warnings []string

	// Kind-specific validation
	switch expectedKind {
	case metadata.ArtifactKindAgent:
		if err := validateAgent(&manifest, &warnings); err != nil {
			return nil, nil, err
		}
	case metadata.ArtifactKindFlow:
		if err := validateFlow(&manifest); err != nil {
			return nil, nil, err
		}
		if len(manifest.Spec.Steps) == 0 {
			warnings = append(warnings, "flow has no steps defined")
		}
	case metadata.ArtifactKindMCP:
		if err := validateMCP(&manifest); err != nil {
			return nil, nil, err
		}
	case metadata.ArtifactKindInstructions:
		if err := validateInstructions(&manifest); err != nil {
			return nil, nil, err
		}
	case metadata.ArtifactKindHook:
		if err := validateHook(&manifest); err != nil {
			return nil, nil, err
		}
	case metadata.ArtifactKindPlugin:
		if err := validatePlugin(&manifest); err != nil {
			return nil, nil, err
		}
	case metadata.ArtifactKindPolicy:
		if err := validatePolicy(&manifest); err != nil {
			return nil, nil, err
		}
	case metadata.ArtifactKindContainer:
		if err := validateContainer(&manifest); err != nil {
			return nil, nil, err
		}
	case metadata.ArtifactKindModelRuntime:
		if err := validateModelRuntime(&manifest); err != nil {
			return nil, nil, err
		}
	case metadata.ArtifactKindModelProfile:
		if err := validateModelProfile(&manifest); err != nil {
			return nil, nil, err
		}
	case metadata.ArtifactKindBundle:
		if len(manifest.Spec.Dependencies) == 0 {
			warnings = append(warnings, "bundle has no dependencies")
		}
	}

	return &manifest, warnings, nil
}

func validateAgent(m *metadata.ArtifactManifest, warnings *[]string) error {
	spec := m.Spec.Agent
	if spec == nil {
		*warnings = append(*warnings, "agent has no spec.agent block; add model, system_prompt, and tools")
		return nil
	}
	if spec.Model == "" && spec.ModelFamily == "" {
		*warnings = append(*warnings, "agent should specify spec.agent.model or spec.agent.model_family")
	}
	if spec.Memory != nil {
		allowed := map[string]bool{"none": true, "buffer": true, "vector": true, "persistent": true}
		if !allowed[spec.Memory.Type] {
			return fmt.Errorf("invalid spec.agent.memory.type %q; must be one of: none, buffer, vector, persistent", spec.Memory.Type)
		}
	}
	return nil
}

func validateFlow(m *metadata.ArtifactManifest) error {
	steps := m.Spec.Steps
	if len(steps) == 0 {
		return nil
	}
	// Collect step IDs and detect duplicates
	ids := make(map[string]bool, len(steps))
	for _, s := range steps {
		if s.ID == "" {
			return fmt.Errorf("every flow step must have an id")
		}
		if ids[s.ID] {
			return fmt.Errorf("duplicate flow step id %q", s.ID)
		}
		ids[s.ID] = true
	}
	// Validate Needs references and detect cycles via Kahn's algorithm
	inDegree := make(map[string]int, len(steps))
	adj := make(map[string][]string, len(steps))
	for _, s := range steps {
		if _, ok := inDegree[s.ID]; !ok {
			inDegree[s.ID] = 0
		}
		for _, dep := range s.Needs {
			if !ids[dep] {
				return fmt.Errorf("flow step %q needs unknown step %q", s.ID, dep)
			}
			adj[dep] = append(adj[dep], s.ID)
			inDegree[s.ID]++
		}
	}
	// Kahn's topological sort
	queue := []string{}
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	visited := 0
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		visited++
		for _, next := range adj[cur] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if visited != len(steps) {
		return fmt.Errorf("flow contains a dependency cycle")
	}
	return nil
}

func validateMCP(m *metadata.ArtifactManifest) error {
	spec := m.Spec.MCP
	if spec == nil {
		return fmt.Errorf("mcp artifact must have a spec.mcp block")
	}
	validTransports := map[string]bool{"stdio": true, "sse": true, "http": true}
	if spec.Transport == "" {
		return fmt.Errorf("spec.mcp.transport is required (stdio, sse, or http)")
	}
	if !validTransports[spec.Transport] {
		return fmt.Errorf("invalid spec.mcp.transport %q; must be one of: stdio, sse, http", spec.Transport)
	}
	if spec.Transport == "stdio" && len(spec.Command) == 0 {
		if spec.Image == "" {
			return fmt.Errorf("spec.mcp.command or digest-pinned image is required for stdio transport")
		}
	}
	if spec.Image != "" && !isDigestPinnedImage(spec.Image) {
		return fmt.Errorf("spec.mcp.image must be pinned by sha256 digest")
	}
	if err := validateEnvironment(spec.Env, spec.SecretRefs); err != nil {
		return fmt.Errorf("spec.mcp: %w", err)
	}
	if spec.Policy != nil {
		if spec.Policy.RateLimitPerMin < 0 {
			return fmt.Errorf("spec.mcp.policy.rate_limit_per_minute must be >= 0")
		}
		if !oneOf(spec.Policy.NetworkMode, "", "none", "isolated", "host") {
			return fmt.Errorf("invalid spec.mcp.policy.network_mode")
		}
	}
	for i, t := range spec.Tools {
		if t.Name == "" {
			return fmt.Errorf("spec.mcp.tools[%d] must have a name", i)
		}
	}
	return nil
}

func validateInstructions(m *metadata.ArtifactManifest) error {
	s := m.Spec.Instructions
	if s == nil {
		return fmt.Errorf("instructions artifact must have a spec.instructions block")
	}
	if !oneOf(s.Format, "agents.md", "claude.md", "gemini.md", "copilot-instructions.md") {
		return fmt.Errorf("unsupported spec.instructions.format %q", s.Format)
	}
	if !oneOf(s.Scope, "global", "repository", "directory") {
		return fmt.Errorf("invalid spec.instructions.scope %q", s.Scope)
	}
	if s.TargetPath == "" || filepath.IsAbs(s.TargetPath) || strings.Contains(filepath.ToSlash(s.TargetPath), "../") {
		return fmt.Errorf("spec.instructions.target_path must be a safe relative path")
	}
	if !oneOf(s.MergeStrategy, "", "replace", "append", "prepend", "fail-on-conflict") {
		return fmt.Errorf("invalid spec.instructions.merge_strategy %q", s.MergeStrategy)
	}
	return nil
}

func validateHook(m *metadata.ArtifactManifest) error {
	s := m.Spec.Hook
	if s == nil {
		return fmt.Errorf("hook artifact must have a spec.hook block")
	}
	if !oneOf(s.Event, "session-start", "session-end", "pre-tool", "post-tool", "stop", "notification", "pre-commit", "post-commit") {
		return fmt.Errorf("unsupported spec.hook.event %q", s.Event)
	}
	if len(s.Command) == 0 && s.Image == "" {
		return fmt.Errorf("spec.hook.command or spec.hook.image is required")
	}
	if s.Image != "" && !isDigestPinnedImage(s.Image) {
		return fmt.Errorf("spec.hook.image must be pinned by sha256 digest")
	}
	if s.TimeoutSeconds < 0 || s.TimeoutSeconds > 3600 {
		return fmt.Errorf("spec.hook.timeout_seconds must be between 0 and 3600")
	}
	if !oneOf(s.NetworkMode, "", "none", "isolated", "host") || !oneOf(s.FailurePolicy, "", "fail-closed", "warn", "ignore") {
		return fmt.Errorf("invalid hook network_mode or failure_policy")
	}
	if err := validateEnvironment(s.Env, s.SecretRefs); err != nil {
		return fmt.Errorf("spec.hook: %w", err)
	}
	return nil
}

func validatePlugin(m *metadata.ArtifactManifest) error {
	s := m.Spec.Plugin
	if s == nil || s.Protocol == "" {
		return fmt.Errorf("plugin artifact requires spec.plugin.protocol")
	}
	if !oneOf(s.Transport, "stdio", "unix", "http") {
		return fmt.Errorf("invalid spec.plugin.transport %q", s.Transport)
	}
	if s.Binary == "" && s.Image == "" {
		return fmt.Errorf("spec.plugin.binary or spec.plugin.image is required")
	}
	if s.Image != "" && !isDigestPinnedImage(s.Image) {
		return fmt.Errorf("spec.plugin.image must be pinned by sha256 digest")
	}
	if s.Binary != "" && (filepath.IsAbs(s.Binary) || strings.Contains(filepath.ToSlash(s.Binary), "../")) {
		return fmt.Errorf("spec.plugin.binary must be a safe package-relative path")
	}
	return nil
}

func validatePolicy(m *metadata.ArtifactManifest) error {
	s := m.Spec.Policy
	if s == nil || !oneOf(s.Engine, "rego", "yaml", "json") {
		return fmt.Errorf("policy artifact requires a supported spec.policy.engine")
	}
	return nil
}

func validateContainer(m *metadata.ArtifactManifest) error {
	s := m.Spec.Container
	if s == nil || !isDigestPinnedImage(s.Image) {
		return fmt.Errorf("container artifact requires a sha256 digest-pinned spec.container.image")
	}
	if err := validateEnvironment(s.Env, s.SecretRefs); err != nil {
		return fmt.Errorf("spec.container: %w", err)
	}
	if s.MemoryMB < 0 || s.CPUs < 0 || s.GPUs < 0 {
		return fmt.Errorf("container resources must be >= 0")
	}
	return nil
}

func validateModelRuntime(m *metadata.ArtifactManifest) error {
	s := m.Spec.ModelRuntime
	if s == nil || !oneOf(s.Engine, "ollama", "vllm", "llama.cpp", "tgi", "openai-compatible") {
		return fmt.Errorf("model-runtime artifact requires a supported engine")
	}
	if !isDigestPinnedImage(s.Image) {
		return fmt.Errorf("spec.model_runtime.image must be pinned by sha256 digest")
	}
	if s.Port < 0 || s.Port > 65535 || s.MemoryMB < 0 || s.GPUs < 0 {
		return fmt.Errorf("invalid model runtime resource or port value")
	}
	return validateEnvironment(s.Env, s.SecretRefs)
}

func validateModelProfile(m *metadata.ArtifactManifest) error {
	s := m.Spec.ModelProfile
	if s == nil || s.ModelID == "" {
		return fmt.Errorf("model-profile artifact requires spec.model_profile.model_id")
	}
	if s.Runtime.Kind != metadata.ArtifactKindModelRuntime || s.Runtime.Name == "" || s.Runtime.Namespace == "" || s.Runtime.Version == "" {
		return fmt.Errorf("spec.model_profile.runtime must reference a versioned model-runtime artifact")
	}
	if s.ContextWindow < 0 {
		return fmt.Errorf("spec.model_profile.context_window must be >= 0")
	}
	return nil
}

func isExecutableArtifact(kind string) bool {
	return oneOf(kind, metadata.ArtifactKindMCP, metadata.ArtifactKindHook, metadata.ArtifactKindPlugin, metadata.ArtifactKindContainer, metadata.ArtifactKindModelRuntime)
}

func isDigestPinnedImage(image string) bool {
	parts := strings.Split(image, "@sha256:")
	if len(parts) != 2 || parts[0] == "" || len(parts[1]) != 64 {
		return false
	}
	for _, r := range parts[1] {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func validateEnvironment(env, secretRefs map[string]string) error {
	for key, ref := range secretRefs {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(ref) == "" {
			return fmt.Errorf("secret_refs keys and values must not be empty")
		}
		if _, duplicate := env[key]; duplicate {
			return fmt.Errorf("environment key %q cannot be both literal and a secret reference", key)
		}
	}
	for key := range env {
		upper := strings.ToUpper(key)
		for _, marker := range []string{"SECRET", "TOKEN", "PASSWORD", "PASSWD", "PRIVATE_KEY", "API_KEY", "CREDENTIAL"} {
			if strings.Contains(upper, marker) {
				return fmt.Errorf("secret-like environment key %q must use secret_refs", key)
			}
		}
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func (v *Validator) artifactFiles(data []byte, packageType string) (map[string][]byte, error) {
	files := map[string][]byte{}
	add := func(name string, reader io.Reader) error {
		if err := v.validatePath(name); err != nil {
			return err
		}
		ext := strings.ToLower(filepath.Ext(name))
		if v.blockedExtensions[ext] {
			return fmt.Errorf("blocked file extension: %s", name)
		}
		content, err := io.ReadAll(io.LimitReader(reader, v.maxSizeBytes+1))
		if err != nil {
			return err
		}
		if int64(len(content)) > v.maxSizeBytes {
			return fmt.Errorf("expanded file exceeds maximum size: %s", name)
		}
		files[filepath.ToSlash(name)] = content
		return nil
	}
	switch packageType {
	case "zip":
		reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, err
		}
		for _, file := range reader.File {
			if file.FileInfo().IsDir() {
				continue
			}
			rc, err := file.Open()
			if err != nil {
				return nil, err
			}
			err = add(file.Name, rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
		}
	case "tgz":
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		tr := tar.NewReader(gz)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if header.Typeflag == tar.TypeReg {
				if err := add(header.Name, tr); err != nil {
					return nil, err
				}
			}
		}
	default:
		return nil, fmt.Errorf("unsupported package type")
	}
	return files, nil
}

func findArtifactFile(files map[string][]byte, name string) ([]byte, bool) {
	for path, content := range files {
		if path == name || filepath.Base(path) == name {
			return content, true
		}
	}
	return nil, false
}
