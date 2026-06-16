import { useState } from 'react'
import { useDocumentTitle } from '../hooks/useDocumentTitle'

function CopyButton({ text }) {
  const [copied, setCopied] = useState(false)
  return (
    <button
      className={`copy-btn ${copied ? 'copied' : ''}`}
      onClick={() => { navigator.clipboard.writeText(text); setCopied(true); setTimeout(() => setCopied(false), 1500) }}
    >{copied ? '✓' : 'Copy'}</button>
  )
}

const PLATFORMS = [
  { id: 'mac', label: 'macOS', icon: '' },
  { id: 'linux', label: 'Linux', icon: '🐧' },
  { id: 'windows', label: 'Windows', icon: '🪟' },
]

const INSTALL_CMDS = {
  mac: {
    brew: 'brew install skillforge/tap/skforge',
    curl: 'curl -fsSL https://get.skillforge.dev/install.sh | sh',
  },
  linux: {
    curl: 'curl -fsSL https://get.skillforge.dev/install.sh | sh',
    deb: 'sudo apt install skforge',
    rpm: 'sudo yum install skforge',
  },
  windows: {
    winget: 'winget install SkillForge.skforge',
    scoop: 'scoop install skforge',
    ps: 'irm https://get.skillforge.dev/install.ps1 | iex',
  },
}

const METHOD_LABELS = {
  brew: 'Homebrew', curl: 'Shell script', deb: 'APT (Debian/Ubuntu)',
  rpm: 'YUM/DNF (RHEL/Fedora)', winget: 'winget', scoop: 'Scoop', ps: 'PowerShell',
}

export default function InstallPage() {
  useDocumentTitle('Install skforge CLI')
  const [platform, setPlatform] = useState('mac')

  const methods = INSTALL_CMDS[platform]

  return (
    <div className="install-page">
      <div className="install-hero">
        <div className="install-hero-inner">
          <h1 className="install-hero-title">Install the SkillForge CLI</h1>
          <p className="install-hero-sub">
            <code>skforge</code> — publish, pull, and manage AI skills from your terminal.
          </p>
          <div className="install-version-badge">Latest: v0.9.2</div>
        </div>
      </div>

      <div className="install-body">
        {/* Platform tabs */}
        <div className="install-platform-tabs">
          {PLATFORMS.map(p => (
            <button
              key={p.id}
              className={`install-platform-btn ${platform === p.id ? 'active' : ''}`}
              onClick={() => setPlatform(p.id)}
            >
              {p.icon} {p.label}
            </button>
          ))}
        </div>

        {/* Install methods */}
        <div className="install-methods">
          {Object.entries(methods).map(([method, cmd], i) => (
            <div key={method} className={`install-method ${i === 0 ? 'install-method-primary' : ''}`}>
              <div className="install-method-label">
                {METHOD_LABELS[method]}
                {i === 0 && <span className="install-recommended">Recommended</span>}
              </div>
              <div className="install-code-row">
                <code className="install-code">{cmd}</code>
                <CopyButton text={cmd} />
              </div>
            </div>
          ))}
        </div>

        {/* Quick start */}
        <div className="install-section">
          <h2 className="install-section-title">Quick start</h2>
          <div className="install-steps">
            {[
              { n: '1', title: 'Authenticate', cmd: 'skforge login' },
              { n: '2', title: 'Pull a skill', cmd: 'skforge pull myorg/my-skill:latest' },
              { n: '3', title: 'Publish your own', cmd: 'skforge publish ./my-skill.tgz' },
            ].map(step => (
              <div key={step.n} className="install-step">
                <div className="install-step-num">{step.n}</div>
                <div className="install-step-body">
                  <div className="install-step-title">{step.title}</div>
                  <div className="install-code-row">
                    <code className="install-code install-code-sm">{step.cmd}</code>
                    <CopyButton text={step.cmd} />
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Useful commands */}
        <div className="install-section">
          <h2 className="install-section-title">Common commands</h2>
          <table className="install-cmd-table">
            <tbody>
              {[
                ['skforge login', 'Authenticate with the registry'],
                ['skforge pull <ns>/<name>[:ver]', 'Download a skill or agent'],
                ['skforge publish <file>', 'Publish a packaged artifact'],
                ['skforge search <query>', 'Search the registry'],
                ['skforge info <ns>/<name>', 'Show artifact metadata'],
                ['skforge yank <ns>/<name>@<ver>', 'Yank a specific version'],
                ['skforge logout', 'Remove stored credentials'],
              ].map(([cmd, desc]) => (
                <tr key={cmd}>
                  <td><code className="install-cmd-cell">{cmd}</code></td>
                  <td className="install-cmd-desc">{desc}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Links */}
        <div className="install-links">
          <a href="/api-docs" className="install-link-card" target="_blank" rel="noopener noreferrer">
            <div className="install-link-title">API Reference</div>
            <div className="install-link-desc">Full REST API documentation</div>
          </a>
          <a href="https://github.com/skillforge/skforge" className="install-link-card" target="_blank" rel="noopener noreferrer">
            <div className="install-link-title">Source on GitHub</div>
            <div className="install-link-desc">Open source CLI, SDKs and more</div>
          </a>
          <a href="/explore" className="install-link-card">
            <div className="install-link-title">Browse the registry</div>
            <div className="install-link-desc">Discover published skills and agents</div>
          </a>
        </div>
      </div>
    </div>
  )
}
