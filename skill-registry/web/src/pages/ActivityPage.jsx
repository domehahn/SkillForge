import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { fetchAuditLog, fetchFollowing, fetchArtifacts } from '../api/client'
import { useDocumentTitle } from '../hooks/useDocumentTitle'
import { timeAgo, avatarColor, kindClass } from '../utils'

const PAGE_SIZE = 30

const ACTION_ICON = {
  publish: '⬆',
  delete: '🗑',
  yank: '⚠',
  unyank: '✓',
  deprecate: '⚠',
  login: '🔑',
  token_create: '🔑',
  token_revoke: '🔑',
  user_create: '👤',
  user_delete: '👤',
}

const ACTION_COLOR = {
  publish: 'act-publish',
  delete: 'act-delete',
  yank: 'act-yank',
  unyank: 'act-publish',
  deprecate: 'act-yank',
}

export default function ActivityPage() {
  useDocumentTitle('Registry Activity')
  const [tab, setTab] = useState('global')
  const [entries, setEntries] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [actionFilter, setActionFilter] = useState('publish')
  const [followingFeed, setFollowingFeed] = useState([])
  const [followingLoading, setFollowingLoading] = useState(false)
  const loggedInUser = localStorage.getItem('sf_user')

  useEffect(() => {
    if (!loggedInUser) {
      setEntries([])
      setTotal(0)
      setError(null)
      setLoading(false)
      return
    }
    setLoading(true)
    fetchAuditLog({ limit: PAGE_SIZE, offset: (page - 1) * PAGE_SIZE, action: actionFilter || undefined })
      .then(d => {
        setEntries(d.entries || d.audit_log || d || [])
        setTotal(d.total || (d.entries || d.audit_log || d || []).length)
      })
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }, [page, actionFilter, loggedInUser])

  useEffect(() => {
    if (tab !== 'following' || !loggedInUser) return
    setFollowingLoading(true)
    fetchFollowing()
      .then(async d => {
        const nss = d.following || []
        if (nss.length === 0) { setFollowingFeed([]); return }
        const results = await Promise.all(
          nss.map(ns => fetchArtifacts({ namespace: ns, sort: 'updated', limit: 5 }).catch(() => ({ artifacts: [] })))
        )
        const all = results.flatMap(r => r.artifacts || [])
        all.sort((a, b) => new Date(b.updated_at) - new Date(a.updated_at))
        setFollowingFeed(all.slice(0, 30))
      })
      .catch(() => {})
      .finally(() => setFollowingLoading(false))
  }, [tab, loggedInUser])

  const totalPages = Math.ceil(total / PAGE_SIZE)

  return (
    <div className="activity-feed-page">
      <div className="activity-feed-header">
        <div>
          <h1 className="activity-feed-title">Activity</h1>
          <p className="activity-feed-sub">Recent events across the SkillForge registry</p>
        </div>
        <button
          className="btn-secondary"
          style={{ fontSize: 13 }}
          onClick={() => {
            const rows = [['Time', 'Actor', 'Action', 'Resource', 'Version']]
            entries.forEach(e => {
              const resource = e.resource || [e.namespace, e.name].filter(Boolean).join('/')
              rows.push([
                e.created_at || e.timestamp || '',
                e.user || e.username || '',
                e.action || '',
                resource || '',
                e.version || '',
              ])
            })
            const csv = rows.map(r => r.map(v => `"${String(v).replace(/"/g, '""')}"`).join(',')).join('\n')
            const blob = new Blob([csv], { type: 'text/csv' })
            const url = URL.createObjectURL(blob)
            const a = document.createElement('a')
            a.href = url
            a.download = `activity-${new Date().toISOString().slice(0, 10)}.csv`
            a.click()
            URL.revokeObjectURL(url)
          }}
        >
          ↓ Export CSV
        </button>
        <select
          className="sort-select"
          value={actionFilter}
          onChange={e => { setActionFilter(e.target.value); setPage(1) }}
        >
          <option value="">All events</option>
          <option value="publish">Publishes</option>
          <option value="delete">Deletes</option>
          <option value="yank">Yanks</option>
          <option value="deprecate">Deprecations</option>
        </select>
      </div>

      {/* Tab switcher */}
      <div className="tabs activity-tabs">
        <button className={`tab-btn ${tab === 'global' ? 'active' : ''}`} onClick={() => setTab('global')}>
          Global
        </button>
        {loggedInUser && (
          <button className={`tab-btn ${tab === 'following' ? 'active' : ''}`} onClick={() => setTab('following')}>
            Following
          </button>
        )}
      </div>

      {/* Following feed */}
      {tab === 'following' && (
        followingLoading ? (
          <div className="loading-state">Loading…</div>
        ) : followingFeed.length === 0 ? (
          <div className="empty-state">
            <h3>Nothing here yet</h3>
            <p>Follow publishers from their namespace page to see their latest artifacts here.</p>
          </div>
        ) : (
          <div className="activity-feed-list">
            {followingFeed.map(a => (
              <div key={`${a.kind}/${a.namespace}/${a.name}`} className="activity-entry">
                <div className="activity-icon act-publish">⬆</div>
                <div className="activity-body">
                  <div className="activity-text">
                    <Link to={`/namespace/${a.namespace}`} className="activity-actor">{a.namespace}</Link>
                    {' published '}
                    <Link to={`/artifacts/${a.kind}/${a.namespace}/${a.name}`} className="activity-target">
                      {a.namespace}/{a.name}
                    </Link>
                    {a.latest_version && <span className="activity-version">@{a.latest_version}</span>}
                  </div>
                  <div className="activity-time">{timeAgo(a.updated_at)}</div>
                </div>
              </div>
            ))}
          </div>
        )
      )}

      {tab === 'global' && error && <div className="error-state">{error}</div>}

      {tab === 'global' && (loading ? (
        <div className="loading-state">Loading activity…</div>
      ) : entries.length === 0 ? (
        <div className="empty-state">
          <h3>No events{actionFilter ? ` for "${actionFilter}"` : ''}</h3>
          <p>Activity will appear here as users interact with the registry.</p>
        </div>
      ) : (
        <div className="activity-feed-list">
          {entries.map((e, i) => {
            const resource = e.resource || [e.namespace, e.name].filter(Boolean).join('/')
            const [ns, artifactName] = (resource || '').split('/')
            const isArtifactEvent = ns && artifactName && ['publish', 'delete', 'yank', 'unyank', 'deprecate'].includes(e.action)
            return (
              <div key={e.id || i} className={`activity-feed-item ${ACTION_COLOR[e.action] || ''}`}>
                <div className="activity-feed-icon">
                  {ACTION_ICON[e.action] || '·'}
                </div>
                <div className="activity-feed-body">
                  <div className="activity-feed-line">
                    {isArtifactEvent ? (
                      <Link to={`/namespace/${ns}`} className="activity-feed-actor">{ns}</Link>
                    ) : (
                      <span className="activity-feed-actor">{e.user || e.username || '–'}</span>
                    )}
                    <span className="activity-feed-verb">
                      {e.action === 'publish' ? 'published' :
                       e.action === 'delete' ? 'deleted' :
                       e.action === 'yank' ? 'yanked' :
                       e.action === 'unyank' ? 'un-yanked' :
                       e.action === 'deprecate' ? 'deprecated' :
                       e.action}
                    </span>
                    {isArtifactEvent ? (
                      <Link
                        to={`/namespace/${ns}`}
                        className="activity-feed-artifact"
                      >
                        <span className="activity-feed-ns-dot" style={{ background: avatarColor(ns) }} />
                        {artifactName}
                      </Link>
                    ) : (
                      resource && <span className="activity-feed-artifact">{resource}</span>
                    )}
                    {e.version && <code className="activity-feed-version">@{e.version}</code>}
                  </div>
                  {e.message && <div className="activity-feed-detail">{e.message}</div>}
                </div>
                <div className="activity-feed-time">
                  {(e.created_at || e.timestamp) ? timeAgo(e.created_at || e.timestamp) : ''}
                </div>
              </div>
            )
          })}
        </div>
      ))}

      {tab === 'global' && totalPages > 1 && (
        <div className="pagination" style={{ marginTop: 20 }}>
          <button className="page-btn" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>← Prev</button>
          <span className="page-info">Page {page} of {totalPages}</span>
          <button className="page-btn" disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>Next →</button>
        </div>
      )}
    </div>
  )
}
