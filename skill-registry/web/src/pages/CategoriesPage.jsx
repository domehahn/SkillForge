import { Link, useNavigate } from 'react-router-dom'
import { useDocumentTitle } from '../hooks/useDocumentTitle'

const CATEGORIES = [
  {
    slug: 'language-models',
    name: 'Language Models',
    icon: '🧠',
    desc: 'LLM wrappers, prompt chains, and fine-tuned model interfaces',
    tags: ['llm', 'gpt', 'claude', 'ollama', 'openai', 'language-model'],
    color: '#6366f1',
  },
  {
    slug: 'code-generation',
    name: 'Code Generation',
    icon: '💻',
    desc: 'Skills that write, review, or refactor code automatically',
    tags: ['code', 'coding', 'codegen', 'programming', 'refactor'],
    color: '#0ea5e9',
  },
  {
    slug: 'data-analysis',
    name: 'Data & Analytics',
    icon: '📊',
    desc: 'Data processing, chart generation, and statistical analysis skills',
    tags: ['data', 'analytics', 'csv', 'sql', 'pandas', 'visualization'],
    color: '#10b981',
  },
  {
    slug: 'automation',
    name: 'Automation',
    icon: '⚙️',
    desc: 'Workflow automation, task scheduling, and process orchestration',
    tags: ['automation', 'workflow', 'schedule', 'pipeline', 'rpa'],
    color: '#f59e0b',
  },
  {
    slug: 'web-scraping',
    name: 'Web & Scraping',
    icon: '🌐',
    desc: 'Browser automation, web scraping, and API interaction skills',
    tags: ['web', 'scraping', 'browser', 'http', 'api', 'crawl'],
    color: '#8b5cf6',
  },
  {
    slug: 'productivity',
    name: 'Productivity',
    icon: '✅',
    desc: 'Email, calendar, document, and personal productivity assistants',
    tags: ['productivity', 'email', 'calendar', 'documents', 'notes'],
    color: '#ef4444',
  },
  {
    slug: 'media',
    name: 'Media & Creative',
    icon: '🎨',
    desc: 'Image generation, audio processing, and creative content skills',
    tags: ['image', 'audio', 'video', 'creative', 'art', 'dalle', 'stable-diffusion'],
    color: '#ec4899',
  },
  {
    slug: 'security',
    name: 'Security',
    icon: '🔒',
    desc: 'Security scanning, vulnerability detection, and compliance tools',
    tags: ['security', 'vulnerability', 'pentest', 'compliance', 'audit'],
    color: '#64748b',
  },
  {
    slug: 'devops',
    name: 'DevOps & Cloud',
    icon: '☁️',
    desc: 'Infrastructure, deployment, monitoring, and cloud management',
    tags: ['devops', 'docker', 'kubernetes', 'terraform', 'cloud', 'ci'],
    color: '#0ea5e9',
  },
  {
    slug: 'nlp',
    name: 'NLP & Text',
    icon: '📝',
    desc: 'Text classification, summarization, translation, and extraction',
    tags: ['nlp', 'text', 'summarize', 'translate', 'classify', 'sentiment'],
    color: '#22c55e',
  },
  {
    slug: 'agents',
    name: 'Autonomous Agents',
    icon: '🤖',
    desc: 'Multi-step reasoning agents with tool use and memory',
    tags: ['agent', 'autonomous', 'reasoning', 'react', 'tool-use'],
    color: '#f97316',
  },
  {
    slug: 'integrations',
    name: 'Integrations',
    icon: '🔌',
    desc: 'Connectors for Slack, GitHub, Notion, Jira, and more',
    tags: ['slack', 'github', 'notion', 'jira', 'integration', 'connector'],
    color: '#a855f7',
  },
]

export default function CategoriesPage() {
  useDocumentTitle('Categories — SkillForge')
  const navigate = useNavigate()

  return (
    <div className="categories-page">
      <div className="categories-hero">
        <h1 className="categories-hero-title">Browse by category</h1>
        <p className="categories-hero-sub">Discover AI skills and agents organized by use case</p>
      </div>

      <div className="categories-grid">
        {CATEGORIES.map(cat => (
          <button
            key={cat.slug}
            className="category-card"
            onClick={() => navigate(`/search?q=${cat.tags[0]}`)}
          >
            <div className="category-card-icon" style={{ background: cat.color + '18', color: cat.color }}>
              {cat.icon}
            </div>
            <div className="category-card-body">
              <div className="category-card-name">{cat.name}</div>
              <div className="category-card-desc">{cat.desc}</div>
              <div className="category-card-tags">
                {cat.tags.slice(0, 3).map(t => (
                  <span key={t} className="tag">{t}</span>
                ))}
              </div>
            </div>
          </button>
        ))}
      </div>

      <div className="categories-footer">
        <p>Can't find what you're looking for? <Link to="/search">Search the full registry →</Link></p>
      </div>
    </div>
  )
}
