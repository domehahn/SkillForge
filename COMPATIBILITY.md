# Toolchain compatibility

The four products remain independent: skcr compiles, skil evaluates trust,
skpm packages and installs, SkillForge stores and governs artifacts.

`.github/workflows/toolchain.yml` runs real consumer tests on every PR,
main push, daily schedule and manual dispatch. The repository being changed
uses the event commit (including PR merge contents); its sibling uses main
or an explicit stable release. Exact checkout SHAs are retained as artifacts.
No cross-repository write token or repository_dispatch secret is required.

Supported stable baselines: skil **v0.6.0**, skpm **v2.3.0**, SkillForge **v1.0.0**.
SkillForge stable compatibility matrix cells (`skpm current × SkillForge v1.0.0` and
`skpm v2.3.0 × SkillForge v1.0.0`) are configured in `.github/workflows/toolchain.yml`.

Each producer runs its consumers: skil PRs compile skcr fixtures and run
skpm's signature verifier; SkillForge PRs run current and stable skpm.
Consumer PRs also run against current providers and available stable baselines.
Select Toolchain compatibility as required checks in branch protection.
Source-controlled workflows cannot configure remote branch protection.

Before moving a stable pin, run the same tests at the new tag and record the
resolved commit. Main is deliberately floating for drift detection; the logged
SHAs make every failure reproducible. No current/stable pairing implies full
native isolation, release signing, HA, or whole-toolchain readiness.
