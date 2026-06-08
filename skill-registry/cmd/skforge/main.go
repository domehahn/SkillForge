package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	"gopkg.in/yaml.v3"
)

const version = "1.0.0"

type Config struct {
	Registry string `yaml:"registry"`
	Token    string `yaml:"token"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "login":
		loginCommand()
	case "package":
		packageCommand()
	case "publish":
		publishCommand()
	case "search":
		searchCommand()
	case "info":
		infoCommand()
	case "install":
		installCommand()
	case "init":
		initCommand()
	case "add":
		addCommand()
	case "remove":
		removeCommand()
	case "lock":
		lockCommand()
	case "verify":
		verifyCommand()
	case "outdated":
		outdatedCommand()
	case "update":
		updateCommand()
	case "deprecate":
		governanceCommand("deprecate")
	case "yank":
		governanceCommand("yank")
	case "unyank":
		governanceCommand("unyank")
	case "validate":
		validateCommand()
	case "delete":
		deleteCommand()
	case "token":
		tokenCommand()
	case "dist-tag":
		distTagCommand()
	case "artifact":
		artifactCommand()
	case "namespace":
		namespaceCommand()
	case "webhook":
		webhookCommand()
	case "version":
		fmt.Printf("skforge version %s\n", version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("skforge - Skill Registry CLI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  skforge login")
	fmt.Println("  skforge package <skill-dir> [--output-dir dist] [--format tgz|zip] [--source-commit sha] [--provenance] [--sign] [--dry-run]")
	fmt.Println("  skforge publish <path-or-archive> --registry <url>")
	fmt.Println("  skforge search <query>")
	fmt.Println("  skforge info <namespace>/<name>")
	fmt.Println("  skforge install <namespace>/<name>@<version> --target <dir>")
	fmt.Println("  skforge init")
	fmt.Println("  skforge add <namespace>/<name>@<constraint>")
	fmt.Println("  skforge remove <namespace>/<name>")
	fmt.Println("  skforge lock")
	fmt.Println("  skforge install --frozen-lockfile")
	fmt.Println("  skforge verify")
	fmt.Println("  skforge outdated")
	fmt.Println("  skforge update [namespace/name]")
	fmt.Println("  skforge deprecate <namespace>/<name>@<version> --reason <reason>")
	fmt.Println("  skforge yank <namespace>/<name>@<version> --reason <reason>")
	fmt.Println("  skforge unyank <namespace>/<name>@<version>")
	fmt.Println("  skforge delete <namespace>/<name>@<version>")
	fmt.Println("  skforge validate <path-or-archive>")
	fmt.Println("  skforge token create --name <name> --scopes <scopes>")
	fmt.Println("  skforge token list")
	fmt.Println("  skforge token revoke <token-id>")
	fmt.Println("  skforge dist-tag add <namespace>/<name>@<version> <tag>")
	fmt.Println("  skforge dist-tag list <namespace>/<name>")
	fmt.Println("  skforge artifact init <kind> <directory>")
	fmt.Println("  skforge artifact publish <kind> <namespace>/<name>@<version> <path>")
	fmt.Println("  skforge artifact list [kind] [query]")
	fmt.Println("  skforge artifact info <kind> <namespace>/<name>")
	fmt.Println("  skforge artifact graph <kind> <namespace>/<name>@<version>")
	fmt.Println("  skforge artifact lock <kind> <namespace>/<name>@<version>")
	fmt.Println("  skforge artifact promote <kind> <namespace>/<name>@<version> <channel>")
	fmt.Println("  skforge artifact install <kind> <namespace>/<name>@<version> <target>")
	fmt.Println("  skforge artifact attest <kind> <namespace>/<name>@<version> <signature|scan|provenance|sbom> <digest>")
	fmt.Println("  skforge namespace member <namespace> <username> <reader|maintainer|owner>")
	fmt.Println("  skforge webhook add <namespace> <url> <event[,event]>")
	fmt.Println("  skforge version")
}

func getConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(homeDir, ".skforge", "config.yaml")
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

	// Override with environment variables
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

	configDir := filepath.Join(homeDir, ".skforge")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(configDir, "config.yaml")
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600)
}

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

	// Create client and login
	c := client.NewClient(registryURL, "")
	loginResp, err := c.Login(username, password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
		os.Exit(1)
	}

	// Save config with token
	cfg := &Config{
		Registry: registryURL,
		Token:    loginResp.Token,
	}

	if err := saveConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Login successful!\n")
	fmt.Printf("   User: %s\n", loginResp.User)
	fmt.Printf("   Role: %s\n", loginResp.Role)
	fmt.Printf("   Scopes: %s\n", strings.Join(loginResp.Scopes, ", "))
}

func publishCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skforge publish <path-or-archive> --registry <url>")
		os.Exit(1)
	}

	pathOrArchive := os.Args[2]
	registryURL := ""
	namespaceOverride, nameOverride, versionOverride := "", "", ""
	force, dryRun, sign := false, false, false
	output := "text"

	// Parse flags
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--registry" && i+1 < len(os.Args) {
			registryURL = os.Args[i+1]
			i++
		} else if os.Args[i] == "--namespace" && i+1 < len(os.Args) {
			namespaceOverride = os.Args[i+1]
			i++
		} else if os.Args[i] == "--name" && i+1 < len(os.Args) {
			nameOverride = os.Args[i+1]
			i++
		} else if os.Args[i] == "--version" && i+1 < len(os.Args) {
			versionOverride = os.Args[i+1]
			i++
		} else if os.Args[i] == "--force" {
			force = true
		} else if os.Args[i] == "--dry-run" {
			dryRun = true
		} else if os.Args[i] == "--sign" {
			sign = true
		} else if os.Args[i] == "--output" && i+1 < len(os.Args) {
			output = os.Args[i+1]
			i++
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
		fmt.Fprintln(os.Stderr, "No registry URL specified. Use --registry or run 'skforge login'")
		os.Exit(1)
	}

	// Check if path is directory or archive
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
		// Read archive file
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

	// Extract metadata to get name and version
	namespace, name, version, err := extractMetadata(pathOrArchive, data, contentType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to extract metadata: %v\n", err)
		os.Exit(1)
	}
	if namespaceOverride != "" {
		if namespaceOverride != namespace {
			fmt.Fprintf(os.Stderr, "Metadata mismatch: --namespace %q does not match skill.yaml namespace %q\n", namespaceOverride, namespace)
			os.Exit(1)
		}
	}
	if nameOverride != "" {
		if nameOverride != name {
			fmt.Fprintf(os.Stderr, "Metadata mismatch: --name %q does not match skill.yaml name %q\n", nameOverride, name)
			os.Exit(1)
		}
	}
	if versionOverride != "" {
		if versionOverride != version {
			fmt.Fprintf(os.Stderr, "Metadata mismatch: --version %q does not match VERSION %q\n", versionOverride, version)
			os.Exit(1)
		}
	}

	if dryRun {
		fmt.Printf("Dry run: would publish %s/%s@%s\n", namespace, name, version)
		return
	}
	signature := ""
	if sign {
		sum := sha256.Sum256(data)
		signature = packaging.LocalSignature(hex.EncodeToString(sum[:]))
	}

	// Publish
	c := client.NewClient(registryURL, cfg.Token)
	skillVersion, err := c.PublishWithOptions(namespace, name, version, data, contentType, force, signature)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to publish: %v\n", err)
		os.Exit(1)
	}

	if output == "json" {
		_ = json.NewEncoder(os.Stdout).Encode(skillVersion)
		return
	}
	fmt.Printf("Published %s/%s@%s\n", namespace, name, version)
	fmt.Printf("Digest: sha256:%s\n", skillVersion.DigestSHA256)
	if signature != "" {
		fmt.Printf("Signature: %s\n", signature)
	}
	fmt.Printf("Package: %s\n", packageName)
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
		fmt.Fprintln(os.Stderr, "No registry URL configured. Run 'skforge login'")
		os.Exit(1)
	}

	c := client.NewClient(cfg.Registry, cfg.Token)
	skills, err := c.ListSkills(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to search: %v\n", err)
		os.Exit(1)
	}

	if len(skills.Skills) == 0 {
		fmt.Println("No skills found")
		return
	}

	for _, skill := range skills.Skills {
		fmt.Printf("%s/%s@%s - %s\n", skill.Namespace, skill.Name, skill.LatestVersion, skill.Description)
	}
}

func infoCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skforge info <namespace>/<name>")
		os.Exit(1)
	}

	parts := strings.Split(os.Args[2], "/")
	if len(parts) != 2 {
		fmt.Fprintln(os.Stderr, "Invalid format. Expected: namespace/name")
		os.Exit(1)
	}

	namespace, name := parts[0], parts[1]

	cfg, err := getConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Registry == "" {
		fmt.Fprintln(os.Stderr, "No registry URL configured. Run 'skforge login'")
		os.Exit(1)
	}

	c := client.NewClient(cfg.Registry, cfg.Token)
	skill, versions, err := c.GetSkill(namespace, name)
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
	tags, err := c.ListDistTags(namespace, name)
	if err == nil && len(tags) > 0 {
		fmt.Printf("\nDist-tags:\n")
		for tag, version := range tags {
			fmt.Printf("  %s -> %s\n", tag, version)
		}
	}
}

func installCommand() {
	if len(os.Args) >= 3 && strings.HasPrefix(os.Args[2], "--") {
		installLockfileCommand()
		return
	}
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skforge install <namespace>/<name>@<version> --target <dir> [--force] [--check] [--prune]")
		os.Exit(1)
	}

	spec := os.Args[2]
	targetDir := ".agents/skills"
	force, checkOnly := false, false

	// Parse flags
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--target" && i+1 < len(os.Args) {
			targetDir = os.Args[i+1]
			i++
		} else if os.Args[i] == "--force" {
			force = true
		} else if os.Args[i] == "--check" {
			checkOnly = true
		}
	}

	// Parse spec: namespace/name@version
	atIndex := strings.LastIndex(spec, "@")
	if atIndex == -1 {
		fmt.Fprintln(os.Stderr, "Invalid format. Expected: namespace/name@version")
		os.Exit(1)
	}

	namespaceName := spec[:atIndex]
	version := spec[atIndex+1:]

	parts := strings.Split(namespaceName, "/")
	if len(parts) != 2 {
		fmt.Fprintln(os.Stderr, "Invalid format. Expected: namespace/name@version")
		os.Exit(1)
	}

	namespace, name := parts[0], parts[1]

	cfg, err := getConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Registry == "" {
		fmt.Fprintln(os.Stderr, "No registry URL configured. Run 'skforge login'")
		os.Exit(1)
	}

	// Download
	c := client.NewClient(cfg.Registry, cfg.Token)
	data, digest, contentType, err := c.Download(namespace, name, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to download: %v\n", err)
		os.Exit(1)
	}

	// Verify digest
	hash := sha256.Sum256(data)
	actualDigest := hex.EncodeToString(hash[:])
	if actualDigest != digest {
		fmt.Fprintln(os.Stderr, "Digest mismatch! Package may be corrupted.")
		os.Exit(1)
	}
	if checkOnly {
		fmt.Printf("Package verified: %s/%s@%s sha256:%s\n", namespace, name, version, digest)
		return
	}

	// Extract
	skillDir := filepath.Join(targetDir, name)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create target directory: %v\n", err)
		os.Exit(1)
	}

	// Check if skill already exists
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
		extractErr = extractZip(data, skillDir)
	} else {
		extractErr = extractTarGz(data, skillDir)
	}
	if extractErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to extract: %v\n", extractErr)
		os.Exit(1)
	}
	_ = writeInstallMarker(skillDir, installMarker{
		Name: name, Namespace: namespace, Version: version, Digest: digest,
		Registry: cfg.Registry, InstalledAt: time.Now().UTC(), Files: listInstalledFiles(skillDir),
	})

	fmt.Printf("Installed %s/%s@%s to %s\n", namespace, name, version, skillDir)
}

func validateCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skforge validate <path-or-archive> [--strict|--publish] [--output json]")
		os.Exit(1)
	}

	pathOrArchive := os.Args[2]
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

	// Check if path is directory or archive
	info, err := os.Stat(pathOrArchive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stat path: %v\n", err)
		os.Exit(1)
	}

	validator := validation.NewValidator(1024, nil)
	var result *validation.ValidationResult
	if info.IsDir() {
		result, err = validator.ValidateDirectory(pathOrArchive, profile)
	} else {
		var data []byte
		data, err = os.ReadFile(pathOrArchive)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read file: %v\n", err)
			os.Exit(1)
		}
		packageType := "tgz"
		if strings.HasSuffix(pathOrArchive, ".zip") {
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
			fmt.Printf("✓ Package is valid (%s)\n", result.Profile)
		} else {
			fmt.Printf("✗ Package is invalid (%s)\n", result.Profile)
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
		fmt.Fprintln(os.Stderr, "Usage: skforge package <skill-dir> [--output-dir dist] [--format tgz|zip] [--source-commit sha] [--provenance] [--sign] [--dry-run] [--output json]")
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

func printIssues(items []string, title string) {
	if len(items) == 0 {
		return
	}
	fmt.Printf("\n%s:\n", title)
	for _, item := range items {
		fmt.Printf("  - %s\n", item)
	}
}

func createTarGz(dir string) ([]byte, error) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and directories
		if strings.HasPrefix(filepath.Base(path), ".") && path != dir {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gzw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func extractTarGz(data []byte, targetDir string) error {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target, err := safeExtractionPath(targetDir, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			file, err := os.Create(target)
			if err != nil {
				return err
			}

			if _, err := io.Copy(file, tr); err != nil {
				file.Close()
				return err
			}
			file.Close()
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("unsafe symlink or hardlink rejected: %s", header.Name)
		}
	}

	return nil
}

func extractZip(data []byte, targetDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, entry := range reader.File {
		target, err := safeExtractionPath(targetDir, entry.Name)
		if err != nil {
			return err
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if entry.FileInfo().Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsafe symlink rejected: %s", entry.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		src, err := entry.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			src.Close()
			return err
		}
		_, copyErr := io.Copy(dst, src)
		src.Close()
		dst.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

func safeExtractionPath(root, name string) (string, error) {
	target := filepath.Join(root, name)
	rootWithSeparator := filepath.Clean(root) + string(os.PathSeparator)
	if target != filepath.Clean(root) && !strings.HasPrefix(filepath.Clean(target), rootWithSeparator) {
		return "", fmt.Errorf("archive path escapes target directory: %s", name)
	}
	return target, nil
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

type installMarker struct {
	Name        string    `json:"name"`
	Namespace   string    `json:"namespace"`
	Version     string    `json:"version"`
	Digest      string    `json:"digest"`
	Registry    string    `json:"registry"`
	InstalledAt time.Time `json:"installed_at"`
	Files       []string  `json:"files"`
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
		fmt.Fprintln(os.Stderr, "Usage: skforge add <namespace>/<name>@<constraint>")
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
	config.Skills = append(config.Skills, DesiredSkill{Name: name, Namespace: ns, Version: constraint, Registry: "default", Target: filepath.ToSlash(filepath.Join(".agents", "skills", name))})
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
		fmt.Fprintln(os.Stderr, "Usage: skforge remove <namespace>/<name>")
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
	lock := SkillsLock{LockfileVersion: 1, GeneratedAt: time.Now().UTC().Format(time.RFC3339), Skills: []LockedSkill{}}
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
			Name: desired.Name, Namespace: desired.Namespace, Version: resolved.Version, Constraint: desired.Version,
			Registry: defaultString(desired.Registry, "default"), RegistryURL: registryURL,
			Artifact: fmt.Sprintf("%s-%s.%s", skill.Name, resolved.Version, resolved.PackageType),
			SHA256:   resolved.DigestSHA256, CompatibleWith: compatible, InstalledTo: []string{target},
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

func installLockfileCommand() {
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
				err = extractZip(data, target)
			} else {
				err = extractTarGz(data, target)
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to install %s/%s: %v\n", skill.Namespace, skill.Name, err)
				os.Exit(1)
			}
			_ = writeInstallMarker(target, installMarker{Name: skill.Name, Namespace: skill.Namespace, Version: skill.Version, Digest: digest, Registry: skill.RegistryURL, InstalledAt: time.Now().UTC(), Files: listInstalledFiles(target)})
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
				fmt.Printf("✗ %s/%s missing install marker at %s\n", skill.Namespace, skill.Name, markerPath)
				failed = true
				continue
			}
			var marker installMarker
			if err := json.Unmarshal(data, &marker); err != nil || marker.Digest != skill.SHA256 || marker.Version != skill.Version {
				fmt.Printf("✗ %s/%s marker does not match lockfile\n", skill.Namespace, skill.Name)
				failed = true
				continue
			}
			fmt.Printf("✓ %s/%s@%s verified\n", skill.Namespace, skill.Name, skill.Version)
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

func governanceCommand(action string) {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: skforge %s <namespace>/<name>@<version> [--reason <reason>]\n", action)
		os.Exit(1)
	}
	reason := ""
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--reason" && i+1 < len(os.Args) {
			reason = os.Args[i+1]
			i++
		}
	}
	ns, name, version, err := parseSpec(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg, _ := getConfig()
	if cfg.Registry == "" {
		fmt.Fprintln(os.Stderr, "No registry URL configured. Run 'skforge login'")
		os.Exit(1)
	}
	c := client.NewClient(cfg.Registry, cfg.Token)
	if err := c.Governance(action, ns, name, version, reason); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to %s: %v\n", action, err)
		os.Exit(1)
	}
	fmt.Printf("%s %s/%s@%s\n", strings.Title(action), ns, name, version)
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
		fmt.Fprintln(os.Stderr, "agent-skills.lock not found. Run 'skforge lock' first.")
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
	for _, version := range versions {
		if version.Yanked {
			continue
		}
		if constraint == "" || constraint == "*" || constraint == "latest" || version.Version == constraint || caretAllows(constraint, version.Version) {
			candidates = append(candidates, version)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return compareVersionStrings(candidates[i].Version, candidates[j].Version) > 0 })
	return &candidates[0]
}

func caretAllows(constraint, version string) bool {
	if !strings.HasPrefix(constraint, "^") {
		return false
	}
	base := strings.TrimPrefix(constraint, "^")
	bp := versionParts(base)
	vp := versionParts(version)
	return vp[0] == bp[0] && compareVersionStrings(version, base) >= 0
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
			fmt.Fprintf(os.Stderr, "Lockfile is stale for %s/%s. Run 'skforge lock'.\n", skill.Namespace, skill.Name)
			os.Exit(1)
		}
	}
}

func writeInstallMarker(dir string, marker installMarker) error {
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".agent-skill-install.json"), append(data, '\n'), 0644)
}

func listInstalledFiles(root string) []string {
	var files []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err == nil {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func extractMetadata(path string, data []byte, contentType string) (namespace, name, version string, err error) {
	packageType := "tgz"
	if contentType == "application/zip" {
		packageType = "zip"
	}
	result, validateErr := validation.NewValidator(1024, nil).ValidatePackageWithProfile(data, packageType, validation.ProfilePublish)
	if validateErr != nil {
		return "", "", "", validateErr
	}
	if !result.Valid {
		return "", "", "", fmt.Errorf("%s", strings.Join(result.Errors, "; "))
	}
	if result.Metadata == nil {
		return "", "", "", fmt.Errorf("skill.yaml and VERSION are required")
	}
	return result.Metadata.Namespace, result.Metadata.Name, result.Metadata.Version, nil
}

func distTagCommand() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: skforge dist-tag add <namespace>/<name>@<version> <tag> | skforge dist-tag list <namespace>/<name>")
		os.Exit(1)
	}
	cfg, err := getConfig()
	if err != nil || cfg.Registry == "" {
		fmt.Fprintln(os.Stderr, "No registry configured. Run 'skforge login' first.")
		os.Exit(1)
	}
	c := client.NewClient(cfg.Registry, cfg.Token)
	switch os.Args[2] {
	case "list":
		parts := strings.Split(os.Args[3], "/")
		if len(parts) != 2 {
			fmt.Fprintln(os.Stderr, "Invalid skill reference")
			os.Exit(1)
		}
		tags, err := c.ListDistTags(parts[0], parts[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for tag, version := range tags {
			fmt.Printf("%s -> %s\n", tag, version)
		}
	case "add":
		if len(os.Args) < 5 {
			fmt.Fprintln(os.Stderr, "Usage: skforge dist-tag add <namespace>/<name>@<version> <tag>")
			os.Exit(1)
		}
		at := strings.LastIndex(os.Args[3], "@")
		if at < 0 {
			fmt.Fprintln(os.Stderr, "Invalid skill reference")
			os.Exit(1)
		}
		parts := strings.Split(os.Args[3][:at], "/")
		if len(parts) != 2 {
			fmt.Fprintln(os.Stderr, "Invalid skill reference")
			os.Exit(1)
		}
		if err := c.SetDistTag(parts[0], parts[1], os.Args[4], os.Args[3][at+1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("Set %s/%s:%s -> %s\n", parts[0], parts[1], os.Args[4], os.Args[3][at+1:])
	default:
		fmt.Fprintln(os.Stderr, "Unknown dist-tag subcommand")
		os.Exit(1)
	}
}

func artifactCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skforge artifact <init|publish|list|info|graph|lock|promote|install|attest>")
		os.Exit(1)
	}
	switch os.Args[2] {
	case "init":
		artifactInitCommand()
	case "publish":
		artifactPublishCommand()
	case "list":
		artifactListCommand()
	case "info":
		artifactInfoCommand()
	case "graph":
		artifactGraphCommand(false)
	case "lock":
		artifactGraphCommand(true)
	case "promote":
		artifactPromoteCommand()
	case "install":
		artifactInstallCommand()
	case "attest":
		artifactAttestCommand()
	default:
		fmt.Fprintln(os.Stderr, "Unknown artifact subcommand")
		os.Exit(1)
	}
}

func artifactClient() *client.Client {
	cfg, err := getConfig()
	if err != nil || cfg.Registry == "" {
		fmt.Fprintln(os.Stderr, "No registry configured. Run 'skforge login' first.")
		os.Exit(1)
	}
	return client.NewClient(cfg.Registry, cfg.Token)
}

func artifactInitCommand() {
	if len(os.Args) < 5 {
		fmt.Fprintln(os.Stderr, "Usage: skforge artifact init <kind> <directory>")
		os.Exit(1)
	}
	kind, dir := strings.ToLower(os.Args[3]), os.Args[4]
	entrypoints := map[string]string{"skill": "SKILL.md", "agent": "AGENT.md", "flow": "FLOW.yaml", "prompt": "PROMPT.md", "tool": "TOOL.md", "bundle": "BUNDLE.md"}
	entrypoint, ok := entrypoints[kind]
	if !ok {
		fmt.Fprintln(os.Stderr, "Unsupported artifact kind")
		os.Exit(1)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	name := filepath.Base(filepath.Clean(dir))
	manifest := fmt.Sprintf("apiVersion: skillforge.dev/v1\nkind: %s\nmetadata:\n  namespace: default\n  name: %s\n  version: 0.1.0\n  description: %s artifact\nspec:\n  entrypoint: %s\n  dependencies: []\n", kind, name, kind, entrypoint)
	if err := os.WriteFile(filepath.Join(dir, "artifact.yaml"), []byte(manifest), 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	entryContent := "# " + name + "\n"
	if kind == "flow" {
		entryContent = manifest
	}
	if err := os.WriteFile(filepath.Join(dir, entrypoint), []byte(entryContent), 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Initialized %s artifact in %s\n", kind, dir)
}

func artifactPublishCommand() {
	if len(os.Args) < 6 {
		fmt.Fprintln(os.Stderr, "Usage: skforge artifact publish <kind> <namespace>/<name>@<version> <path>")
		os.Exit(1)
	}
	kind, namespace, name, version := parseArtifactReference(os.Args[3], os.Args[4])
	data, contentType, err := packageArtifactPath(os.Args[5])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	published, err := artifactClient().PublishArtifact(kind, namespace, name, version, data, contentType)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Published %s/%s/%s@%s\nDigest: %s\n", kind, namespace, name, version, published.DigestSHA256)
}

func artifactListCommand() {
	kind, query := "", ""
	if len(os.Args) > 3 {
		kind = os.Args[3]
	}
	if len(os.Args) > 4 {
		query = os.Args[4]
	}
	result, err := artifactClient().ListArtifacts(kind, query)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, artifact := range result.Artifacts {
		fmt.Printf("%s/%s/%s@%s (%d pulls) - %s\n", artifact.Kind, artifact.Namespace, artifact.Name, artifact.LatestVersion, artifact.Downloads, artifact.Description)
	}
}

func artifactInfoCommand() {
	if len(os.Args) < 5 {
		fmt.Fprintln(os.Stderr, "Usage: skforge artifact info <kind> <namespace>/<name>")
		os.Exit(1)
	}
	parts := strings.Split(os.Args[4], "/")
	if len(parts) != 2 {
		fmt.Fprintln(os.Stderr, "Invalid artifact reference")
		os.Exit(1)
	}
	result, err := artifactClient().GetArtifact(os.Args[3], parts[0], parts[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s/%s/%s\n%s\n", result.Artifact.Kind, result.Artifact.Namespace, result.Artifact.Name, result.Artifact.Description)
	for tag, version := range result.DistTags {
		fmt.Printf("  %s -> %s\n", tag, version)
	}
}

func artifactGraphCommand(lock bool) {
	if len(os.Args) < 5 {
		fmt.Fprintln(os.Stderr, "Usage: skforge artifact graph|lock <kind> <namespace>/<name>@<version>")
		os.Exit(1)
	}
	kind, namespace, name, version := parseArtifactReference(os.Args[3], os.Args[4])
	if lock {
		result, err := artifactClient().ArtifactLockfile(kind, namespace, name, version)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		data, _ := yaml.Marshal(result)
		fmt.Print(string(data))
		return
	}
	result, err := artifactClient().ArtifactGraph(kind, namespace, name, version)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, edge := range result.Edges {
		fmt.Printf("%s -> %s\n", edge.From, edge.To)
	}
}

func artifactPromoteCommand() {
	if len(os.Args) < 6 {
		fmt.Fprintln(os.Stderr, "Usage: skforge artifact promote <kind> <namespace>/<name>@<version> <channel>")
		os.Exit(1)
	}
	kind, namespace, name, version := parseArtifactReference(os.Args[3], os.Args[4])
	if err := artifactClient().PromoteArtifact(kind, namespace, name, version, os.Args[5]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Promoted %s/%s/%s@%s to %s\n", kind, namespace, name, version, os.Args[5])
}

func artifactInstallCommand() {
	if len(os.Args) < 6 {
		fmt.Fprintln(os.Stderr, "Usage: skforge artifact install <kind> <namespace>/<name>@<version> <target>")
		os.Exit(1)
	}
	kind, namespace, name, version := parseArtifactReference(os.Args[3], os.Args[4])
	target := os.Args[5]
	c := artifactClient()
	lock, err := c.ArtifactLockfile(kind, namespace, name, version)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	installOne := func(kind, namespace, name, version string) error {
		data, digest, contentType, err := c.DownloadArtifact(kind, namespace, name, version)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != digest {
			return fmt.Errorf("digest mismatch for %s/%s/%s@%s", kind, namespace, name, version)
		}
		dir := filepath.Join(target, kind, namespace, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		if strings.HasPrefix(contentType, "application/zip") {
			return extractZip(data, dir)
		}
		return extractTarGz(data, dir)
	}
	for _, dependency := range lock.Resolved {
		if err := installOne(dependency.Kind, dependency.Namespace, dependency.Name, dependency.Version); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if err := installOne(kind, namespace, name, version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	lockData, _ := yaml.Marshal(lock)
	if err := os.WriteFile(filepath.Join(target, "skillforge.lock.yaml"), lockData, 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Installed %s/%s/%s@%s with %d locked dependencies\n", kind, namespace, name, version, len(lock.Resolved))
}

func artifactAttestCommand() {
	if len(os.Args) < 7 {
		fmt.Fprintln(os.Stderr, "Usage: skforge artifact attest <kind> <namespace>/<name>@<version> <type> <digest>")
		os.Exit(1)
	}
	kind, namespace, name, version := parseArtifactReference(os.Args[3], os.Args[4])
	if err := artifactClient().CreateArtifactAttestation(kind, namespace, name, version, os.Args[5], os.Args[6]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Attestation recorded")
}

func namespaceCommand() {
	if len(os.Args) < 6 || os.Args[2] != "member" {
		fmt.Fprintln(os.Stderr, "Usage: skforge namespace member <namespace> <username> <role>")
		os.Exit(1)
	}
	if err := artifactClient().UpsertNamespaceMember(os.Args[3], os.Args[4], os.Args[5]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Namespace member updated")
}

func webhookCommand() {
	if len(os.Args) < 6 || os.Args[2] != "add" {
		fmt.Fprintln(os.Stderr, "Usage: skforge webhook add <namespace> <url> <event[,event]>")
		os.Exit(1)
	}
	if err := artifactClient().CreateWebhook(os.Args[3], os.Args[4], strings.Split(os.Args[5], ",")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Webhook registered")
}

func parseArtifactReference(kind, ref string) (string, string, string, string) {
	at := strings.LastIndex(ref, "@")
	if at < 0 {
		fmt.Fprintln(os.Stderr, "Invalid artifact reference")
		os.Exit(1)
	}
	parts := strings.Split(ref[:at], "/")
	if len(parts) != 2 {
		fmt.Fprintln(os.Stderr, "Invalid artifact reference")
		os.Exit(1)
	}
	return strings.ToLower(kind), parts[0], parts[1], ref[at+1:]
}

func packageArtifactPath(path string) ([]byte, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", err
	}
	if info.IsDir() {
		data, err := createTarGz(path)
		return data, "application/gzip", err
	}
	data, err := os.ReadFile(path)
	if strings.HasSuffix(strings.ToLower(path), ".zip") {
		return data, "application/zip", err
	}
	return data, "application/gzip", err
}

func deleteCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skforge delete <namespace>/<name>@<version>")
		os.Exit(1)
	}

	skillRef := os.Args[2]

	// Parse namespace/name@version
	parts := strings.Split(skillRef, "@")
	if len(parts) != 2 {
		fmt.Fprintln(os.Stderr, "Invalid skill reference. Expected: <namespace>/<name>@<version>")
		os.Exit(1)
	}

	namespaceName := parts[0]
	version := parts[1]

	nameParts := strings.Split(namespaceName, "/")
	if len(nameParts) != 2 {
		fmt.Fprintln(os.Stderr, "Invalid skill reference. Expected: <namespace>/<name>@<version>")
		os.Exit(1)
	}

	namespace := nameParts[0]
	name := nameParts[1]

	// Get config
	cfg, err := getConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Registry == "" {
		fmt.Fprintln(os.Stderr, "No registry configured. Run 'skforge login' first.")
		os.Exit(1)
	}

	// Confirm deletion
	fmt.Printf("⚠️  Delete %s/%s@%s? (y/N): ", namespace, name, version)
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Println("Cancelled")
		return
	}

	// Send DELETE request
	url := fmt.Sprintf("%s/api/v1/skills/%s/%s/versions/%s", cfg.Registry, namespace, name, version)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create request: %v\n", err)
		os.Exit(1)
	}

	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Delete failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Delete failed: %s (status %d)\n", string(body), resp.StatusCode)
		os.Exit(1)
	}

	fmt.Printf("✅ Deleted %s/%s@%s\n", namespace, name, version)
}

func tokenCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  skforge token create --name <name> --scopes <scopes>")
		fmt.Fprintln(os.Stderr, "  skforge token list")
		fmt.Fprintln(os.Stderr, "  skforge token revoke <token-id>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "create":
		tokenCreateCommand()
	case "list":
		tokenListCommand()
	case "revoke":
		tokenRevokeCommand()
	default:
		fmt.Fprintf(os.Stderr, "Unknown token subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}

func tokenCreateCommand() {
	// Parse flags
	var name string
	var scopesStr string

	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--name" && i+1 < len(os.Args) {
			name = os.Args[i+1]
			i++
		} else if os.Args[i] == "--scopes" && i+1 < len(os.Args) {
			scopesStr = os.Args[i+1]
			i++
		}
	}

	if name == "" {
		fmt.Fprintln(os.Stderr, "Error: --name is required")
		os.Exit(1)
	}

	if scopesStr == "" {
		fmt.Fprintln(os.Stderr, "Error: --scopes is required")
		os.Exit(1)
	}

	scopes := strings.Split(scopesStr, ",")

	// Get config
	cfg, err := getConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Registry == "" || cfg.Token == "" {
		fmt.Fprintln(os.Stderr, "Not logged in. Run 'skforge login' first.")
		os.Exit(1)
	}

	// Create client and token
	c := client.NewClient(cfg.Registry, cfg.Token)
	token, err := c.CreateToken(name, scopes, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create token: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Token created successfully!\n")
	fmt.Printf("   Name: %s\n", token.Name)
	fmt.Printf("   Scopes: %s\n", strings.Join(token.Scopes, ", "))
	fmt.Printf("   Token: %s\n", token.Token)
	fmt.Printf("\n⚠️  Save this token - it will only be shown once!\n")
}

func tokenListCommand() {
	// Get config
	cfg, err := getConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Registry == "" || cfg.Token == "" {
		fmt.Fprintln(os.Stderr, "Not logged in. Run 'skforge login' first.")
		os.Exit(1)
	}

	// Create client and list tokens
	c := client.NewClient(cfg.Registry, cfg.Token)
	tokens, err := c.ListTokens()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list tokens: %v\n", err)
		os.Exit(1)
	}

	if len(tokens) == 0 {
		fmt.Println("No tokens found")
		return
	}

	fmt.Printf("Tokens:\n\n")
	for _, token := range tokens {
		status := "active"
		if token.RevokedAt != nil {
			status = "revoked"
		}

		fmt.Printf("  ID: %d\n", token.ID)
		fmt.Printf("  Name: %s\n", token.Name)
		fmt.Printf("  Scopes: %s\n", strings.Join(token.Scopes, ", "))
		fmt.Printf("  Status: %s\n", status)
		fmt.Printf("  Created: %s\n", token.CreatedAt)
		if token.ExpiresAt != nil {
			fmt.Printf("  Expires: %s\n", *token.ExpiresAt)
		}
		fmt.Println()
	}
}

func tokenRevokeCommand() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: skforge token revoke <token-id>")
		os.Exit(1)
	}

	tokenIDStr := os.Args[3]
	tokenID, err := strconv.ParseInt(tokenIDStr, 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid token ID: %s\n", tokenIDStr)
		os.Exit(1)
	}

	// Get config
	cfg, err := getConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Registry == "" || cfg.Token == "" {
		fmt.Fprintln(os.Stderr, "Not logged in. Run 'skforge login' first.")
		os.Exit(1)
	}

	// Confirm revocation
	fmt.Printf("⚠️  Revoke token #%d? (y/N): ", tokenID)
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Println("Cancelled")
		return
	}

	// Create client and revoke token
	c := client.NewClient(cfg.Registry, cfg.Token)
	if err := c.RevokeToken(tokenID); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to revoke token: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Token #%d revoked\n", tokenID)
}
