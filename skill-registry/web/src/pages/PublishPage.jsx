import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useDocumentTitle } from '../hooks/useDocumentTitle'

function CodeBlock({ code, lang = '' }) {
  const [copied, setCopied] = useState(false)
  const copy = () => {
    navigator.clipboard.writeText(code).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }
  return (
    <div className="pub-code-block">
      {lang && <span className="pub-code-lang">{lang}</span>}
      <button className="pub-code-copy" onClick={copy}>{copied ? '✓ Copied' : 'Copy'}</button>
      <pre><code>{code}</code></pre>
    </div>
  )
}

function Step({ number, title, children }) {
  return (
    <div className="pub-step">
      <div className="pub-step-num">{number}</div>
      <div className="pub-step-body">
        <h3 className="pub-step-title">{title}</h3>
        <div className="pub-step-content">{children}</div>
      </div>
    </div>
  )
}

function Callout({ type = 'info', children }) {
  const icons = { info: 'ℹ', tip: '💡', warn: '⚠' }
  return (
    <div className={`pub-callout pub-callout-${type}`}>
      <span className="pub-callout-icon">{icons[type]}</span>
      <div>{children}</div>
    </div>
  )
}

const SKILL_YAML = `name: my-skill
version: 1.0.0
kind: skill
namespace: yourname
description: A concise description of what this skill does.
author: yourname

# Entry point executed when the skill is invoked
entrypoint: skill.py

# Runtime compatibility (optional)
runtime:
  python: ">=3.10"

# Skills this one depends on (optional)
requires:
  - acme/utils@^1.0.0`

const AGENT_YAML = `name: research-agent
version: 1.0.0
kind: agent
namespace: yourname
description: Autonomously researches topics and produces structured reports.
author: yourname

entrypoint: agent.py

# Agent-specific configuration
agent:
  model: claude-3-5-sonnet
  tools:
    - web_search
    - file_write`

const FLOW_YAML = `name: etl-pipeline
version: 1.0.0
kind: flow
namespace: yourname
description: Extracts, transforms, and loads data from CSV to database.
author: yourname

entrypoint: flow.yaml

steps:
  - name: extract
    skill: acme/csv-reader@^1.0.0
  - name: transform
    skill: acme/data-cleaner@^1.0.0
  - name: load
    skill: acme/db-writer@^1.0.0`

const CI_GITHUB = `name: Publish to SkillForge

on:
  push:
    tags:
      - 'v*'

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Python
        uses: actions/setup-python@v5
        with:
          python-version: '3.11'

      - name: Install skforge
        run: pip install skforge

      - name: Publish
        env:
          SKFORGE_TOKEN: \${{ secrets.SKFORGE_TOKEN }}
        run: |
          VERSION=\${GITHUB_REF_NAME#v}
          skforge publish --version "\$VERSION"`

const VERSIONS_GUIDE = `# Semantic versioning — required
# MAJOR.MINOR.PATCH — e.g. 2.3.1

# ✓ Good
skforge publish --version 1.0.0   # initial release
skforge publish --version 1.0.1   # patch: bug fix
skforge publish --version 1.1.0   # minor: new feature
skforge publish --version 2.0.0   # major: breaking change

# Tag a version as stable for users who install without specifying a version
skforge dist-tag add yourname/my-skill@1.2.0 latest

# Optional channels
skforge dist-tag add yourname/my-skill@1.2.0-rc.1 next
skforge dist-tag add yourname/my-skill@1.1.5 stable`

const KINDS = [
  {
    kind: 'skill',
    icon: '⬟',
    title: 'Skill',
    desc: 'A callable, single-purpose AI function. Use for discrete tasks: translate text, summarize a document, classify sentiment.',
    when: 'When your artifact has one clear input → output contract.',
  },
  {
    kind: 'agent',
    icon: '⬡',
    title: 'Agent',
    desc: 'An autonomous system that can use tools, maintain memory, and make multi-step decisions to complete a goal.',
    when: 'When your artifact needs to plan, act, and react — not just respond once.',
  },
  {
    kind: 'flow',
    icon: '⇄',
    title: 'Flow',
    desc: 'A directed graph of skills and agents wired together into a pipeline. Use for ETL, multi-stage workflows, or orchestration.',
    when: 'When you need to sequence or branch multiple skills.',
  },
  {
    kind: 'prompt',
    icon: '❝',
    title: 'Prompt',
    desc: 'A reusable, versioned prompt template. Use for system prompts, few-shot examples, or chain-of-thought patterns.',
    when: 'When the prompt itself is the reusable artifact.',
  },
  {
    kind: 'tool',
    icon: '⚙',
    title: 'Tool',
    desc: 'A function exposed to AI models as a callable tool (function calling / tool use). Use for search, file I/O, APIs.',
    when: 'When models need to call your code during generation.',
  },
  {
    kind: 'bundle',
    icon: '▣',
    title: 'Bundle',
    desc: 'A curated collection of related skills, agents, or tools distributed together. Use for starter kits or org-wide toolsets.',
    when: 'When you want to distribute a cohesive set of artifacts as one unit.',
  },
]

