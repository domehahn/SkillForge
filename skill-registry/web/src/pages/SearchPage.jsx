import { useState, useEffect, useCallback } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { fetchArtifacts, fetchFacets } from '../api/client'
import ArtifactCard from '../components/ArtifactCard'
import { useDocumentTitle } from '../hooks/useDocumentTitle'
import { fmtNumber } from '../utils'

const KINDS = ['skill', 'agent', 'flow', 'prompt', 'tool', 'bundle']
const SORTS = [
  { value: '', label: 'Most recent' },
  { value: 'pulls', label: 'Most downloaded' },
  { value: 'stars', label: 'Most starred' },
  { value: 'name', label: 'Name A–Z' },
]
const PAGE_SIZE = 20

export default function SearchPage() {
  const [params, setParams] = useSearchParams()
  const q = params.get('q') || ''
  const kind = params.get('kind') || ''
  const sort = params.get('sort') || ''
  const verified = params.get('verified') === 'true'
  const page = parseInt(params.get('page') || '1', 10)
  const offset = (page - 1) * PAGE_SIZE

  useDocumentTitle(q ? `"${q}" — Search` : 'Search')

  const [results, setResults] = useState([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [facets, setFacets] = useState({})
  const [inputVal, setInputVal] = useState(q)

  const setP = useCallback((updates) => {
    const next = new URLSearchParams(params)
    Object.entries(updates).forEach(([k, v]) => {
      if (v === '' || v === false || v == null) next.delete(k)
      else next.set(k, String(v))
    })
    next.delete('page')
    setParams(next)
  }, [params, setParams])

  useEffect(() => { setInputVal(q) }, [q])

  useEffect(() => {
    setLoading(true)
    fetchArtifacts({ q, kind, sort, verified, limit: PAGE_SIZE, offset })
      .then(d => { setResults(d.artifacts || []); setTotal(d.total || 0) })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [q, kind, sort, verified, offset])

  useEffect(() => {
    fetchFacets(q).then(setFacets).catch(() => {})
  }, [q])

  const totalPages = Math.ceil(total / PAGE_SIZE)

  const setPage = (p) => {
    const next = new URLSearchParams(params)
    if (p <= 1) next.delete('page')
    else next.set('page', String(p))
    setParams(next)
    window.scrollTo(0, 0)
  }

  return (
    <div className="search-page">
      {/* Search bar */}
      <div className="search-page-hero">
        <form
          className="search-page-form"
          onSubmit={e => { e.preventDefault(); setP({ q: inputVal }) }}
        >
          <input
            className="search-page-input"
            value={inputVal}
            onChange={e => setInputVal(e.target.value)}
            placeholder="Search skills, agents, flows, prompts…"
            autoFocus
          />
          <button className="btn-primary search-page-submit" type="submit">Search</button>
        </form>
      </div>

      <div className="search-page-body">
        {/* Sidebar filters */}
        <aside className="search-sidebar">
          <div className="search-filter-group">
            <div className="search-filter-label">Type</div>
            <button
              className={`search-filter-btn ${!kind ? 'active' : ''}`}
              onClick={() => setP({ kind: '' })}
            >
              All <span className="sf-count">{Object.values(facets).reduce((s, v) => s + v, 0)}</span>
            </button>
            {KINDS.map(k => (
              <button
                key={k}
                className={`search-filter-btn ${kind === k ? 'active' : ''}`}
                onClick={() => setP({ kind: k })}
              >
                {k} {facets[k] != null && <span className="sf-count">{facets[k]}</span>}
              </button>
            ))}
          </div>

          <div className="search-filter-group">
            <div className="search-filter-label">Sort by</div>
            {SORTS.map(s => (
              <button
                key={s.value}
                className={`search-filter-btn ${sort === s.value ? 'active' : ''}`}
                onClick={() => setP({ sort: s.value })}
              >
                {s.label}
              </button>
            ))}
          </div>

          <div className="search-filter-group">
            <div className="search-filter-label">Publisher</div>
            <label className="search-filter-check">
              <input
                type="checkbox"
                checked={verified}
                onChange={e => setP({ verified: e.target.checked || '' })}
              />
              <span>Verified only</span>
              <span className="verified-icon" title="Verified publisher">✓</span>
            </label>
          </div>
        </aside>

        {/* Results */}
        <div className="search-results">
          <div className="search-results-header">
            {loading ? (
              <span className="search-results-count">Searching…</span>
            ) : (
              <span className="search-results-count">
                {fmtNumber(total)} result{total !== 1 ? 's' : ''}
                {q && <> for <strong>"{q}"</strong></>}
              </span>
            )}
          </div>

          {!loading && results.length === 0 ? (
            <div className="empty-state">
              <h3>No results found</h3>
              <p>Try different keywords, remove filters, or browse the <Link to="/explore">Explore</Link> page.</p>
            </div>
          ) : (
            <div className="search-results-grid">
              {results.map(a => (
                <ArtifactCard
                  key={`${a.kind}/${a.namespace}/${a.name}`}
                  kind={a.kind}
                  namespace={a.namespace}
                  name={a.name}
                  description={a.description}
                  latest_version={a.latest_version}
                  updated_at={a.updated_at}
                  downloads={a.downloads}
                  stars={a.stars}
                />
              ))}
            </div>
          )}

          {totalPages > 1 && (
            <div className="search-pagination">
              <button
                className="btn-secondary"
                disabled={page <= 1}
                onClick={() => setPage(page - 1)}
              >← Prev</button>
              <span className="search-page-info">Page {page} of {totalPages}</span>
              <button
                className="btn-secondary"
                disabled={page >= totalPages}
                onClick={() => setPage(page + 1)}
              >Next →</button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
