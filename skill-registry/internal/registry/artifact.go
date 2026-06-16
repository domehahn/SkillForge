package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/domehahn/sklib/spec"
	"github.com/skillforge/skill-registry/internal/metadata"
	"github.com/skillforge/skill-registry/internal/validation"
)

type ArtifactPublishResult struct {
	Artifact *metadata.Artifact        `json:"artifact"`
	Version  *metadata.ArtifactVersion `json:"version"`
	Warnings []string                  `json:"warnings"`
}

func (r *Registry) PublishArtifact(ctx context.Context, kind, namespace, name, version string, data []byte, packageType, actor string) (*ArtifactPublishResult, error) {
	kind = strings.ToLower(kind)
	if err := validation.ValidateArtifactKind(kind); err != nil {
		return nil, err
	}
	if err := validation.ValidateNamespace(namespace); err != nil {
		return nil, err
	}
	if err := validation.ValidateSkillName(name); err != nil {
		return nil, err
	}
	if err := validation.ValidateVersion(version); err != nil {
		return nil, err
	}
	if existing, err := r.repo.GetArtifactVersion(ctx, kind, namespace, name, version); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, fmt.Errorf("version %s already exists; published versions are immutable", version)
	}

	manifest, warnings, err := r.validator.ValidateArtifactPackage(data, packageType, kind)
	if err != nil {
		return nil, fmt.Errorf("artifact validation failed: %w", err)
	}
	if manifest.Metadata.Name != "" && manifest.Metadata.Name != name {
		return nil, fmt.Errorf("manifest name %q does not match published name %q", manifest.Metadata.Name, name)
	}
	if manifest.Metadata.Namespace != "" && manifest.Metadata.Namespace != namespace {
		return nil, fmt.Errorf("manifest namespace %q does not match published namespace %q", manifest.Metadata.Namespace, namespace)
	}
	if manifest.Metadata.Version != "" && manifest.Metadata.Version != version {
		return nil, fmt.Errorf("manifest version %q does not match published version %q", manifest.Metadata.Version, version)
	}
	manifest.Metadata.Name, manifest.Metadata.Namespace, manifest.Metadata.Version = name, namespace, version

	lockfile, err := r.ResolveDependencies(ctx, manifest.Spec.Dependencies, kind+"/"+namespace+"/"+name+"@"+version)
	if err != nil {
		return nil, fmt.Errorf("dependency resolution failed: %w", err)
	}
	digest, err := r.storage.StoreArtifact(kind, namespace, name, version, data)
	if err != nil {
		return nil, err
	}
	artifact := &metadata.Artifact{
		Kind: kind, Namespace: namespace, Name: name, Description: manifest.Metadata.Description,
		LatestVersion: version, Visibility: manifest.Metadata.Visibility, Tags: manifest.Metadata.Tags,
		Owners: manifest.Metadata.Owners,
	}
	if err := r.repo.CreateArtifact(ctx, artifact); err != nil {
		return nil, err
	}
	if actor != "" && actor != "anonymous" {
		if members, memberErr := r.repo.ListNamespaceMembers(ctx, namespace); memberErr == nil && len(members) == 0 {
			_ = r.repo.UpsertNamespaceMember(ctx, &metadata.NamespaceMember{Namespace: namespace, Username: actor, Role: "owner"})
		}
	}
	descriptor := &metadata.OCIDescriptor{
		MediaType: "application/vnd.skillforge.artifact.manifest.v1+json",
		Digest:    "sha256:" + digest, Size: int64(len(data)),
		ArtifactType: "application/vnd.skillforge." + kind + ".v1",
		Annotations: map[string]string{
			"org.opencontainers.image.title":   name,
			"org.opencontainers.image.version": version,
			"dev.skillforge.namespace":         namespace,
		},
	}
	artifactVersion := &metadata.ArtifactVersion{
		ArtifactID: artifact.ID, Kind: kind, Namespace: namespace, Name: name, Version: version,
		DigestSHA256: digest, PackageType: packageType, SizeBytes: int64(len(data)),
		Entrypoint: manifest.Spec.Entrypoint, Manifest: manifest, Lockfile: lockfile,
		OCIDescriptor: descriptor, SignatureStatus: "unsigned", ScanStatus: "pending",
		ValidationStatus: "valid", Source: "local", CreatedBy: actor,
	}
	if err := r.repo.CreateArtifactVersion(ctx, artifactVersion); err != nil {
		return nil, err
	}
	if err := r.repo.SetArtifactDistTag(ctx, artifact.ID, "latest", version, actor); err != nil {
		return nil, err
	}
	_ = r.repo.LogAudit(ctx, &metadata.AuditLog{Action: "artifact.publish", Namespace: namespace, Name: name, Version: version, Actor: actor, Success: true, Message: kind})
	r.deliverWebhookEvent(ctx, namespace, "artifact.published", map[string]interface{}{"artifact": artifact, "version": artifactVersion})
	if members, err := r.repo.ListNamespaceMembers(ctx, namespace); err == nil {
		for _, m := range members {
			if m.Username == actor {
				continue
			}
			_ = r.repo.AddNotification(ctx, &metadata.Notification{
				Username: m.Username,
				Type:     "publish",
				Message:  fmt.Sprintf("%s published %s/%s@%s", actor, namespace, name, version),
				Link:     "/artifacts/" + kind + "/" + namespace + "/" + name,
			})
		}
	}
	return &ArtifactPublishResult{Artifact: artifact, Version: artifactVersion, Warnings: warnings}, nil
}

