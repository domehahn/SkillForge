import { useState, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useDocumentTitle } from '../hooks/useDocumentTitle'
import { useToast } from '../contexts/ToastContext'

const PREF_KEY = 'sf_notif_prefs'

const GROUPS = [
  {
    label: 'Activity',
    prefs: [
      { key: 'publish', label: 'New publish in your namespace', desc: 'When a co-maintainer publishes a new version in a namespace you belong to.' },
      { key: 'transfer', label: 'Artifact transferred to you', desc: 'When someone transfers an artifact into your namespace.' },
    ],
  },
  {
    label: 'Social',
    prefs: [
      { key: 'star', label: 'Someone stars your artifact', desc: 'Get notified when your published artifacts receive new stars.' },
      { key: 'comment', label: 'New comment on your artifact', desc: 'When someone starts or replies to a discussion on your artifact.' },
    ],
  },
  {
    label: 'Security',
    prefs: [
      { key: 'cve', label: 'New CVE in your artifact', desc: 'When a security scan detects a new vulnerability in one of your published artifacts.' },
      { key: 'cve_critical', label: 'Critical CVE only', desc: 'Only notify on Critical severity findings (requires CVE notifications enabled).' },
    ],
  },
  {
    label: 'Account',
    prefs: [
      { key: 'token_expiry', label: 'Access token expiry warning', desc: 'Remind you 7 days before an access token expires.' },
      { key: 'member_added', label: 'Added to a namespace', desc: 'When you are added as a member to a namespace.' },
    ],
  },
]

const CHANNELS = [
  { key: 'in_app', label: 'In-app', desc: 'Bell icon in the header', always: true },
  { key: 'email', label: 'Email', desc: 'Sent to your registered email address', comingSoon: true },
]

function loadPrefs() {
  try {
    return JSON.parse(localStorage.getItem(PREF_KEY) || '{}')
  } catch {
    return {}
  }
}

export default function NotificationPrefsPage() {
  useDocumentTitle('Notification Preferences')
  const navigate = useNavigate()
  const toast = useToast()

  useEffect(() => {
    if (!localStorage.getItem('sf_token')) navigate('/login')
  }, [])

  const [prefs, setPrefs] = useState(() => {
    const saved = loadPrefs()
    const defaults = {}
    GROUPS.forEach(g => g.prefs.forEach(p => { defaults[p.key] = p.key in saved ? saved[p.key] : true }))
    return defaults
  })
  const [channel, setChannel] = useState(() => loadPrefs().__channel || 'in_app')
  const [saved, setSaved] = useState(false)

  const toggle = (key) => setPrefs(p => ({ ...p, [key]: !p[key] }))

  const save = () => {
    localStorage.setItem(PREF_KEY, JSON.stringify({ ...prefs, __channel: channel }))
    setSaved(true)
    toast('Preferences saved', 'success')
    setTimeout(() => setSaved(false), 2000)
  }

  return (
    <div className="notif-prefs-page">
      <div className="notif-prefs-header">
        <div>
          <h1 className="notif-prefs-title">Notification preferences</h1>
          <p className="notif-prefs-sub">
            Control which events send you notifications.{' '}
            <Link to="/account/settings">Account settings</Link>
          </p>
        </div>
        <button className="btn-primary" onClick={save}>{saved ? '✓ Saved' : 'Save preferences'}</button>
      </div>

      {/* Channel selector */}
      <div className="notif-prefs-section">
        <div className="notif-prefs-section-title">Delivery channel</div>
        <div className="notif-channel-row">
          {CHANNELS.map(ch => (
            <label
              key={ch.key}
              className={`notif-channel-card ${channel === ch.key ? 'selected' : ''} ${ch.comingSoon ? 'disabled' : ''}`}
            >
              <input
                type="radio"
                name="channel"
                value={ch.key}
                checked={channel === ch.key}
                disabled={ch.comingSoon}
                onChange={() => setChannel(ch.key)}
                style={{ display: 'none' }}
              />
              <div className="notif-channel-label">{ch.label}</div>
              <div className="notif-channel-desc">{ch.desc}</div>
              {ch.comingSoon && <span className="notif-coming-soon">Coming soon</span>}
              {ch.always && <span className="notif-always-on">Always on</span>}
            </label>
          ))}
        </div>
      </div>

      {/* Preference groups */}
      {GROUPS.map(g => (
        <div key={g.label} className="notif-prefs-section">
          <div className="notif-prefs-section-title">{g.label}</div>
          <div className="notif-pref-list">
            {g.prefs.map(p => (
              <label key={p.key} className="notif-pref-row">
                <div className="notif-pref-text">
                  <div className="notif-pref-label">{p.label}</div>
                  <div className="notif-pref-desc">{p.desc}</div>
                </div>
                <div className={`toggle-switch ${prefs[p.key] ? 'on' : ''}`} onClick={() => toggle(p.key)}>
                  <div className="toggle-knob" />
                </div>
              </label>
            ))}
          </div>
        </div>
      ))}

      <div className="notif-prefs-footer">
        <button className="btn-primary" onClick={save}>{saved ? '✓ Saved' : 'Save preferences'}</button>
      </div>
    </div>
  )
}
