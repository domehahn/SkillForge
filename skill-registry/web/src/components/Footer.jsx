import { Link } from 'react-router-dom'

export default function Footer() {
  return (
    <footer className="site-footer">
      <div className="footer-inner">
        <div className="footer-brand">
          <Link to="/" className="footer-logo">
            <div className="footer-logo-icon">⚡</div>
            <span>SkillForge</span>
          </Link>
          <p className="footer-tagline">
            The open registry for AI skills, agents, and tools.
          </p>
        </div>
        <div className="footer-links">
          <div className="footer-col">
            <div className="footer-col-title">Registry</div>
            <Link to="/">Explore all</Link>
            <Link to="/categories">Categories</Link>
            <Link to="/trending">Trending</Link>
            <Link to="/?kind=skill">Skills</Link>
            <Link to="/?kind=agent">Agents</Link>
            <Link to="/?kind=tool">Tools</Link>
            <Link to="/?kind=bundle">Bundles</Link>
          </div>
          <div className="footer-col">
            <div className="footer-col-title">Developers</div>
            <Link to="/install">Install CLI</Link>
            <a href="/api-docs" target="_blank" rel="noopener noreferrer">API Docs</a>
            <a href="/api/v1/openapi.yaml" target="_blank" rel="noopener noreferrer">OpenAPI Spec</a>
          </div>
          <div className="footer-col">
            <div className="footer-col-title">Account</div>
            <Link to="/login">Sign in</Link>
            <Link to="/account/tokens">Access Tokens</Link>
            <Link to="/account/security">Security</Link>
            <Link to="/account/notifications">Notifications</Link>
          </div>
        </div>
      </div>
      <div className="footer-bottom">
        <span>© {new Date().getFullYear()} SkillForge</span>
        <span className="footer-sep">·</span>
        <a href="/api-docs">API</a>
        <span className="footer-sep">·</span>
        <a href="/api/v1/openapi.yaml">OpenAPI Spec</a>
      </div>
    </footer>
  )
}
