import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { listTokens, createToken, revokeToken } from '../api/client'
import { timeAgo } from '../utils'
import { useToast } from '../contexts/ToastContext'
import { useConfirm } from '../contexts/ConfirmContext'
import { useDocumentTitle } from '../hooks/useDocumentTitle'

const ALL_SCOPES = [
  { value: 'read', desc: 'Read artifacts and metadata' },
  { value: 'write', desc: 'Publish and update artifacts' },
  { value: 'delete', desc: 'Delete artifacts and versions' },
  { value: 'token:create', desc: 'Create new access tokens' },
  { value: 'token:revoke', desc: 'Revoke access tokens' },
  { value: 'namespace:admin', desc: 'Manage namespace members and settings' },
]

const EXPIRY_OPTIONS = [
  { value: '7', label: '7 days' },
  { value: '30', label: '30 days' },
  { value: '60', label: '60 days' },
  { value: '90', label: '90 days' },
  { value: '180', label: '180 days' },
  { value: '365', label: '1 year' },
  { value: '', label: 'No expiration' },
]

export default function TokensPage() {
  const navigate = useNavigate()
  const user = localStorage.getItem('sf_user')
  const toast = useToast()
  const confirm = useConfirm()
  useDocumentTitle('Access Tokens')

  const [tokens, setTokens] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [newScopes, setNewScopes] = useState(['read'])
  const [newExpiry, setNewExpiry] = useState('90')
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState(null)
  const [createdToken, setCreatedToken] = useState(null)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!localStorage.getItem('sf_token')) {
      navigate('/login', { state: { from: '/account/tokens' } })
      return
    }
    load()
  }, [])

  const load = () => {
    setLoading(true)
    listTokens()
      .then(data => setTokens(data.tokens || []))
      .catch(err => {
        if (err.message.includes('401') || err.message.includes('Unauthorized')) {
          navigate('/login', { state: { from: '/account/tokens' } })
        } else {
          setError(err.message)
        }
      })
      .finally(() => setLoading(false))
  }

  const handleCreate = async (e) => {
    e.preventDefault()
    setCreateError(null)
    setCreating(true)
    try {
      const expirySecs = newExpiry ? parseInt(newExpiry, 10) * 24 * 60 * 60 : null
      const data = await createToken(newName.trim(), newScopes, expirySecs)
      setCreatedToken(data.token)
      setNewName('')
      setNewScopes(['read'])
      setNewExpiry('90')
      setShowCreate(false)
      load()
    } catch (err) {
      setCreateError(err.message)
    } finally {
      setCreating(false)
    }
  }

  const handleRevoke = async (id, tokenName) => {
    const ok = await confirm(
      `Revoke token "${tokenName}"? Any integrations using it will stop working immediately.`,
      { confirmLabel: 'Revoke token', danger: true }
    )
    if (!ok) return
    try {
      await revokeToken(id)
      setTokens(ts => ts.filter(t => t.id !== id))
      toast(`Token "${tokenName}" revoked`, 'success')
    } catch (err) {
      toast(err.message, 'error')
    }
  }

  const copyToken = async () => {
    await navigator.clipboard.writeText(createdToken)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const toggleScope = (s) => {
    setNewScopes(prev =>
      prev.includes(s) ? prev.filter(x => x !== s) : [...prev, s]
    )
  }

  return (
    <div className="tokens-page">
      <div className="tokens-header">
        <div>
          <h1>Access Tokens</h1>
          <p className="tokens-sub">
            Tokens for <strong>{user}</strong> — authenticate with{' '}
            <code>skforge login --token &lt;TOKEN&gt;</code>
          </p>
        </div>
        <button className="btn-primary" onClick={() => { setCreatedToken(null); setShowCreate(true) }}>
          + New token
        </button>
      </div>

      {createdToken && (
        <div className="token-reveal">
          <div className="token-reveal-icon">🔑</div>
          <div className="token-reveal-body">
            <p className="token-reveal-title">Your new access token</p>
            <p className="token-reveal-sub">
              Copy it now — it won't be shown again after you leave this page.
            </p>
            <div className="token-value">
              <code>{createdToken}</code>
              <button className="copy-btn" onClick={copyToken}>
                {copied ? '✓ Copied' : 'Copy'}
              </button>
            </div>
          </div>
        </div>
      )}

      {showCreate && (
        <div className="create-token-form">
          <h2>New access token</h2>
          <form onSubmit={handleCreate}>
            <div className="field-group">
              <label>Token name <span className="field-required">*</span></label>
              <input
                type="text"
                value={newName}
                onChange={e => setNewName(e.target.value)}
                placeholder="e.g. ci-pipeline, deploy-bot"
                required
                autoFocus
              />
              <div className="field-hint">A descriptive name so you can identify this token later.</div>
            </div>

            <div className="field-group">
              <label>Expiration</label>
              <div className="expiry-options">
                {EXPIRY_OPTIONS.map(opt => (
                  <button
                    key={opt.value}
                    type="button"
                    className={`expiry-btn ${newExpiry === opt.value ? 'active' : ''}`}
                    onClick={() => setNewExpiry(opt.value)}
                  >
                    {opt.label}
                  </button>
                ))}
              </div>
            </div>

            <div className="field-group">
              <label>Permissions</label>
              <div className="scope-grid">
                {ALL_SCOPES.map(s => (
                  <label key={s.value} className={`scope-checkbox ${newScopes.includes(s.value) ? 'checked' : ''}`}>
                    <input
                      type="checkbox"
                      checked={newScopes.includes(s.value)}
                      onChange={() => toggleScope(s.value)}
                    />
                    <div className="scope-checkbox-body">
                      <span className="scope-name">{s.value}</span>
                      <span className="scope-desc">{s.desc}</span>
                    </div>
                  </label>
                ))}
              </div>
            </div>

            {createError && <div className="auth-error">{createError}</div>}
            <div className="form-actions">
              <button type="submit" className="btn-primary" disabled={creating || newScopes.length === 0}>
                {creating ? 'Generating…' : 'Generate token'}
              </button>
              <button type="button" className="btn-secondary" onClick={() => setShowCreate(false)}>
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      {error && <div className="error-state">{error}</div>}

      {loading ? (
        <div className="loading-state">Loading tokens…</div>
      ) : tokens.length === 0 && !createdToken ? (
        <div className="empty-state tokens-empty">
          <h3>No access tokens</h3>
          <p>Create a token to authenticate the CLI or automate publishing in CI/CD pipelines.</p>
          <button className="btn-primary" style={{ marginTop: 16 }} onClick={() => setShowCreate(true)}>
            Create your first token
          </button>
        </div>
      ) : tokens.length > 0 ? (
        <table className="tokens-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Permissions</th>
              <th>Created</th>
              <th>Expires</th>
              <th>Last used</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {tokens.map(t => {
              const isExpired = t.expires_at && new Date(t.expires_at) < new Date()
              return (
                <tr key={t.id} className={isExpired ? 'token-row-expired' : ''}>
                  <td className="token-name">
                    <div className="token-name-cell">
                      <span className={`token-status-dot ${isExpired ? 'dot-expired' : 'dot-active'}`} />
                      {t.name}
                      {isExpired && <span className="badge badge-unsigned" style={{ marginLeft: 6 }}>expired</span>}
                    </div>
                  </td>
                  <td>
                    <div className="scope-tags">
                      {(t.scopes || []).map(s => (
                        <span key={s} className="scope-tag">{s}</span>
                      ))}
                    </div>
                  </td>
                  <td className="token-date">{timeAgo(t.created_at)}</td>
                  <td className="token-date">
                    {t.expires_at
                      ? <span className={isExpired ? 'token-expired-text' : ''}>{timeAgo(t.expires_at)}</span>
                      : <span className="text-muted">Never</span>}
                  </td>
                  <td className="token-date">
                    {t.last_used_at
                      ? timeAgo(t.last_used_at)
                      : <span className="text-muted">Never used</span>}
                  </td>
                  <td>
                    <button
                      className="revoke-btn"
                      onClick={() => handleRevoke(t.id, t.name)}
                    >
                      Revoke
                    </button>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      ) : null}
    </div>
  )
}
