package validation

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/skillforge/skill-registry/internal/metadata"
	"gopkg.in/yaml.v3"
)

type Profile string

const (
	ProfileDefault Profile = "default"
	ProfileStrict  Profile = "strict"
	ProfilePublish Profile = "publish"
)

type Issue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

// ValidationResult represents the result of skill validation.
type ValidationResult struct {
	Valid            bool                    `json:"valid"`
	Errors           []string                `json:"errors"`
	Warnings         []string                `json:"warnings"`
	StructuredErrors []Issue                 `json:"structured_errors,omitempty"`
	StructuredWarns  []Issue                 `json:"structured_warnings,omitempty"`
	Metadata         *metadata.SkillManifest `json:"metadata,omitempty"`
	Files            []string                `json:"files,omitempty"`
	Deterministic    bool                    `json:"deterministic"`
	Profile          Profile                 `json:"profile"`
}

type packageFile struct {
	Name     string
	Mode     os.FileMode
	Size     int64
	Type     byte
	Content  []byte
	Linkname string
}

// Validator validates skill packages.
type Validator struct {
	maxSizeBytes      int64
	maxFileCount      int
	maxSingleFileSize int64
	blockedExtensions map[string]bool
}

func NewValidator(maxSizeMB int, blockedExtensions []string) *Validator {
	blocked := map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".dylib": true, ".class": true,
		".jar": true, ".war": true, ".pyc": true, ".pyo": true,
	}
	for _, ext := range blockedExtensions {
		blocked[strings.ToLower(ext)] = true
	}
	return &Validator{
		maxSizeBytes:      int64(maxSizeMB) * 1024 * 1024,
		maxFileCount:      1000,
		maxSingleFileSize: 10 * 1024 * 1024,
		blockedExtensions: blocked,
	}
}

var skillNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,127}$`)
var namespaceRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
var distTagRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
var semverRegex = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
var secretRegexes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|secret|token|password)\s*[:=]\s*['"]?[A-Za-z0-9_./+=-]{16,}`),
	regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`(?i)github_pat_[A-Za-z0-9_]{20,}`),
}

var supportedPlatforms = map[string]bool{
	"codex": true, "claude-code": true, "gitlab-duo": true, "github-copilot": true,
	"cursor": true, "windsurf": true, "generic": true, "all": true,
}

