package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/skillforge/skill-registry/internal/metadata"
	"github.com/skillforge/skill-registry/internal/packaging"
	"github.com/skillforge/skill-registry/internal/validation"
	"github.com/skillforge/skill-registry/pkg/client"
	"github.com/skillforge/skill-registry/pkg/install"
	"gopkg.in/yaml.v3"
)

const version = "1.0.0"

type Config struct {
	Registry string `yaml:"registry"`
	Token    string `yaml:"token"`
}

type SkillsConfig struct {
	Registries map[string]RegistryRef `yaml:"registries" json:"registries"`
	Skills     []DesiredSkill         `yaml:"skills" json:"skills"`
}

type RegistryRef struct {
	URL string `yaml:"url" json:"url"`
}

type DesiredSkill struct {
	Name      string `yaml:"name" json:"name"`
	Namespace string `yaml:"namespace" json:"namespace"`
	Version   string `yaml:"version" json:"version"`
	Registry  string `yaml:"registry" json:"registry"`
	Target    string `yaml:"target" json:"target"`
}

type SkillsLock struct {
	LockfileVersion int           `yaml:"lockfile_version" json:"lockfile_version"`
	GeneratedAt     string        `yaml:"generated_at" json:"generated_at"`
	Skills          []LockedSkill `yaml:"skills" json:"skills"`
}

