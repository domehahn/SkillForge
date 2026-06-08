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
	metadata.ArtifactKindSkill: true, metadata.ArtifactKindAgent: true,
	metadata.ArtifactKindFlow: true, metadata.ArtifactKindPrompt: true,
	metadata.ArtifactKindTool: true, metadata.ArtifactKindBundle: true,
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
			metadata.ArtifactKindSkill: "SKILL.md", metadata.ArtifactKindAgent: "AGENT.md",
			metadata.ArtifactKindPrompt: "PROMPT.md", metadata.ArtifactKindTool: "TOOL.md",
			metadata.ArtifactKindBundle: "BUNDLE.md", metadata.ArtifactKindFlow: "FLOW.yaml",
		}[expectedKind]
		content, ok := findArtifactFile(files, entrypoint)
		if !ok {
			return nil, nil, fmt.Errorf("artifact.yaml or %s not found", entrypoint)
		}
		if expectedKind == metadata.ArtifactKindFlow {
			if err := yaml.Unmarshal(content, &manifest); err != nil {
				return nil, nil, fmt.Errorf("invalid FLOW.yaml: %w", err)
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
			metadata.ArtifactKindSkill: "SKILL.md", metadata.ArtifactKindAgent: "AGENT.md",
			metadata.ArtifactKindPrompt: "PROMPT.md", metadata.ArtifactKindTool: "TOOL.md",
			metadata.ArtifactKindBundle: "BUNDLE.md", metadata.ArtifactKindFlow: "FLOW.yaml",
		}[expectedKind]
	}
	if _, ok := findArtifactFile(files, manifest.Spec.Entrypoint); !ok {
		return nil, nil, fmt.Errorf("entrypoint %q not found", manifest.Spec.Entrypoint)
	}
	var warnings []string
	if len(manifest.Spec.Dependencies) == 0 && (expectedKind == metadata.ArtifactKindAgent || expectedKind == metadata.ArtifactKindFlow || expectedKind == metadata.ArtifactKindBundle) {
		warnings = append(warnings, "composite artifact has no dependencies")
	}
	return &manifest, warnings, nil
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