func SupportedPlatforms() []string {
	out := make([]string, 0, len(supportedPlatforms))
	for p := range supportedPlatforms {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func ValidateSkillName(name string) error {
	if !skillNameRegex.MatchString(name) {
		return fmt.Errorf("invalid skill name: must match ^[a-z0-9][a-z0-9._-]{1,127}$")
	}
	return nil
}

func ValidateNamespace(namespace string) error {
	if !namespaceRegex.MatchString(namespace) {
		return fmt.Errorf("invalid namespace")
	}
	return nil
}

func ValidateDistTag(tag string) error {
	if !distTagRegex.MatchString(tag) {
		return fmt.Errorf("invalid dist-tag")
	}
	return nil
}

func ValidateVersion(version string) error {
	if !semverRegex.MatchString(version) {
		return fmt.Errorf("invalid version: must be valid SemVer")
	}
	return nil
}

func (v *Validator) ValidatePackage(data []byte, packageType string) (*ValidationResult, error) {
	return v.ValidatePackageWithProfile(data, packageType, ProfileDefault)
}

func (v *Validator) ValidatePackageWithProfile(data []byte, packageType string, profile Profile) (*ValidationResult, error) {
	result := newResult(profile)
	if int64(len(data)) > v.maxSizeBytes {
		result.addError("PACKAGE_TOO_LARGE", "", fmt.Sprintf("package exceeds maximum size of %d MB", v.maxSizeBytes/(1024*1024)))
		return result, nil
	}
	files, err := v.readArchive(data, packageType, result)
	if err != nil {
		result.addError("ARCHIVE_READ_FAILED", "", err.Error())
		return result, nil
	}
	v.validateFiles(files, result)
	return result, nil
}

func (v *Validator) ValidateDirectory(path string, profile Profile) (*ValidationResult, error) {
	result := newResult(profile)
	var files []packageFile
	err := filepath.WalkDir(path, func(p string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == path {
			return nil
		}
		rel, err := filepath.Rel(path, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		pf := packageFile{Name: rel, Mode: info.Mode(), Size: info.Size()}
		if info.Mode()&os.ModeSymlink != 0 {
			pf.Type = tar.TypeSymlink
			link, _ := os.Readlink(p)
			pf.Linkname = link
		} else if entry.IsDir() {
			pf.Type = tar.TypeDir
		} else {
			pf.Type = tar.TypeReg
			pf.Content, err = os.ReadFile(p)
			if err != nil {
				return err
			}
		}
		files = append(files, pf)
		return nil
	})
	if err != nil {
		return nil, err
	}
	v.validateFiles(files, result)
	return result, nil
}

func newResult(profile Profile) *ValidationResult {
	return &ValidationResult{
		Valid: true, Errors: []string{}, Warnings: []string{},
		StructuredErrors: []Issue{}, StructuredWarns: []Issue{}, Files: []string{},
		Profile: profile, Deterministic: true,
	}
}

func (r *ValidationResult) addError(code, path, message string) {
	r.Valid = false
	r.Errors = append(r.Errors, message)
	r.StructuredErrors = append(r.StructuredErrors, Issue{Code: code, Path: path, Message: message})
}

func (r *ValidationResult) addWarning(code, path, message string) {
	r.Warnings = append(r.Warnings, message)
	r.StructuredWarns = append(r.StructuredWarns, Issue{Code: code, Path: path, Message: message})
}

func (v *Validator) readArchive(data []byte, packageType string, result *ValidationResult) ([]packageFile, error) {
	switch packageType {
	case "zip":
		return v.readZip(data, result)
	case "tgz", "tar.gz":
		return v.readTarGz(data, result)
	default:
		return nil, fmt.Errorf("unsupported package type")
	}
}

func (v *Validator) readZip(data []byte, result *ValidationResult) ([]packageFile, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to read zip: %w", err)
	}
	files := make([]packageFile, 0, len(reader.File))
	for _, file := range reader.File {
		pf := packageFile{Name: file.Name, Mode: file.Mode(), Size: int64(file.UncompressedSize64)}
		if file.FileInfo().IsDir() {
			pf.Type = tar.TypeDir
		} else if file.Mode()&os.ModeSymlink != 0 {
			pf.Type = tar.TypeSymlink
			rc, err := file.Open()
			if err == nil {
				content, _ := io.ReadAll(rc)
				rc.Close()
				pf.Linkname = string(content)
			}
		} else {
			pf.Type = tar.TypeReg
			rc, err := file.Open()
			if err == nil {
				pf.Content, _ = io.ReadAll(rc)
				rc.Close()
			}
		}
		files = append(files, pf)
	}
	return files, nil
}

func (v *Validator) readTarGz(data []byte, result *ValidationResult) ([]packageFile, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to read gzip: %w", err)
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	var files []packageFile
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar: %w", err)
		}
		pf := packageFile{Name: header.Name, Mode: os.FileMode(header.Mode), Size: header.Size, Type: header.Typeflag, Linkname: header.Linkname}
		if header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA {
			pf.Content, _ = io.ReadAll(tr)
		}
		files = append(files, pf)
	}
	return files, nil
}

