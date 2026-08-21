# Cross-repository compatibility matrix

SkillForge (`skill-registry`) is the registry/server end of a four-repo
agentic skill supply chain:

```
skcr (author/compile)  →  skil (scan/attest)  →  skpm (package/publish)  →  SkillForge (registry)
```

SkillForge is a server with a REST API contract; the compatibility that
matters is "does skpm's client code still work against this server",
which is naturally tested from skpm's side (the client), not by
SkillForge pinning a version of skpm and driving it — SkillForge has no
CI job of its own that checks out skpm or skil.

| Pairing                                             | Enforced by                                                                                 | Currently pinned to | Status |
|-------------------------------------------------------|-----------------------------------------------------------------------------------------------|----------------------|--------|
| skpm `main` (current) × SkillForge `main` (current)   | [skpm's `.github/workflows/ci.yml` → `skillforge-e2e`](https://github.com/domehahn/skpm/blob/main/.github/workflows/ci.yml) | [this repo @ 5e80b37](https://github.com/domehahn/SkillForge/commit/5e80b37ce5c7d6d4d51c5ab578e2bbfb23e5600a) (as pinned by skpm) | ✅ enforced (from skpm's side) |
| skpm `main` (current) × SkillForge stable             | —                                                                                               | — | ⏳ not yet available: SkillForge has no tagged release yet |

## What this means in practice

Whenever this repo's `main` changes in a way that could break skpm's
`AttestationRegistry`/package-publish/download client contract, that
won't be caught by *this* repo's own CI — it surfaces the next time
skpm's `skillforge-e2e` job bumps its pinned commit and re-runs the real
cross-repo contract test (`skpm/tests/integration/skillforge_e2e_test.go`,
tag `e2e`) against a real server built from the new commit. See skpm's
own `COMPATIBILITY.md` for exactly what that test checks and how the pin
gets bumped.

## Once SkillForge has a tagged release

The "SkillForge stable" cell becomes available once this repo cuts its
first tag (mirroring what skil did for [v0.2.0](https://github.com/domehahn/skil/releases/tag/v0.2.0)) — at that point skpm's
`skillforge-e2e` job should track a release tag instead of a commit SHA,
the same upgrade already made for skpm/skcr's skil-interop pins.
