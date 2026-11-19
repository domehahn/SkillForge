package api

import (
	"testing"

	"github.com/skillforge/skill-registry/internal/metadata"
)

func TestActivationErrorDetailsAreMaintainerOnly(t *testing.T) {
	activation := &metadata.Activation{LastError: "container failed at /private/runtime/path"}
	redactActivationError(activation, false)
	if activation.LastError != "" {
		t.Fatalf("non-maintainer should not see runtime details: %q", activation.LastError)
	}
	activation.LastError = "safe diagnostic"
	redactActivationError(activation, true)
	if activation.LastError != "safe diagnostic" {
		t.Fatalf("maintainer error detail was unexpectedly redacted: %q", activation.LastError)
	}
}
