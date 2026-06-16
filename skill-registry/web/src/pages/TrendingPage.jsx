import { useState, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { fetchArtifacts, fetchTopPublishers } from '../api/client'
import { useDocumentTitle } from '../hooks/useDocumentTitle'
import { avatarColor, kindClass, fmtNumber } from '../utils'

const PERIODS = [
  { label: 'Today', value: '1d', sort: 'trending' },
  { label: 'This week', value: '7d', sort: 'trending' },
  { label: 'This month', value: '30d', sort: 'pulls' },
  { label: 'All time', value: 'all', sort: 'pulls' },
]

const KINDS = [
  { label: 'All', value: '' },
  { label: 'Skills', value: 'skill' },
  { label: 'Agents', value: 'agent' },
  { label: 'Tools', value: 'tool' },
  { label: 'Bundles', value: 'bundle' },
]

export default function TrendingPage() {
  useDocumentTitle('Trending — SkillForge')
  const navigate = useNavigate()
  const [period, setPeriod] = useState('7d')
  const [kind, setKind] = useState('')
  const [artifacts, setArtifacts] = useState([])
  const [loading, setLoading] = useState(true)
  const [stars, setStars] = useState([])
  const [starsLoading, setStarsLoading] = useState(true)
  const [publishers, setPublishers] = useState([])

  const currentPeriod = PERIODS.find(p => p.value === period) || PERIODS[1]

  useEffect(() => {
    setLoading(true)
    fetchArtifacts({ kind, sort: currentPeriod.sort, limit: 20, offset: 0 })
      .then(d => setArtifacts(d.artifacts || []))
      .catch(() => setArtifacts([]))
      .finally(() => setLoading(false))
  }, [period, kind])

  useEffect(() => {
    setStarsLoading(true)
    fetchArtifacts({ sort: 'stars', limit: 10, offset: 0 })
      .then(d => setStars(d.artifacts || []))
      .catch(() => setStars([]))
      .finally(() => setStarsLoading(false))
    fetchTopPublishers(8)
      .then(d => setPublishers(d.publishers || []))
      .catch(() => setPublishers([]))
  }, [])

  return (
    <div className="trending-page">
      <div className="trending-hero">
        <h1 className="trending-hero-title">🔥 Trending</h1>
        <p className="trending-hero-sub">The most popular AI skills, agents, and tools right now</p>
      </div>

      <div className="trending-layout">
        {/* Main column */}
        <div className="trending-main">
          {/* Period + kind controls */}
          <div className="trending-controls">
            <div className="trending-period-tabs">
              {PERIODS.map(p => (
                <button
                  key={p.value}
                  className={`trending-period-tab ${period === p.value ? 'active' : ''}`}
                  onClick={() => setPeriod(p.value)}
                >{p.label}</button>
              ))}
            </div>
            <div className="trending-kind-tabs">
              {KINDS.map(k => (
                <button
                  key={k.value}
                  className={`trending-kind-tab ${kind === k.value ? 'active' : ''}`}
                  onClick={() => setKind(k.value)}
                >{k.label}</button>
              ))}
            </div>
          </div>

          {/* Ranked list */}
          {loading ? (
            <div className="loading-state">Loading trending artifacts…</div>
          ) : artifacts.length === 0 ? (
            <div className="empty-state"><h3>No results</h3><p>No artifacts found for this filter.</p></div>
          ) : (
            <div className="trending-list">
              {artifacts.map((a, i) => (
                <button
                  key={`${a.kind}/${a.namespace}/${a.name}`}
                  className="trending-row"
                  onClick={() => navigate(`/artifacts/${a.kind}/${a.namespace}/${a.name}`)}
                >
                  <div className="trending-rank">#{i + 1}</div>
                  <div className={`trending-avatar ${kindClass(a.kind)}`} style={{ background: avatarColor(a.namespace) }}>
                    {a.namespace[0].toUpperCase()}
                  </div>
                  <div className="trending-info">
                    <div className="trending-name">
                      {a.name}
                      <span className="kind-chip kind-chip-sm">{a.kind}</span>
                    </div>
                    <div className="trending-ns">
                      <Link
                        to={`/namespace/${a.namespace}`}
                        className="trending-ns-link"
                        onClick={e => e.stopPropagation()}
                      >{a.namespace}</Link>
                      {a.latest_version && <span className="trending-ver"> · v{a.latest_version}</span>}
                    </div>
                    {a.description && <div className="trending-desc">{a.description}</div>}
                  </div>
                  <div className="trending-stats">
                    <span className="trending-pulls">⬇ {fmtNumber(a.downloads || 0)}</span>
                    {a.stars > 0 && <span className="trending-stars">★ {fmtNumber(a.stars)}</span>}
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Sidebar */}
        <aside className="trending-sidebar">
          {/* Stars leaderboard */}
          <div className="trending-sidebar-card">
            <div className="trending-sidebar-title">⭐ Most starred</div>
            {starsLoading ? (
              <div className="loading-state" style={{ padding: '12px 0', fontSize: 13 }}>Loading…</div>
            ) : (
              <div className="trending-sidebar-list">
                {stars.map((a, i) => (
                  <Link
                    key={`${a.kind}/${a.namespace}/${a.name}`}
                    to={`/artifacts/${a.kind}/${a.namespace}/${a.name}`}
                    className="trending-sidebar-row"
                  >
                    <span className="trending-sidebar-rank">{i + 1}</span>
                    <div className={`trending-sidebar-avatar ${kindClass(a.kind)}`} style={{ background: avatarColor(a.namespace) }}>
                      {a.namespace[0].toUpperCase()}
                    </div>
                    <div className="trending-sidebar-info">
                      <div className="trending-sidebar-name">{a.name}</div>
                      <div className="trending-sidebar-ns">{a.namespace}</div>
                    </div>
                    <span className="trending-sidebar-count">★ {fmtNumber(a.stars || 0)}</span>
                  </Link>
                ))}
              </div>
            )}
          </div>

          {/* Featured publishers */}
          {publishers.length > 0 && (
            <div className="trending-sidebar-card">
              <div className="trending-sidebar-title">🏆 Top publishers</div>
              <div className="trending-sidebar-list">
                {publishers.map((p, i) => (
                  <Link
                    key={p.namespace}
                    to={`/namespace/${p.namespace}`}
                    className="trending-sidebar-row"
                  >
                    <span className="trending-sidebar-rank">{i + 1}</span>
                    <div className="trending-sidebar-pub-avatar" style={{ background: avatarColor(p.namespace) }}>
                      {p.namespace[0].toUpperCase()}
                    </div>
                    <div className="trending-sidebar-info">
                      <div className="trending-sidebar-name">
                        {p.display_name || p.namespace}
                        {p.verified && <span className="ns-verified-badge" style={{ marginLeft: 4 }}>✓</span>}
                      </div>
                      <div className="trending-sidebar-ns">{fmtNumber(p.artifact_count || 0)} artifacts</div>
                    </div>
                    <span className="trending-sidebar-count">{fmtNumber(p.total_downloads || 0)}</span>
                  </Link>
                ))}
              </div>
            </div>
          )}
        </aside>
      </div>
    </div>
  )
}
