import { Link } from 'react-router-dom'
import { useState, useEffect } from 'react'
import { fetchMetadata } from '../api/client'

function Header() {
  const [metadata, setMetadata] = useState(null)

  useEffect(() => {
    fetchMetadata()
      .then(setMetadata)
      .catch(err => console.error('Failed to load metadata:', err))
  }, [])

  return (
    <header>
      <div className="container">
        <div>
          <Link to="/" style={{ textDecoration: 'none', color: 'white' }}>
            <h1>🎯 Skill Registry</h1>
          </Link>
          {metadata && (
            <div style={{ fontSize: '14px', marginTop: '4px', opacity: 0.9 }}>
              {metadata.name} v{metadata.version}
            </div>
          )}
        </div>
        <nav>
          <Link to="/">Browse</Link>
          <a href="/api/v1/skills" target="_blank" rel="noopener noreferrer">API</a>
        </nav>
      </div>
    </header>
  )
}

export default Header