func (v *Validator) validateFiles(files []packageFile, result *ValidationResult) {
	byPath := map[string]packageFile{}
	for _, f := range files {
		name := normalizeArchivePath(f.Name)
		if name == "" || strings.HasSuffix(name, "/") {
			continue
		}
		result.Files = append(result.Files, name)
		if err := v.validatePath(name); err != nil {
			result.addError("UNSAFE_PATH", name, err.Error())
			continue
		}
		if f.Type == tar.TypeSymlink {
			result.addError("SYMLINK_NOT_ALLOWED", name, "symlinks are not allowed in skill packages")
			continue
		}
		if strings.Contains(name, "/.git/") || strings.HasPrefix(name, ".git/") {
			result.addError("GIT_CONTENT", name, ".git content must not be packaged")
		}
		base := filepath.Base(name)
		if base == ".env" || strings.HasSuffix(base, ".pem") || strings.Contains(strings.ToLower(base), "credentials") {
			result.addError("SENSITIVE_FILE", name, "likely secret or credential file must not be packaged")
		}
		if strings.HasPrefix(base, ".") && base != ".agent-skill-install.json" {
			result.addWarning("HIDDEN_FILE", name, "hidden files should not be packaged")
		}
		if ignoredTree(name) {
			result.addWarning("BUILD_OUTPUT", name, "build output, dependency directories, and caches should be excluded")
		}
		if f.Type == tar.TypeReg || f.Type == tar.TypeRegA || f.Type == 0 {
			ext := strings.ToLower(filepath.Ext(name))
			if v.blockedExtensions[ext] {
				result.addError("BLOCKED_EXTENSION", name, fmt.Sprintf("blocked file extension: %s", name))
			}
			if f.Size > v.maxSingleFileSize {
				result.addError("FILE_TOO_LARGE", name, "single file exceeds maximum allowed size")
			}
			if result.Profile != ProfileDefault && looksExecutable(f.Mode, name) && !declaredExecutable(name) {
				result.addError("UNKNOWN_EXECUTABLE", name, "executable files must be intentionally declared or avoided")
			}
			if len(f.Content) > 0 {
				v.scanContent(name, f.Content, result)
			}
			byPath[name] = f
		}
	}
	sort.Strings(result.Files)
	if len(result.Files) > v.maxFileCount {
		result.addError("TOO_MANY_FILES", "", fmt.Sprintf("package contains more than %d files", v.maxFileCount))
	}

	skillFile, hasSkill := findByBase(byPath, "SKILL.md")
	if !hasSkill {
		result.addError("MISSING_SKILL_MD", "", "SKILL.md not found in package")
	} else if len(bytes.TrimSpace(skillFile.Content)) == 0 {
		result.addError("EMPTY_SKILL_MD", skillFile.Name, "SKILL.md is empty")
	}
	if _, ok := findByBase(byPath, "README.md"); !ok {
		result.addWarning("MISSING_README", "", "README.md is recommended")
	}

	manifestFile, hasManifest := findByBase(byPath, "skill.yaml")
	versionFile, hasVersion := findByBase(byPath, "VERSION")
	changelogFile, hasChangelog := findByBase(byPath, "CHANGELOG.md")

	if hasManifest {
		manifest, err := ParseSkillManifest(manifestFile.Content)
		if err != nil {
			result.addError("INVALID_MANIFEST", manifestFile.Name, err.Error())
		} else {
			normalizeManifest(manifest)
			result.Metadata = manifest
		}
	} else if hasSkill && result.Profile == ProfileDefault {
		result.Metadata = v.extractFrontmatterManifest(skillFile.Content)
	}

	requireCanonical := result.Profile == ProfileStrict || result.Profile == ProfilePublish
	if requireCanonical {
		if !hasVersion {
			result.addError("MISSING_VERSION", "", "VERSION is required")
		}
		if !hasManifest {
			result.addError("MISSING_SKILL_YAML", "", "skill.yaml is required")
		}
		if !hasChangelog {
			result.addError("MISSING_CHANGELOG", "", "CHANGELOG.md is required")
		}
	}

	versionText := strings.TrimSpace(string(versionFile.Content))
	if hasVersion {
		if err := ValidateVersion(versionText); err != nil {
			result.addError("INVALID_VERSION", versionFile.Name, err.Error())
		}
	}
	if result.Metadata != nil {
		validateManifest(result.Metadata, result, requireCanonical)
		if hasVersion && result.Metadata.Version != "" && result.Metadata.Version != versionText {
			result.addError("VERSION_MISMATCH", "VERSION", fmt.Sprintf("skill.yaml version %q does not match VERSION %q", result.Metadata.Version, versionText))
		}
		if result.Metadata.Namespace == "" {
			result.Metadata.Namespace = "default"
		}
		if result.Metadata.Entrypoint == "" {
			result.Metadata.Entrypoint = "SKILL.md"
		}
		if result.Metadata.Entrypoint != "" {
			if _, ok := byPath[trimRootPrefix(result.Metadata.Entrypoint, result.Files)]; !ok {
				if _, ok := findByBase(byPath, filepath.Base(result.Metadata.Entrypoint)); !ok {
					result.addError("ENTRYPOINT_MISSING", result.Metadata.Entrypoint, "entrypoint file not found")
				}
			}
		}
	}
	if hasChangelog && hasVersion && !changelogContainsVersion(changelogFile.Content, versionText) {
		result.addError("CHANGELOG_ENTRY_MISSING", changelogFile.Name, fmt.Sprintf("CHANGELOG.md must contain an entry for %s", versionText))
	}
}

