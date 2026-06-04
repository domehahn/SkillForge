package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
	case "publish":
		publishCommand()
	case "search":
		searchCommand()
	case "info":
		infoCommand()
	case "install":
		installCommand()
	case "validate":
		validateCommand()
	case "version":
		fmt.Printf("skillctl version %s\n", version)
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
	fmt.Println("  skforge publish <path-or-archive> --registry <url>")
	fmt.Println("  skforge search <query>")
	fmt.Println("  skforge info <namespace>/<name>")
	fmt.Println("  skforge install <namespace>/<name>@<version> --target <dir>")
	fmt.Println("  skforge validate <path-or-archive>")
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
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skillctl login")
		os.Exit(1)
	}

	fmt.Print("Registry URL: ")
	var registryURL string
	fmt.Scanln(&registryURL)

	fmt.Print("Token: ")
	var token string
	fmt.Scanln(&token)

	cfg := &Config{
		Registry: registryURL,
		Token:    token,
	}

	if err := saveConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Login successful!")
}

func publishCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skillctl publish <path-or-archive> --registry <url>")
		os.Exit(1)
	}

	pathOrArchive := os.Args[2]
	registryURL := ""

	// Parse flags
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--registry" && i+1 < len(os.Args) {
			registryURL = os.Args[i+1]
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
		fmt.Fprintln(os.Stderr, "No registry URL specified. Use --registry or run 'skillctl login'")
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

	if info.IsDir() {
		// Create tar.gz from directory
		data, err = createTarGz(pathOrArchive)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create archive: %v\n", err)
			os.Exit(1)
		}
		contentType = "application/gzip"
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
	namespace, name, version, err := extractMetadata(pathOrArchive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to extract metadata: %v\n", err)
		os.Exit(1)
	}

	// Publish
	c := client.NewClient(registryURL, cfg.Token)
	skillVersion, err := c.Publish(namespace, name, version, data, contentType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to publish: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Published %s/%s@%s\n", namespace, name, version)
	fmt.Printf("Digest: %s\n", skillVersion.DigestSHA256)
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
		fmt.Fprintln(os.Stderr, "No registry URL configured. Run 'skillctl login'")
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
		fmt.Fprintln(os.Stderr, "Usage: skillctl info <namespace>/<name>")
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
		fmt.Fprintln(os.Stderr, "No registry URL configured. Run 'skillctl login'")
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
		fmt.Printf("  - %s (created: %s)\n", v.Version, v.CreatedAt.Format("2006-01-02 15:04:05"))
	}
}

func installCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skillctl install <namespace>/<name>@<version> --target <dir>")
		os.Exit(1)
	}

	spec := os.Args[2]
	targetDir := ".agents/skills"

	// Parse flags
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--target" && i+1 < len(os.Args) {
			targetDir = os.Args[i+1]
			i++
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
		fmt.Fprintln(os.Stderr, "No registry URL configured. Run 'skillctl login'")
		os.Exit(1)
	}

	// Download
	c := client.NewClient(cfg.Registry, cfg.Token)
	data, digest, err := c.Download(namespace, name, version)
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

	// Extract
	skillDir := filepath.Join(targetDir, name)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create target directory: %v\n", err)
		os.Exit(1)
	}

	// Check if skill already exists
	if _, err := os.Stat(skillDir); err == nil {
		fmt.Fprintf(os.Stderr, "Skill already exists at %s. Use --force to overwrite.\n", skillDir)
		os.Exit(1)
	}

	if err := extractTarGz(data, skillDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to extract: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Installed %s/%s@%s to %s\n", namespace, name, version, skillDir)
}

func validateCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: skillctl validate <path-or-archive>")
		os.Exit(1)
	}

	pathOrArchive := os.Args[2]

	// Check if path is directory or archive
	info, err := os.Stat(pathOrArchive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stat path: %v\n", err)
		os.Exit(1)
	}

	var data []byte
	var contentType string

	if info.IsDir() {
		// Create tar.gz from directory
		data, err = createTarGz(pathOrArchive)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create archive: %v\n", err)
			os.Exit(1)
		}
		contentType = "application/gzip"
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

	cfg, err := getConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Registry != "" {
		// Use remote validation
		c := client.NewClient(cfg.Registry, cfg.Token)
		result, err := c.Validate(data, contentType)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to validate: %v\n", err)
			os.Exit(1)
		}

		if result.Valid {
			fmt.Println("✓ Package is valid")
		} else {
			fmt.Println("✗ Package is invalid")
		}

		if len(result.Errors) > 0 {
			fmt.Println("\nErrors:")
			for _, e := range result.Errors {
				fmt.Printf("  - %s\n", e)
			}
		}

		if len(result.Warnings) > 0 {
			fmt.Println("\nWarnings:")
			for _, w := range result.Warnings {
				fmt.Printf("  - %s\n", w)
			}
		}

		if !result.Valid {
			os.Exit(1)
		}
	} else {
		fmt.Println("No registry configured. Basic validation only.")
		// Could implement local validation here
		fmt.Println("✓ Package exists and can be read")
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

		target := filepath.Join(targetDir, header.Name)

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
		}
	}

	return nil
}

func extractMetadata(path string) (namespace, name, version string, err error) {
	// Simple default values for demo
	// In production, would parse SKILL.md frontmatter
	namespace = "default"
	name = filepath.Base(path)
	name = strings.TrimSuffix(name, ".tgz")
	name = strings.TrimSuffix(name, ".tar.gz")
	name = strings.TrimSuffix(name, ".zip")
	version = "1.0.0"

	// TODO: Actually parse SKILL.md or skill.yaml for metadata
	return namespace, name, version, nil
}
