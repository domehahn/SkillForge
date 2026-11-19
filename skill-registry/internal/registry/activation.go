package registry

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/skillforge/skill-registry/internal/metadata"
	"github.com/skillforge/skill-registry/internal/validation"
)

func (r *Registry) ListActivations(ctx context.Context, namespace string) ([]metadata.Activation, error) {
	return r.repo.ListActivations(ctx, namespace)
}

func (r *Registry) GetActivation(ctx context.Context, namespace, kind, name string) (*metadata.Activation, []metadata.ActivationEvent, error) {
	a, err := r.repo.GetActivation(ctx, namespace, kind, name)
	if err != nil || a == nil {
		return a, nil, err
	}
	events, err := r.repo.ListActivationEvents(ctx, a.ID)
	return a, events, err
}

func (r *Registry) ChangeActivation(ctx context.Context, namespace, kind, name, version, action, reason, actor string, requireVerifiedSignature bool) (*metadata.Activation, error) {
	kind = strings.ToLower(kind)
	if err := validation.ValidateArtifactKind(kind); err != nil {
		return nil, err
	}
	if reason != "" && len(reason) > 1000 {
		return nil, fmt.Errorf("reason must be <= 1000 characters")
	}
	current, err := r.repo.GetActivation(ctx, namespace, kind, name)
	if err != nil {
		return nil, err
	}
	fromState := ""
	if current == nil {
		current = &metadata.Activation{
			Namespace: namespace, Kind: kind, Name: name,
			DesiredState:  metadata.ActivationStateDiscovered,
			ObservedState: metadata.ActivationStateDiscovered,
			Generation:    0, CreatedBy: actor,
		}
	} else {
		fromState = current.DesiredState
	}
	current.Reason = reason
	current.Generation++

	switch action {
	case "install":
		if version == "" {
			return nil, fmt.Errorf("version is required for install")
		}
		artifactVersion, err := r.ResolveArtifactVersion(ctx, kind, namespace, name, version)
		if err != nil {
			return nil, err
		}
		if current.Version != "" && current.Version != artifactVersion.Version {
			current.PreviousVersion = current.Version
		}
		current.Version = artifactVersion.Version
		current.DesiredState = metadata.ActivationStateInstalled
		current.ApprovedBy = ""
		current.ApprovedAt = nil
		current.LastError = ""
	case "approve":
		if current.Version == "" {
			return nil, fmt.Errorf("artifact must be installed before approval")
		}
		if err := r.activationAdmission(ctx, kind, namespace, name, current.Version, requireVerifiedSignature); err != nil {
			return nil, err
		}
		now := time.Now().UTC()
		current.ApprovedBy = actor
		current.ApprovedAt = &now
		current.DesiredState = metadata.ActivationStateApproved
		current.LastError = ""
	case "activate":
		if current.ApprovedAt == nil || current.ApprovedBy == "" {
			return nil, fmt.Errorf("artifact must be explicitly approved before activation")
		}
		if err := r.activationAdmission(ctx, kind, namespace, name, current.Version, requireVerifiedSignature); err != nil {
			return nil, err
		}
		current.DesiredState = metadata.ActivationStateActive
		current.LastError = ""
	case "disable":
		if current.Version == "" {
			return nil, fmt.Errorf("activation does not exist")
		}
		current.DesiredState = metadata.ActivationStateDisabled
	case "rollback":
		if current.PreviousVersion == "" {
			return nil, fmt.Errorf("no previous version is available for rollback")
		}
		if _, err := r.ResolveArtifactVersion(ctx, kind, namespace, name, current.PreviousVersion); err != nil {
			return nil, fmt.Errorf("previous version is unavailable: %w", err)
		}
		current.Version, current.PreviousVersion = current.PreviousVersion, current.Version
		current.DesiredState = metadata.ActivationStateRollback
		current.ApprovedBy = ""
		current.ApprovedAt = nil
	default:
		return nil, fmt.Errorf("unsupported activation action %q", action)
	}

	if err := r.repo.UpsertActivation(ctx, current, action, actor, fromState); err != nil {
		return nil, err
	}
	_ = r.repo.LogAudit(ctx, &metadata.AuditLog{Action: "activation." + action, Namespace: namespace, Name: name, Version: current.Version, Actor: actor, Success: true, Message: kind})
	r.deliverWebhookEvent(ctx, namespace, "activation."+action, current)
	return current, nil
}

func (r *Registry) ReportActivation(ctx context.Context, namespace, kind, name, observedState, lastError, actor string, generation int64) (*metadata.Activation, error) {
	a, err := r.repo.GetActivation(ctx, namespace, kind, name)
	if err != nil || a == nil {
		return nil, fmt.Errorf("activation not found")
	}
	if generation != a.Generation {
		return nil, fmt.Errorf("stale generation %d; current generation is %d", generation, a.Generation)
	}
	if !validActivationState(observedState) {
		return nil, fmt.Errorf("invalid observed state %q", observedState)
	}
	fromState := a.ObservedState
	a.ObservedState = observedState
	a.LastError = lastError
	if len(a.LastError) > 2000 {
		a.LastError = a.LastError[:2000]
	}
	if err := r.repo.UpsertActivation(ctx, a, "report", actor, fromState); err != nil {
		return nil, err
	}
	return a, nil
}

func (r *Registry) activationAdmission(ctx context.Context, kind, namespace, name, version string, requireVerifiedSignature bool) error {
	v, err := r.ResolveArtifactVersion(ctx, kind, namespace, name, version)
	if err != nil {
		return err
	}
	if v.ValidationStatus != "valid" {
		return fmt.Errorf("artifact validation status %q does not permit activation", v.ValidationStatus)
	}
	if executableArtifactKind(kind) {
		if v.ScanStatus != "passed" && v.ScanStatus != "clean" && v.ScanStatus != "verified" {
			return fmt.Errorf("executable artifact requires a passed scan before approval")
		}
		if requireVerifiedSignature && v.SignatureStatus != "verified" {
			return fmt.Errorf("verified artifact signature is required for activation")
		}
	}
	return nil
}

func executableArtifactKind(kind string) bool {
	switch kind {
	case metadata.ArtifactKindMCP, metadata.ArtifactKindHook, metadata.ArtifactKindPlugin,
		metadata.ArtifactKindContainer, metadata.ArtifactKindModelRuntime:
		return true
	default:
		return false
	}
}

func validActivationState(state string) bool {
	switch state {
	case metadata.ActivationStateDiscovered, metadata.ActivationStateInstalled,
		metadata.ActivationStateApproved, metadata.ActivationStateActive,
		metadata.ActivationStateDisabled, metadata.ActivationStateFailed,
		metadata.ActivationStateRollback:
		return true
	default:
		return false
	}
}
