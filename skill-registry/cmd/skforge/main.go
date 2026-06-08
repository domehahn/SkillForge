package main

import (
	"archive/tar"
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
	"strconv"
	"strings"

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

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
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
	case "validate":
		validateCommand()
	case "deprecate":
		governanceCommand("deprecate")
	case "yank":
		governanceCommand("yank")
	case "unyank":
		governanceCommand("unyank")
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
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("skforge - Skill Registry Admin CLI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  skforge login")
	fmt.Println("  skforge package <skill-dir> [--output-dir dist] [--format tgz|zip] [--source-commit sha] [--provenance] [--sign] [--dry-run]")
	fmt.Println("  skforge publish <path-or-dir> --registry <url>")
	fmt.Println("  skforge search <query>")
	fmt.Println("  skforge info <namespace>/<name>")
	fmt.Println("  skforge validate <path-or-dir> [--strict|--publish] [--output json]")
	fmt.Println("  skforge deprecate <namespace>/<name>@<version> --reason <reason>")
	fmt.Println("  skforge yank <namespace>/<name>@<version> --reason <reason>")
	fmt.Println("  skforge unyank <namespace>/<name>@<version>")
	fmt.Println("  skforge delete <namespace>/<name>@<version>")
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
	fmt.Println("  skforge artifact attest <kind> <namespace>/<name>@<version> <type> <digest>")
	fmt.Println("  skforge namespace member <namespace> <username> <reader|maintainer|owner>")
	fmt.Println("  skforge webhook add <namespace> <url> <event[,event]>")
	fmt.Println("  skforge version")
	fmt.Println()
	fmt.Println("Consumer lifecycle (install, lock, verify, add, remove) → use skpm")
}

// ── config ────────────────────────────────────────────────────────────────────

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

func publishCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skforge publish <path-or-dir> --registry <url>")
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
		fmt.Fprintln(os.Stderr, "No registry URL specified. Use --registry or run 'skforge login'")
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

	namespace, name, ver, err := extractMetadata(pathOrArchive, data, contentType)
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
		fmt.Fprintln(os.Stderr, "Usage: skforge info <namespace>/<name>")
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
		fmt.Fprintln(os.Stderr, "No registry URL configured. Run 'skforge login'")
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
		fmt.Fprintln(os.Stderr, "Usage: skforge validate <path-or-dir> [--strict|--publish] [--output json]")
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
	ns, name, ver, err := parseSpec(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg, err := getConfig()
	if err != nil || cfg.Registry == "" {
		fmt.Fprintln(os.Stderr, "No registry URL configured. Run 'skforge login'")
		os.Exit(1)
	}
	c := client.NewClient(cfg.Registry, cfg.Token)
	if err := c.Governance(action, ns, name, ver, reason); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to %s: %v\n", action, err)
		os.Exit(1)
	}
	label := strings.ToUpper(action[:1]) + action[1:]
	fmt.Printf("%s %s/%s@%s\n", label, ns, name, ver)
}

func deleteCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skforge delete <namespace>/<name>@<version>")
		os.Exit(1)
	}
	ns, name, ver, err := parseSpec(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg, err := getConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}
	if cfg.Registry == "" {
		fmt.Fprintln(os.Stderr, "No registry configured. Run 'skforge login' first.")
		os.Exit(1)
	}
	fmt.Printf("Delete %s/%s@%s? (y/N): ", ns, name, ver)
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Println("Cancelled")
		return
	}
	url := fmt.Sprintf("%s/api/v1/skills/%s/%s/versions/%s", cfg.Registry, ns, name, ver)
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
	fmt.Printf("Deleted %s/%s@%s\n", ns, name, ver)
}

func tokenCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  skforge token create --name <name> --scopes <scopes>")
		fmt.Fprintln(os.Stderr, "  skforge token list")
		fmt.Fprintln(os.Stderr, "  skforge token revoke <token-id>")
		os.Exit(1)
	}
	switch os.Args[2] {
	case "create":
		tokenCreateCommand()
	case "list":
		tokenListCommand()
	case "revoke":
		tokenRevokeCommand()
	default:
		fmt.Fprintf(os.Stderr, "Unknown token subcommand: %s\n", os.Args[2])
		os.Exit(1)
	}
}

