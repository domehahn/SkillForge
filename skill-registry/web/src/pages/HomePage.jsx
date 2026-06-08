import { useState, useEffect } from 'react'
import { fetchArtifacts } from '../api/client'
import ArtifactCard from '../components/ArtifactCard'

function HomePage() {
  const [artifacts, setArtifacts] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [kind, setKind] = useState('')

  useEffect(() => {
    loadSkills()
  }, [])

  const loadSkills = async (query = '') => {
    setLoading(true)
    setError(null)
    try {
      const data = await fetchArtifacts({ q: query, kind, limit: 100 })
      setArtifacts(data.artifacts || [])
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleSearch = (e) => {
    e.preventDefault()
    loadSkills(searchQuery)
  }

  return (
    <div className="container">
      <div className="stats">
        <div className="stat-card">
          <div className="number">{artifacts.length}</div>
          <div className="label">Artifacts Available</div>
        </div>
        <div className="stat-card">
          <div className="number">
            {artifacts.reduce((sum, artifact) => sum + (artifact.downloads || 0), 0)}
          </div>
          <div className="label">Total Pulls</div>
        </div>
      </div>

      <form onSubmit={handleSearch} className="search-bar">
        <input
          type="text"
          placeholder="Search skills..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
        />
        <select value={kind} onChange={(e) => setKind(e.target.value)}>
          <option value="">All kinds</option>
          {['skill', 'agent', 'flow', 'prompt', 'tool', 'bundle'].map((value) => <option key={value} value={value}>{value}</option>)}
        </select>
        <button type="submit">Search</button>
      </form>

      {loading && <div className="loading">Loading skills...</div>}
      
      {error && (
        <div className="error">
          <strong>Error:</strong> {error}
        </div>
      )}

      {!loading && !error && artifacts.length === 0 && (
        <div className="empty">
          <h3>No artifacts found</h3>
          <p>Try a different search query or publish your first artifact</p>
        </div>
      )}

      {!loading && !error && artifacts.length > 0 && (
        <div className="skills-grid">
          {artifacts.map((artifact) => (
            <ArtifactCard key={`${artifact.kind}/${artifact.namespace}/${artifact.name}`} artifact={artifact} />
          ))}
        </div>
      )}
    </div>
  )
}

export default HomePage
