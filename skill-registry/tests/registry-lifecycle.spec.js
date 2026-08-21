import { expect, test } from '@playwright/test'
import crypto from 'node:crypto'
import zlib from 'node:zlib'

const baseURL = process.env.SKILLFORGE_E2E_BASE_URL || 'http://localhost:8082'

function tarHeader(name, size) {
  const header = Buffer.alloc(512, 0)
  header.write(name, 0, 100)
  header.write('0000644\0', 100, 8)
  header.write('0000000\0', 108, 8)
  header.write('0000000\0', 116, 8)
  header.write(size.toString(8).padStart(11, '0') + '\0', 124, 12)
  header.write(Math.floor(Date.now() / 1000).toString(8).padStart(11, '0') + '\0', 136, 12)
  header.fill(0x20, 148, 156)
  header.write('0', 156, 1)
  header.write('ustar\0', 257, 6)
  header.write('00', 263, 2)

  let checksum = 0
  for (const byte of header) checksum += byte
  header.write(checksum.toString(8).padStart(6, '0') + '\0 ', 148, 8)
  return header
}

function tgz(files) {
  const chunks = []
  for (const [name, content] of Object.entries(files)) {
    const body = Buffer.from(content)
    chunks.push(tarHeader(name, body.length), body)
    const padding = (512 - (body.length % 512)) % 512
    if (padding) chunks.push(Buffer.alloc(padding, 0))
  }
  chunks.push(Buffer.alloc(1024, 0))
  return zlib.gzipSync(Buffer.concat(chunks))
}

function artifactManifest(kind, namespace, name, version, entrypoint, extraSpec = '') {
  return `apiVersion: skillforge.dev/v1
kind: ${kind}
metadata:
  namespace: ${namespace}
  name: ${name}
  version: ${version}
  description: ${kind} lifecycle artifact
  visibility: public
  tags:
    - e2e
    - ${kind}
  owners:
    - admin
spec:
  entrypoint: ${entrypoint}
${extraSpec}`
}

function genericPackage(kind, namespace, name, version) {
  const entrypoints = {
    skill: 'SKILL.md',
    agent: 'AGENT.md',
    prompt: 'PROMPT.md',
    mcp: 'mcp.yaml',
    tool: 'TOOL.md',
    bundle: 'BUNDLE.md',
    flow: 'FLOW.yaml',
  }
  const entrypoint = entrypoints[kind]
  const files = {
    'artifact.yaml': artifactManifest(kind, namespace, name, version, entrypoint),
    [entrypoint]: `# ${name}\n\nLifecycle test package for ${kind}.\n`,
  }
  if (kind === 'agent') {
    files['artifact.yaml'] = artifactManifest(kind, namespace, name, version, entrypoint, `  agent:
    model: gpt-4.1-mini
    system_prompt: You verify SkillForge lifecycle behavior.
    tools:
      - registry.search
`)
  }
  if (kind === 'mcp') {
    files[entrypoint] = `transport: stdio
command:
  - node
args:
  - server.js
tools:
  - name: echo
    description: Echo input
`
    files['artifact.yaml'] = artifactManifest(kind, namespace, name, version, entrypoint, `  mcp:
    transport: stdio
    command:
      - node
    args:
      - server.js
    tools:
      - name: echo
        description: Echo input
`)
  }
  if (kind === 'flow') {
    files[entrypoint] = `apiVersion: skillforge.dev/v1
kind: flow
metadata:
  namespace: ${namespace}
  name: ${name}
  version: ${version}
spec:
  entrypoint: FLOW.yaml
  steps:
    - id: start
      uses: prompt/${namespace}/prompt-${name}@1.0.0
`
    files['artifact.yaml'] = artifactManifest(kind, namespace, name, version, entrypoint, `  steps:
    - id: start
      uses: prompt/${namespace}/prompt-${name}@1.0.0
`)
  }
  return tgz(files)
}

function legacySkillPackage(namespace, name, version) {
  const skillMD = `# ${name}\n`
  const sha = crypto.createHash('sha256').update(skillMD).digest('hex')
  return tgz({
    'SKILL.md': skillMD,
    VERSION: `${version}\n`,
    'skill.yaml': `name: ${name}
namespace: ${namespace}
version: ${version}
description: legacy lifecycle skill
owners:
  - admin
license: MIT
compatible_with:
  - codex
entrypoint: SKILL.md
security:
  requires_network: false
  requires_secrets: false
  writes_files: false
  runs_commands: false
`,
    'CHANGELOG.md': `# Changelog\n\n## ${version}\n\n### Added\n- lifecycle test\n`,
    'manifest.json': JSON.stringify({
      spec_version: 1,
      name,
      namespace,
      version,
      entrypoint: 'SKILL.md',
      compatible_with: ['codex'],
      package_type: 'tgz',
      packaged_by: 'playwright',
      packaged_at: '1970-01-01T00:00:00Z',
    }),
    'checksums.txt': `${sha}  SKILL.md\n`,
  })
}

