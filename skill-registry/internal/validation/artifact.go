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
	metadata.ArtifactKindSkill:  true,
	metadata.ArtifactKindAgent:  true,
	metadata.ArtifactKindFlow:   true,
	metadata.ArtifactKindPrompt: true,
	metadata.ArtifactKindTool:   true,
	metadata.ArtifactKindBundle: true,
	metadata.ArtifactKindMCP:    true,
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
		entrypoint := map[string]string{
			metadata.ArtifactKindSkill:  "SKILL.md",
			metadata.ArtifactKindAgent:  "AGENT.md",
			metadata.ArtifactKindPrompt: "PROMPT.md",
			metadata.ArtifactKindTool:   "TOOL.md",
			metadata.ArtifactKindBundle: "BUNDLE.md",
			metadata.ArtifactKindFlow:   "FLOW.yaml",
			metadata.ArtifactKindMCP:    "mcp.yaml",
		}[expectedKind]
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
		manifest.Spec.Entrypoint = map[string]string{
			metadata.ArtifactKindSkill:  "SKILL.md",
			metadata.ArtifactKindAgent:  "AGENT.md",
			metadata.ArtifactKindPrompt: "PROMPT.md",
			metadata.ArtifactKindTool:   "TOOL.md",
			metadata.ArtifactKindBundle: "BUNDLE.md",
			metadata.ArtifactKindFlow:   "FLOW.yaml",
			metadata.ArtifactKindMCP:    "mcp.yaml",
		}[expectedKind]
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
		return fmt.Errorf("spec.mcp.command is required for stdio transport")
	}
	for i, t := range spec.Tools {
		if t.Name == "" {
			return fmt.Errorf("spec.mcp.tools[%d] must have a name", i)
		}
	}
	return nil
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
