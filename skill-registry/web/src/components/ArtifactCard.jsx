import { Link } from 'react-router-dom'

function ArtifactCard({ artifact }) {
  return (
    <Link to={`/artifacts/${artifact.kind}/${artifact.namespace}/${artifact.name}`} style={{ textDecoration: 'none', color: 'inherit' }}>
      <div className="skill-card">
        <div className={`kind-badge kind-${artifact.kind}`}>{artifact.kind}</div>
        <h3>{artifact.name}</h3>
        <div className="namespace">{artifact.namespace}</div>
        {artifact.description && <p>{artifact.description}</p>}
        <div className="tags">
          {(artifact.tags || []).map((tag) => <span key={tag} className="tag">{tag}</span>)}
        </div>
        <div className="version">Latest: {artifact.latest_version || 'N/A'} · {artifact.downloads || 0} pulls</div>
      </div>
    </Link>
  )
}

export default ArtifactCard
