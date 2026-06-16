import { useState, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { fetchArtifacts, fetchStats, fetchTopPublishers } from '../api/client'
import { avatarColor, kindClass, fmtNumber, timeAgo } from '../utils'
import { useDocumentTitle } from '../hooks/useDocumentTitle'

const KINDS = [
  { id: 'skill',  icon: '⬟', label: 'Skills',   desc: 'Reusable AI capabilities and functions' },
  { id: 'agent',  icon: '⬡', label: 'Agents',   desc: 'Autonomous AI agents and assistants' },
  { id: 'flow',   icon: '⇄', label: 'Flows',    desc: 'Multi-step pipelines and workflows' },
  { id: 'prompt', icon: '❝', label: 'Prompts',  desc: 'Prompt templates and instructions' },
  { id: 'tool',   icon: '⚙', label: 'Tools',    desc: 'External tools and integrations' },
  { id: 'bundle', icon: '▣', label: 'Bundles',  desc: 'Curated collections of artifacts' },
]

export default function ExplorePage() {
  useDocumentTitle('Explore · SkillForge')
  const navigate = useNavigate()

  const [stats, setStats] = useState(null)
  const [trending, setTrending] = useState([])
  const [publishers, setPublishers] = useState([])
  const [kindCounts, setKindCounts] = useState({})
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchStats().then(setStats).catch(() => {})
    fetchTopPublishers(12).then(d => setPublishers(d.publishers || [])).catch(() => {})

    Promise.all(
      KINDS.map(k =>
        fetchArtifacts({ kind: k.id, limit: 1, offset: 0 })
          .then(d => ({ kind: k.id, count: d.total || 0 }))
          .catch(() => ({ kind: k.id, count: 0 }))
      )
    ).then(results => {
      const counts = {}
      results.forEach(r => { counts[r.kind] = r.count })
      setKindCounts(counts)
    })

    fetchArtifacts({ sort: 'pulls', limit: 20, offset: 0 })
      .then(d => {
        const now = Date.now()
        const scored = (d.artifacts || []).map(a => {
          const ageDays = (now - new Date(a.updated_at || a.created_at).getTime()) / 86400000
          return { ...a, _score: (a.downloads || 0) / Math.sqrt(ageDays + 2) }
        })
        scored.sort((a, b) => b._score - a._score)
        setTrending(scored.slice(0, 8))
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  const verifiedPublishers = publishers.filter(p => p.verified)
  const allPublishers = publishers.filter(p => !p.verified)

  return (
    <div className="explore-page">
      {/* Hero */}
      <div className="explore-hero">
        <h1 className="explore-hero-title">Explore the Registry</h1>
        <p className="explore-hero-sub">
          Discover skills, agents, flows, and tools built by the community
        </p>
        {stats && (
          <div className="explore-hero-stats">
            <span><strong>{fmtNumber(stats.total_artifacts || 0)}</strong> artifacts</span>
            <span className="explore-stat-sep">·</span>
            <span><strong>{fmtNumber(stats.total_namespaces || 0)}</strong> publishers</span>
            <span className="explore-stat-sep">·</span>
            <span><strong>{fmtNumber(stats.total_downloads || 0)}</strong> total pulls</span>
          </div>
        )}
      </div>

      {/* Category grid */}
      <section className="explore-section">
        <h2 className="explore-section-title">Browse by type</h2>
        <div className="explore-kind-grid">
          {KINDS.map(k => (
            <Link
              key={k.id}
              to={`/?kind=${k.id}`}
              className={`explore-kind-card kind-border-${k.id}`}
            >
              <div className={`explore-kind-icon ${kindClass(k.id)}`}>{k.icon}</div>
              <div className="explore-kind-info">
                <div className="explore-kind-label">{k.label}</div>
                <div className="explore-kind-desc">{k.desc}</div>
              </div>
              {kindCounts[k.id] > 0 && (
                <div className="explore-kind-count">{fmtNumber(kindCounts[k.id])}</div>
              )}
            </Link>
          ))}
        </div>
      </section>

      {/* Trending artifacts */}
      {trending.length > 0 && (
        <section className="explore-section">
          <div className="explore-section-header">
            <h2 className="explore-section-title">🔥 Trending</h2>
            <Link to="/?sort=pulls" className="explore-view-all">View all →</Link>
          </div>
          <div className="explore-trending-grid">
            {trending.map((a, i) => (
              <Link
                key={`${a.kind}/${a.namespace}/${a.name}`}
                to={`/artifacts/${a.kind}/${a.namespace}/${a.name}`}
                className="explore-trending-card"
              >
                <div className="explore-trending-rank">#{i + 1}</div>
                <div className={`explore-trending-avatar ${kindClass(a.kind)}`} style={{ background: avatarColor(a.namespace) }}>
                  {a.namespace[0].toUpperCase()}
                </div>
                <div className="explore-trending-body">
                  <div className="explore-trending-name">{a.name}</div>
                  <div className="explore-trending-ns">{a.namespace}</div>
                  <div className="explore-trending-meta">
                    <span className="kind-chip kind-chip-sm">{a.kind}</span>
                    <span className="explore-trending-pulls">{fmtNumber(a.downloads || 0)} pulls</span>
                  </div>
                </div>
              </Link>
            ))}
          </div>
        </section>
      )}

      {/* Verified publishers */}
      {verifiedPublishers.length > 0 && (
        <section className="explore-section">
          <h2 className="explore-section-title">✓ Verified publishers</h2>
          <p className="explore-section-sub">
            Verified publishers are trusted organizations and individuals whose identity has been confirmed.
          </p>
          <div className="explore-publishers-grid">
            {verifiedPublishers.map(p => (
              <Link key={p.namespace} to={`/namespace/${p.namespace}`} className="explore-publisher-card">
                <div className="explore-pub-avatar" style={{ background: avatarColor(p.namespace) }}>
                  {p.namespace[0].toUpperCase()}
                </div>
                <div className="explore-pub-info">
                  <div className="explore-pub-name">
                    {p.display_name || p.namespace}
                    <span className="ns-verified-badge" title="Verified">✓</span>
                  </div>
                  {p.display_name && <div className="explore-pub-handle">@{p.namespace}</div>}
                  <div className="explore-pub-stats">
                    {fmtNumber(p.artifact_count)} artifact{p.artifact_count !== 1 ? 's' : ''}
                    {p.total_downloads > 0 && <> · {fmtNumber(p.total_downloads)} pulls</>}
                  </div>
                </div>
              </Link>
            ))}
          </div>
        </section>
      )}

      {/* All publishers */}
      {allPublishers.length > 0 && (
        <section className="explore-section">
          <h2 className="explore-section-title">Top publishers</h2>
          <div className="explore-publishers-grid">
            {allPublishers.slice(0, 12).map(p => (
              <Link key={p.namespace} to={`/namespace/${p.namespace}`} className="explore-publisher-card">
                <div className="explore-pub-avatar" style={{ background: avatarColor(p.namespace) }}>
                  {p.namespace[0].toUpperCase()}
                </div>
                <div className="explore-pub-info">
                  <div className="explore-pub-name">{p.display_name || p.namespace}</div>
                  {p.display_name && <div className="explore-pub-handle">@{p.namespace}</div>}
                  <div className="explore-pub-stats">
                    {fmtNumber(p.artifact_count)} artifact{p.artifact_count !== 1 ? 's' : ''}
                    {p.total_downloads > 0 && <> · {fmtNumber(p.total_downloads)} pulls</>}
                  </div>
                </div>
              </Link>
            ))}
          </div>
        </section>
      )}
    </div>
  )
}
