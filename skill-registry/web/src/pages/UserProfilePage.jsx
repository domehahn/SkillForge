import { useState, useEffect } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { fetchNamespace, fetchArtifacts } from '../api/client'
import ArtifactCard from '../components/ArtifactCard'
import { avatarColor, timeAgo, fmtNumber } from '../utils'
import { useDocumentTitle } from '../hooks/useDocumentTitle'
import { getStarred } from '../hooks/useStarred'

export default function UserProfilePage() {
  const { username } = useParams()
  useDocumentTitle(username)
  const navigate = useNavigate()

  const loggedInUser = localStorage.getItem('sf_user')
  const isSelf = loggedInUser === username
  const color = avatarColor(username)

  const [nsData, setNsData] = useState(null)
  const [artifacts, setArtifacts] = useState([])
  const [loading, setLoading] = useState(true)
  const [tab, setTab] = useState('artifacts')
  const starredItems = getStarred ? getStarred() : []

  useEffect(() => {
    Promise.all([
      fetchNamespace(username),
      fetchArtifacts({ namespace: username, sort: 'pulls', limit: 30 }),
    ])
      .then(([ns, arts]) => {
        setNsData(ns)
        setArtifacts(arts.artifacts || [])
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [username])

  if (loading) return <div className="loading-state">Loading…</div>

  const ns = nsData
  const displayName = ns?.display_name || username

  return (
    <div className="user-profile-page">
      {/* Profile header */}
      <div className="user-profile-header">
        <div className="user-profile-avatar" style={{ background: color }}>
          {username[0].toUpperCase()}
        </div>
        <div className="user-profile-info">
          <div className="user-profile-name-row">
            <h1 className="user-profile-name">{displayName}</h1>
            {ns?.verified && <span className="ns-verified-badge" title="Verified publisher">✓</span>}
          </div>
          <div className="user-profile-username">@{username}</div>
          {ns?.bio && <p className="user-profile-bio">{ns.bio}</p>}
          <div className="user-profile-stats">
            <span><strong>{fmtNumber(ns?.artifact_count || 0)}</strong> artifacts</span>
            <span className="ns-stat-sep">·</span>
            <span><strong>{fmtNumber(ns?.total_pulls || 0)}</strong> total pulls</span>
            {ns?.member_count > 0 && (
              <>
                <span className="ns-stat-sep">·</span>
                <span><strong>{fmtNumber(ns.member_count)}</strong> members</span>
              </>
            )}
          </div>
        </div>
        <div className="user-profile-actions">
          {isSelf ? (
            <Link to="/account/settings" className="btn-secondary">Edit profile</Link>
          ) : (
            <Link to={`/namespace/${username}`} className="btn-secondary">View namespace</Link>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="tabs user-profile-tabs">
        <button className={`tab-btn ${tab === 'artifacts' ? 'active' : ''}`} onClick={() => setTab('artifacts')}>
          Artifacts
          <span className="tab-count">{artifacts.length}</span>
        </button>
        {isSelf && (
          <button className={`tab-btn ${tab === 'starred' ? 'active' : ''}`} onClick={() => setTab('starred')}>
            Starred
            <span className="tab-count">{starredItems.length}</span>
          </button>
        )}
        <button className={`tab-btn ${tab === 'collections' ? 'active' : ''}`} onClick={() => navigate(`/namespace/${username}/collections`)}>
          Collections ↗
        </button>
      </div>

      {/* Tab panels */}
      {tab === 'artifacts' && (
        artifacts.length === 0 ? (
          <div className="empty-state">
            <h3>No published artifacts</h3>
            {isSelf && <p>Publish your first artifact with <code>skforge publish</code>.</p>}
          </div>
        ) : (
          <div className="artifact-grid">
            {artifacts.map(a => (
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
        )
      )}

      {tab === 'starred' && isSelf && (
        starredItems.length === 0 ? (
          <div className="empty-state">
            <h3>No starred artifacts</h3>
            <p>Star artifacts you want to come back to — the ★ button appears on every artifact page.</p>
          </div>
        ) : (
          <div className="artifact-grid">
            {starredItems.map(a => (
              <ArtifactCard
                key={`${a.kind}/${a.namespace}/${a.name}`}
                kind={a.kind}
                namespace={a.namespace}
                name={a.name}
                description={a.description}
                latest_version={a.latest_version}
                updated_at={a.updated_at}
              />
            ))}
          </div>
        )
      )}
    </div>
  )
}