type LockedSkill struct {
	Name           string   `yaml:"name" json:"name"`
	Namespace      string   `yaml:"namespace" json:"namespace"`
	Version        string   `yaml:"version" json:"version"`
	Constraint     string   `yaml:"constraint" json:"constraint"`
	Registry       string   `yaml:"registry" json:"registry"`
	RegistryURL    string   `yaml:"registry_url" json:"registry_url"`
	Artifact       string   `yaml:"artifact" json:"artifact"`
	SHA256         string   `yaml:"sha256" json:"sha256"`
	CompatibleWith []string `yaml:"compatible_with,omitempty" json:"compatible_with,omitempty"`
	InstalledTo    []string `yaml:"installed_to" json:"installed_to"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "login":
		loginCommand()
	case "search":
		searchCommand()
	case "info":
		infoCommand()
	case "validate":
		validateCommand()
	case "package":
		packageCommand()
	case "publish":
		publishCommand()
	case "init":
		initCommand()
	case "add":
		addCommand()
	case "remove":
		removeCommand()
	case "lock":
		lockCommand()
	case "install":
		installCommand()
	case "verify":
		verifyCommand()
	case "outdated":
		outdatedCommand()
	case "update":
		updateCommand()
	case "version":
		fmt.Printf("skpm version %s\n", version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("skpm - Skill Package Manager")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  skpm login")
	fmt.Println("  skpm search <query>")
	fmt.Println("  skpm info <namespace>/<name>")
	fmt.Println("  skpm validate <path-or-dir> [--strict|--publish] [--output json]")
	fmt.Println("  skpm package <skill-dir> [--output-dir dist] [--format tgz|zip] [--source-commit sha] [--provenance] [--sign] [--dry-run]")
	fmt.Println("  skpm publish <path-or-dir> --registry <url>")
	fmt.Println("  skpm init")
	fmt.Println("  skpm add <namespace>/<name>@<constraint>")
	fmt.Println("  skpm remove <namespace>/<name>")
	fmt.Println("  skpm lock")
	fmt.Println("  skpm install <namespace>/<name>@<version> --target <dir> [--force]")
	fmt.Println("  skpm install --frozen-lockfile [--force] [--check]")
	fmt.Println("  skpm verify")
	fmt.Println("  skpm outdated")
	fmt.Println("  skpm update")
	fmt.Println("  skpm version")
}

// ── config ────────────────────────────────────────────────────────────────────

func getConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	configPath := filepath.Join(homeDir, ".skpm", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{
				Registry: os.Getenv("SKILL_REGISTRY_URL"),
				Token:    os.Getenv("SKILL_REGISTRY_TOKEN"),
			}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if url := os.Getenv("SKILL_REGISTRY_URL"); url != "" {
		cfg.Registry = url
	}
	if token := os.Getenv("SKILL_REGISTRY_TOKEN"); token != "" {
		cfg.Token = token
	}
	return &cfg, nil
}

func saveConfig(cfg *Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configDir := filepath.Join(homeDir, ".skpm")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, "config.yaml"), data, 0600)
}

// ── commands ──────────────────────────────────────────────────────────────────

func loginCommand() {
	fmt.Print("Registry URL: ")
	var registryURL string
	fmt.Scanln(&registryURL)

	fmt.Print("Username: ")
	var username string
	fmt.Scanln(&username)

	fmt.Print("Password: ")
	var password string
	fmt.Scanln(&password)

	c := client.NewClient(registryURL, "")
	loginResp, err := c.Login(username, password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
		os.Exit(1)
	}
	if err := saveConfig(&Config{Registry: registryURL, Token: loginResp.Token}); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Login successful\n")
	fmt.Printf("   User: %s\n", loginResp.User)
	fmt.Printf("   Role: %s\n", loginResp.Role)
	fmt.Printf("   Scopes: %s\n", strings.Join(loginResp.Scopes, ", "))
}

func searchCommand() {
	query := ""
	if len(os.Args) >= 3 {
		query = os.Args[2]
	}
	cfg, err := getConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}
	if cfg.Registry == "" {
		fmt.Fprintln(os.Stderr, "No registry URL configured. Run 'skpm login'")
		os.Exit(1)
	}
	c := client.NewClient(cfg.Registry, cfg.Token)
	result, err := c.ListSkills(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to search: %v\n", err)
		os.Exit(1)
	}
	if len(result.Skills) == 0 {
		fmt.Println("No skills found")
		return
	}
	for _, skill := range result.Skills {
		fmt.Printf("%s/%s@%s - %s\n", skill.Namespace, skill.Name, skill.LatestVersion, skill.Description)
	}
}

func infoCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skpm info <namespace>/<name>")
		os.Exit(1)
	}
	parts := strings.Split(os.Args[2], "/")
	if len(parts) != 2 {
		fmt.Fprintln(os.Stderr, "Invalid format. Expected: namespace/name")
		os.Exit(1)
	}
	cfg, err := getConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}
	if cfg.Registry == "" {
		fmt.Fprintln(os.Stderr, "No registry URL configured. Run 'skpm login'")
		os.Exit(1)
	}
	c := client.NewClient(cfg.Registry, cfg.Token)
	skill, versions, err := c.GetSkill(parts[0], parts[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get skill info: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Name: %s/%s\n", skill.Namespace, skill.Name)
	fmt.Printf("Description: %s\n", skill.Description)
	fmt.Printf("Latest Version: %s\n", skill.LatestVersion)
	fmt.Printf("Tags: %s\n", strings.Join(skill.Tags, ", "))
	fmt.Printf("\nVersions:\n")
	for _, v := range versions {
		fmt.Printf("  - %s (%d pulls, created: %s)\n", v.Version, v.Downloads, v.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	tags, err := c.ListDistTags(parts[0], parts[1])
	if err == nil && len(tags) > 0 {
		fmt.Printf("\nDist-tags:\n")
		for tag, ver := range tags {
			fmt.Printf("  %s -> %s\n", tag, ver)
		}
	}
}

func validateCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skpm validate <path-or-dir> [--strict|--publish] [--output json]")
		os.Exit(1)
	}
	pathArg := os.Args[2]
	profile := validation.ProfileDefault
	output := "text"
	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--strict":
			profile = validation.ProfileStrict
		case "--publish":
			profile = validation.ProfilePublish
		case "--output":
			if i+1 < len(os.Args) {
				output = os.Args[i+1]
				i++
			}
		}
	}
	info, err := os.Stat(pathArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stat path: %v\n", err)
		os.Exit(1)
	}
	validator := validation.NewValidator(1024, nil)
	var result *validation.ValidationResult
	if info.IsDir() {
		result, err = validator.ValidateDirectory(pathArg, profile)
	} else {
		data, readErr := os.ReadFile(pathArg)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to read file: %v\n", readErr)
			os.Exit(1)
		}
		packageType := "tgz"
		if strings.HasSuffix(pathArg, ".zip") {
			packageType = "zip"
		}
		result, err = validator.ValidatePackageWithProfile(data, packageType, profile)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to validate: %v\n", err)
		os.Exit(1)
	}
	if output == "json" {
		_ = json.NewEncoder(os.Stdout).Encode(result)
	} else {
		if result.Valid {
			fmt.Printf("Package is valid (%s)\n", result.Profile)
		} else {
			fmt.Printf("Package is invalid (%s)\n", result.Profile)
		}
		printIssues(result.Errors, "Errors")
		printIssues(result.Warnings, "Warnings")
	}
	if !result.Valid {
		os.Exit(1)
	}
}

func packageCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skpm package <skill-dir> [--output-dir dist] [--format tgz|zip] [--source-commit sha] [--provenance] [--sign] [--dry-run] [--output json]")
		os.Exit(1)
	}
	opts := packaging.Options{Format: "tgz", OutputDir: "dist"}
	output := "text"
	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--output-dir":
			if i+1 < len(os.Args) {
				opts.OutputDir = os.Args[i+1]
				i++
			}
		case "--format":
			if i+1 < len(os.Args) {
				opts.Format = os.Args[i+1]
				i++
			}
		case "--source-commit":
			if i+1 < len(os.Args) {
				opts.SourceCommit = os.Args[i+1]
				i++
			}
		case "--provenance":
			opts.Provenance = true
		case "--sign":
			opts.Sign = true
		case "--dry-run":
			opts.DryRun = true
		case "--output":
			if i+1 < len(os.Args) {
				output = os.Args[i+1]
				i++
			}
		}
	}
	result, _, err := packaging.Build(os.Args[2], opts, validation.NewValidator(1024, nil))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to package skill: %v\n", err)
		os.Exit(1)
	}
	if output == "json" {
		_ = json.NewEncoder(os.Stdout).Encode(result)
		return
	}
	if opts.DryRun {
		fmt.Printf("Dry run: would create %s\n", result.FileName)
	} else {
		fmt.Printf("Packaged %s/%s@%s\n", result.Namespace, result.Name, result.Version)
		fmt.Printf("Package: %s\n", result.Path)
	}
	fmt.Printf("Digest: sha256:%s\n", result.SHA256)
	if result.Signature != "" {
		fmt.Printf("Signature: %s\n", result.Signature)
	}
}

func publishCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skpm publish <path-or-dir> --registry <url>")
		os.Exit(1)
	}
	pathOrArchive := os.Args[2]
	registryURL, namespaceOverride, nameOverride, versionOverride := "", "", "", ""
	force, dryRun, sign := false, false, false
	output := "text"

	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--registry":
			if i+1 < len(os.Args) {
				registryURL = os.Args[i+1]
				i++
			}
		case "--namespace":
			if i+1 < len(os.Args) {
				namespaceOverride = os.Args[i+1]
				i++
			}
		case "--name":
			if i+1 < len(os.Args) {
				nameOverride = os.Args[i+1]
				i++
			}
		case "--version":
			if i+1 < len(os.Args) {
				versionOverride = os.Args[i+1]
				i++
			}
		case "--force":
			force = true
		case "--dry-run":
			dryRun = true
		case "--sign":
			sign = true
		case "--output":
			if i+1 < len(os.Args) {
				output = os.Args[i+1]
				i++
			}
		}
	}

	cfg, err := getConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}
	if registryURL == "" {
		registryURL = cfg.Registry
	}
	if registryURL == "" {
		fmt.Fprintln(os.Stderr, "No registry URL specified. Use --registry or run 'skpm login'")
		os.Exit(1)
	}

	info, err := os.Stat(pathOrArchive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stat path: %v\n", err)
		os.Exit(1)
	}

	var data []byte
	var contentType string
	packageName := filepath.Base(pathOrArchive)

	if info.IsDir() {
		result, pkgData, err := packaging.Build(pathOrArchive, packaging.Options{Format: "tgz", DryRun: true, Sign: sign}, validation.NewValidator(1024, nil))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to package skill: %v\n", err)
			os.Exit(1)
		}
		data = pkgData
		contentType = "application/gzip"
		packageName = result.FileName
	} else {
		data, err = os.ReadFile(pathOrArchive)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read file: %v\n", err)
			os.Exit(1)
		}
		if strings.HasSuffix(pathOrArchive, ".zip") {
			contentType = "application/zip"
		} else {
			contentType = "application/gzip"
		}
	}

	namespace, name, ver, err := extractMetadata(data, contentType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to extract metadata: %v\n", err)
		os.Exit(1)
	}
	if namespaceOverride != "" && namespaceOverride != namespace {
		fmt.Fprintf(os.Stderr, "Metadata mismatch: --namespace %q does not match skill.yaml namespace %q\n", namespaceOverride, namespace)
		os.Exit(1)
	}
	if nameOverride != "" && nameOverride != name {
		fmt.Fprintf(os.Stderr, "Metadata mismatch: --name %q does not match skill.yaml name %q\n", nameOverride, name)
		os.Exit(1)
	}
	if versionOverride != "" && versionOverride != ver {
		fmt.Fprintf(os.Stderr, "Metadata mismatch: --version %q does not match VERSION %q\n", versionOverride, ver)
		os.Exit(1)
	}

	if dryRun {
		fmt.Printf("Dry run: would publish %s/%s@%s\n", namespace, name, ver)
		return
	}

	signature := ""
	if sign {
		sum := sha256.Sum256(data)
		signature = packaging.LocalSignature(hex.EncodeToString(sum[:]))
	}

	c := client.NewClient(registryURL, cfg.Token)
	skillVersion, err := c.PublishWithOptions(namespace, name, ver, data, contentType, force, signature)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to publish: %v\n", err)
		os.Exit(1)
	}

	if output == "json" {
		_ = json.NewEncoder(os.Stdout).Encode(skillVersion)
		return
	}
	fmt.Printf("Published %s/%s@%s\n", namespace, name, ver)
	fmt.Printf("Digest: sha256:%s\n", skillVersion.DigestSHA256)
	if signature != "" {
		fmt.Printf("Signature: %s\n", signature)
	}
	fmt.Printf("Package: %s\n", packageName)
}

func initCommand() {
	if _, err := os.Stat("agent-skills.yaml"); err == nil {
		fmt.Println("agent-skills.yaml already exists")
		return
	}
	cfg, _ := getConfig()
	registryURL := cfg.Registry
	if registryURL == "" {
		registryURL = "http://localhost:8080"
	}
	config := SkillsConfig{
		Registries: map[string]RegistryRef{"default": {URL: registryURL}},
		Skills:     []DesiredSkill{},
	}
	if err := writeYAML("agent-skills.yaml", config); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write agent-skills.yaml: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created agent-skills.yaml")
}

func addCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skpm add <namespace>/<name>@<constraint>")
		os.Exit(1)
	}
	config := readSkillsConfigOrDefault()
	ns, name, constraint, err := parseSpec(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for i, s := range config.Skills {
		if s.Namespace == ns && s.Name == name {
			config.Skills[i].Version = constraint
			config.Skills[i].Registry = defaultString(config.Skills[i].Registry, "default")
			config.Skills[i].Target = defaultString(config.Skills[i].Target, filepath.ToSlash(filepath.Join(".agents", "skills", name)))
			_ = writeYAML("agent-skills.yaml", config)
			fmt.Printf("Updated %s/%s@%s\n", ns, name, constraint)
			return
		}
	}
	config.Skills = append(config.Skills, DesiredSkill{
		Name: name, Namespace: ns, Version: constraint,
		Registry: "default", Target: filepath.ToSlash(filepath.Join(".agents", "skills", name)),
	})
	sort.Slice(config.Skills, func(i, j int) bool {
		return config.Skills[i].Namespace+"/"+config.Skills[i].Name < config.Skills[j].Namespace+"/"+config.Skills[j].Name
	})
	if err := writeYAML("agent-skills.yaml", config); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to update agent-skills.yaml: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Added %s/%s@%s\n", ns, name, constraint)
}

func removeCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skpm remove <namespace>/<name>")
		os.Exit(1)
	}
	config := readSkillsConfigOrDefault()
	parts := strings.Split(os.Args[2], "/")
	if len(parts) != 2 {
		fmt.Fprintln(os.Stderr, "Invalid format. Expected namespace/name")
		os.Exit(1)
	}
	var kept []DesiredSkill
	for _, s := range config.Skills {
		if !(s.Namespace == parts[0] && s.Name == parts[1]) {
			kept = append(kept, s)
		}
	}
	config.Skills = kept
	if err := writeYAML("agent-skills.yaml", config); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to update agent-skills.yaml: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Removed %s\n", os.Args[2])
}

func lockCommand() {
	config := readSkillsConfigOrDefault()
	cfg, _ := getConfig()
	lock := SkillsLock{
		LockfileVersion: 1,
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		Skills:          []LockedSkill{},
	}
	for _, desired := range config.Skills {
		registryURL := registryURLFor(config, desired.Registry, cfg.Registry)
		c := client.NewClient(registryURL, cfg.Token)
		skill, versions, err := c.GetSkill(desired.Namespace, desired.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to resolve %s/%s: %v\n", desired.Namespace, desired.Name, err)
			os.Exit(1)
		}
		resolved := resolveConstraint(versions, desired.Version)
		if resolved == nil {
			fmt.Fprintf(os.Stderr, "No version of %s/%s satisfies %s\n", desired.Namespace, desired.Name, desired.Version)
			os.Exit(1)
		}
		target := defaultString(desired.Target, filepath.ToSlash(filepath.Join(".agents", "skills", desired.Name)))
		compatible := []string{}
		if resolved.Manifest != nil {
			compatible = resolved.Manifest.CompatibleWith
		}
		lock.Skills = append(lock.Skills, LockedSkill{
			Name: desired.Name, Namespace: desired.Namespace,
			Version: resolved.Version, Constraint: desired.Version,
			Registry: defaultString(desired.Registry, "default"), RegistryURL: registryURL,
			Artifact:       fmt.Sprintf("%s-%s.%s", skill.Name, resolved.Version, resolved.PackageType),
			SHA256:         resolved.DigestSHA256,
			CompatibleWith: compatible, InstalledTo: []string{target},
		})
	}
	sort.Slice(lock.Skills, func(i, j int) bool {
		return lock.Skills[i].Namespace+"/"+lock.Skills[i].Name < lock.Skills[j].Namespace+"/"+lock.Skills[j].Name
	})
	if err := writeYAML("agent-skills.lock", lock); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write agent-skills.lock: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote agent-skills.lock with %d skill(s)\n", len(lock.Skills))
}

func installCommand() {
	if len(os.Args) >= 3 && strings.HasPrefix(os.Args[2], "--") {
		installFromLockfile()
		return
	}
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skpm install <namespace>/<name>@<version> --target <dir> [--force] [--check]")
		os.Exit(1)
	}

	spec := os.Args[2]
	targetDir := ".agents/skills"
	force, checkOnly := false, false

	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--target":
			if i+1 < len(os.Args) {
				targetDir = os.Args[i+1]
				i++
			}
		case "--force":
			force = true
		case "--check":
			checkOnly = true
		}
	}

	atIndex := strings.LastIndex(spec, "@")
	if atIndex == -1 {
		fmt.Fprintln(os.Stderr, "Invalid format. Expected: namespace/name@version")
		os.Exit(1)
	}
	parts := strings.Split(spec[:atIndex], "/")
	if len(parts) != 2 {
		fmt.Fprintln(os.Stderr, "Invalid format. Expected: namespace/name@version")
		os.Exit(1)
	}
	namespace, name, ver := parts[0], parts[1], spec[atIndex+1:]

	cfg, err := getConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}
	if cfg.Registry == "" {
		fmt.Fprintln(os.Stderr, "No registry URL configured. Run 'skpm login'")
		os.Exit(1)
	}

	c := client.NewClient(cfg.Registry, cfg.Token)
	data, digest, contentType, err := c.Download(namespace, name, ver)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to download: %v\n", err)
		os.Exit(1)
	}

	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != digest {
		fmt.Fprintln(os.Stderr, "Digest mismatch! Package may be corrupted.")
		os.Exit(1)
	}
	if checkOnly {
		fmt.Printf("Package verified: %s/%s@%s sha256:%s\n", namespace, name, ver, digest)
		return
	}

	skillDir := filepath.Join(targetDir, name)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create target directory: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stat(skillDir); err == nil {
		if !force {
			fmt.Fprintf(os.Stderr, "Skill already exists at %s. Use --force to overwrite.\n", skillDir)
			os.Exit(1)
		}
		if err := os.RemoveAll(skillDir); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to remove existing skill: %v\n", err)
			os.Exit(1)
		}
	}

	var extractErr error
	if strings.HasPrefix(contentType, "application/zip") {
		extractErr = install.ExtractZip(data, skillDir)
	} else {
		extractErr = install.ExtractTarGz(data, skillDir)
	}
	if extractErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to extract: %v\n", extractErr)
		os.Exit(1)
	}
	_ = install.WriteMarker(skillDir, install.Marker{
		Name: name, Namespace: namespace, Version: ver, Digest: digest,
		Registry: cfg.Registry, InstalledAt: time.Now().UTC(), Files: install.ListFiles(skillDir),
	})
	fmt.Printf("Installed %s/%s@%s to %s\n", namespace, name, ver, skillDir)
}

func installFromLockfile() {
	frozen, checkOnly, force := false, false, false
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--frozen-lockfile":
			frozen = true
		case "--check":
			checkOnly = true
		case "--force":
			force = true
		}
	}
	if frozen {
		ensureLockMatchesManifest()
	}
	lock := readLock()
	cfg, _ := getConfig()
	for _, skill := range lock.Skills {
		c := client.NewClient(skill.RegistryURL, cfg.Token)
		data, digest, contentType, err := c.Download(skill.Namespace, skill.Name, skill.Version)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to download %s/%s@%s: %v\n", skill.Namespace, skill.Name, skill.Version, err)
			os.Exit(1)
		}
		if digest != skill.SHA256 {
			fmt.Fprintf(os.Stderr, "Digest mismatch for %s/%s@%s\n", skill.Namespace, skill.Name, skill.Version)
			os.Exit(1)
		}
		if checkOnly {
			continue
		}
		for _, target := range skill.InstalledTo {
			if _, err := os.Stat(target); err == nil {
				if !force {
					fmt.Fprintf(os.Stderr, "%s already exists. Use --force to overwrite.\n", target)
					os.Exit(1)
				}
				_ = os.RemoveAll(target)
			}
			if strings.HasPrefix(contentType, "application/zip") {
				err = install.ExtractZip(data, target)
			} else {
				err = install.ExtractTarGz(data, target)
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to install %s/%s: %v\n", skill.Namespace, skill.Name, err)
				os.Exit(1)
			}
			_ = install.WriteMarker(target, install.Marker{
				Name: skill.Name, Namespace: skill.Namespace, Version: skill.Version,
				Digest: digest, Registry: skill.RegistryURL,
				InstalledAt: time.Now().UTC(), Files: install.ListFiles(target),
			})
			fmt.Printf("Installed %s/%s@%s to %s\n", skill.Namespace, skill.Name, skill.Version, target)
		}
	}
	if checkOnly {
		fmt.Println("Lockfile install check passed")
	}
}

func verifyCommand() {
	lock := readLock()
	failed := false
	for _, skill := range lock.Skills {
		for _, target := range skill.InstalledTo {
			markerPath := filepath.Join(target, ".agent-skill-install.json")
			data, err := os.ReadFile(markerPath)
			if err != nil {
				fmt.Printf("FAIL %s/%s missing install marker at %s\n", skill.Namespace, skill.Name, markerPath)
				failed = true
				continue
			}
			var marker install.Marker
			if err := json.Unmarshal(data, &marker); err != nil || marker.Digest != skill.SHA256 || marker.Version != skill.Version {
				fmt.Printf("FAIL %s/%s marker does not match lockfile\n", skill.Namespace, skill.Name)
				failed = true
				continue
			}
			fmt.Printf("OK   %s/%s@%s\n", skill.Namespace, skill.Name, skill.Version)
		}
	}
	if failed {
		os.Exit(1)
	}
}

func outdatedCommand() {
	config := readSkillsConfigOrDefault()
	lock := readLock()
	cfg, _ := getConfig()
	for _, locked := range lock.Skills {
		c := client.NewClient(locked.RegistryURL, cfg.Token)
		_, versions, err := c.GetSkill(locked.Namespace, locked.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to check %s/%s: %v\n", locked.Namespace, locked.Name, err)
			continue
		}
		constraint := locked.Constraint
		for _, desired := range config.Skills {
			if desired.Namespace == locked.Namespace && desired.Name == locked.Name {
				constraint = desired.Version
			}
		}
		latest := resolveConstraint(versions, constraint)
		if latest != nil && latest.Version != locked.Version {
			fmt.Printf("%s/%s %s -> %s\n", locked.Namespace, locked.Name, locked.Version, latest.Version)
		}
	}
}

func updateCommand() {
	lockCommand()
}

// ── helpers ───────────────────────────────────────────────────────────────────

func printIssues(items []string, title string) {
	if len(items) == 0 {
		return
	}
	fmt.Printf("\n%s:\n", title)
	for _, item := range items {
		fmt.Printf("  - %s\n", item)
	}
}

func parseSpec(spec string) (namespace, name, version string, err error) {
	atIndex := strings.LastIndex(spec, "@")
	if atIndex == -1 {
		return "", "", "", fmt.Errorf("invalid format. Expected namespace/name@version")
	}
	parts := strings.Split(spec[:atIndex], "/")
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid format. Expected namespace/name@version")
	}
	return parts[0], parts[1], spec[atIndex+1:], nil
}

func extractMetadata(data []byte, contentType string) (namespace, name, version string, err error) {
	packageType := "tgz"
	if contentType == "application/zip" {
		packageType = "zip"
	}
	result, err := validation.NewValidator(1024, nil).ValidatePackageWithProfile(data, packageType, validation.ProfilePublish)
	if err != nil {
		return "", "", "", err
	}
	if !result.Valid {
		return "", "", "", fmt.Errorf("%s", strings.Join(result.Errors, "; "))
	}
	if result.Metadata == nil {
		return "", "", "", fmt.Errorf("skill.yaml and VERSION are required")
	}
	return result.Metadata.Namespace, result.Metadata.Name, result.Metadata.Version, nil
}

func readSkillsConfigOrDefault() SkillsConfig {
	var config SkillsConfig
	data, err := os.ReadFile("agent-skills.yaml")
	if err == nil {
		_ = yaml.Unmarshal(data, &config)
	}
	if config.Registries == nil {
		cfg, _ := getConfig()
		url := defaultString(cfg.Registry, "http://localhost:8080")
		config.Registries = map[string]RegistryRef{"default": {URL: url}}
	}
	return config
}

func readLock() SkillsLock {
	data, err := os.ReadFile("agent-skills.lock")
	if err != nil {
		fmt.Fprintln(os.Stderr, "agent-skills.lock not found. Run 'skpm lock' first.")
		os.Exit(1)
	}
	var lock SkillsLock
	if err := yaml.Unmarshal(data, &lock); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read agent-skills.lock: %v\n", err)
		os.Exit(1)
	}
	return lock
}

func writeYAML(path string, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func registryURLFor(config SkillsConfig, name, fallback string) string {
	if name == "" {
		name = "default"
	}
	if ref, ok := config.Registries[name]; ok && ref.URL != "" {
		return ref.URL
	}
	return defaultString(fallback, "http://localhost:8080")
}

func resolveConstraint(versions []metadata.SkillVersion, constraint string) *metadata.SkillVersion {
	var candidates []metadata.SkillVersion
	for _, v := range versions {
		if v.Yanked {
			continue
		}
		if constraint == "" || constraint == "*" || constraint == "latest" || v.Version == constraint || caretAllows(constraint, v.Version) {
			candidates = append(candidates, v)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return compareVersionStrings(candidates[i].Version, candidates[j].Version) > 0 })
	return &candidates[0]
}

func caretAllows(constraint, ver string) bool {
	if !strings.HasPrefix(constraint, "^") {
		return false
	}
	base := strings.TrimPrefix(constraint, "^")
	bp := versionParts(base)
	vp := versionParts(ver)
	return vp[0] == bp[0] && compareVersionStrings(ver, base) >= 0
}

func compareVersionStrings(a, b string) int {
	ap, bp := versionParts(a), versionParts(b)
	for i := 0; i < 3; i++ {
		if ap[i] > bp[i] {
			return 1
		}
		if ap[i] < bp[i] {
			return -1
		}
	}
	return strings.Compare(a, b)
}

func versionParts(v string) [3]int {
	base := strings.Split(strings.Split(v, "+")[0], "-")[0]
	parts := strings.Split(base, ".")
	var out [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		n, _ := strconv.Atoi(parts[i])
		out[i] = n
	}
	return out
}

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func ensureLockMatchesManifest() {
	config := readSkillsConfigOrDefault()
	lock := readLock()
	want := map[string]string{}
	for _, skill := range config.Skills {
		want[skill.Namespace+"/"+skill.Name] = skill.Version
	}
	for _, skill := range lock.Skills {
		if want[skill.Namespace+"/"+skill.Name] != skill.Constraint {
			fmt.Fprintf(os.Stderr, "Lockfile is stale for %s/%s. Run 'skpm lock'.\n", skill.Namespace, skill.Name)
			os.Exit(1)
		}
	}
}