func (r *Registry) ListArtifacts(ctx context.Context, kind, query, namespace, sort string, limit, offset int, verified bool) ([]metadata.Artifact, int, error) {
	if kind != "" {
		if err := validation.ValidateArtifactKind(kind); err != nil {
			return nil, 0, err
		}
	}
	return r.repo.ListArtifacts(ctx, kind, query, namespace, sort, limit, offset, verified)
}

// KindFacets returns per-kind artifact counts for a query.
func (r *Registry) KindFacets(ctx context.Context, query string) (map[string]int, error) {
	return r.repo.KindFacets(ctx, query)
}

// NamespaceStats returns summary stats for a namespace.
func (r *Registry) NamespaceStats(ctx context.Context, namespace string) (int, int64, error) {
	return r.repo.NamespaceStats(ctx, namespace)
}

func (r *Registry) GetArtifact(ctx context.Context, kind, namespace, name string) (*metadata.Artifact, []metadata.ArtifactVersion, map[string]string, error) {
	artifact, err := r.repo.GetArtifact(ctx, kind, namespace, name)
	if err != nil || artifact == nil {
		return artifact, nil, nil, err
	}
	versions, err := r.repo.ListArtifactVersions(ctx, artifact.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	tags, err := r.repo.ListArtifactDistTags(ctx, artifact.ID)
	return artifact, versions, tags, err
}

func (r *Registry) ResolveArtifactVersion(ctx context.Context, kind, namespace, name, ref string) (*metadata.ArtifactVersion, error) {
	if version, err := r.repo.GetArtifactVersion(ctx, kind, namespace, name, ref); err != nil {
		return nil, err
	} else if version != nil {
		return version, nil
	}
	artifact, err := r.repo.GetArtifact(ctx, kind, namespace, name)
	if err != nil || artifact == nil {
		return nil, fmt.Errorf("artifact not found")
	}
	tags, err := r.repo.ListArtifactDistTags(ctx, artifact.ID)
	if err != nil {
		return nil, err
	}
	if version, ok := tags[ref]; ok {
		return r.repo.GetArtifactVersion(ctx, kind, namespace, name, version)
	}
	versions, err := r.repo.ListArtifactVersions(ctx, artifact.ID)
	if err != nil {
		return nil, err
	}
	sort.Slice(versions, func(i, j int) bool { cmp, _ := spec.CompareVersionsSafe(versions[i].Version, versions[j].Version); return cmp > 0 })
	for i := range versions {
		if semverMatches(versions[i].Version, ref) {
			return &versions[i], nil
		}
	}
	return nil, fmt.Errorf("artifact version or tag not found")
}

func (r *Registry) DownloadArtifact(ctx context.Context, kind, namespace, name, ref string) ([]byte, *metadata.ArtifactVersion, error) {
	version, err := r.ResolveArtifactVersion(ctx, kind, namespace, name, ref)
	if err != nil {
		return nil, nil, err
	}
	data, err := r.storage.Retrieve(version.DigestSHA256)
	if err != nil {
		return nil, nil, err
	}
	_ = r.repo.IncrementArtifactDownload(ctx, version.ArtifactID, version.Version)
	return data, version, nil
}

func (r *Registry) PromoteArtifact(ctx context.Context, kind, namespace, name, version, channel, actor string) error {
	if err := validation.ValidateDistTag(channel); err != nil {
		return err
	}
	artifact, err := r.repo.GetArtifact(ctx, kind, namespace, name)
	if err != nil || artifact == nil {
		return fmt.Errorf("artifact not found")
	}
	if err := r.repo.SetArtifactDistTag(ctx, artifact.ID, channel, version, actor); err != nil {
		return err
	}
	promotion := &metadata.Promotion{Kind: kind, Namespace: namespace, Name: name, Version: version, Channel: channel, Actor: actor}
	if err := r.repo.RecordPromotion(ctx, promotion); err != nil {
		return err
	}
	r.deliverWebhookEvent(ctx, namespace, "artifact.promoted", promotion)
	return nil
}

func (r *Registry) SetArtifactAttestation(ctx context.Context, kind, namespace, name, version, signatureStatus, scanStatus string) error {
	artifactVersion, err := r.ResolveArtifactVersion(ctx, kind, namespace, name, version)
	if err != nil {
		return err
	}
	return r.repo.SetArtifactAttestation(ctx, artifactVersion.ID, signatureStatus, scanStatus)
}

func (r *Registry) CreateArtifactAttestation(ctx context.Context, kind, namespace, name, version, attestationType, digest, actor string, predicate map[string]interface{}) (*metadata.Attestation, error) {
	artifactVersion, err := r.ResolveArtifactVersion(ctx, kind, namespace, name, version)
	if err != nil {
		return nil, err
	}
	if attestationType != "signature" && attestationType != "scan" && attestationType != "provenance" && attestationType != "sbom" {
		return nil, fmt.Errorf("unsupported attestation type")
	}
	attestation := &metadata.Attestation{ArtifactVersionID: artifactVersion.ID, Type: attestationType, Digest: digest, Predicate: predicate, CreatedBy: actor}
	if err := r.repo.CreateAttestation(ctx, attestation); err != nil {
		return nil, err
	}
	signatureStatus, scanStatus := "", ""
	if attestationType == "signature" {
		signatureStatus = "verified"
	}
	if attestationType == "scan" {
		scanStatus = "passed"
	}
	_ = r.repo.SetArtifactAttestation(ctx, artifactVersion.ID, signatureStatus, scanStatus)
	return attestation, nil
}

func (r *Registry) ListArtifactAttestations(ctx context.Context, kind, namespace, name, version string) ([]metadata.Attestation, error) {
	artifactVersion, err := r.ResolveArtifactVersion(ctx, kind, namespace, name, version)
	if err != nil {
		return nil, err
	}
	return r.repo.ListAttestations(ctx, artifactVersion.ID)
}

func (r *Registry) ListPromotions(ctx context.Context, kind, namespace, name string) ([]metadata.Promotion, error) {
	return r.repo.ListPromotions(ctx, kind, namespace, name)
}

func (r *Registry) UpsertNamespaceMember(ctx context.Context, namespace, username, role string) (*metadata.NamespaceMember, error) {
	if err := validation.ValidateNamespace(namespace); err != nil {
		return nil, err
	}
	if role != "reader" && role != "maintainer" && role != "owner" {
		return nil, fmt.Errorf("role must be reader, maintainer, or owner")
	}
	member := &metadata.NamespaceMember{Namespace: namespace, Username: username, Role: role}
	return member, r.repo.UpsertNamespaceMember(ctx, member)
}

func (r *Registry) ListNamespaceMembers(ctx context.Context, namespace string) ([]metadata.NamespaceMember, error) {
	return r.repo.ListNamespaceMembers(ctx, namespace)
}

func (r *Registry) RemoveNamespaceMember(ctx context.Context, namespace, username string) error {
	return r.repo.RemoveNamespaceMember(ctx, namespace, username)
}

func (r *Registry) AuthorizeNamespace(ctx context.Context, namespace, username, requiredRole string) (bool, error) {
	members, err := r.repo.ListNamespaceMembers(ctx, namespace)
	if err != nil {
		return false, err
	}
	if len(members) == 0 {
		return true, nil
	}
	role, err := r.repo.NamespaceRole(ctx, namespace, username)
	if err != nil {
		return false, err
	}
	ranks := map[string]int{"reader": 1, "maintainer": 2, "owner": 3}
	return ranks[role] >= ranks[requiredRole], nil
}

func (r *Registry) CreateWebhook(ctx context.Context, namespace, url string, events []string) (*metadata.Webhook, error) {
	if err := validation.ValidateNamespace(namespace); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return nil, fmt.Errorf("webhook URL must use http or https")
	}
	webhook := &metadata.Webhook{Namespace: namespace, URL: url, Events: events}
	return webhook, r.repo.CreateWebhook(ctx, webhook)
}