async function ok(response, label) {
  if (!response.ok()) {
    throw new Error(`${label}: ${response.status()} ${await response.text()}`)
  }
  return response
}

async function json(response, label) {
  await ok(response, label)
  return response.json()
}

async function authHeaders(request) {
  const capabilitiesResp = await request.get(`${baseURL}/api/v1/capabilities`)
  if (capabilitiesResp.ok()) {
    const capabilities = await capabilitiesResp.json()
    if (capabilities.auth_enabled === false) {
      return {
        headers: { Authorization: 'Bearer e2e-auth-disabled' },
        user: 'anonymous',
      }
    }
  }

  const login = await json(await request.post(`${baseURL}/api/v1/auth/login`, {
    data: { username: 'admin', password: 'changeme' },
  }), 'login')
  return {
    headers: { Authorization: `Bearer ${login.token}` },
    user: login.user || 'admin',
  }
}

test('registry lifecycle: publish, download, update metadata, tag, comment, collect, attest, and delete', async ({ request }) => {
  const stamp = Date.now().toString(36)
  const namespace = `e2e-${stamp}`
  const auth = await authHeaders(request)
  const headers = auth.headers

  const kinds = ['skill', 'agent', 'prompt', 'mcp', 'tool', 'bundle']
  for (const kind of kinds) {
    const name = `${kind}-${stamp}`
    const version = '1.0.0'
    const pkg = genericPackage(kind, namespace, name, version)

    const published = await json(await request.put(
      `${baseURL}/api/v1/artifacts/${kind}/${namespace}/${name}/versions/${version}`,
      {
        headers: { ...headers, 'Content-Type': 'application/gzip' },
        data: pkg,
      },
    ), `${kind} publish`)
    expect(published.artifact.kind).toBe(kind)
    expect(published.version.digest_sha256).toMatch(/^[a-f0-9]{64}$/)

    const detail = await json(await request.get(`${baseURL}/api/v1/artifacts/${kind}/${namespace}/${name}`), `${kind} detail`)
    expect(detail.artifact.latest_version).toBe(version)
    expect(detail.dist_tags.latest).toBe(version)

    const download = await ok(await request.get(
      `${baseURL}/api/v1/artifacts/${kind}/${namespace}/${name}/versions/${version}/download`,
    ), `${kind} download`)
    expect(Buffer.from(await download.body()).equals(pkg)).toBeTruthy()
    expect(download.headers()['x-artifact-digest-sha256']).toBe(published.version.digest_sha256)

    const updatedDescription = `${kind} description updated by lifecycle test`
    const patched = await json(await request.patch(`${baseURL}/api/v1/artifacts/${kind}/${namespace}/${name}`, {
      headers,
      data: {
        description: updatedDescription,
        readme: `# ${name}\nUpdated README from lifecycle test.`,
        tags: ['e2e', 'updated', kind],
        visibility: 'public',
      },
    }), `${kind} patch metadata`)
    expect(patched.artifact.description).toBe(updatedDescription)
    expect(patched.artifact.tags).toContain('updated')

    await json(await request.post(`${baseURL}/api/v1/artifacts/${kind}/${namespace}/${name}/promotions`, {
      headers,
      data: { version, channel: 'beta' },
    }), `${kind} promote`)
    const promotions = await json(await request.get(`${baseURL}/api/v1/artifacts/${kind}/${namespace}/${name}/promotions`), `${kind} promotions`)
    expect(promotions.promotions.some(p => p.channel === 'beta' && p.version === version)).toBeTruthy()

    await json(await request.patch(`${baseURL}/api/v1/artifacts/${kind}/${namespace}/${name}/versions/${version}`, {
      headers,
      data: { release_notes: `Release notes for ${kind}` },
    }), `${kind} release notes`)

    await json(await request.post(`${baseURL}/api/v1/artifacts/${kind}/${namespace}/${name}/versions/${version}/attestations`, {
      headers,
      data: { type: 'provenance', digest: `sha256:${published.version.digest_sha256}`, predicate: { e2e: true, kind } },
    }), `${kind} attestation`)
    const attestations = await json(await request.get(
      `${baseURL}/api/v1/artifacts/${kind}/${namespace}/${name}/versions/${version}/attestations`,
    ), `${kind} attestations`)
    expect(attestations.attestations.some(a => a.type === 'provenance')).toBeTruthy()
  }

  const promptName = `prompt-${stamp}`
  const comment = await json(await request.post(`${baseURL}/api/v1/artifacts/prompt/${namespace}/${promptName}/comments`, {
    headers,
    data: { body: 'Initial lifecycle comment' },
  }), 'add comment')
  await json(await request.patch(`${baseURL}/api/v1/comments/${comment.id}`, {
    headers,
    data: { body: 'Updated lifecycle comment' },
  }), 'update comment')
  const comments = await json(await request.get(`${baseURL}/api/v1/artifacts/prompt/${namespace}/${promptName}/comments`), 'list comments')
  expect(comments.comments.some(c => c.body === 'Updated lifecycle comment')).toBeTruthy()
  await ok(await request.delete(`${baseURL}/api/v1/comments/${comment.id}`, { headers }), 'delete comment')

  await json(await request.post(`${baseURL}/api/v1/artifacts/prompt/${namespace}/${promptName}/star`, { headers }), 'star prompt')
  let stars = await json(await request.get(`${baseURL}/api/v1/artifacts/prompt/${namespace}/${promptName}/stars`, { headers }), 'star info')
  expect(stars.starred).toBeTruthy()
  expect(stars.stars).toBeGreaterThanOrEqual(1)
  await ok(await request.delete(`${baseURL}/api/v1/artifacts/prompt/${namespace}/${promptName}/star`, { headers }), 'unstar prompt')
  stars = await json(await request.get(`${baseURL}/api/v1/artifacts/prompt/${namespace}/${promptName}/stars`, { headers }), 'star info after unstar')
  expect(stars.starred).toBeFalsy()

  const collection = await json(await request.post(`${baseURL}/api/v1/collections`, {
    headers,
    data: { name: `Lifecycle ${stamp}`, description: 'Lifecycle collection', visibility: 'public' },
  }), 'create collection')
  const collectionWithArtifact = await json(await request.put(`${baseURL}/api/v1/namespaces/${auth.user}/collections/${collection.slug}`, {
    headers,
    data: {
      name: collection.name,
      description: 'Updated lifecycle collection',
      visibility: 'public',
      artifacts: [{ kind: 'prompt', namespace, name: promptName }],
    },
  }), 'update collection')
  expect(collectionWithArtifact.artifacts).toEqual([{ kind: 'prompt', namespace, name: promptName }])
  await ok(await request.delete(`${baseURL}/api/v1/namespaces/${auth.user}/collections/${collection.slug}`, { headers }), 'delete collection')

  for (const kind of kinds) {
    const name = `${kind}-${stamp}`
    await json(await request.delete(`${baseURL}/api/v1/artifacts/${kind}/${namespace}/${name}/versions/1.0.0`, { headers }), `${kind} delete`)
    const deletedDownload = await request.get(`${baseURL}/api/v1/artifacts/${kind}/${namespace}/${name}/versions/1.0.0/download`)
    expect(deletedDownload.status(), `${kind} deleted download status`).toBe(404)
  }

  const legacyName = `legacy-${stamp}`
  const legacyPkg = legacySkillPackage(namespace, legacyName, '1.0.0')
  const legacyPublish = await json(await request.put(`${baseURL}/api/v1/skills/${namespace}/${legacyName}/versions/1.0.0`, {
    headers: { ...headers, 'Content-Type': 'application/gzip' },
    data: legacyPkg,
  }), 'legacy publish')
  expect(legacyPublish.sha256).toMatch(/^[a-f0-9]{64}$/)

  await json(await request.put(`${baseURL}/api/v1/skills/${namespace}/${legacyName}/dist-tags/stable`, {
    headers,
    data: { version: '1.0.0' },
  }), 'legacy dist tag')
  await json(await request.post(`${baseURL}/api/v1/skills/${namespace}/${legacyName}/versions/1.0.0/yank`, {
    headers,
    data: { reason: 'Lifecycle yank check' },
  }), 'legacy yank')
  await json(await request.post(`${baseURL}/api/v1/skills/${namespace}/${legacyName}/versions/1.0.0/unyank`, { headers }), 'legacy unyank')
  await json(await request.post(`${baseURL}/api/v1/skills/${namespace}/${legacyName}/versions/1.0.0/deprecate`, {
    headers,
    data: { reason: 'Lifecycle deprecation check' },
  }), 'legacy deprecate')

  const legacyDownload = await ok(await request.get(
    `${baseURL}/api/v1/skills/${namespace}/${legacyName}/versions/1.0.0/download`,
  ), 'legacy download')
  expect(Buffer.from(await legacyDownload.body()).equals(legacyPkg)).toBeTruthy()

  await json(await request.delete(`${baseURL}/api/v1/skills/${namespace}/${legacyName}/versions/1.0.0`, { headers }), 'legacy delete')
  const deletedDownload = await request.get(`${baseURL}/api/v1/skills/${namespace}/${legacyName}/versions/1.0.0/download`)
  expect(deletedDownload.status()).toBe(404)
})