export default function PublishPage() {
  useDocumentTitle('Publish to SkillForge')
  const [activeKind, setActiveKind] = useState('skill')
  const YAML_BY_KIND = { skill: SKILL_YAML, agent: AGENT_YAML, flow: FLOW_YAML }
  const selectedKind = KINDS.find(k => k.kind === activeKind) || KINDS[0]

  return (
    <div className="publish-page">
      <div className="publish-hero">
        <h1>Publish to SkillForge</h1>
        <p>Share your AI skills, agents, and tools with the community in minutes.</p>
        <div className="pub-hero-actions">
          <Link to="/account/tokens" className="btn-primary">Create access token</Link>
          <a href="/api-docs" target="_blank" rel="noopener noreferrer" className="btn-secondary">API docs ↗</a>
        </div>
      </div>

      <div className="publish-layout">
        <div className="publish-main">

          {/* What to publish */}
          <section className="pub-section">
            <h2 className="pub-section-title">Choose your artifact kind</h2>
            <div className="pub-kind-grid">
              {KINDS.map(k => (
                <button
                  key={k.kind}
                  className={`pub-kind-card ${activeKind === k.kind ? 'active' : ''}`}
                  onClick={() => setActiveKind(k.kind)}
                >
                  <span className="pub-kind-icon">{k.icon}</span>
                  <span className="pub-kind-name">{k.title}</span>
                </button>
              ))}
            </div>
            <div className="pub-kind-detail">
              <div className="pub-kind-detail-header">
                <span className="pub-kind-icon">{selectedKind.icon}</span>
                <strong>{selectedKind.title}</strong>
              </div>
              <p>{selectedKind.desc}</p>
              <div className="pub-kind-when"><strong>Use when:</strong> {selectedKind.when}</div>
            </div>
          </section>

          <div className="pub-divider" />

          {/* Steps */}
          <Step number={1} title="Install the skforge CLI">
            <p>Install via pip (requires Python 3.10+):</p>
            <CodeBlock code="pip install skforge" lang="sh" />
            <p>Verify the installation:</p>
            <CodeBlock code="skforge --version" lang="sh" />
            <Callout type="tip">
              Use <code>pipx install skforge</code> for a globally isolated install that won't conflict with project dependencies.
            </Callout>
          </Step>

          <Step number={2} title="Authenticate">
            <p>Log in interactively with your SkillForge credentials:</p>
            <CodeBlock code="skforge login" lang="sh" />
            <p>For CI/CD, create an access token and export it:</p>
            <CodeBlock code={`export SKFORGE_TOKEN=skf_your_token_here\nskforge whoami  # verify`} lang="sh" />
            <Callout type="tip">
              Generate tokens at <Link to="/account/tokens">Account → Access Tokens</Link>. Use minimal scopes — CI only needs <code>write</code>.
            </Callout>
          </Step>

          <Step number={3} title="Create your manifest">
            <p>
              Create a <code>skill.yaml</code> at the root of your project. The <code>kind</code> field determines how the registry treats your artifact.
            </p>
            {YAML_BY_KIND[activeKind] ? (
              <CodeBlock code={YAML_BY_KIND[activeKind]} lang="yaml" />
            ) : (
              <CodeBlock code={SKILL_YAML} lang="yaml" />
            )}
            <Callout type="info">
              Add a <code>README.md</code> to your project root — it will be rendered on the registry page. Supports GitHub-flavored Markdown.
            </Callout>
            <div className="pub-field-table">
              {[
                ['name', 'required', 'Artifact name. Lowercase, hyphens allowed. Must be unique within your namespace.'],
                ['version', 'required', 'Semantic version (MAJOR.MINOR.PATCH). Must be incremented on each publish.'],
                ['kind', 'required', 'One of: skill, agent, flow, prompt, tool, bundle.'],
                ['description', 'recommended', 'One-line description shown in search results.'],
                ['entrypoint', 'recommended', 'File executed when the artifact runs.'],
                ['requires', 'optional', 'Dependencies on other SkillForge artifacts.'],
                ['readme', 'optional', 'Path to README file. Defaults to README.md if present.'],
              ].map(([field, req, desc]) => (
                <div key={field} className="pub-field-row">
                  <code className="pub-field-name">{field}</code>
                  <span className={`pub-field-req pub-req-${req}`}>{req}</span>
                  <span className="pub-field-desc">{desc}</span>
                </div>
              ))}
            </div>
          </Step>

          <Step number={4} title="Publish">
            <p>From your project directory:</p>
            <CodeBlock code="skforge publish" lang="sh" />
            <p>Publish a specific version explicitly:</p>
            <CodeBlock code="skforge publish --version 1.2.0" lang="sh" />
            <p>Publish as a different kind than declared in the manifest:</p>
            <CodeBlock code="skforge publish --kind agent" lang="sh" />
            <Callout type="warn">
              Published versions are <strong>immutable</strong>. You cannot overwrite an existing version. Use <code>--force</code> only in development.
            </Callout>
          </Step>

          <Step number={5} title="Versioning & channels">
            <p>SkillForge uses <a href="https://semver.org" target="_blank" rel="noopener noreferrer">Semantic Versioning</a>. The <code>latest</code> dist-tag is automatically pointed at the newest published version.</p>
            <CodeBlock code={VERSIONS_GUIDE} lang="sh" />
          </Step>

          <Step number={6} title="CI/CD integration">
            <p>Automatically publish on every tagged release using GitHub Actions:</p>
            <CodeBlock code={CI_GITHUB} lang="yaml" />
            <p>Add your token as a repository secret named <code>SKFORGE_TOKEN</code> in your GitHub repository settings.</p>
          </Step>

          <Step number={7} title="Managing your artifact">
            <p>After publishing, you can manage your artifact from the registry UI or CLI:</p>
            <CodeBlock code={`# Deprecate a version with a migration message
skforge deprecate yourname/my-skill@1.0.0 --message "Upgrade to 2.0.0 for improved performance"

# Yank a version (hides it from install resolution)
skforge yank yourname/my-skill@1.0.0

# Update description and tags
skforge artifact update yourname/my-skill \\
  --description "Updated description" \\
  --tag production --tag stable`} lang="sh" />
            <Callout type="tip">
              You can also edit the description, tags, README, and visibility directly from the artifact's <strong>Settings</strong> tab in the registry UI.
            </Callout>
          </Step>

        </div>

        <aside className="publish-aside">
          <div className="pub-aside-card">
            <div className="pub-aside-title">Quick links</div>
            <Link to="/account/tokens" className="pub-aside-link">Access Tokens</Link>
            <a href="/api-docs" target="_blank" rel="noopener noreferrer" className="pub-aside-link">API Documentation ↗</a>
            <a href="/api/v1/openapi.yaml" target="_blank" rel="noopener noreferrer" className="pub-aside-link">OpenAPI Spec ↗</a>
            <Link to="/" className="pub-aside-link">Browse Registry</Link>
          </div>

          <div className="pub-aside-card">
            <div className="pub-aside-title">Checklist</div>
            {[
              'skill.yaml with name, version, kind',
              'Semantic version (1.0.0)',
              'One-line description',
              'README.md with usage examples',
              'Dependencies declared in requires',
              'Tested locally before publish',
            ].map(item => (
              <div key={item} className="pub-check-row">
                <span className="pub-check-icon">◻</span>
                <span>{item}</span>
              </div>
            ))}
          </div>

          <div className="pub-aside-card">
            <div className="pub-aside-title">Artifact kinds</div>
            {KINDS.map(({ kind, icon, title, desc }) => (
              <div key={kind} className="pub-kind-row">
                <Link to={`/?kind=${kind}`} className="kind-chip kind-chip-sm">{kind}</Link>
                <span className="pub-kind-desc">{title}</span>
              </div>
            ))}
          </div>

          <div className="pub-aside-card pub-aside-info">
            <div className="pub-aside-title">Need help?</div>
            <p>Browse the <a href="/api-docs" target="_blank" rel="noopener noreferrer">API docs</a> or <Link to="/">explore the registry</Link> to see how others structure their artifacts.</p>
          </div>
        </aside>
      </div>
    </div>
  )
}
