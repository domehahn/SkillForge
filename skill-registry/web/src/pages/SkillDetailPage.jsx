import { useState, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import { fetchSkillDetails, getDownloadUrl } from '../api/client'

function SkillDetailPage() {
  const { namespace, name } = useParams()
  const [data, setData] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  useEffect(() => {
    loadSkillDetails()
  }, [namespace, name])

  const loadSkillDetails = async () => {
    setLoading(true)
    setError(null)
    try {
      const result = await fetchSkillDetails(namespace, name)
      setData(result)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  const formatDate = (dateString) => {
    return new Date(dateString).toLocaleDateString('de-DE', {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    })
  }

  const formatBytes = (bytes) => {
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  }

  if (loading) {
    return (
      <div className="container">
        <div className="loading">Loading skill details...</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="container">
        <Link to="/" className="back-button">← Back to skills</Link>
        <div className="error">
          <strong>Error:</strong> {error}
        </div>
      </div>
    )
  }

  const { skill, versions, dist_tags: distTags } = data
  const safeVersions = versions || []
  const preferredVersion = safeVersions[0]
  const compatibleWith = preferredVersion?.manifest?.compatible_with || []
  const installConstraint = preferredVersion ? `^${preferredVersion.version}` : '^1.0.0'

  return (
    <div className="container">
      <Link to="/" className="back-button">← Back to skills</Link>
      
      <div className="skill-detail">
        <h2>{skill.name}</h2>
        <div className="meta">
          <span>{skill.namespace}</span>
          {skill.owners && skill.owners.length > 0 && (
            <span> • Owners: {skill.owners.join(', ')}</span>
          )}
        </div>
        
        {skill.description && <p style={{ marginTop: '20px', fontSize: '16px', lineHeight: '1.6' }}>{skill.description}</p>}
        
        {skill.tags && skill.tags.length > 0 && (
          <div className="tags" style={{ marginTop: '20px' }}>
            {skill.tags.map((tag, i) => (
              <span key={i} className="tag">{tag}</span>
            ))}
          </div>
        )}

        <div className="registry-facts">
          <span><strong>{skill.downloads || 0}</strong> pulls</span>
          {Object.entries(distTags || {}).map(([tag, version]) => (
            <span key={tag} className="dist-tag">{tag}: {version}</span>
          ))}
        </div>

        <div className="install-panel">
          <h3>Production install</h3>
          <p>Use a lockfile for production. <code>latest</code> is only for local experiments.</p>
          <pre>{`skforge add ${namespace}/${name}@${installConstraint}
skforge lock
skforge install --frozen-lockfile
skforge verify`}</pre>
        </div>

        <div className="versions-list">
          <h3>Versions ({safeVersions.length})</h3>
          {compatibleWith.length > 0 && (
            <div className="version-meta" style={{ marginBottom: '12px' }}>
              Compatible with: {compatibleWith.join(', ')}
            </div>
          )}
          {safeVersions.length > 0 ? (
            safeVersions.map((version) => (
              <div key={version.id} className="version-item">
                <div className="version-info">
                  <div className="version-number">
                    v{version.version}
                    {version.deprecated && <span style={{ color: '#c33', marginLeft: '10px' }}>(deprecated)</span>}
                    {version.yanked && <span style={{ color: '#c33', marginLeft: '10px' }}>(yanked)</span>}
                  </div>
                  <div className="version-meta">
                    {formatBytes(version.size_bytes)} • 
                    {version.package_type} • 
                    {version.downloads || 0} pulls •
                    validation: {version.validation_status || 'unknown'} •
                    signature: {version.signature_status || 'unsigned'} •
                    Published {formatDate(version.created_at)}
                    {version.created_by && ` by ${version.created_by}`}
                  </div>
                  {version.deprecation_reason && (
                    <div className="version-meta" style={{ marginTop: '4px', color: '#c33' }}>
                      Deprecation reason: {version.deprecation_reason}
                    </div>
                  )}
                  {version.yank_reason && (
                    <div className="version-meta" style={{ marginTop: '4px', color: '#c33' }}>
                      Yank reason: {version.yank_reason}
                    </div>
                  )}
                  {version.digest_sha256 && (
                    <div className="version-meta" style={{ marginTop: '4px', fontFamily: 'monospace', fontSize: '11px' }}>
                      SHA256: {version.digest_sha256.substring(0, 16)}...
                    </div>
                  )}
                  {version.manifest?.compatible_with?.length > 0 && (
                    <div className="version-meta" style={{ marginTop: '4px' }}>
                      Platforms: {version.manifest.compatible_with.join(', ')}
                    </div>
                  )}
                  <div className="version-meta" style={{ marginTop: '4px', fontFamily: 'monospace', fontSize: '12px' }}>
                    skforge install {namespace}/{name}@{version.version} --target .agents/skills/{name}
                  </div>
                </div>
                <a
                  href={getDownloadUrl(namespace, name, version.version)}
                  className="download-btn"
                  download
                >
                  Download
                </a>
              </div>
            ))
          ) : (
            <p style={{ color: '#999' }}>No versions available</p>
          )}
        </div>
      </div>
    </div>
  )
}

export default SkillDetailPage
