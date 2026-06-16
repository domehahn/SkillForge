import { useState } from 'react'
import { Link, useNavigate, useLocation } from 'react-router-dom'
import { login } from '../api/client'
import { useDocumentTitle } from '../hooks/useDocumentTitle'

export default function LoginPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const from = location.state?.from || '/'

  useDocumentTitle('Sign in')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState(null)
  const [loading, setLoading] = useState(false)
  const [showPassword, setShowPassword] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      const data = await login(username.trim(), password)
      localStorage.setItem('sf_token', data.token)
      localStorage.setItem('sf_user', data.user)
      localStorage.setItem('sf_role', data.role)
      window.dispatchEvent(new Event('storage'))
      navigate(from === '/login' ? `/namespace/${data.user}` : from, { replace: true })
    } catch (err) {
      setError(err.message === 'HTTP 401' ? 'Invalid username or password.' : err.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-page">
      <div className="login-split">
        {/* Left branding panel */}
        <div className="login-brand">
          <div className="login-brand-inner">
            <div className="login-brand-logo">
              <span className="login-brand-icon">⚡</span>
              <span className="login-brand-name">SkillForge</span>
            </div>
            <h2 className="login-brand-headline">
              The registry for AI skills, agents, and tools
            </h2>
            <p className="login-brand-sub">
              Discover, publish, and deploy AI capabilities across your organization.
            </p>
            <div className="login-brand-features">
              <div className="login-brand-feature">
                <span className="login-feature-icon">⬟</span>
                <span>Skills, agents, flows, prompts, tools</span>
              </div>
              <div className="login-brand-feature">
                <span className="login-feature-icon">🔏</span>
                <span>Signed and attested artifacts</span>
              </div>
              <div className="login-brand-feature">
                <span className="login-feature-icon">⬇</span>
                <span>One-command installs via skforge CLI</span>
              </div>
            </div>
          </div>
        </div>

        {/* Right form panel */}
        <div className="login-form-panel">
          <div className="login-form-inner">
            <div className="login-form-header">
              <h1 className="login-title">Sign in</h1>
              <p className="login-sub">
                New to SkillForge?{' '}
                <span className="login-register-note">
                  Ask your admin to create an account, or use the CLI:<br />
                  <code>skforge account create</code>
                </span>
              </p>
            </div>

            <form onSubmit={handleSubmit} className="login-form" autoComplete="on">
              <div className="field-group">
                <label htmlFor="username">Username</label>
                <input
                  id="username"
                  type="text"
                  value={username}
                  onChange={e => setUsername(e.target.value)}
                  autoComplete="username"
                  required
                  disabled={loading}
                  autoFocus
                  placeholder="your-username"
                />
              </div>
              <div className="field-group">
                <label htmlFor="password">Password</label>
                <div className="password-input-wrap">
                  <input
                    id="password"
                    type={showPassword ? 'text' : 'password'}
                    value={password}
                    onChange={e => setPassword(e.target.value)}
                    autoComplete="current-password"
                    required
                    disabled={loading}
                    placeholder="••••••••"
                  />
                  <button
                    type="button"
                    className="password-toggle"
                    onClick={() => setShowPassword(v => !v)}
                    tabIndex={-1}
                  >
                    {showPassword ? '🙈' : '👁'}
                  </button>
                </div>
              </div>

              {error && (
                <div className="login-error">
                  <span className="login-error-icon">⚠</span>
                  {error}
                </div>
              )}

              <button
                type="submit"
                className="btn-primary login-submit"
                disabled={loading || !username.trim() || !password}
              >
                {loading ? (
                  <span className="login-loading">
                    <span className="login-spinner" />
                    Signing in…
                  </span>
                ) : 'Sign in'}
              </button>
            </form>

            <div className="login-divider">
              <span>or</span>
            </div>

            <div className="login-token-section">
              <p className="login-token-title">Have an access token?</p>
              <p className="login-token-sub">
                Use the CLI to authenticate with a token:<br />
                <code className="login-token-cmd">skforge login --token &lt;TOKEN&gt;</code>
              </p>
            </div>

            <div className="login-footer">
              <Link to="/" className="login-back">← Back to registry</Link>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
