package validation

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/skillforge/skill-registry/internal/metadata"
	"gopkg.in/yaml.v3"
)

// ValidationResult represents the result of skill validation
type ValidationResult struct {
	Valid    bool                   `json:"valid"`
	Errors   []string               `json:"errors"`
	Warnings []string               `json:"warnings"`
	Metadata *metadata.SkillManifest `json:"metadata,omitempty"`
}

// Validator validates skill packages
type Validator struct {
	maxSizeBytes      int64
	blockedExtensions map[string]bool
}

// NewValidator creates a new validator
func NewValidator(maxSizeMB int, blockedExtensions []string) *Validator {
	blocked := make(map[string]bool)
	for _, ext := range blockedExtensions {
		blocked[strings.ToLower(ext)] = true
	}

	return &Validator{
		maxSizeBytes:      int64(maxSizeMB) * 1024 * 1024,
		blockedExtensions: blocked,
	}
}

var skillNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,127}$`)
var semverRegex = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

// ValidateSkillName validates a skill name
func ValidateSkillName(name string) error {
	if !skillNameRegex.MatchString(name) {
		return fmt.Errorf("invalid skill name: must match ^[a-z0-9][a-z0-9._-]{1,127}$")
	}
	return nil
}

// ValidateVersion validates a semantic version
func ValidateVersion(version string) error {
	if !semverRegex.MatchString(version) {
		return fmt.Errorf("invalid version: must be valid SemVer")
	}
	return nil
}

// ValidatePackage validates a skill package
func (v *Validator) ValidatePackage(data []byte, packageType string) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Check size
	if int64(len(data)) > v.maxSizeBytes {
		result.Errors = append(result.Errors, fmt.Sprintf("package exceeds maximum size of %d MB", v.maxSizeBytes/(1024*1024)))
		result.Valid = false
		return result, nil
	}

	// Validate based on package type
	switch packageType {
	case "zip":
		return v.validateZip(data, result)
	case "tgz":
		return v.validateTarGz(data, result)
	default:
		result.Errors = append(result.Errors, "unsupported package type")
		result.Valid = false
		return result, nil
	}
}

func (v *Validator) validateZip(data []byte, result *ValidationResult) (*ValidationResult, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to read zip: %v", err))
		result.Valid = false
		return result, nil
	}

	var skillMDFound bool
	var skillMDContent []byte
	var rootDir string
	hasReadme := false

	for _, file := range reader.File {
		// Security checks
		if err := v.validatePath(file.Name, result); err != nil {
			result.Valid = false
			continue
		}

		// Check blocked extensions
		ext := strings.ToLower(filepath.Ext(file.Name))
		if v.blockedExtensions[ext] {
			result.Errors = append(result.Errors, fmt.Sprintf("blocked file extension: %s", file.Name))
			result.Valid = false
		}

		// Detect root directory
		parts := strings.Split(strings.TrimPrefix(file.Name, "/"), "/")
		if len(parts) > 0 && rootDir == "" && !file.FileInfo().IsDir() {
			if len(parts) > 1 {
				rootDir = parts[0]
			}
		}

		// Check for SKILL.md
		baseName := filepath.Base(file.Name)
		if baseName == "SKILL.md" && !file.FileInfo().IsDir() {
			skillMDFound = true
			rc, err := file.Open()
			if err == nil {
				skillMDContent, _ = io.ReadAll(rc)
				rc.Close()
			}
		}

		if strings.ToLower(baseName) == "readme.md" {
			hasReadme = true
		}
	}

	if !skillMDFound {
		result.Errors = append(result.Errors, "SKILL.md not found in package")
		result.Valid = false
	} else if len(skillMDContent) == 0 {
		result.Errors = append(result.Errors, "SKILL.md is empty")
		result.Valid = false
	}

	if !hasReadme {
		result.Warnings = append(result.Warnings, "no README.md found")
	}

	// Try to extract manifest
	if skillMDFound {
		manifest := v.extractManifest(skillMDContent)
		result.Metadata = manifest
	}

	return result, nil
}

func (v *Validator) validateTarGz(data []byte, result *ValidationResult) (*ValidationResult, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to read gzip: %v", err))
		result.Valid = false
		return result, nil
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	var skillMDFound bool
	var skillMDContent []byte
	hasReadme := false

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to read tar: %v", err))
			result.Valid = false
			return result, nil
		}

		// Security checks
		if err := v.validatePath(header.Name, result); err != nil {
			result.Valid = false
			continue
		}

		// Check blocked extensions
		ext := strings.ToLower(filepath.Ext(header.Name))
		if v.blockedExtensions[ext] {
			result.Errors = append(result.Errors, fmt.Sprintf("blocked file extension: %s", header.Name))
			result.Valid = false
		}

		// Check for SKILL.md
		baseName := filepath.Base(header.Name)
		if baseName == "SKILL.md" && header.Typeflag == tar.TypeReg {
			skillMDFound = true
			skillMDContent, _ = io.ReadAll(tr)
		}

		if strings.ToLower(baseName) == "readme.md" {
			hasReadme = true
		}
	}

	if !skillMDFound {
		result.Errors = append(result.Errors, "SKILL.md not found in package")
		result.Valid = false
	} else if len(skillMDContent) == 0 {
		result.Errors = append(result.Errors, "SKILL.md is empty")
		result.Valid = false
	}

	if !hasReadme {
		result.Warnings = append(result.Warnings, "no README.md found")
	}

	// Try to extract manifest
	if skillMDFound {
		manifest := v.extractManifest(skillMDContent)
		result.Metadata = manifest
	}

	return result, nil
}

func (v *Validator) validatePath(path string, result *ValidationResult) error {
	// Check for absolute paths
	if filepath.IsAbs(path) {
		result.Errors = append(result.Errors, fmt.Sprintf("absolute path not allowed: %s", path))
		return fmt.Errorf("absolute path")
	}

	// Check for path traversal
	cleaned := filepath.Clean(path)
	if strings.Contains(cleaned, "..") {
		result.Errors = append(result.Errors, fmt.Sprintf("path traversal not allowed: %s", path))
		return fmt.Errorf("path traversal")
	}

	return nil
}

func (v *Validator) extractManifest(skillMDContent []byte) *metadata.SkillManifest {
	// Try to extract YAML frontmatter
	manifest := &metadata.SkillManifest{}
	
	scanner := bufio.NewScanner(bytes.NewReader(skillMDContent))
	var frontmatterLines []string
	inFrontmatter := false
	foundFrontmatter := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				foundFrontmatter = true
			} else {
				break
			}
			continue
		}
		if inFrontmatter {
			frontmatterLines = append(frontmatterLines, line)
		}
	}

	if foundFrontmatter && len(frontmatterLines) > 0 {
		frontmatterYAML := strings.Join(frontmatterLines, "\n")
		yaml.Unmarshal([]byte(frontmatterYAML), manifest)
	}

	if manifest.Entrypoint == "" {
		manifest.Entrypoint = "SKILL.md"
	}

	return manifest
}
