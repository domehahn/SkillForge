## Integration Test Plan

This document outlines integration test scenarios for the Skill Registry.

### Test Scenarios

#### 1. Full Publish-Download Cycle
- Create a skill package
- Publish to registry
- Verify metadata is stored
- Download the package
- Verify digest matches
- Extract and validate content

#### 2. Duplicate Version Handling
- Publish version 1.0.0
- Attempt to publish 1.0.0 again (should fail)
- Attempt to force-publish 1.0.0 again (should fail because versions are immutable)

#### 3. Version Resolution
- Publish versions 1.0.0, 1.1.0, 2.0.0
- Request "latest" (should return 2.0.0)
- Request specific version
- Request non-existent version (should 404)

#### 4. Authentication Flow
- Enable auth in config
- Attempt publish without token (should 401)
- Publish with read-only token (should 403)
- Publish with write token (should succeed)
- Delete with write token (should 403)
- Delete with delete token (should succeed)

#### 5. Validation Errors
- Package without SKILL.md (should fail)
- Package with empty SKILL.md (should fail)
- Package with path traversal (should fail)
- Package with blocked extension (should fail)
- Package exceeding size limit (should fail)

#### 6. Search and List
- Publish multiple skills
- List all skills
- Search by query
- Filter by namespace
- Pagination

#### 7. CLI Integration
- skforge login
- skforge publish (directory)
- skforge publish (archive)
- skforge search
- skforge info
- skforge install
- skforge validate

### Running Integration Tests

```bash
# Start test registry
docker-compose -f docker-compose.test.yml up -d

# Run integration tests
go test -v -tags=integration ./tests/...

# Cleanup
docker-compose -f docker-compose.test.yml down
```

### Test Data

Create test skills in `tests/fixtures/`:
- valid-skill/ - Minimal valid skill
- no-skill-md/ - Missing SKILL.md
- path-traversal/ - Contains ../../../
- blocked-extension/ - Contains .exe file
- with-metadata/ - Full metadata in frontmatter
