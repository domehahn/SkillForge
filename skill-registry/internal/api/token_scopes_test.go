package api

import (
	"math"
	"testing"
)

func TestUserTokenScopesAllowOnlyBoundedRuntimeReporting(t *testing.T) {
	loginScopes := loginScopesForRole("user")
	for _, want := range []string{"read", "write", "token:create", "token:revoke"} {
		if !contains(loginScopes, want) {
			t.Errorf("user login scopes missing %q: %v", want, loginScopes)
		}
	}
	for _, forbidden := range []string{"admin", "delete", "namespace:admin", "runtime:report"} {
		if contains(loginScopes, forbidden) {
			t.Errorf("user login unexpectedly has %q: %v", forbidden, loginScopes)
		}
	}

	creatable := creatableTokenScopesForRole("user")
	if !contains(creatable, "runtime:report") {
		t.Fatalf("users must be able to create a narrowly scoped runtime token: %v", creatable)
	}
	for _, forbidden := range []string{"admin", "delete", "namespace:admin", "token:create", "token:revoke"} {
		if contains(creatable, forbidden) {
			t.Errorf("user can create token with excessive scope %q: %v", forbidden, creatable)
		}
	}
}

func TestAdminTokenScopesRemainExplicit(t *testing.T) {
	if !contains(creatableTokenScopesForRole("admin"), "runtime:report") {
		t.Fatal("admin must be able to provision the dedicated runtime report token")
	}
}

func TestValidateTokenExpiryRejectsInvalidAndOverflowingDurations(t *testing.T) {
	for _, value := range []int64{0, -1, math.MaxInt64, 365*24*60*60 + 1} {
		if _, err := validateTokenExpiry(&value); err == nil {
			t.Errorf("expected expiry %d seconds to be rejected", value)
		}
	}
	valid := int64(365 * 24 * 60 * 60)
	duration, err := validateTokenExpiry(&valid)
	if err != nil || duration == nil || *duration != 365*24*60*60*1e9 {
		t.Fatalf("valid maximum expiry was not accepted: duration=%v err=%v", duration, err)
	}
}
