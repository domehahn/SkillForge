import { useState, useEffect } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { fetchNamespaceInsights, fetchNamespace } from '../api/client'
import { fmtNumber, timeAgo, avatarColor } from '../utils'
import { useDocumentTitle } from '../hooks/useDocumentTitle'
import { useToast } from '../contexts/ToastContext'

function MiniTrendBar({ trend, days }) {
  const filled = []
  const now = new Date()
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date(now)
    d.setDate(d.getDate() - i)
    const key = d.toISOString().slice(0, 10)
    const entry = (trend || []).find(t => t.day === key)
    filled.push(entry?.count || 0)
  }
  const max = Math.max(...filled, 1)
  return (
    <div className="insight-mini-trend">
      {filled.map((v, i) => (
        <div
          key={i}
          className="insight-mini-bar"
          style={{ height: `${Math.max(2, (v / max) * 28)}px` }}
          title={`${v} pulls`}
        />
      ))}
    </div>
  )
}

export default function InsightsPage() {
  const { namespace } = useParams()
  useDocumentTitle(`Insights · ${namespace}`)
  const navigate = useNavigate()
  const toast = useToast()

  const loggedInUser = localStorage.getItem('sf_user')
  const [days, setDays] = useState(30)
  const [insights, setInsights] = useState([])
  const [nsData, setNsData] = useState(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!loggedInUser) { navigate('/login'); return }
    fetchNamespace(namespace).then(setNsData).catch(() => {})
  }, [namespace])

  useEffect(() => {
    setLoading(true)
    fetchNamespaceInsights(namespace, days)
      .then(d => setInsights(d.insights || []))
      .catch(err => toast(err.message, 'error'))
      .finally(() => setLoading(false))
  }, [namespace, days])

  const totalDownloads = insights.reduce((s, i) => s + i.downloads, 0)
  const totalStars = insights.reduce((s, i) => s + i.stars, 0)
  const color = avatarColor(namespace)

  return (
    <div className="insights-page">
      {/* Header */}
      <div className="insights-header">
        <div className="insights-breadcrumb">
          <Link to={`/namespace/${namespace}`}>{namespace}</Link>
          <span> › </span>
          <span>Insights</span>
        </div>
        <div className="insights-title-row">
          <div className="insights-avatar" style={{ background: color }}>
            {namespace[0].toUpperCase()}
          </div>
          <div>
            <h1 className="insights-title">
              {nsData?.display_name || namespace}
            </h1>
            <div className="insights-sub">Publisher analytics</div>
          </div>
        </div>
      </div>

      {/* Summary cards */}
      <div className="insights-summary">
        <div className="insight-stat-card">
          <div className="insight-stat-value">{fmtNumber(nsData?.artifact_count || 0)}</div>
          <div className="insight-stat-label">Artifacts</div>
        </div>
        <div className="insight-stat-card">
          <div className="insight-stat-value">{fmtNumber(totalDownloads)}</div>
          <div className="insight-stat-label">Total pulls (all time)</div>
        </div>
        <div className="insight-stat-card">
          <div className="insight-stat-value">{fmtNumber(totalStars)}</div>
          <div className="insight-stat-label">Total stars</div>
        </div>
      </div>

      {/* Controls */}
      <div className="insights-controls">
        <span className="insights-controls-label">Time range:</span>
        {[7, 14, 30, 60, 90].map(d => (
          <button
            key={d}
            className={`insights-period-btn ${days === d ? 'active' : ''}`}
            onClick={() => setDays(d)}
          >
            {d}d
          </button>
        ))}
      </div>

      {/* Per-artifact table */}
      {loading ? (
        <div className="loading-state">Loading insights…</div>
      ) : insights.length === 0 ? (
        <div className="empty-state">
          <h3>No data yet</h3>
          <p>Publish artifacts and they'll appear here once they get pulls.</p>
        </div>
      ) : (
        <div className="insights-table-wrap">
          <table className="insights-table">
            <thead>
              <tr>
                <th>Artifact</th>
                <th>Kind</th>
                <th>Stars</th>
                <th>Total pulls</th>
                <th className="insight-trend-col">Last {days} days</th>
              </tr>
            </thead>
            <tbody>
              {insights.map((ins, i) => (
                <tr key={`${ins.kind}/${ins.name}`} className={i === 0 ? 'insight-row-top' : ''}>
                  <td>
                    <Link
                      to={`/artifacts/${ins.kind}/${namespace}/${ins.name}`}
                      className="insight-artifact-link"
                    >
                      {ins.name}
                    </Link>
                  </td>
                  <td><span className="kind-chip kind-chip-sm">{ins.kind}</span></td>
                  <td className="insight-stars">
                    {ins.stars > 0 ? `★ ${fmtNumber(ins.stars)}` : '—'}
                  </td>
                  <td className="insight-pulls">
                    <strong>{fmtNumber(ins.downloads)}</strong>
                  </td>
                  <td>
                    <MiniTrendBar trend={ins.trend} days={days} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
