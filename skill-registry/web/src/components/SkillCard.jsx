import { Link } from 'react-router-dom'

function SkillCard({ skill }) {
  return (
    <Link 
      to={`/skills/${skill.namespace}/${skill.name}`} 
      style={{ textDecoration: 'none', color: 'inherit' }}
    >
      <div className="skill-card">
        <h3>{skill.name}</h3>
        <div className="namespace">{skill.namespace}</div>
        {skill.description && <p>{skill.description}</p>}
        
        {skill.tags && skill.tags.length > 0 && (
          <div className="tags">
            {skill.tags.map((tag, i) => (
              <span key={i} className="tag">{tag}</span>
            ))}
          </div>
        )}
        
        <div className="version">
          Latest: {skill.latest_version || 'N/A'} · {skill.downloads || 0} pulls
        </div>
      </div>
    </Link>
  )
}

export default SkillCard
