import { useState, useEffect, useCallback } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { fetchAuditLog } from '../api/client'
import { timeAgo } from '../utils'
import { useDocumentTitle } from '../hooks/useDocumentTitle'

const ACTION_COLORS = {
  publish:   '#22c55e',
  delete:    '#ef4444',
  yank:      '#f59e0b',
  unyank:    '#10b981',
  login:     '#6366f1',
  token:     '#8b5cf6',
  transfer:  '#0ea5e9',
  deprecate: '#f97316',
}

function actionColor(action) {
  for (const [key, color] of Object.entries(ACTION_COLORS)) {
    if (action?.includes(key)) return color
  }
  return '#94a3b8'
}

const PAGE_SIZE = 50

export default function AuditLogPage() {
  useDocumentTitle('Audit Log')
  const navigate = useNavigate()

  const role = localStorage.getItem('sf_role')
  useEffect(() => {
    if (!localStorage.getItem('sf_token')) navigate('/login')
  }, [])

  const [entries, setEntries] = useState([])
  const [loading, setLoading] = useState(true)
  const [offset, setOffset] = useState(0)
  const [hasMore, setHasMore] = useState(false)
  const [filterActor, setFilterActor] = useState('')
  const [filterAction, setFilterAction] = useState('')
  const [inputActor, setInputActor] = useState('')
  const [inputAction, setInputAction] = useState('')

  const load = useCallback((off = 0) => {
    setLoading(true)
    fetchAuditLog({ limit: PAGE_SIZE, offset: off, action: filterAction || undefined })
      .then(d => {
        const rows = d.entries || []
        setEntries(rows)
        setHasMore(rows.length === PAGE_SIZE)
        setOffset(off)
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [filterAction])

  useEffect(() => { load(0) }, [load])

  const filtered = filterActor
    ? entries.filter(e => e.actor?.toLowerCase().includes(filterActor.toLowerCase()))
    : entries

  return (
    <div className="audit-log-page">
      <div className="audit-log-header">
        <div>
          <h1 className="audit-log-title">Audit Log</h1>
          <p className="audit-log-sub">All registry events across all users and namespaces</p>
        </div>
      </div>

      {/* Filters */}
      <form
        className="audit-filters"
        onSubmit={e => { e.preventDefault(); setFilterActor(inputActor); setFilterAction(inputAction); load(0) }}
      >
        <input
          className="audit-filter-input"
          placeholder="Filter by actor…"
          value={inputActor}
          onChange={e => setInputActor(e.target.value)}
        />
        <input
          className="audit-filter-input"
          placeholder="Filter by action…"
          value={inputAction}
          onChange={e => setInputAction(e.target.value)}
        />
        <button className="btn-secondary" type="submit">Filter</button>
        {(filterActor || filterAction) && (
          <button
            className="btn-secondary"
            type="button"
            onClick={() => { setInputActor(''); setInputAction(''); setFilterActor(''); setFilterAction('') }}
          >Clear</button>
        )}
      </form>

      {loading ? (
        <div className="loading-state">Loading…</div>
      ) : filtered.length === 0 ? (
        <div className="empty-state">
          <h3>No audit entries</h3>
          <p>Actions will appear here once users interact with the registry.</p>
        </div>
      ) : (
        <>
          <div className="audit-table-wrap">
            <table className="audit-table">
              <thead>
                <tr>
                  <th>Time</th>
                  <th>Actor</th>
                  <th>Action</th>
                  <th>Target</th>
                  <th>IP</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map(e => (
                  <tr key={e.id} className="audit-row">
                    <td className="audit-time" title={e.created_at}>
                      {timeAgo(e.created_at)}
                    </td>
                    <td className="audit-actor">
                      <Link to={`/u/${e.actor}`}>{e.actor}</Link>
                    </td>
                    <td className="audit-action">
                      <span
                        className="audit-action-chip"
                        style={{ background: actionColor(e.action) + '22', color: actionColor(e.action), borderColor: actionColor(e.action) + '44' }}
                      >
                        {e.action}
                      </span>
                    </td>
                    <td className="audit-target">{e.target || '—'}</td>
                    <td className="audit-ip">{e.ip_address || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="audit-pagination">
            <button
              className="btn-secondary"
              disabled={offset === 0}
              onClick={() => load(Math.max(0, offset - PAGE_SIZE))}
            >← Newer</button>
            <span className="audit-page-info">Showing {offset + 1}–{offset + filtered.length}</span>
            <button
              className="btn-secondary"
              disabled={!hasMore}
              onClick={() => load(offset + PAGE_SIZE)}
            >Older →</button>
          </div>
        </>
      )}
    </div>
  )
}