func tokenCreateCommand() {
	var name, scopesStr string
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
	cfg, err := getConfig()
	if err != nil || cfg.Registry == "" || cfg.Token == "" {
		fmt.Fprintln(os.Stderr, "Not logged in. Run 'skforge login' first.")
		os.Exit(1)
	}
	c := client.NewClient(cfg.Registry, cfg.Token)
	token, err := c.CreateToken(name, strings.Split(scopesStr, ","), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create token: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Token created\n")
	fmt.Printf("   Name: %s\n", token.Name)
	fmt.Printf("   Scopes: %s\n", strings.Join(token.Scopes, ", "))
	fmt.Printf("   Token: %s\n", token.Token)
	fmt.Printf("\nSave this token — it will only be shown once.\n")
}

func tokenListCommand() {
	cfg, err := getConfig()
	if err != nil || cfg.Registry == "" || cfg.Token == "" {
		fmt.Fprintln(os.Stderr, "Not logged in. Run 'skforge login' first.")
		os.Exit(1)
	}
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
	tokenID, err := strconv.ParseInt(os.Args[3], 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid token ID: %s\n", os.Args[3])
		os.Exit(1)
	}
	cfg, err := getConfig()
	if err != nil || cfg.Registry == "" || cfg.Token == "" {
		fmt.Fprintln(os.Stderr, "Not logged in. Run 'skforge login' first.")
		os.Exit(1)
	}
	fmt.Printf("Revoke token #%d? (y/N): ", tokenID)
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Println("Cancelled")
		return
	}
	c := client.NewClient(cfg.Registry, cfg.Token)
	if err := c.RevokeToken(tokenID); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to revoke token: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Token #%d revoked\n", tokenID)
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
		for tag, ver := range tags {
			fmt.Printf("%s -> %s\n", tag, ver)
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
	entrypoints := map[string]string{
		"skill": "SKILL.md", "agent": "AGENT.md", "flow": "FLOW.yaml",
		"prompt": "PROMPT.md", "tool": "TOOL.md", "bundle": "BUNDLE.md",
	}
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
	kind, namespace, name, ver := parseArtifactReference(os.Args[3], os.Args[4])
	data, contentType, err := packageArtifactPath(os.Args[5])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	published, err := artifactClient().PublishArtifact(kind, namespace, name, ver, data, contentType)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Published %s/%s/%s@%s\nDigest: %s\n", kind, namespace, name, ver, published.DigestSHA256)
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
	for tag, ver := range result.DistTags {
		fmt.Printf("  %s -> %s\n", tag, ver)
	}
}

func artifactGraphCommand(lock bool) {
	if len(os.Args) < 5 {
		fmt.Fprintln(os.Stderr, "Usage: skforge artifact graph|lock <kind> <namespace>/<name>@<version>")
		os.Exit(1)
	}
	kind, namespace, name, ver := parseArtifactReference(os.Args[3], os.Args[4])
	if lock {
		result, err := artifactClient().ArtifactLockfile(kind, namespace, name, ver)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		data, _ := yaml.Marshal(result)
		fmt.Print(string(data))
		return
	}
	result, err := artifactClient().ArtifactGraph(kind, namespace, name, ver)
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
	kind, namespace, name, ver := parseArtifactReference(os.Args[3], os.Args[4])
	if err := artifactClient().PromoteArtifact(kind, namespace, name, ver, os.Args[5]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Promoted %s/%s/%s@%s to %s\n", kind, namespace, name, ver, os.Args[5])
}

func artifactInstallCommand() {
	if len(os.Args) < 6 {
		fmt.Fprintln(os.Stderr, "Usage: skforge artifact install <kind> <namespace>/<name>@<version> <target>")
		os.Exit(1)
	}
	kind, namespace, name, ver := parseArtifactReference(os.Args[3], os.Args[4])
	target := os.Args[5]
	c := artifactClient()
	lock, err := c.ArtifactLockfile(kind, namespace, name, ver)
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
			return install.ExtractZip(data, dir)
		}
		return install.ExtractTarGz(data, dir)
	}

	for _, dep := range lock.Resolved {
		if err := installOne(dep.Kind, dep.Namespace, dep.Name, dep.Version); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if err := installOne(kind, namespace, name, ver); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	lockData, _ := yaml.Marshal(lock)
	if err := os.WriteFile(filepath.Join(target, "skillforge.lock.yaml"), lockData, 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Installed %s/%s/%s@%s with %d locked dependencies\n", kind, namespace, name, ver, len(lock.Resolved))
}

func artifactAttestCommand() {
	if len(os.Args) < 7 {
		fmt.Fprintln(os.Stderr, "Usage: skforge artifact attest <kind> <namespace>/<name>@<version> <type> <digest>")
		os.Exit(1)
	}
	kind, namespace, name, ver := parseArtifactReference(os.Args[3], os.Args[4])
	if err := artifactClient().CreateArtifactAttestation(kind, namespace, name, ver, os.Args[5], os.Args[6]); err != nil {
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

func extractMetadata(path string, data []byte, contentType string) (namespace, name, version string, err error) {
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
		return "", "", "", fmt.Errorf("skill.yaml and VERSION are required in %s", path)
	}
	return result.Metadata.Namespace, result.Metadata.Name, result.Metadata.Version, nil
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

func createTarGz(dir string) ([]byte, error) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
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
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(tw, f); err != nil {
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
