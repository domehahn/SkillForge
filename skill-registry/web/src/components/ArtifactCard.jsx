import { useState, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { avatarColor, kindClass, timeAgo, fmtNumber, isNew } from '../utils'
import { isStarred, toggleStar } from '../hooks/useStarred'

export default function ArtifactCard({ artifact, verified = false }) {
  const { kind, namespace, name, description, tags = [], downloads = 0, stars = 0, latest_version, updated_at, created_at } = artifact
  const navigate = useNavigate()
  const color = avatarColor(namespace)
  const installCmd = latest_version
    ? `skforge install ${namespace}/${name}@${latest_version}`
    : `skforge install ${namespace}/${name}`
  const [copied, setCopied] = useState(false)
  const [starred, setStarred] = useState(() => isStarred(kind, namespace, name))
  const fresh = isNew(updated_at || created_at)

  useEffect(() => {
    const sync = () => setStarred(isStarred(kind, namespace, name))
    window.addEventListener('sf_starred', sync)
    return () => window.removeEventListener('sf_starred', sync)
  }, [kind, namespace, name])

  const copyInstall = (e) => {
    e.preventDefault()
    e.stopPropagation()
    navigator.clipboard.writeText(installCmd)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleStar = (e) => {
    e.preventDefault()
    e.stopPropagation()
    const next = toggleStar({ kind, namespace, name, description, latest_version, updated_at })
    setStarred(next)
  }

  return (
    <Link
      to={`/artifacts/${kind}/${namespace}/${name}`}
      className="artifact-card"
    >
      <div className={`artifact-avatar ${kindClass(kind)}`} style={{ background: color }}>
        {namespace[0].toUpperCase()}
      </div>
      <div className="artifact-body">
        <div className="artifact-header">
          <span className="artifact-name">{name}</span>
          <span className="kind-chip">{kind}</span>
          {fresh && <span className="badge-new">New</span>}
        </div>
        <div className="artifact-namespace">
          {namespace}
          {verified && <span className="card-verified-badge" title="Verified publisher">✓</span>}
          {latest_version ? (
            <span className="artifact-version"> · v{latest_version}</span>
          ) : null}
        </div>
        {description && <div className="artifact-desc">{description}</div>}
        <div className="artifact-footer">
          <div className="artifact-tags">
            {(tags || []).slice(0, 4).map(t => (
              <span
                key={t}
                className="tag tag-clickable"
                onClick={e => { e.preventDefault(); navigate(`/search?q=${encodeURIComponent(t)}`) }}
              >{t}</span>
            ))}
          </div>
          <div className="artifact-meta">
            {downloads > 0 && (
              <span className="meta-stat meta-pulls">⬇ {fmtNumber(downloads)}</span>
            )}
            {updated_at && <span className="meta-stat">{timeAgo(updated_at)}</span>}
            <button
              className={`card-star-btn ${starred ? 'starred' : ''}`}
              onClick={handleStar}
              title={starred ? 'Remove star' : 'Star this artifact'}
            >
              {starred ? '★' : '☆'}{stars > 0 ? ` ${fmtNumber(stars)}` : ''}
            </button>
            <button
              className={`card-copy-btn ${copied ? 'copied' : ''}`}
              onClick={copyInstall}
              title={installCmd}
            >
              {copied ? '✓' : '⎘'}
            </button>
          </div>
        </div>
      </div>
    </Link>
  )
}
