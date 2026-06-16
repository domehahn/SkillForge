import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { listTokens, changePassword, updateProfile, fetchNamespace } from '../api/client'
import { useDocumentTitle } from '../hooks/useDocumentTitle'
import { useToast } from '../contexts/ToastContext'
import { useConfirm } from '../contexts/ConfirmContext'
import { avatarColor, timeAgo } from '../utils'

export default function AccountSettingsPage() {
  const navigate = useNavigate()
  const toast = useToast()
  const confirm = useConfirm()
  useDocumentTitle('Account Settings')

  const user = localStorage.getItem('sf_user')
  const role = localStorage.getItem('sf_role') || 'user'
  const color = user ? avatarColor(user) : '#6366f1'

  const [tokenCount, setTokenCount] = useState(null)
  const [displayName, setDisplayName] = useState('')
  const [displayNameInput, setDisplayNameInput] = useState('')
  const [displayNameEditing, setDisplayNameEditing] = useState(false)
  const [displayNameSaving, setDisplayNameSaving] = useState(false)
  const [pwForm, setPwForm] = useState(null)
  const [pwSaving, setPwSaving] = useState(false)
  const [pwError, setPwError] = useState('')
  const [joinedAt] = useState(() => {
    const raw = localStorage.getItem('sf_joined_at')
    return raw || null
  })

  useEffect(() => {
    if (!localStorage.getItem('sf_token')) {
      navigate('/login', { state: { from: '/account/settings' } })
      return
    }
    listTokens()
      .then(d => setTokenCount((d.tokens || []).length))
      .catch(() => {})
    fetchNamespace(user)
      .then(d => { setDisplayName(d.display_name || ''); setDisplayNameInput(d.display_name || '') })
      .catch(() => {})
  }, [])

  const handleSignOut = async () => {
    const ok = await confirm('Sign out of SkillForge?', { confirmLabel: 'Sign out' })
    if (!ok) return
    localStorage.removeItem('sf_token')
    localStorage.removeItem('sf_user')
    localStorage.removeItem('sf_role')
    localStorage.removeItem('sf_joined_at')
    toast('Signed out', 'success')
    navigate('/')
    window.dispatchEvent(new Event('storage'))
  }

  if (!user) return null

  return (
    <div className="settings-page">
      <div className="settings-header">
        <div className="settings-avatar" style={{ background: color }}>
          {user[0].toUpperCase()}
        </div>
        <div>
          <h1 className="settings-username">{user}</h1>
          <p className="settings-sub">Account settings</p>
        </div>
      </div>

      <div className="settings-layout">
        {/* Left nav */}
        <nav className="settings-nav">
          <a href="#profile" className="settings-nav-item active">Profile</a>
          <a href="#security" className="settings-nav-item">Security</a>
          <a href="#cli" className="settings-nav-item">CLI Setup</a>
          <a href="#quotas" className="settings-nav-item">Pull Quotas</a>
          <a href="#danger" className="settings-nav-item danger">Danger zone</a>
        </nav>

        {/* Main */}
        <div className="settings-main">
          {/* Profile section */}
          <section className="settings-section" id="profile">
            <h2 className="settings-section-title">Profile</h2>
            <div className="settings-card">
              <div className="settings-field">
                <div className="settings-field-label">Display name</div>
                <div className="settings-field-value settings-field-action">
                  {displayNameEditing ? (
                    <div className="display-name-form">
                      <input
                        type="text"
                        value={displayNameInput}
                        onChange={e => setDisplayNameInput(e.target.value)}
                        placeholder="Your display name"
                        maxLength={100}
                        autoFocus
                      />
                      <div className="change-password-actions">
                        <button
                          className="btn-primary"
                          disabled={displayNameSaving}
                          onClick={async () => {
                            setDisplayNameSaving(true)
                            try {
                              await updateProfile({ display_name: displayNameInput })
                              setDisplayName(displayNameInput)
                              setDisplayNameEditing(false)
                              toast('Display name updated', 'success')
                            } catch (err) {
                              toast(err.message, 'error')
                            } finally {
                              setDisplayNameSaving(false)
                            }
                          }}
                        >
                          {displayNameSaving ? 'Saving…' : 'Save'}
                        </button>
                        <button className="btn-secondary" onClick={() => { setDisplayNameInput(displayName); setDisplayNameEditing(false) }}>
                          Cancel
                        </button>
                      </div>
                    </div>
                  ) : (
                    <>
                      <div>
                        <span className="settings-field-text">{displayName || <span style={{ color: 'var(--text-muted)' }}>Not set</span>}</span>
                        <span className="settings-field-note">Shown alongside your username on your profile.</span>
                      </div>
                      <button className="btn-secondary" onClick={() => setDisplayNameEditing(true)}>
                        {displayName ? 'Edit' : 'Add name'}
                      </button>
                    </>
                  )}
                </div>
              </div>
              <div className="settings-field">
                <div className="settings-field-label">Username</div>
                <div className="settings-field-value">
                  <span className="settings-field-text">{user}</span>
                  <span className="settings-field-note">Username cannot be changed.</span>
                </div>
              </div>
              <div className="settings-field">
                <div className="settings-field-label">Role</div>
                <div className="settings-field-value">
                  <span className={`role-badge role-${role}`}>{role}</span>
                </div>
              </div>
              <div className="settings-field">
                <div className="settings-field-label">Profile URL</div>
                <div className="settings-field-value">
                  <Link to={`/namespace/${user}`} className="settings-link">
                    /namespace/{user}
                  </Link>
                </div>
              </div>
              {joinedAt && (
                <div className="settings-field">
                  <div className="settings-field-label">Member since</div>
                  <div className="settings-field-value">
                    <span className="settings-field-text">{timeAgo(joinedAt)}</span>
                  </div>
                </div>
              )}
            </div>
          </section>

          {/* Security section */}
          <section className="settings-section" id="security">
            <h2 className="settings-section-title">Security</h2>
            <div className="settings-card">
              <div className="settings-field">
                <div className="settings-field-label">Access tokens</div>
                <div className="settings-field-value settings-field-action">
                  <div>
                    <span className="settings-field-text">
                      {tokenCount === null ? '…' : `${tokenCount} active token${tokenCount !== 1 ? 's' : ''}`}
                    </span>
                    <span className="settings-field-note">
                      Tokens let you authenticate the CLI and CI/CD pipelines.
                    </span>
                  </div>
                  <Link to="/account/tokens" className="btn-secondary">
                    Manage tokens
                  </Link>
                </div>
              </div>
              <div className="settings-field">
                <div className="settings-field-label">Password</div>
                <div className="settings-field-value">
                  {pwForm ? (
                    <form
                      className="change-password-form"
                      onSubmit={async (e) => {
                        e.preventDefault()
                        if (pwForm.newPassword !== pwForm.confirmPassword) {
                          setPwError('New passwords do not match')
                          return
                        }
                        setPwError('')
                        setPwSaving(true)
                        try {
                          await changePassword(pwForm.currentPassword, pwForm.newPassword)
                          toast('Password changed successfully', 'success')
                          setPwForm(null)
                        } catch (err) {
                          setPwError(err.message)
                        } finally {
                          setPwSaving(false)
                        }
                      }}
                    >
                      <input
                        type="password"
                        placeholder="Current password"
                        value={pwForm.currentPassword}
                        onChange={e => setPwForm(f => ({ ...f, currentPassword: e.target.value }))}
                        required
                        autoFocus
                      />
                      <input
                        type="password"
                        placeholder="New password (min 8 chars)"
                        value={pwForm.newPassword}
                        onChange={e => setPwForm(f => ({ ...f, newPassword: e.target.value }))}
                        required
                        minLength={8}
                      />
                      <input
                        type="password"
                        placeholder="Confirm new password"
                        value={pwForm.confirmPassword}
                        onChange={e => setPwForm(f => ({ ...f, confirmPassword: e.target.value }))}
                        required
                      />
                      {pwError && <div className="auth-error">{pwError}</div>}
                      <div className="change-password-actions">
                        <button type="submit" className="btn-primary" disabled={pwSaving}>
                          {pwSaving ? 'Saving…' : 'Change password'}
                        </button>
                        <button type="button" className="btn-secondary" onClick={() => { setPwForm(null); setPwError('') }}>
                          Cancel
                        </button>
                      </div>
                    </form>
                  ) : (
                    <div className="settings-field-action">
                      <div>
                        <span className="settings-field-text">••••••••••••</span>
                        <span className="settings-field-note">Choose a strong password.</span>
                      </div>
                      <button
                        className="btn-secondary"
                        onClick={() => setPwForm({ currentPassword: '', newPassword: '', confirmPassword: '' })}
                      >
                        Change password
                      </button>
                    </div>
                  )}
                </div>
              </div>
              <div className="settings-field">
                <div className="settings-field-label">Active session</div>
                <div className="settings-field-value settings-field-action">
                  <span className="settings-field-text">Current browser session</span>
                  <button className="btn-secondary" onClick={handleSignOut}>
                    Sign out
                  </button>
                </div>
              </div>
            </div>
          </section>

          {/* CLI config section */}
          <section className="settings-section" id="cli">
            <h2 className="settings-section-title">CLI Configuration</h2>
            <div className="settings-card">
              <div className="settings-field" style={{ flexDirection: 'column', alignItems: 'flex-start', gap: 8 }}>
                <div className="settings-field-label">Registry endpoint</div>
                <div className="settings-field-note">
                  Use this URL when configuring <code>skforge</code> to point at this registry.
                </div>
                <div className="cli-config-block">
                  <pre className="cli-config-pre">{window.location.origin}/api/v1</pre>
                  <button
                    className="cli-config-copy"
                    onClick={() => {
                      navigator.clipboard.writeText(`${window.location.origin}/api/v1`)
                      toast('Copied!', 'success')
                    }}
                  >Copy</button>
                </div>
              </div>
              <div className="settings-field" style={{ flexDirection: 'column', alignItems: 'flex-start', gap: 8 }}>
                <div className="settings-field-label">skforge config YAML</div>
                <div className="settings-field-note">
                  Add this to your <code>~/.skforge/config.yaml</code> to authenticate the CLI.
                </div>
                <div className="cli-config-block">
                  <pre className="cli-config-pre">{`registry: ${window.location.origin}/api/v1\nnamespace: ${user}\n# Run: skforge login --token <your-token>`}</pre>
                  <button
                    className="cli-config-copy"
                    onClick={() => {
                      navigator.clipboard.writeText(`registry: ${window.location.origin}/api/v1\nnamespace: ${user}\n`)
                      toast('Copied!', 'success')
                    }}
                  >Copy</button>
                </div>
              </div>
            </div>
          </section>

          {/* Pull Quotas */}
          <section className="settings-section" id="quotas">
            <h2 className="settings-section-title">Pull quotas</h2>
            <div className="settings-card">
              <div className="quota-info-banner">
                <div className="quota-info-icon">📦</div>
                <div>
                  <div className="quota-info-title">Authenticated pulls: unlimited</div>
                  <div className="quota-info-desc">
                    Signed-in users have unlimited pulls. Anonymous users may be rate-limited during high demand.
                  </div>
                </div>
              </div>
              <div className="quota-tier-grid">
                <div className="quota-tier">
                  <div className="quota-tier-name">Anonymous</div>
                  <div className="quota-tier-limit">100 / 6h</div>
                  <div className="quota-tier-note">Shared IP pool</div>
                </div>
                <div className="quota-tier quota-tier-active">
                  <div className="quota-tier-name">Authenticated ✓</div>
                  <div className="quota-tier-limit">Unlimited</div>
                  <div className="quota-tier-note">Your plan</div>
                </div>
                <div className="quota-tier quota-tier-pro">
                  <div className="quota-tier-name">Verified publisher</div>
                  <div className="quota-tier-limit">Unlimited + CDN</div>
                  <div className="quota-tier-note">Contact us</div>
                </div>
              </div>
            </div>
          </section>

          {/* Danger zone */}
          <section className="settings-section" id="danger">
            <h2 className="settings-section-title settings-section-danger">Danger zone</h2>
            <div className="settings-card settings-card-danger">
              <div className="settings-field">
                <div>
                  <div className="settings-field-label">Delete account</div>
                  <div className="settings-field-note" style={{ marginTop: 4 }}>
                    Permanently delete your account and all published artifacts.
                    This action cannot be undone.
                  </div>
                </div>
                <button
                  className="btn-danger"
                  onClick={async () => {
                    const ok = await confirm(
                      'Are you sure? This will permanently delete your account and all artifacts you have published. This cannot be undone.',
                      { confirmLabel: 'Delete my account', danger: true }
                    )
                    if (ok) toast('Account deletion is not yet available via the web UI. Use the CLI: skforge account delete', 'info')
                  }}
                >
                  Delete account
                </button>
              </div>
            </div>
          </section>
        </div>
      </div>
    </div>
  )
}
