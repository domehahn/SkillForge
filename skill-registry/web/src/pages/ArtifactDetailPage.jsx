import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { fetchArtifactDetails, fetchArtifactGraph } from '../api/client'

function ArtifactDetailPage() {
  const { kind, namespace, name } = useParams()
  const [data, setData] = useState(null)
  const [graph, setGraph] = useState(null)
  const [error, setError] = useState(null)

  useEffect(() => {
    fetchArtifactDetails(kind, namespace, name).then(setData).catch((err) => setError(err.message))
    fetchArtifactGraph(kind, namespace, name).then(setGraph).catch(() => {})
  }, [kind, namespace, name])

  if (error) return <div className="container"><div className="error">{error}</div></div>
  if (!data) return <div className="container"><div className="loading">Loading artifact...</div></div>

  const { artifact, versions, dist_tags: distTags } = data
  const safeVersions = versions || []
  const graphEdges = graph?.edges || []
  return (
    <div className="container">
      <Link to="/" className="back-button">← Back to artifacts</Link>
      <div className="skill-detail">
        <div className={`kind-badge kind-${artifact.kind}`}>{artifact.kind}</div>
        <h2>{artifact.name}</h2>
        <div className="meta">{artifact.namespace} · {artifact.visibility} · {artifact.downloads || 0} pulls</div>
        <p>{artifact.description}</p>
        <div className="registry-facts">
          {Object.entries(distTags || {}).map(([tag, version]) => <span key={tag} className="dist-tag">{tag}: {version}</span>)}
        </div>
        <div className="versions-list">
          <h3>Versions</h3>
          {safeVersions.map((version) => (
            <div key={version.id} className="version-item">
              <div className="version-info">
                <div className="version-number">{version.version}</div>
                <div className="version-meta">{version.package_type} · {version.downloads || 0} pulls · signature: {version.signature_status} · scan: {version.scan_status}</div>
                <div className="version-meta">{version.digest_sha256}</div>
              </div>
              <a className="download-btn" href={`/api/v1/artifacts/${kind}/${namespace}/${name}/versions/${version.version}/download`}>Download</a>
            </div>
          ))}
        </div>
        {graph && (
          <div className="dependency-graph">
            <h3>Dependency Graph</h3>
            {graphEdges.length === 0 ? <p>No dependencies</p> : graphEdges.map((edge) => <div key={`${edge.from}-${edge.to}`}><code>{edge.from}</code> → <code>{edge.to}</code></div>)}
          </div>
        )}
      </div>
    </div>
  )
}

export default ArtifactDetailPage
