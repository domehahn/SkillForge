import { Link, useNavigate } from 'react-router-dom'
import { useState, useEffect, useRef } from 'react'
import { avatarColor, kindClass } from '../utils'
import { useTheme } from '../contexts/ThemeContext'
import { fetchArtifacts, fetchNotifications, markNotificationsRead, deleteNotification, listMembers } from '../api/client'
import { getRecentlyViewed } from '../hooks/useRecentlyViewed'

function useDebounce(value, delay) {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(t)
  }, [value, delay])
  return debounced
}

export default function Header() {
  const [q, setQ] = useState('')
  const [user, setUser] = useState(() => localStorage.getItem('sf_user'))
  const [recentItems, setRecentItems] = useState(() => getRecentlyViewed())
  const [dropdownOpen, setDropdownOpen] = useState(false)
  const [mobileOpen, setMobileOpen] = useState(false)
  const [notifOpen, setNotifOpen] = useState(false)
  const [memberNamespaces, setMemberNamespaces] = useState([])
  const [notifications, setNotifications] = useState([])
  const [unreadCount, setUnreadCount] = useState(0)
  const notifRef = useRef(null)
  const [suggestions, setSuggestions] = useState([])
  const [showSuggestions, setShowSuggestions] = useState(false)
  const [selectedIdx, setSelectedIdx] = useState(-1)
  const navigate = useNavigate()
  const dropdownRef = useRef(null)
  const searchInputRef = useRef(null)
  const suggestionsRef = useRef(null)
  const { theme, toggleTheme } = useTheme()

  const debouncedQ = useDebounce(q, 200)

  useEffect(() => {
    const sync = () => setUser(localStorage.getItem('sf_user'))
    window.addEventListener('storage', sync)
    return () => window.removeEventListener('storage', sync)
  }, [])

  useEffect(() => {
    const syncRecent = () => setRecentItems(getRecentlyViewed())
    window.addEventListener('sf_recent', syncRecent)
    return () => window.removeEventListener('sf_recent', syncRecent)
  }, [])

  // Fetch notifications and member namespaces when user logs in
  useEffect(() => {
    if (!user) return
    fetchNotifications()
      .then(d => { setNotifications(d.notifications || []); setUnreadCount(d.unread || 0) })
      .catch(() => {})
    listMembers(user)
      .then(d => {
        const otherNS = (d.members || [])
          .map(m => m.namespace)
          .filter(ns => ns !== user)
        setMemberNamespaces(otherNS)
      })
      .catch(() => {})
  }, [user])

  // Close dropdowns on outside click
  useEffect(() => {
    const handleClick = (e) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target)) {
        setDropdownOpen(false)
      }
      if (notifRef.current && !notifRef.current.contains(e.target)) {
        setNotifOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  const openNotifications = () => {
    setNotifOpen(v => !v)
    if (!notifOpen && unreadCount > 0) {
      markNotificationsRead()
        .then(() => {
          setUnreadCount(0)
          setNotifications(ns => ns.map(n => ({ ...n, read: true })))
        })
        .catch(() => {})
    }
  }

  const dismissNotification = async (id) => {
    try {
      await deleteNotification(id)
      setNotifications(ns => ns.filter(n => n.id !== id))
    } catch {}
  }

  // "/" shortcut focuses search
  useEffect(() => {
    const handler = (e) => {
      if (e.key === '/' && e.target.tagName !== 'INPUT' && e.target.tagName !== 'TEXTAREA') {
        e.preventDefault()
        searchInputRef.current?.focus()
      }
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [])

  // Autocomplete fetch
  useEffect(() => {
    if (!debouncedQ.trim() || debouncedQ.length < 2) {
      setSuggestions([])
      return
    }
    fetchArtifacts({ q: debouncedQ.trim(), limit: 6, offset: 0 })
      .then(d => setSuggestions(d.artifacts || []))
      .catch(() => setSuggestions([]))
  }, [debouncedQ])

  const handleSearch = (e) => {
    e.preventDefault()
    setShowSuggestions(false)
    setSuggestions([])
    setMobileOpen(false)
    let term = q.trim()
    const params = {}
    term = term.replace(/\bkind:(\S+)/gi, (_, k) => { params.kind = k; return '' })
    term = term.replace(/\b(?:ns|namespace):(\S+)/gi, (_, k) => { params.namespace = k; return '' })
    term = term.trim()
    if (term) params.q = term
    const qs = new URLSearchParams(params).toString()
    navigate(qs ? `/?${qs}` : '/')
  }

  const pickSuggestion = (a) => {
    setQ('')
    setSuggestions([])
    setShowSuggestions(false)
    navigate(`/artifacts/${a.kind}/${a.namespace}/${a.name}`)
  }

  const handleKeyDown = (e) => {
    if (!showSuggestions || suggestions.length === 0) return
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setSelectedIdx(i => Math.min(i + 1, suggestions.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setSelectedIdx(i => Math.max(i - 1, -1))
    } else if (e.key === 'Enter' && selectedIdx >= 0) {
      e.preventDefault()
      pickSuggestion(suggestions[selectedIdx])
    } else if (e.key === 'Escape') {
      setShowSuggestions(false)
      setSelectedIdx(-1)
    }
  }

  const handleSignOut = () => {
    localStorage.removeItem('sf_token')
    localStorage.removeItem('sf_user')
    localStorage.removeItem('sf_role')
    setUser(null)
    setDropdownOpen(false)
    setMobileOpen(false)
    navigate('/')
    window.dispatchEvent(new Event('storage'))
  }

  const color = user ? avatarColor(user) : '#6366f1'

  return (
    <header>
      <div className="header-inner">
        <Link to="/" className="header-logo">
          <div className="header-logo-icon">⚡</div>
          SkillForge
        </Link>

        <div className="header-search-wrap">
          <form onSubmit={handleSearch} style={{ position: 'relative' }} autoComplete="off">
            <div style={{ position: 'relative' }}>
              <input
                ref={searchInputRef}
                value={q}
                onChange={e => {
                  setQ(e.target.value)
                  setSelectedIdx(-1)
                  setShowSuggestions(true)
                }}
                onFocus={() => setShowSuggestions(true)}
                onBlur={() => setTimeout(() => setShowSuggestions(false), 150)}
                onKeyDown={handleKeyDown}
                placeholder="Search skills, agents, tools…"
              />
              {!q && <span className="header-search-hint">/</span>}
            </div>
            <button type="submit">Search</button>

            {/* Autocomplete dropdown */}
            {showSuggestions && suggestions.length > 0 && (
              <div className="search-suggestions" ref={suggestionsRef}>
                {suggestions.map((a, i) => (
                  <button
                    key={`${a.kind}/${a.namespace}/${a.name}`}
                    type="button"
                    className={`search-suggestion-item ${i === selectedIdx ? 'selected' : ''}`}
                    onMouseDown={() => pickSuggestion(a)}
                  >
                    <div className={`suggestion-avatar ${kindClass(a.kind)}`} style={{ background: avatarColor(a.namespace) }}>
                      {a.namespace[0].toUpperCase()}
                    </div>
                    <div className="suggestion-body">
                      <span className="suggestion-name">{a.name}</span>
                      <span className="suggestion-ns">{a.namespace}</span>
                    </div>
                    <span className="kind-chip kind-chip-sm">{a.kind}</span>
                  </button>
                ))}
                <div className="suggestion-footer">
                  Press Enter to see all results
                </div>
              </div>
            )}
          </form>
        </div>

        {/* Desktop nav */}
        <nav className="header-nav">
          <Link to="/explore">Explore</Link>
          <Link to="/trending">Trending</Link>
          <Link to="/categories">Categories</Link>
          <Link to="/search">Search</Link>
          <Link to="/activity">Activity</Link>
          <Link to="/publish">Publish</Link>
          <a href="/api-docs" target="_blank" rel="noopener noreferrer">API</a>

          {user && (
            <div className="notif-bell-wrap" ref={notifRef}>
              <button
                className="notif-bell-btn"
                onClick={openNotifications}
                title="Notifications"
                aria-label="Notifications"
              >
                🔔
                {unreadCount > 0 && (
                  <span className="notif-badge">{unreadCount > 9 ? '9+' : unreadCount}</span>
                )}
              </button>
              {notifOpen && (
                <div className="notif-dropdown">
                  <div className="notif-dropdown-header">
                    <span className="notif-dropdown-title">Notifications</span>
                  </div>
                  {notifications.length === 0 ? (
                    <div className="notif-empty">No notifications yet</div>
                  ) : (
                    <div className="notif-list">
                      {notifications.map(n => (
                        <div key={n.id} className={`notif-item ${n.read ? '' : 'notif-unread'}`}>
                          <Link
                            to={n.link || '/'}
                            className="notif-item-msg"
                            onClick={() => setNotifOpen(false)}
                          >
                            {n.message}
                          </Link>
                          <button
                            className="notif-dismiss"
                            onClick={() => dismissNotification(n.id)}
                            title="Dismiss"
                          >✕</button>
                        </div>
                      ))}
                    </div>
                  )}
                  <div className="notif-dropdown-footer">
                    <Link to="/account/notifications" className="notif-prefs-link" onClick={() => setNotifOpen(false)}>
                      Notification preferences
                    </Link>
                  </div>
                </div>
              )}
            </div>
          )}

          <button
            className="theme-toggle-btn"
            onClick={toggleTheme}
            title={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
            aria-label="Toggle color theme"
          >
            {theme === 'dark' ? '☀' : '☾'}
          </button>

          {user ? (
            <div className="header-user-menu" ref={dropdownRef}>
              <button
                className="header-avatar-btn"
                onClick={() => setDropdownOpen(v => !v)}
                aria-expanded={dropdownOpen}
                style={{ background: color }}
              >
                {user[0].toUpperCase()}
              </button>
              {dropdownOpen && (
                <div className="header-dropdown">
                  <div className="header-dropdown-user">
                    <div className="header-dropdown-avatar" style={{ background: color }}>
                      {user[0].toUpperCase()}
                    </div>
                    <span className="header-dropdown-name">{user}</span>
                  </div>
                  {recentItems.length > 0 && (
                    <>
                      <div className="header-dropdown-divider" />
                      <div className="header-dropdown-section-label">Recently viewed</div>
                      {recentItems.slice(0, 4).map(r => (
                        <Link
                          key={`${r.kind}/${r.namespace}/${r.name}`}
                          to={`/artifacts/${r.kind}/${r.namespace}/${r.name}`}
                          className="header-dropdown-item header-dropdown-recent"
                          onClick={() => setDropdownOpen(false)}
                        >
                          <div className="recent-dot" style={{ background: avatarColor(r.namespace) }} />
                          <div className="recent-info">
                            <span className="recent-name">{r.name}</span>
                            <span className="recent-ns">{r.namespace}</span>
                          </div>
                        </Link>
                      ))}
                    </>
                  )}
                  <div className="header-dropdown-divider" />
                  <Link to={`/u/${user}`} className="header-dropdown-item" onClick={() => setDropdownOpen(false)}>
                    Public profile
                  </Link>
                  <Link to={`/namespace/${user}`} className="header-dropdown-item" onClick={() => setDropdownOpen(false)}>
                    Your namespace
                  </Link>
                  {memberNamespaces.length > 0 && (
                    <>
                      <div className="header-dropdown-section-label">Switch namespace</div>
                      {memberNamespaces.map(ns => (
                        <Link
                          key={ns}
                          to={`/namespace/${ns}`}
                          className="header-dropdown-item header-dropdown-recent"
                          onClick={() => setDropdownOpen(false)}
                        >
                          <div className="recent-dot" style={{ background: avatarColor(ns) }} />
                          <div className="recent-info">
                            <span className="recent-name">{ns}</span>
                            <span className="recent-ns">collaborator</span>
                          </div>
                        </Link>
                      ))}
                    </>
                  )}
                  <Link to="/account/tokens" className="header-dropdown-item" onClick={() => setDropdownOpen(false)}>
                    Access Tokens
                  </Link>
                  <Link to="/account/settings" className="header-dropdown-item" onClick={() => setDropdownOpen(false)}>
                    Settings
                  </Link>
                  <Link to="/account/notifications" className="header-dropdown-item" onClick={() => setDropdownOpen(false)}>
                    Notification preferences
                  </Link>
                  <Link to="/account/security" className="header-dropdown-item" onClick={() => setDropdownOpen(false)}>
                    Security
                  </Link>
                  {localStorage.getItem('sf_role') === 'admin' && (
                    <>
                    <Link to="/admin" className="header-dropdown-item" onClick={() => setDropdownOpen(false)}>
                      Admin Panel
                    </Link>
                    <Link to="/admin/audit" className="header-dropdown-item" onClick={() => setDropdownOpen(false)}>
                      Audit Log
                    </Link>
                    </>
                  )}
                  <div className="header-dropdown-divider" />
                  <button className="header-dropdown-item header-dropdown-signout" onClick={handleSignOut}>
                    Sign out
                  </button>
                </div>
              )}
            </div>
          ) : (
            <Link to="/login" className="btn-signin">Sign in</Link>
          )}
        </nav>

        {/* Mobile hamburger */}
        <button
          className="hamburger-btn"
          onClick={() => setMobileOpen(v => !v)}
          aria-label="Toggle menu"
        >
          <span className={`hamburger-icon ${mobileOpen ? 'open' : ''}`}>
            <span /><span /><span />
          </span>
        </button>
      </div>

      {/* Mobile menu */}
      {mobileOpen && (
        <div className="mobile-menu">
          <form onSubmit={handleSearch} className="mobile-search">
            <input value={q} onChange={e => setQ(e.target.value)} placeholder="Search…" />
            <button type="submit">Go</button>
          </form>
          <Link to="/explore" className="mobile-nav-item" onClick={() => setMobileOpen(false)}>Explore</Link>
          <Link to="/search" className="mobile-nav-item" onClick={() => setMobileOpen(false)}>Search</Link>
          <Link to="/publish" className="mobile-nav-item" onClick={() => setMobileOpen(false)}>Publish</Link>
          <a href="/api-docs" className="mobile-nav-item" onClick={() => setMobileOpen(false)}>API Docs</a>
          {user ? (
            <>
              <Link to={`/namespace/${user}`} className="mobile-nav-item" onClick={() => setMobileOpen(false)}>Your profile</Link>
              <Link to="/account/tokens" className="mobile-nav-item" onClick={() => setMobileOpen(false)}>Access Tokens</Link>
              <Link to="/account/settings" className="mobile-nav-item" onClick={() => setMobileOpen(false)}>Settings</Link>
              <button className="mobile-nav-item mobile-signout" onClick={handleSignOut}>Sign out</button>
            </>
          ) : (
            <Link to="/login" className="mobile-nav-item mobile-signin" onClick={() => setMobileOpen(false)}>Sign in</Link>
          )}
        </div>
      )}
    </header>
  )
}
