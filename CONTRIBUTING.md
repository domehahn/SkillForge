# Contributing

Thanks for helping improve SkillForge. Keep changes focused, describe user-visible
effects, and update the relevant documentation when behavior or configuration
changes.

## Local development

The registry service lives in `skill-registry/`. From that directory:

```bash
go test ./...
go vet ./...
make build
```

For the web interface, use the package scripts in `skill-registry/web`:

```bash
npm ci
npm run build
```

Run the checks relevant to your change before opening a pull request. If an
environment limitation prevents a check, include the command and its output in
the pull request description.

## Pull requests

- Explain the problem and the chosen change.
- Include or update tests for changed behavior.
- Document configuration, API, and operational changes.
- Keep credentials, local databases, generated bundles, and build outputs out
  of commits.
- Call out known limitations and migration requirements.
- Use the repository pull request template to record validation and compatibility
  impact.

Bug reports and feature requests can be opened with the repository issue
templates. Report security vulnerabilities privately as described in
[`SECURITY.md`](SECURITY.md).

## Commit messages

Use a short, imperative summary, for example `fix: preserve data on clean` or
`docs: clarify runtime activation boundaries`.