func (r *Registry) ListWebhooks(ctx context.Context, namespace string) ([]metadata.Webhook, error) {
	return r.repo.ListWebhooks(ctx, namespace)
}

func (r *Registry) deliverWebhookEvent(ctx context.Context, namespace, event string, payload interface{}) {
	hooks, err := r.repo.ListWebhooks(ctx, namespace)
	if err != nil {
		r.logger.Warn("failed to list webhooks", "error", err)
		return
	}
	body, _ := json.Marshal(map[string]interface{}{"event": event, "namespace": namespace, "payload": payload})
	for _, hook := range hooks {
		if !hook.Active || !containsEvent(hook.Events, event) {
			continue
		}
		go func(hook metadata.Webhook) {
			request, err := http.NewRequest(http.MethodPost, hook.URL, bytes.NewReader(body))
			if err != nil {
				return
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-SkillForge-Event", event)
			client := &http.Client{Timeout: 5 * time.Second}
			response, err := client.Do(request)
			if err != nil {
				r.logger.Warn("webhook delivery failed", "url", hook.URL, "error", err)
				return
			}
			response.Body.Close()
		}(hook)
	}
}

func containsEvent(events []string, event string) bool {
	for _, candidate := range events {
		if candidate == "*" || candidate == event {
			return true
		}
	}
	return false
}

func (r *Registry) ResolveDependencies(ctx context.Context, dependencies []metadata.ArtifactDependency, root string) (*metadata.ArtifactLockfile, error) {
	lock := &metadata.ArtifactLockfile{APIVersion: "skillforge.dev/v1", Root: root, CreatedAt: time.Now()}
	seen := map[string]bool{}
	var visit func(metadata.ArtifactDependency, []string) error
	visit = func(dep metadata.ArtifactDependency, path []string) error {
		key := dep.Kind + "/" + dep.Namespace + "/" + dep.Name
		for _, item := range path {
			if item == key {
				return fmt.Errorf("dependency cycle: %s -> %s", strings.Join(path, " -> "), key)
			}
		}
		version, err := r.ResolveArtifactVersion(ctx, dep.Kind, dep.Namespace, dep.Name, dep.Version)
		if err != nil {
			if dep.Optional {
				return nil
			}
			return fmt.Errorf("%s: %w", key, err)
		}
		resolvedKey := key + "@" + version.Version
		if seen[resolvedKey] {
			return nil
		}
		seen[resolvedKey] = true
		lock.Resolved = append(lock.Resolved, metadata.ResolvedArtifact{
			Kind: dep.Kind, Namespace: dep.Namespace, Name: dep.Name,
			Version: version.Version, DigestSHA256: version.DigestSHA256,
		})
		if version.Manifest != nil {
			for _, child := range version.Manifest.Spec.Dependencies {
				if err := visit(child, append(path, key)); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, dep := range dependencies {
		if err := visit(dep, nil); err != nil {
			return nil, err
		}
	}
	sort.Slice(lock.Resolved, func(i, j int) bool {
		a, b := lock.Resolved[i], lock.Resolved[j]
		return a.Kind+"/"+a.Namespace+"/"+a.Name < b.Kind+"/"+b.Namespace+"/"+b.Name
	})
	return lock, nil
}

func (r *Registry) ArtifactGraph(ctx context.Context, kind, namespace, name, ref string) (*metadata.ArtifactGraph, error) {
	root, err := r.ResolveArtifactVersion(ctx, kind, namespace, name, ref)
	if err != nil {
		return nil, err
	}
	graph := &metadata.ArtifactGraph{
		Nodes: []metadata.ResolvedArtifact{},
		Edges: []metadata.ArtifactEdge{},
	}
	nodes := map[string]metadata.ResolvedArtifact{}
	edges := map[string]metadata.ArtifactEdge{}
	var walk func(*metadata.ArtifactVersion) error
	walk = func(version *metadata.ArtifactVersion) error {
		from := version.Kind + "/" + version.Namespace + "/" + version.Name + "@" + version.Version
		nodes[from] = metadata.ResolvedArtifact{Kind: version.Kind, Namespace: version.Namespace, Name: version.Name, Version: version.Version, DigestSHA256: version.DigestSHA256}
		if version.Manifest == nil {
			return nil
		}
		for _, dep := range version.Manifest.Spec.Dependencies {
			child, err := r.ResolveArtifactVersion(ctx, dep.Kind, dep.Namespace, dep.Name, dep.Version)
			if err != nil {
				if dep.Optional {
					continue
				}
				return err
			}
			to := child.Kind + "/" + child.Namespace + "/" + child.Name + "@" + child.Version
			edges[from+"->"+to] = metadata.ArtifactEdge{From: from, To: to}
			if _, exists := nodes[to]; !exists {
				if err := walk(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(root); err != nil {
		return nil, err
	}
	for _, node := range nodes {
		graph.Nodes = append(graph.Nodes, node)
	}
	for _, edge := range edges {
		graph.Edges = append(graph.Edges, edge)
	}
	return graph, nil
}

func semverParts(version string) [3]int {
	base := strings.SplitN(version, "-", 2)[0]
	parts := strings.Split(base, ".")
	var result [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		result[i], _ = strconv.Atoi(parts[i])
	}
	return result
}

func semverMatches(version, constraint string) bool {
	if constraint == "" || constraint == "*" || constraint == "latest" {
		return true
	}
	if strings.HasPrefix(constraint, "^") {
		base := strings.TrimPrefix(constraint, "^")
		v, b := semverParts(version), semverParts(base)
		cmp, _ := spec.CompareVersionsSafe(version, base)
		return v[0] == b[0] && cmp >= 0
	}
	if strings.HasPrefix(constraint, "~") {
		base := strings.TrimPrefix(constraint, "~")
		v, b := semverParts(version), semverParts(base)
		cmp, _ := spec.CompareVersionsSafe(version, base)
		return v[0] == b[0] && v[1] == b[1] && cmp >= 0
	}
	return version == constraint
}

func (r *Registry) UpdateArtifactMeta(ctx context.Context, kind, namespace, name string, description, readme *string, tags []string, visibility *string) error {
	return r.repo.UpdateArtifactMeta(ctx, kind, namespace, name, description, readme, tags, visibility)
}

func (r *Registry) GetNamespaceBio(ctx context.Context, namespace string) string {
	return r.repo.GetNamespaceBio(ctx, namespace)
}

func (r *Registry) SetNamespaceBio(ctx context.Context, namespace, bio string) error {
	return r.repo.SetNamespaceBio(ctx, namespace, bio)
}

func (r *Registry) GetNamespaceVerified(ctx context.Context, namespace string) bool {
	return r.repo.GetNamespaceVerified(ctx, namespace)
}

func (r *Registry) SetNamespaceVerified(ctx context.Context, namespace string, verified bool) error {
	return r.repo.SetNamespaceVerified(ctx, namespace, verified)
}

func (r *Registry) GetDisplayName(ctx context.Context, namespace string) string {
	return r.repo.GetDisplayName(ctx, namespace)
}

func (r *Registry) SetDisplayName(ctx context.Context, namespace, displayName string) error {
	return r.repo.SetDisplayName(ctx, namespace, displayName)
}

func (r *Registry) ListAdminNamespaces(ctx context.Context) ([]metadata.NamespaceInfo, error) {
	return r.repo.ListAdminNamespaces(ctx)
}

func (r *Registry) GetArtifactDownloadTrend(ctx context.Context, kind, namespace, name string, days int) ([]metadata.DayCount, error) {
	a, err := r.repo.GetArtifact(ctx, kind, namespace, name)
	if err != nil || a == nil {
		return nil, err
	}
	return r.repo.GetArtifactDownloadTrend(ctx, a.ID, days)
}

func (r *Registry) SetVersionReleaseNotes(ctx context.Context, kind, namespace, name, version, notes string) error {
	return r.repo.SetVersionReleaseNotes(ctx, kind, namespace, name, version, notes)
}

func (r *Registry) LogWebhookDelivery(ctx context.Context, d *metadata.WebhookDelivery) error {
	return r.repo.LogWebhookDelivery(ctx, d)
}

func (r *Registry) ListWebhookDeliveries(ctx context.Context, webhookID int64) ([]metadata.WebhookDelivery, error) {
	return r.repo.ListWebhookDeliveries(ctx, webhookID)
}

func (r *Registry) DeleteWebhook(ctx context.Context, webhookID int64) error {
	return r.repo.DeleteWebhook(ctx, webhookID)
}

func (r *Registry) GetArtifactUsedBy(ctx context.Context, namespace, name string) ([]metadata.DependentArtifact, error) {
	return r.repo.GetArtifactUsedBy(ctx, namespace, name)
}

func (r *Registry) ListTopPublishers(ctx context.Context, limit int) ([]metadata.PublisherInfo, error) {
	return r.repo.ListTopPublishers(ctx, limit)
}

func (r *Registry) GetPinnedArtifacts(ctx context.Context, namespace string) []string {
	return r.repo.GetPinnedArtifacts(ctx, namespace)
}

func (r *Registry) SetPinnedArtifacts(ctx context.Context, namespace string, pinned []string) error {
	return r.repo.SetPinnedArtifacts(ctx, namespace, pinned)
}

func (r *Registry) AddNotification(ctx context.Context, n *metadata.Notification) error {
	return r.repo.AddNotification(ctx, n)
}

func (r *Registry) GetNotifications(ctx context.Context, username string) ([]metadata.Notification, error) {
	return r.repo.GetNotifications(ctx, username)
}

func (r *Registry) MarkNotificationsRead(ctx context.Context, username string) error {
	return r.repo.MarkNotificationsRead(ctx, username)
}

func (r *Registry) DeleteNotification(ctx context.Context, id int64, username string) error {
	return r.repo.DeleteNotification(ctx, id, username)
}

func (r *Registry) StarArtifact(ctx context.Context, kind, namespace, name, username string) error {
	artifact, err := r.repo.GetArtifact(ctx, kind, namespace, name)
	if err != nil || artifact == nil {
		return fmt.Errorf("artifact not found")
	}
	return r.repo.StarArtifact(ctx, artifact.ID, username)
}

func (r *Registry) UnstarArtifact(ctx context.Context, kind, namespace, name, username string) error {
	artifact, err := r.repo.GetArtifact(ctx, kind, namespace, name)
	if err != nil || artifact == nil {
		return fmt.Errorf("artifact not found")
	}
	return r.repo.UnstarArtifact(ctx, artifact.ID, username)
}

func (r *Registry) GetStarInfo(ctx context.Context, kind, namespace, name, username string) (int64, bool) {
	artifact, err := r.repo.GetArtifact(ctx, kind, namespace, name)
	if err != nil || artifact == nil {
		return 0, false
	}
	count := r.repo.GetStarCount(ctx, artifact.ID)
	starred := username != "" && r.repo.IsStarred(ctx, artifact.ID, username)
	return count, starred
}

func (r *Registry) ListScanResults(ctx context.Context, kind, namespace, name, version string) ([]metadata.ScanResult, error) {
	v, err := r.ResolveArtifactVersion(ctx, kind, namespace, name, version)
	if err != nil {
		return nil, err
	}
	return r.repo.ListScanResults(ctx, v.ID)
}

func (r *Registry) AddScanResult(ctx context.Context, s *metadata.ScanResult) error {
	return r.repo.AddScanResult(ctx, s)
}

func (r *Registry) ClearScanResults(ctx context.Context, kind, namespace, name, version string) error {
	v, err := r.ResolveArtifactVersion(ctx, kind, namespace, name, version)
	if err != nil {
		return err
	}
	return r.repo.ClearScanResults(ctx, v.ID)
}

func (r *Registry) ListComments(ctx context.Context, kind, namespace, name string) ([]metadata.ArtifactComment, error) {
	return r.repo.ListComments(ctx, kind, namespace, name)
}

func (r *Registry) AddComment(ctx context.Context, c *metadata.ArtifactComment) error {
	return r.repo.AddComment(ctx, c)
}

func (r *Registry) UpdateComment(ctx context.Context, id int64, username, body string) error {
	return r.repo.UpdateComment(ctx, id, username, body)
}

func (r *Registry) DeleteComment(ctx context.Context, id int64, username string) error {
	return r.repo.DeleteComment(ctx, id, username)
}

func (r *Registry) CreateCollection(ctx context.Context, c *metadata.Collection) error {
	return r.repo.CreateCollection(ctx, c)
}

func (r *Registry) GetCollection(ctx context.Context, owner, slug string) (*metadata.Collection, error) {
	return r.repo.GetCollection(ctx, owner, slug)
}

func (r *Registry) ListCollections(ctx context.Context, owner string) ([]metadata.Collection, error) {
	return r.repo.ListCollections(ctx, owner)
}

func (r *Registry) UpdateCollection(ctx context.Context, owner, slug, name, description string, artifacts []metadata.CollectionArtifact, visibility string) error {
	return r.repo.UpdateCollection(ctx, owner, slug, name, description, artifacts, visibility)
}

func (r *Registry) DeleteCollection(ctx context.Context, owner, slug string) error {
	return r.repo.DeleteCollection(ctx, owner, slug)
}

func (r *Registry) GetNamespaceInsights(ctx context.Context, namespace string, days int) ([]metadata.ArtifactInsight, error) {
	return r.repo.GetNamespaceInsights(ctx, namespace, days)
}

func (r *Registry) TransferArtifact(ctx context.Context, kind, fromNS, name, toNS string) error {
	return r.repo.TransferArtifact(ctx, kind, fromNS, name, toNS)
}

func (r *Registry) FollowNamespace(ctx context.Context, follower, namespace string) error {
	return r.repo.FollowNamespace(ctx, follower, namespace)
}

func (r *Registry) UnfollowNamespace(ctx context.Context, follower, namespace string) error {
	return r.repo.UnfollowNamespace(ctx, follower, namespace)
}

func (r *Registry) IsFollowing(ctx context.Context, follower, namespace string) bool {
	return r.repo.IsFollowing(ctx, follower, namespace)
}

func (r *Registry) GetFollowerCount(ctx context.Context, namespace string) int64 {
	return r.repo.GetFollowerCount(ctx, namespace)
}

func (r *Registry) GetFollowing(ctx context.Context, follower string) ([]string, error) {
	return r.repo.GetFollowing(ctx, follower)
}
