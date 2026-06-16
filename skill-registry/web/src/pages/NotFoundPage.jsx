import { useState, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { fetchArtifacts } from '../api/client'
import { avatarColor, kindClass, fmtNumber } from '../utils'
import { useDocumentTitle } from '../hooks/useDocumentTitle'

export default function NotFoundPage() {
  useDocumentTitle('Page not found')
  const navigate = useNavigate()
  const [q, setQ] = useState('')
  const [popular, setPopular] = useState([])

  useEffect(() => {
    fetchArtifacts({ sort: 'pulls', limit: 6, offset: 0 })
      .then(d => setPopular(d.artifacts || []))
      .catch(() => {})
  }, [])

  const handleSearch = (e) => {
    e.preventDefault()
    if (q.trim()) navigate(`/?q=${encodeURIComponent(q.trim())}`)
    else navigate('/')
  }

  return (
    <div className="not-found-page">
      <div className="not-found-code">404</div>
      <h1 className="not-found-title">Page not found</h1>
      <p className="not-found-desc">
        The page you're looking for doesn't exist or has been moved.
      </p>

      <form className="not-found-search" onSubmit={handleSearch}>
        <input
          value={q}
          onChange={e => setQ(e.target.value)}
          placeholder="Search skills, agents, tools…"
          autoFocus
        />
        <button type="submit" className="btn-primary">Search</button>
      </form>

      {popular.length > 0 && (
        <div className="not-found-popular">
          <div className="not-found-popular-title">Popular artifacts</div>
          <div className="not-found-popular-grid">
            {popular.map(a => (
              <Link
                key={`${a.kind}/${a.namespace}/${a.name}`}
                to={`/artifacts/${a.kind}/${a.namespace}/${a.name}`}
                className="not-found-artifact-card"
              >
                <div className={`not-found-avatar ${kindClass(a.kind)}`} style={{ background: avatarColor(a.namespace) }}>
                  {a.namespace[0].toUpperCase()}
                </div>
                <div className="not-found-card-body">
                  <div className="not-found-card-name">{a.name}</div>
                  <div className="not-found-card-ns">{a.namespace}</div>
                  {a.downloads > 0 && (
                    <div className="not-found-card-pulls">⬇ {fmtNumber(a.downloads)}</div>
                  )}
                </div>
              </Link>
            ))}
          </div>
        </div>
      )}

      <div className="not-found-actions" style={{ marginTop: 24 }}>
        <Link to="/" className="btn-secondary">← Back to Explore</Link>
      </div>
    </div>
  )
}
