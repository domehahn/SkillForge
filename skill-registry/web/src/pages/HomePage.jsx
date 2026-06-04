import { useState, useEffect } from 'react'
import { fetchSkills } from '../api/client'
import SkillCard from '../components/SkillCard'

function HomePage() {
  const [skills, setSkills] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [searchQuery, setSearchQuery] = useState('')

  useEffect(() => {
    loadSkills()
  }, [])

  const loadSkills = async (query = '') => {
    setLoading(true)
    setError(null)
    try {
      const data = await fetchSkills({ q: query, limit: 100 })
      setSkills(data.skills || [])
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
          <div className="number">{skills.length}</div>
          <div className="label">Skills Available</div>
        </div>
        <div className="stat-card">
          <div className="number">
            {skills.reduce((sum, skill) => sum + (skill.versions?.length || 0), 0)}
          </div>
          <div className="label">Total Versions</div>
        </div>
      </div>

      <form onSubmit={handleSearch} className="search-bar">
        <input
          type="text"
          placeholder="Search skills..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
        />
        <button type="submit">Search</button>
      </form>

      {loading && <div className="loading">Loading skills...</div>}
      
      {error && (
        <div className="error">
          <strong>Error:</strong> {error}
        </div>
      )}

      {!loading && !error && skills.length === 0 && (
        <div className="empty">
          <h3>No skills found</h3>
          <p>Try a different search query or publish your first skill</p>
        </div>
      )}

      {!loading && !error && skills.length > 0 && (
        <div className="skills-grid">
          {skills.map((skill) => (
            <SkillCard key={`${skill.namespace}/${skill.name}`} skill={skill} />
          ))}
        </div>
      )}
    </div>
  )
}

export default HomePage
