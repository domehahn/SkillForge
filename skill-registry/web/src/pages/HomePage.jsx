import { useState, useEffect } from 'react'
import { useSearchParams, Link } from 'react-router-dom'
import { fetchArtifacts, fetchFacets, fetchStats, fetchTopPublishers, fetchFollowing } from '../api/client'
import ArtifactCard from '../components/ArtifactCard'
import { SkeletonCards, SkeletonGridCards } from '../components/Skeleton'
import { avatarColor, kindClass, fmtNumber, timeAgo } from '../utils'
import { useDocumentTitle } from '../hooks/useDocumentTitle'
import { getRecentlyViewed } from '../hooks/useRecentlyViewed'

const KINDS = ['skill', 'agent', 'flow', 'prompt', 'tool', 'bundle']

const KIND_ICONS = {
  skill: '⬟',
  agent: '⬡',
  flow: '⇄',
  prompt: '❝',
  tool: '⚙',
  bundle: '▣',
}

const PAGE_SIZE = 20

function FeaturedCard({ artifact }) {
  const { kind, namespace, name, description, downloads = 0, updated_at } = artifact
  const color = avatarColor(namespace)
  return (
    <Link to={`/artifacts/${kind}/${namespace}/${name}`} className="featured-card">
      <div className={`featured-card-avatar ${kindClass(kind)}`} style={{ background: color }}>
        {namespace[0].toUpperCase()}
      </div>
      <div className="featured-card-body">
        <div className="featured-card-title">{name}</div>
        <div className="featured-card-ns">
          <span className="kind-chip kind-chip-sm">{kind}</span>
          <span className="featured-card-ns-name">{namespace}</span>
        </div>
        {description && <div className="featured-card-desc">{description}</div>}
        <div className="featured-card-meta">
          {downloads > 0 && <span>⬇ {fmtNumber(downloads)}</span>}
          {updated_at && <span>{timeAgo(updated_at)}</span>}
        </div>
      </div>
    </Link>
  )
}

function GridCard({ artifact }) {
  const { kind, namespace, name, description, downloads = 0, updated_at } = artifact
  const color = avatarColor(namespace)
  return (
    <Link to={`/artifacts/${kind}/${namespace}/${name}`} className="grid-card">
      <div className="grid-card-top">
        <div className={`grid-card-avatar ${kindClass(kind)}`} style={{ background: color }}>
          {namespace[0].toUpperCase()}
        </div>
        <span className="kind-chip kind-chip-sm">{kind}</span>
      </div>
      <div className="grid-card-name">{name}</div>
      <div className="grid-card-ns">{namespace}</div>
      {description && <div className="grid-card-desc">{description}</div>}
      <div className="grid-card-footer">
        {downloads > 0 && <span className="grid-card-stat">⬇ {fmtNumber(downloads)}</span>}
        {updated_at && <span className="grid-card-stat">{timeAgo(updated_at)}</span>}
      </div>
    </Link>
  )
}