func (v *Validator) validatePath(path string) error {
	if filepath.IsAbs(path) || strings.HasPrefix(path, "/") {
		return fmt.Errorf("absolute path not allowed: %s", path)
	}
	cleaned := filepath.Clean(path)
	if cleaned == "." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) || cleaned == ".." || strings.Contains(cleaned, string(os.PathSeparator)+".."+string(os.PathSeparator)) {
		return fmt.Errorf("path traversal not allowed: %s", path)
	}
	if strings.Contains(path, `\`) {
		return fmt.Errorf("backslash paths are not allowed: %s", path)
	}
	return nil
}

func ParseSkillManifest(content []byte) (*metadata.SkillManifest, error) {
	var manifest metadata.SkillManifest
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return nil, err
	}
	normalizeManifest(&manifest)
	return &manifest, nil
}

func normalizeManifest(manifest *metadata.SkillManifest) {
	if manifest.Namespace == "" {
		manifest.Namespace = "default"
	}
	if manifest.Entrypoint == "" {
		manifest.Entrypoint = "SKILL.md"
	}
	if len(manifest.CompatibleWith) == 0 && len(manifest.Compatibility) > 0 {
		for platform := range manifest.Compatibility {
			manifest.CompatibleWith = append(manifest.CompatibleWith, platform)
		}
		sort.Strings(manifest.CompatibleWith)
	}
}

func validateManifest(manifest *metadata.SkillManifest, result *ValidationResult, requireCanonical bool) {
	if manifest.Name == "" {
		if requireCanonical {
			result.addError("MANIFEST_NAME_REQUIRED", "skill.yaml", "skill.yaml name is required")
		}
	} else if err := ValidateSkillName(manifest.Name); err != nil {
		result.addError("INVALID_NAME", "skill.yaml", err.Error())
	}
	if manifest.Namespace != "" {
		if err := ValidateNamespace(manifest.Namespace); err != nil {
			result.addError("INVALID_NAMESPACE", "skill.yaml", err.Error())
		}
	}
	if manifest.Version == "" {
		if requireCanonical {
			result.addError("MANIFEST_VERSION_REQUIRED", "skill.yaml", "skill.yaml version is required")
		}
	} else if err := ValidateVersion(manifest.Version); err != nil {
		result.addError("INVALID_VERSION", "skill.yaml", err.Error())
	}
	if requireCanonical {
		if strings.TrimSpace(manifest.Description) == "" {
			result.addError("DESCRIPTION_REQUIRED", "skill.yaml", "description is required")
		}
		if len(manifest.CompatibleWith) == 0 {
			result.addError("COMPATIBLE_WITH_REQUIRED", "skill.yaml", "compatible_with is required")
		}
		if strings.TrimSpace(manifest.Entrypoint) == "" {
			result.addError("ENTRYPOINT_REQUIRED", "skill.yaml", "entrypoint is required")
		}
	}
	for _, platform := range manifest.CompatibleWith {
		if !supportedPlatforms[platform] {
			result.addError("UNKNOWN_PLATFORM", "skill.yaml", fmt.Sprintf("unknown compatible_with platform %q", platform))
		}
	}
	if len(manifest.Owners) == 0 {
		result.addWarning("OWNERS_RECOMMENDED", "skill.yaml", "owners are recommended")
	}
	if manifest.License == "" {
		result.addWarning("LICENSE_RECOMMENDED", "skill.yaml", "license is recommended")
	}
}

func (v *Validator) scanContent(path string, content []byte, result *ValidationResult) {
	lower := strings.ToLower(path)
	text := string(content)
	if strings.Contains(text, "http://") || strings.Contains(text, "https://") {
		result.addWarning("EXTERNAL_URL", path, "external URLs should be reviewed")
	}
	if strings.HasSuffix(lower, ".sh") || strings.Contains(text, "```bash") || strings.Contains(text, "```sh") {
		result.addWarning("SCRIPT_CONTENT", path, "scripts or command snippets should be declared in security metadata")
	}
	for _, re := range secretRegexes {
		if re.Match(content) {
			result.addError("LIKELY_SECRET", path, "likely secret detected")
			return
		}
	}
}

func (v *Validator) extractFrontmatterManifest(skillMDContent []byte) *metadata.SkillManifest {
	manifest := &metadata.SkillManifest{Entrypoint: "SKILL.md", Namespace: "default"}
	scanner := bufio.NewScanner(bytes.NewReader(skillMDContent))
	var lines []string
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
			lines = append(lines, line)
		}
	}
	if foundFrontmatter && len(lines) > 0 {
		_ = yaml.Unmarshal([]byte(strings.Join(lines, "\n")), manifest)
		normalizeManifest(manifest)
	}
	return manifest
}

func (v *Validator) extractManifest(skillMDContent []byte) *metadata.SkillManifest {
	return v.extractFrontmatterManifest(skillMDContent)
}

func normalizeArchivePath(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	path = strings.TrimPrefix(path, "./")
	return path
}

func findByBase(files map[string]packageFile, base string) (packageFile, bool) {
	for path, file := range files {
		if filepath.Base(path) == base {
			return file, true
		}
	}
	return packageFile{}, false
}

func trimRootPrefix(path string, all []string) string {
	path = filepath.ToSlash(path)
	for _, candidate := range all {
		if candidate == path {
			return path
		}
		parts := strings.Split(candidate, "/")
		if len(parts) > 1 && strings.Join(parts[1:], "/") == path {
			return candidate
		}
	}
	return path
}

func changelogContainsVersion(content []byte, version string) bool {
	text := string(content)
	return strings.Contains(text, "## "+version) || strings.Contains(text, "["+version+"]")
}

func ignoredTree(path string) bool {
	parts := strings.Split(path, "/")
	for _, p := range parts {
		switch p {
		case "node_modules", ".venv", "target", "dist", ".cache", "__pycache__":
			return true
		}
	}
	return false
}

func looksExecutable(mode os.FileMode, path string) bool {
	if mode&0111 != 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".sh" || ext == ".bash" || ext == ".ps1" || ext == ".bat" || ext == ".cmd"
}

func declaredExecutable(path string) bool {
	base := filepath.Base(path)
	return base == "SKILL.md" || strings.HasPrefix(path, "examples/")
}
