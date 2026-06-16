package database

import (
	"strings"
	"testing"
)

func TestRebindPlaceholders(t *testing.T) {
	got := rebindPlaceholders(`SELECT '?' AS literal, a FROM t WHERE b = ? AND c = ?`)
	want := `SELECT '?' AS literal, a FROM t WHERE b = $1 AND c = $2`
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRewritePostgresDialect(t *testing.T) {
	got := rewritePostgres(`INSERT OR IGNORE INTO artifact_stars (artifact_id, username) VALUES (?, ?)`)
	if !strings.Contains(got, "INSERT INTO artifact_stars") {
		t.Fatalf("expected INSERT OR IGNORE rewrite, got %s", got)
	}
	if !strings.Contains(got, "ON CONFLICT DO NOTHING") {
		t.Fatalf("expected conflict clause, got %s", got)
	}
	if !strings.Contains(got, "$1") || !strings.Contains(got, "$2") {
		t.Fatalf("expected postgres placeholders, got %s", got)
	}
}

func TestRewritePostgresJSONDependencyQuery(t *testing.T) {
	got := rewritePostgres(`
		SELECT 1
		FROM artifact_versions av
		WHERE json_extract(av.manifest, '$.spec.dependencies') IS NOT NULL
		  AND EXISTS (
			SELECT 1 FROM json_each(json_extract(av.manifest, '$.spec.dependencies')) dep
			WHERE json_extract(dep.value, '$.namespace') = ?
			  AND json_extract(dep.value, '$.name') = ?
		  )
	`)
	for _, want := range []string{
		"av.manifest::jsonb #> '{spec,dependencies}'",
		"jsonb_array_elements",
		"dep.value->>'namespace' = $1",
		"dep.value->>'name' = $2",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in rewritten query, got %s", want, got)
		}
	}
}
