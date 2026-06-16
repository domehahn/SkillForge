import { useState, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useDocumentTitle } from '../hooks/useDocumentTitle'
import { useToast } from '../contexts/ToastContext'
import { useConfirm } from '../contexts/ConfirmContext'

const FAKE_SESSIONS = [
  { id: 1, device: 'Chrome on macOS', location: 'Current session', ip: '127.0.0.1', last_seen: new Date().toISOString(), current: true },
]

export default function AccountSecurityPage() {
  useDocumentTitle('Security — Account')
  const navigate = useNavigate()
  const toast = useToast()
  const confirm = useConfirm()

  useEffect(() => {
    if (!localStorage.getItem('sf_token')) navigate('/login')
  }, [])

  const [twoFAEnabled, setTwoFAEnabled] = useState(() => localStorage.getItem('sf_2fa') === 'true')
  const [show2FASetup, setShow2FASetup] = useState(false)
  const [twoFACode, setTwoFACode] = useState('')
  const [sessions] = useState(FAKE_SESSIONS)

  const enable2FA = () => {
    if (twoFACode.length < 6) { toast('Enter the 6-digit code', 'error'); return }
    localStorage.setItem('sf_2fa', 'true')
    setTwoFAEnabled(true)
    setShow2FASetup(false)
    setTwoFACode('')
    toast('Two-factor authentication enabled', 'success')
  }

  const disable2FA = async () => {
    const ok = await confirm('Disable two-factor authentication? Your account will be less secure.', { confirmLabel: 'Disable 2FA', danger: true })
    if (!ok) return
    localStorage.removeItem('sf_2fa')
    setTwoFAEnabled(false)
    toast('Two-factor authentication disabled', 'success')
  }

  const revokeSession = async (session) => {
    if (session.current) { toast('Cannot revoke current session', 'error'); return }
    toast('Session revoked', 'success')
  }

  return (
    <div className="account-security-page">
      <div className="account-security-header">
        <h1 className="account-security-title">Account security</h1>
        <p className="account-security-sub">
          <Link to="/account/settings">← Account settings</Link>
        </p>
      </div>

      {/* Password */}
      <div className="security-section">
        <div className="security-section-title">Password</div>
        <div className="security-row">
          <div className="security-row-info">
            <div className="security-row-label">Registry password</div>
            <div className="security-row-desc">Use a strong, unique password for your account.</div>
          </div>
          <Link to="/account/settings" className="btn-secondary">Change password</Link>
        </div>
      </div>

      {/* 2FA */}
      <div className="security-section">
        <div className="security-section-title">Two-factor authentication (2FA)</div>
        <div className="security-row">
          <div className="security-row-info">
            <div className="security-row-label">
              Authenticator app
              {twoFAEnabled && <span className="security-badge-on">Enabled</span>}
            </div>
            <div className="security-row-desc">
              Use an authenticator app like Google Authenticator, Authy, or 1Password to generate time-based codes.
            </div>
          </div>
          {twoFAEnabled ? (
            <button className="btn-danger" onClick={disable2FA}>Disable</button>
          ) : (
            <button className="btn-primary" onClick={() => setShow2FASetup(true)}>Enable</button>
          )}
        </div>

        {show2FASetup && !twoFAEnabled && (
          <div className="twofa-setup">
            <div className="twofa-setup-step">
              <div className="twofa-step-num">1</div>
              <div>
                <strong>Scan this QR code</strong> with your authenticator app.
                <div className="twofa-qr-placeholder">
                  <div className="twofa-qr-inner">QR</div>
                  <div className="twofa-qr-note">
                    Or enter this key manually:<br />
                    <code className="twofa-secret">JBSWY3DPEHPK3PXP</code>
                  </div>
                </div>
              </div>
            </div>
            <div className="twofa-setup-step">
              <div className="twofa-step-num">2</div>
              <div style={{ flex: 1 }}>
                <strong>Enter the 6-digit code</strong> from your app to verify setup.
                <div className="twofa-code-row">
                  <input
                    className="twofa-code-input"
                    value={twoFACode}
                    onChange={e => setTwoFACode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                    placeholder="000000"
                    maxLength={6}
                  />
                  <button className="btn-primary" onClick={enable2FA} disabled={twoFACode.length < 6}>
                    Verify & enable
                  </button>
                  <button className="btn-secondary" onClick={() => setShow2FASetup(false)}>Cancel</button>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Active sessions */}
      <div className="security-section">
        <div className="security-section-title">Active sessions</div>
        <div className="sessions-list">
          {sessions.map(s => (
            <div key={s.id} className="session-row">
              <div className="session-icon">{s.current ? '💻' : '📱'}</div>
              <div className="session-info">
                <div className="session-device">
                  {s.device}
                  {s.current && <span className="session-current-badge">Current</span>}
                </div>
                <div className="session-meta">{s.ip} · {s.location}</div>
              </div>
              {!s.current && (
                <button className="btn-danger" onClick={() => revokeSession(s)}>Revoke</button>
              )}
            </div>
          ))}
        </div>
      </div>

      {/* Danger zone */}
      <div className="security-section security-danger-zone">
        <div className="security-section-title">Danger zone</div>
        <div className="security-row">
          <div className="security-row-info">
            <div className="security-row-label">Delete account</div>
            <div className="security-row-desc">Permanently delete your account and all published artifacts. This cannot be undone.</div>
          </div>
          <button
            className="btn-danger"
            onClick={async () => {
              const ok = await confirm(
                'Delete your account? All your artifacts, tokens, and data will be permanently removed.',
                { confirmLabel: 'Delete my account', danger: true }
              )
              if (ok) toast('Account deletion is not available in this environment.', 'error')
            }}
          >
            Delete account
          </button>
        </div>
      </div>
    </div>
  )
}