export default function HomePage() {
  const [searchParams, setSearchParams] = useSearchParams()

  const q = searchParams.get('q') || ''
  const kind = searchParams.get('kind') || ''
  const namespace = searchParams.get('namespace') || ''
  const sort = searchParams.get('sort') || 'updated'
  const page = parseInt(searchParams.get('page') || '1', 10)

  const [heroInput, setHeroInput] = useState(q)
  const [artifacts, setArtifacts] = useState([])
  const [total, setTotal] = useState(0)
  const [stats, setStats] = useState(null)
  const [kindCounts, setKindCounts] = useState({})
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [mostPulled, setMostPulled] = useState([])
  const [recentlyAdded, setRecentlyAdded] = useState([])
  const [newThisWeek, setNewThisWeek] = useState([])
  const [trending, setTrending] = useState([])
  const [recentlyViewed, setRecentlyViewed] = useState(() => getRecentlyViewed())
  const [viewMode, setViewMode] = useState(() => localStorage.getItem('sf_view') || 'list')
  const [topPublishers, setTopPublishers] = useState([])
  const [followingFeed, setFollowingFeed] = useState([])
  const loggedInUser = localStorage.getItem('sf_user')

  useEffect(() => {
    if (loggedInUser) {
      fetchFollowing()
        .then(async d => {
          const nss = d.following || []
          if (!nss.length) return
          const results = await Promise.all(
            nss.map(ns => fetchArtifacts({ namespace: ns, sort: 'updated', limit: 4 }).catch(() => ({ artifacts: [] })))
          )
          const all = results.flatMap(r => r.artifacts || [])
          all.sort((a, b) => new Date(b.updated_at) - new Date(a.updated_at))
          setFollowingFeed(all.slice(0, 4))
        })
        .catch(() => {})
    }
  }, [loggedInUser])

  useEffect(() => {
    fetchStats().then(setStats).catch(() => {})
    const syncRecent = () => setRecentlyViewed(getRecentlyViewed())
    window.addEventListener('sf_recent', syncRecent)
    return () => window.removeEventListener('sf_recent', syncRecent)
  }, [])

  useEffect(() => {
    fetchArtifacts({ sort: 'pulls', limit: 4, offset: 0 })
      .then(d => setMostPulled(d.artifacts || []))
      .catch(() => {})
    fetchArtifacts({ sort: 'updated', limit: 4, offset: 0 })
      .then(d => setRecentlyAdded(d.artifacts || []))
      .catch(() => {})
    fetchArtifacts({ sort: 'name', limit: 20, offset: 0 })
      .then(d => {
        const cutoff = Date.now() - 7 * 24 * 60 * 60 * 1000
        const fresh = (d.artifacts || []).filter(a => new Date(a.created_at).getTime() > cutoff)
        setNewThisWeek(fresh.slice(0, 4))
      })
      .catch(() => {})
    // Trending: score = downloads / sqrt(age_days + 2) — HN-style gravity
    fetchArtifacts({ sort: 'pulls', limit: 20, offset: 0 })
      .then(d => {
        const now = Date.now()
        const scored = (d.artifacts || []).map(a => {
          const ageDays = (now - new Date(a.updated_at || a.created_at).getTime()) / 86400000
          const score = (a.downloads || 0) / Math.sqrt(ageDays + 2)
          return { ...a, _score: score }
        })
        scored.sort((a, b) => b._score - a._score)
        setTrending(scored.slice(0, 4))
      })
      .catch(() => {})
    fetchTopPublishers(8)
      .then(d => setTopPublishers(d.publishers || []))
      .catch(() => {})
  }, [])

  useEffect(() => {
    setLoading(true)
    setError(null)
    const offset = (page - 1) * PAGE_SIZE
    fetchArtifacts({ q, kind, namespace, sort, limit: PAGE_SIZE, offset })
      .then(data => {
        setArtifacts(data.artifacts || [])
        setTotal(data.total || 0)
      })
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }, [q, kind, sort, page])

  useEffect(() => {
    fetchFacets(q)
      .then(data => setKindCounts(data.facets || {}))
      .catch(() => {})
  }, [q])

  const showFeatured = !q && !kind && !namespace

  const setFilter = (updates) => {
    const next = {}
    if (q) next.q = q
    if (kind) next.kind = kind
    if (namespace) next.namespace = namespace
    if (sort !== 'updated') next.sort = sort
    Object.assign(next, updates)
    if (!next.kind) delete next.kind
    if (!next.namespace) delete next.namespace
    if (!next.sort || next.sort === 'updated') delete next.sort
    delete next.page
    setSearchParams(next)
  }

  const handleHeroSearch = (e) => {
    e.preventDefault()
    const trimmed = heroInput.trim()
    setSearchParams(trimmed ? { q: trimmed } : {})
  }

  const switchView = (mode) => {
    setViewMode(mode)
    localStorage.setItem('sf_view', mode)
  }

  const totalPages = Math.ceil(total / PAGE_SIZE)

  useDocumentTitle(q ? `Search: ${q}` : namespace ? `${namespace}'s artifacts` : kind ? `${kind}s` : null)

  return (
    <>
      {/* Hero */}
      {!q && !namespace && (
        <div className="hero">
          <h2>The SkillForge Registry</h2>
          <p>Discover, share, and deploy AI skills, agents, and tools</p>
          <form className="hero-search" onSubmit={handleHeroSearch}>
            <input
              value={heroInput}
              onChange={e => setHeroInput(e.target.value)}
              placeholder="Search skills, agents, tools…"
            />
            <button type="submit">Search</button>
          </form>
        </div>
      )}

      {/* Kind category tiles — only on clean home, between hero and stats */}
      {!q && !namespace && (
        <div className="category-tiles-bar">
          <div className="category-tiles-inner">
            {KINDS.map(k => (
              <button
                key={k}
                className={`category-tile ${kind === k ? 'active' : ''}`}
                onClick={() => setFilter({ kind: kind === k ? '' : k })}
              >
                <span className="category-tile-icon">{KIND_ICONS[k]}</span>
                <span className="category-tile-name">{k}s</span>
                {kindCounts[k] > 0 && (
                  <span className="category-tile-count">{fmtNumber(kindCounts[k])}</span>
                )}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Stats bar */}
      {stats && (
        <div className="stats-bar">
          <div className="stats-inner">
            <div className="stat-item">
              <span className="stat-number">{fmtNumber(stats.total_artifacts || 0)}</span>
              <span className="stat-label">Artifacts</span>
            </div>
            <div className="stat-item">
              <span className="stat-number">{fmtNumber(stats.total_downloads || 0)}</span>
              <span className="stat-label">Total pulls</span>
            </div>
            <div className="stat-item">
              <span className="stat-number">{fmtNumber(stats.total_namespaces || 0)}</span>
              <span className="stat-label">Publishers</span>
            </div>
          </div>
        </div>
      )}

      {/* Continue exploring (recently viewed) */}
      {showFeatured && recentlyViewed.length > 0 && localStorage.getItem('sf_user') && (
        <div className="recent-viewed-bar">
          <div className="recent-viewed-inner">
            <span className="recent-viewed-label">Continue exploring</span>
            <div className="recent-viewed-chips">
              {recentlyViewed.slice(0, 6).map(r => (
                <Link
                  key={`${r.kind}/${r.namespace}/${r.name}`}
                  to={`/artifacts/${r.kind}/${r.namespace}/${r.name}`}
                  className="recent-viewed-chip"
                >
                  <div className="recent-chip-dot" style={{ background: avatarColor(r.namespace) }} />
                  <span>{r.name}</span>
                  <span className="recent-chip-ns">{r.namespace}</span>
                </Link>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* Featured sections */}
      {showFeatured && (mostPulled.length > 0 || followingFeed.length > 0) && (
        <div className="featured-sections">
          {followingFeed.length > 0 && (
            <div className="featured-section">
              <div className="featured-section-header">
                <h3 className="featured-section-title">✨ From publishers you follow</h3>
                <Link to="/activity" className="featured-view-all">See all activity →</Link>
              </div>
              <div className="featured-grid">
                {followingFeed.map(a => (
                  <FeaturedCard key={`${a.kind}/${a.namespace}/${a.name}`} artifact={a} />
                ))}
              </div>
            </div>
          )}

          {mostPulled.length > 0 && (
          <div className="featured-section">
            <div className="featured-section-header">
              <h3 className="featured-section-title">Most pulled</h3>
              <button className="featured-view-all" onClick={() => setFilter({ sort: 'pulls' })}>
                View all →
              </button>
            </div>
            <div className="featured-grid">
              {mostPulled.map(a => (
                <FeaturedCard key={`${a.kind}/${a.namespace}/${a.name}`} artifact={a} />
              ))}
            </div>
          </div>
          )}

          {recentlyAdded.length > 0 && (
            <div className="featured-section">
              <div className="featured-section-header">
                <h3 className="featured-section-title">Recently updated</h3>
                <button className="featured-view-all" onClick={() => setFilter({ sort: 'updated' })}>
                  View all →
                </button>
              </div>
              <div className="featured-grid">
                {recentlyAdded.map(a => (
                  <FeaturedCard key={`${a.kind}/${a.namespace}/${a.name}`} artifact={a} />
                ))}
              </div>
            </div>
          )}

          {newThisWeek.length > 0 && (
            <div className="featured-section">
              <div className="featured-section-header">
                <h3 className="featured-section-title">New this week</h3>
                <button className="featured-view-all" onClick={() => setFilter({ sort: 'updated' })}>
                  View all →
                </button>
              </div>
              <div className="featured-grid">
                {newThisWeek.map(a => (
                  <FeaturedCard key={`${a.kind}/${a.namespace}/${a.name}`} artifact={a} />
                ))}
              </div>
            </div>
          )}

          {trending.length > 0 && (
            <div className="featured-section">
              <div className="featured-section-header">
                <h3 className="featured-section-title">🔥 Trending</h3>
                <button className="featured-view-all" onClick={() => setFilter({ sort: 'pulls' })}>
                  View all →
                </button>
              </div>
              <div className="featured-grid">
                {trending.map((a, i) => (
                  <FeaturedCard key={`${a.kind}/${a.namespace}/${a.name}`} artifact={a} rank={i + 1} />
                ))}
              </div>
            </div>
          )}

          {topPublishers.length > 0 && (
            <div className="featured-section">
              <div className="featured-section-header">
                <h3 className="featured-section-title">Top publishers</h3>
                <Link to="/explore/publishers" className="featured-view-all">View all →</Link>
              </div>
              <div className="publishers-grid">
                {topPublishers.map(p => (
                  <Link
                    key={p.namespace}
                    to={`/namespace/${p.namespace}`}
                    className="publisher-card"
                  >
                    <div className="publisher-card-avatar" style={{ background: avatarColor(p.namespace) }}>
                      {p.namespace[0].toUpperCase()}
                    </div>
                    <div className="publisher-card-info">
                      <div className="publisher-card-name">
                        {p.display_name || p.namespace}
                        {p.verified && <span className="ns-verified-badge" title="Verified">✓</span>}
                      </div>
                      {p.display_name && <div className="publisher-card-handle">@{p.namespace}</div>}
                      <div className="publisher-card-stats">
                        {fmtNumber(p.artifact_count)} artifact{p.artifact_count !== 1 ? 's' : ''}
                        {p.total_downloads > 0 && <> · {fmtNumber(p.total_downloads)} pulls</>}
                      </div>
                    </div>
                  </Link>
                ))}
              </div>
            </div>
          )}

          <div className="featured-divider" />
        </div>
      )}

      <div className="browse-layout">
        {/* Sidebar */}
        <aside className="sidebar">
          <div className="sidebar-section">
            <div className="sidebar-title">Type</div>
            <button
              className={`filter-btn ${kind === '' ? 'active' : ''}`}
              onClick={() => setFilter({ kind: '' })}
            >
              <span className="filter-btn-label">
                <span className="filter-kind-icon">◈</span> All
              </span>
              <span className="filter-count">{fmtNumber(total)}</span>
            </button>
            {KINDS.map(k => (
              <button
                key={k}
                className={`filter-btn ${kind === k ? 'active' : ''}`}
                onClick={() => setFilter({ kind: k })}
              >
                <span className="filter-btn-label">
                  <span className="filter-kind-icon">{KIND_ICONS[k] || '·'}</span>
                  {k}
                </span>
                <span className="filter-count">{fmtNumber(kindCounts[k] || 0)}</span>
              </button>
            ))}
          </div>
        </aside>

        {/* Main */}
        <main className="main-content">
          <div className="content-toolbar">
            <span className="result-count">
              {loading ? 'Loading…' : total === 0 ? 'No results' : (() => {
                const from = (page - 1) * PAGE_SIZE + 1
                const to = Math.min(page * PAGE_SIZE, total)
                const suffix = q ? ` for "${q}"` : ''
                return total <= PAGE_SIZE
                  ? `${fmtNumber(total)} result${total !== 1 ? 's' : ''}${suffix}`
                  : `${from}–${to} of ${fmtNumber(total)} results${suffix}`
              })()}
            </span>
            <div className="toolbar-right">
              <select
                className="sort-select"
                value={sort}
                onChange={e => setFilter({ sort: e.target.value })}
              >
                <option value="updated">Recently updated</option>
                <option value="pulls">Most pulled</option>
                <option value="name">A–Z</option>
              </select>
              <div className="view-toggle">
                <button
                  className={`view-btn ${viewMode === 'list' ? 'active' : ''}`}
                  onClick={() => switchView('list')}
                  title="List view"
                >
                  ☰
                </button>
                <button
                  className={`view-btn ${viewMode === 'grid' ? 'active' : ''}`}
                  onClick={() => switchView('grid')}
                  title="Grid view"
                >
                  ⊞
                </button>
              </div>
            </div>
          </div>

          {/* Active filter chips */}
          {(q || kind || namespace) && (
            <div className="active-filters">
              {q && (
                <span className="filter-chip">
                  Search: &ldquo;{q}&rdquo;
                  <button
                    className="filter-chip-remove"
                    onClick={() => {
                      const next = {}
                      if (kind) next.kind = kind
                      if (namespace) next.namespace = namespace
                      if (sort !== 'updated') next.sort = sort
                      setSearchParams(next)
                    }}
                  >×</button>
                </span>
              )}
              {kind && (
                <span className="filter-chip">
                  Type: {kind}
                  <button className="filter-chip-remove" onClick={() => setFilter({ kind: '' })}>×</button>
                </span>
              )}
              {namespace && (
                <span className="filter-chip">
                  Publisher: {namespace}
                  <button className="filter-chip-remove" onClick={() => setFilter({ namespace: '' })}>×</button>
                </span>
              )}
              <button className="filter-chip-clear" onClick={() => setSearchParams({})}>
                Clear all
              </button>
            </div>
          )}

          {error && <div className="error-state">{error}</div>}

          {loading && viewMode === 'list' && <SkeletonCards count={5} />}
          {loading && viewMode === 'grid' && <SkeletonGridCards count={8} />}

          {!loading && !error && artifacts.length === 0 && (
            <div className="empty-state">
              <h3>No artifacts found</h3>
              <p>Try a different search or publish your first artifact.</p>
            </div>
          )}

          {!loading && viewMode === 'list' && (
            <div className="artifacts-list">
              {artifacts.map(a => (
                <ArtifactCard key={`${a.kind}/${a.namespace}/${a.name}`} artifact={a} />
              ))}
            </div>
          )}

          {!loading && viewMode === 'grid' && (
            <div className="artifacts-grid">
              {artifacts.map(a => (
                <GridCard key={`${a.kind}/${a.namespace}/${a.name}`} artifact={a} />
              ))}
            </div>
          )}

          {totalPages > 1 && (
            <div className="pagination">
              <button
                className="page-btn"
                disabled={page <= 1}
                onClick={() => setFilter({ page: page - 1 })}
              >
                ← Prev
              </button>
              {Array.from({ length: Math.min(totalPages, 7) }, (_, i) => {
                let p = i + 1
                if (totalPages > 7) {
                  const start = Math.max(1, page - 3)
                  p = start + i
                  if (p > totalPages) return null
                }
                return (
                  <button
                    key={p}
                    className={`page-btn ${p === page ? 'active' : ''}`}
                    onClick={() => setFilter({ page: p })}
                  >
                    {p}
                  </button>
                )
              })}
              <button
                className="page-btn"
                disabled={page >= totalPages}
                onClick={() => setFilter({ page: page + 1 })}
              >
                Next →
              </button>
            </div>
          )}
        </main>
      </div>
    </>
  )
}
