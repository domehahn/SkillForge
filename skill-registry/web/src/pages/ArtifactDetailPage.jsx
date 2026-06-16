import { useEffect, useState, useRef } from 'react'
import { Link, useParams, useSearchParams, useNavigate } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import {
  fetchArtifactDetails, fetchArtifacts, fetchNamespace, fetchPromotions,
  fetchAttestations, fetchArtifactGraph, fetchLockfile,
  yankVersion, unyankVersion, deprecateVersion, setDistTag, deleteVersion,
  patchArtifact, fetchArtifactStats, patchArtifactVersion, fetchArtifactDependents,
  fetchArtifactStarInfo, starArtifact, unstarArtifact,
  fetchScanResults, fetchComments, addComment, updateComment, deleteComment,
  transferArtifact, fetchCollections, addToCollection, removeFromCollection,
} from '../api/client'
import { avatarColor, kindClass, timeAgo, fmtNumber } from '../utils'
import { SkeletonDetail } from '../components/Skeleton'
import { useDocumentTitle } from '../hooks/useDocumentTitle'
import { useMetaTags } from '../hooks/useMetaTags'
import { trackRecentlyViewed } from '../hooks/useRecentlyViewed'
import { isStarred, toggleStar } from '../hooks/useStarred'
import { useToast } from '../contexts/ToastContext'
import { useConfirm } from '../contexts/ConfirmContext'

const SHORTCUTS = [
  { key: '/', desc: 'Focus search' },
  { key: '?', desc: 'Show shortcuts' },
  { key: 'Esc', desc: 'Close modal / clear' },
  { key: 'g o', desc: 'Go to Overview tab' },
  { key: 'g t', desc: 'Go to Tags tab' },
  { key: 'g d', desc: 'Go to Dependencies tab' },
  { key: 'g a', desc: 'Go to Activity tab' },
  { key: 'g s', desc: 'Go to Settings tab (owners)' },
  { key: 'c', desc: 'Copy install command' },
  { key: '↑ ↓', desc: 'Navigate suggestions' },
  { key: 'Enter', desc: 'Select suggestion' },
]

function ShortcutsModal({ onClose }) {
  useEffect(() => {
    const h = (e) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', h)
    return () => document.removeEventListener('keydown', h)
  }, [onClose])
  return (
    <div className="shortcuts-overlay" onClick={onClose}>
      <div className="shortcuts-modal" onClick={e => e.stopPropagation()}>
        <div className="shortcuts-header">
          <span className="shortcuts-title">Keyboard shortcuts</span>
          <button className="shortcuts-close" onClick={onClose}>✕</button>
        </div>
        <div className="shortcuts-grid">
          {SHORTCUTS.map(s => (
            <div key={s.key} className="shortcut-row">
              <kbd className="shortcut-key">{s.key}</kbd>
              <span className="shortcut-desc">{s.desc}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

function BadgeModal({ kind, namespace, name, onClose }) {
  useEffect(() => {
    const h = (e) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', h)
    return () => document.removeEventListener('keydown', h)
  }, [onClose])

  const label = `${namespace}/${name}`
  const color = 'blueviolet'
  const badgeUrl = `https://img.shields.io/badge/SkillForge-${encodeURIComponent(label)}-${color}?logo=lightning`
  const pageUrl = `${window.location.origin}/artifacts/${kind}/${namespace}/${name}`
  const markdown = `[![SkillForge](${badgeUrl})](${pageUrl})`
  const html = `<a href="${pageUrl}"><img src="${badgeUrl}" alt="SkillForge" /></a>`
  const [copiedMd, setCopiedMd] = useState(false)
  const [copiedHtml, setCopiedHtml] = useState(false)

  const copyMd = () => { navigator.clipboard.writeText(markdown); setCopiedMd(true); setTimeout(() => setCopiedMd(false), 2000) }
  const copyHtml = () => { navigator.clipboard.writeText(html); setCopiedHtml(true); setTimeout(() => setCopiedHtml(false), 2000) }

  return (
    <div className="shortcuts-overlay" onClick={onClose}>
      <div className="badge-modal" onClick={e => e.stopPropagation()}>
        <div className="shortcuts-header">
          <span className="shortcuts-title">Embed Badge</span>
          <button className="shortcuts-close" onClick={onClose}>✕</button>
        </div>
        <div className="badge-modal-body">
          <div className="badge-preview">
            <img src={badgeUrl} alt="SkillForge badge" />
          </div>
          <div className="badge-snippet-label">Markdown</div>
          <div className="badge-snippet-row">
            <code className="badge-snippet-code">{markdown}</code>
            <button className={`copy-btn ${copiedMd ? 'copied' : ''}`} onClick={copyMd}>{copiedMd ? '✓' : 'Copy'}</button>
          </div>
          <div className="badge-snippet-label">HTML</div>
          <div className="badge-snippet-row">
            <code className="badge-snippet-code">{html}</code>
            <button className={`copy-btn ${copiedHtml ? 'copied' : ''}`} onClick={copyHtml}>{copiedHtml ? '✓' : 'Copy'}</button>
          </div>
        </div>
      </div>
    </div>
  )
}

function CollectionPickerModal({ kind, namespace, name, collections, busy, onToggle, onClose }) {
  useEffect(() => {
    const h = (e) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', h)
    return () => document.removeEventListener('keydown', h)
  }, [onClose])

  return (
    <div className="shortcuts-overlay" onClick={onClose}>
      <div className="collection-picker-modal" onClick={e => e.stopPropagation()}>
        <div className="shortcuts-header">
          <span className="shortcuts-title">Add to collection</span>
          <button className="shortcuts-close" onClick={onClose}>✕</button>
        </div>
        {collections.length === 0 ? (
          <div className="collection-picker-empty">
            <p>You have no collections yet.</p>
            <a href={`/namespace/${localStorage.getItem('sf_user')}/collections`} className="btn-primary" style={{ display: 'inline-block', marginTop: 8 }}>Create one</a>
          </div>
        ) : (
          <div className="collection-picker-list">
            {collections.map(c => {
              const inCol = (c.artifacts || []).some(
                a => a.kind === kind && a.namespace === namespace && a.name === name
              )
              return (
                <button
                  key={c.slug}
                  className={`collection-picker-item ${inCol ? 'in-col' : ''}`}
                  disabled={busy[c.slug]}
                  onClick={() => onToggle(c, inCol)}
                >
                  <span className="cpi-check">{inCol ? '✓' : '+'}</span>
                  <span className="cpi-name">{c.name}</span>
                  <span className="cpi-count">{c.artifacts?.length || 0}</span>
                  {busy[c.slug] && <span className="cpi-spinner">…</span>}
                </button>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}

function CompareModal({ a, b, onClose }) {
  useEffect(() => {
    const h = (e) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', h)
    return () => document.removeEventListener('keydown', h)
  }, [onClose])

  const fields = [
    ['Version', a.version, b.version],
    ['Size', fmtBytes(a.size_bytes), fmtBytes(b.size_bytes)],
    ['Digest', a.digest_sha256 ? `sha256:${a.digest_sha256.slice(0, 16)}…` : '–', b.digest_sha256 ? `sha256:${b.digest_sha256.slice(0, 16)}…` : '–'],
    ['Package type', a.package_type || '–', b.package_type || '–'],
    ['Status', a.validation_status || '–', b.validation_status || '–'],
    ['Published by', a.created_by || '–', b.created_by || '–'],
    ['Published', a.created_at ? timeAgo(a.created_at) : '–', b.created_at ? timeAgo(b.created_at) : '–'],
    ['Yanked', a.yanked ? 'Yes' : 'No', b.yanked ? 'Yes' : 'No'],
    ['Deprecated', a.deprecated ? 'Yes' : 'No', b.deprecated ? 'Yes' : 'No'],
  ]

  return (
    <div className="shortcuts-overlay" onClick={onClose}>
      <div className="compare-modal" onClick={e => e.stopPropagation()}>
        <div className="shortcuts-header">
          <span className="shortcuts-title">Compare versions</span>
          <button className="shortcuts-close" onClick={onClose}>✕</button>
        </div>
        <table className="compare-table">
          <thead>
            <tr>
              <th className="compare-field-col">Field</th>
              <th className="compare-ver-col">{a.version}</th>
              <th className="compare-ver-col">{b.version}</th>
            </tr>
          </thead>
          <tbody>
            {fields.map(([label, va, vb]) => (
              <tr key={label} className={va !== vb ? 'compare-row-diff' : ''}>
                <td className="compare-field">{label}</td>
                <td className="compare-val">{va}</td>
                <td className="compare-val">{vb}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function CopyButton({ text, label = 'Copy', mini = false }) {
  const [copied, setCopied] = useState(false)
  const copy = () => {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }
  return (
    <button
      className={`copy-btn ${copied ? 'copied' : ''} ${mini ? 'copy-btn-mini' : ''}`}
      onClick={copy}
      title={text}
    >
      {copied ? '✓' : label}
    </button>
  )
}

function StatusBadge({ status }) {
  if (!status || status === 'valid') return <span className="badge badge-valid">valid</span>
  if (status === 'signed') return <span className="badge badge-signed">signed</span>
  if (status === 'pending') return <span className="badge badge-pending">pending</span>
  return <span className="badge badge-unsigned">{status}</span>
}

function fmtBytes(bytes) {
  if (!bytes) return '–'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function MdCodeBlock({ node, inline, className, children, ...props }) {
  const [copied, setCopied] = useState(false)
  const text = String(children).replace(/\n$/, '')
  if (inline) return <code className={`md-inline-code ${className || ''}`} {...props}>{children}</code>
  const lang = (className || '').replace('language-', '') || ''
  return (
    <div className="md-code-block">
      <div className="md-code-block-header">
        {lang && <span className="md-code-lang">{lang}</span>}
        <button
          className={`md-code-copy ${copied ? 'copied' : ''}`}
          onClick={() => {
            navigator.clipboard.writeText(text)
            setCopied(true)
            setTimeout(() => setCopied(false), 2000)
          }}
        >
          {copied ? '✓ Copied' : 'Copy'}
        </button>
      </div>
      <pre className={`md-code-pre ${className || ''}`} {...props}>
        <code>{children}</code>
      </pre>
    </div>
  )
}

function AttestationBadge({ type }) {
  const MAP = {
    'slsa-provenance': { label: 'SLSA', cls: 'att-slsa' },
    'sbom': { label: 'SBOM', cls: 'att-sbom' },
    'cosign': { label: 'Cosign', cls: 'att-cosign' },
    'vet': { label: 'Vet', cls: 'att-vet' },
  }
  const info = MAP[type] || { label: type, cls: 'att-other' }
  return <span className={`att-badge ${info.cls}`}>{info.label}</span>
}

function Sparkline({ seed, versions }) {
  const W = 72, H = 28, PTS = 8
  // Deterministic pseudo-random from seed so it's stable per artifact
  const rand = (i) => {
    let x = Math.sin(seed * 9301 + i * 49297 + versions * 233) * 1000
    return x - Math.floor(x)
  }
  const raw = Array.from({ length: PTS }, (_, i) => rand(i))
  const min = Math.min(...raw), max = Math.max(...raw)
  const norm = max === min ? raw.map(() => 0.5) : raw.map(v => (v - min) / (max - min))
  const step = W / (PTS - 1)
  const points = norm.map((v, i) => `${i * step},${H - 4 - v * (H - 8)}`).join(' ')
  const areaPoints = `0,${H} ${points} ${W},${H}`
  return (
    <svg className="sparkline-svg" width={W} height={H} viewBox={`0 0 ${W} ${H}`}>
      <defs>
        <linearGradient id={`sg${seed}`} x1="0" x2="0" y1="0" y2="1">
          <stop offset="0%" stopColor="var(--primary)" stopOpacity=".3" />
          <stop offset="100%" stopColor="var(--primary)" stopOpacity="0" />
        </linearGradient>
      </defs>
      <polygon points={areaPoints} fill={`url(#sg${seed})`} />
      <polyline points={points} fill="none" stroke="var(--primary)" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function DepGraph({ graph }) {
  if (!graph) return <div className="loading-state">Loading dependency graph…</div>
  const nodes = graph.nodes || []
  const edges = graph.edges || []
  if (nodes.length === 0) {
    return (
      <div className="deps-empty">
        <div className="deps-empty-icon">◈</div>
        <p>No dependencies declared.</p>
        <p className="deps-empty-sub">
          Declare dependencies in your <code>skill.yaml</code> under the <code>requires</code> key.
        </p>
      </div>
    )
  }

  // Build adjacency list from edges
  const children = {}
  const hasParent = new Set()
  edges.forEach(e => {
    const src = e.source || e.from
    const tgt = e.target || e.to
    if (!children[src]) children[src] = []
    children[src].push(tgt)
    hasParent.add(tgt)
  })
  const roots = nodes.filter(n => !hasParent.has(n.id || n.name))

  function renderNode(nodeId, depth = 0) {
    const node = nodes.find(n => (n.id || n.name) === nodeId)
    if (!node) return null
    const kids = children[nodeId] || []
    return (
      <div key={nodeId} className="dep-node" style={{ marginLeft: depth * 20 }}>
        <div className="dep-node-row">
          <span className="dep-connector">{depth > 0 ? '└─' : '●'}</span>
          <span className="dep-name">{node.name || nodeId}</span>
          {node.version && <code className="dep-version">{node.version}</code>}
          {node.kind && <span className="kind-chip kind-chip-sm">{node.kind}</span>}
          {node.optional && <span className="dep-optional">optional</span>}
        </div>
        {kids.map(kid => renderNode(kid, depth + 1))}
      </div>
    )
  }

  return (
    <div className="deps-tree">
      {roots.length > 0
        ? roots.map(r => renderNode(r.id || r.name))
        : nodes.map(n => renderNode(n.id || n.name))}
      <div className="deps-summary">
        {nodes.length} package{nodes.length !== 1 ? 's' : ''},&nbsp;
        {edges.length} edge{edges.length !== 1 ? 's' : ''}
      </div>
    </div>
  )
}

export default function ArtifactDetailPage() {
  const { kind, namespace, name } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const tab = searchParams.get('tab') || 'overview'
  const setTab = (t) => setSearchParams(p => { const n = new URLSearchParams(p); n.set('tab', t); return n }, { replace: true })
  const [data, setData] = useState(null)
  const [nsData, setNsData] = useState(null)
  const [error, setError] = useState(null)
  const [tagFilter, setTagFilter] = useState('')
  const [publisherArtifacts, setPublisherArtifacts] = useState([])
  const [promotions, setPromotions] = useState([])
  const [openMenu, setOpenMenu] = useState(null)
  const [attestationData, setAttestationData] = useState({})
  const [expandedAttestation, setExpandedAttestation] = useState(null)
  const [attestationLoading, setAttestationLoading] = useState(null)
  const [graph, setGraph] = useState(null)
  const [graphLoading, setGraphLoading] = useState(false)
  const [lockfile, setLockfile] = useState(null)
  const [lockfileLoading, setLockfileLoading] = useState(false)
  const [similarArtifacts, setSimilarArtifacts] = useState([])
  const [snippetLang, setSnippetLang] = useState('cli')
  const [readmeEditing, setReadmeEditing] = useState(false)
  const [readmeDraft, setReadmeDraft] = useState('')
  const [readmeSaving, setReadmeSaving] = useState(false)
  const [readmePreview, setReadmePreview] = useState(false)
  const [settingsForm, setSettingsForm] = useState(null)
  const [settingsSaving, setSettingsSaving] = useState(false)
  const [tagInput, setTagInput] = useState('')
  const [showShortcuts, setShowShortcuts] = useState(false)
  const [showBadge, setShowBadge] = useState(false)
  const [compareSelected, setCompareSelected] = useState([])
  const [showCompare, setShowCompare] = useState(false)
  const [installCopied, setInstallCopied] = useState(false)
  const [expandedManifest, setExpandedManifest] = useState(null)
  const [starredState, setStarredState] = useState(false)
  const [dlTrend, setDlTrend] = useState([])
  const [dependents, setDependents] = useState([])
  const [releaseNotesEditing, setReleaseNotesEditing] = useState(null)
  const [releaseNotesDraft, setReleaseNotesDraft] = useState('')
  const [releaseNotesSaving, setReleaseNotesSaving] = useState(false)
  const [serverStars, setServerStars] = useState(0)
  const [serverStarred, setServerStarred] = useState(false)
  const [scanResults, setScanResults] = useState(null)
  const [scanSummary, setScanSummary] = useState(null)
  const [comments, setComments] = useState([])
  const [commentDraft, setCommentDraft] = useState('')
  const [commentPosting, setCommentPosting] = useState(false)
  const [editingComment, setEditingComment] = useState(null)
  const [transferTarget, setTransferTarget] = useState('')
  const [transferring, setTransferring] = useState(false)
  const [showCollectionModal, setShowCollectionModal] = useState(false)
  const [userCollections, setUserCollections] = useState([])
  const [collectionBusy, setCollectionBusy] = useState({})
  const [editCommentDraft, setEditCommentDraft] = useState('')
  const [mcpCopied, setMcpCopied] = useState(false)
  const toast = useToast()
  const confirm = useConfirm()
  const menuRef = useRef(null)

  useDocumentTitle(data ? `${name} · ${namespace}` : name)
  useMetaTags({
    title: data ? `${namespace}/${name}` : name,
    description: data?.artifact?.description,
  })

  const navigate = useNavigate()
  const loggedInUser = localStorage.getItem('sf_user')
  const loggedInRole = localStorage.getItem('sf_role')
  const isOwner = loggedInUser === namespace || loggedInRole === 'admin'

  useEffect(() => {
    setStarredState(isStarred(kind, namespace, name))
    const sync = () => setStarredState(isStarred(kind, namespace, name))
    window.addEventListener('sf_starred', sync)
    return () => window.removeEventListener('sf_starred', sync)
  }, [kind, namespace, name])

  useEffect(() => {
    setData(null)
    setSearchParams({}, { replace: true })
    setTagFilter('')
    setGraph(null)
    setLockfile(null)
    setAttestationData({})
    setExpandedAttestation(null)
    fetchArtifactDetails(kind, namespace, name)
      .then(d => {
        setData(d)
        trackRecentlyViewed({
          kind, namespace, name,
          description: d.artifact?.description || '',
          latest_version: d.artifact?.latest_version || '',
          updated_at: d.artifact?.updated_at || '',
        })
      })
      .catch(err => setError(err.message))
  }, [kind, namespace, name])

  useEffect(() => {
    fetchNamespace(namespace)
      .then(d => { setNsData(d); setPublisherArtifacts((d.artifacts || []).filter(a => a.name !== name).slice(0, 4)) })
      .catch(() => {})
    fetchPromotions(kind, namespace, name)
      .then(d => setPromotions(d.promotions || d || []))
      .catch(() => {})
    fetchArtifacts({ kind, sort: 'pulls', limit: 8, offset: 0 })
      .then(d => setSimilarArtifacts((d.artifacts || []).filter(a => !(a.namespace === namespace && a.name === name)).slice(0, 4)))
      .catch(() => {})
    fetchArtifactStats(kind, namespace, name, 30)
      .then(d => setDlTrend(d.trend || []))
      .catch(() => {})
    fetchArtifactDependents(namespace, name)
      .then(d => setDependents(d.dependents || []))
      .catch(() => {})
    fetchArtifactStarInfo(kind, namespace, name)
      .then(d => { setServerStars(d.stars || 0); setServerStarred(d.starred || false) })
      .catch(() => {})
    fetchComments(kind, namespace, name)
      .then(d => setComments(d.comments || []))
      .catch(() => {})
  }, [kind, namespace, name])

  // Close owner dropdown on outside click
  useEffect(() => {
    if (!openMenu) return
    const handle = (e) => {
      if (menuRef.current && !menuRef.current.contains(e.target)) setOpenMenu(null)
    }
    document.addEventListener('mousedown', handle)
    return () => document.removeEventListener('mousedown', handle)
  }, [openMenu])

  // Load scan results when security tab opened
  useEffect(() => {
    if (tab !== 'security' || !data || scanResults !== null) return
    const ver = data.artifact?.latest_version || 'latest'
    fetchScanResults(kind, namespace, name, ver)
      .then(d => { setScanResults(d.results || []); setScanSummary(d.summary || {}) })
      .catch(() => { setScanResults([]); setScanSummary({}) })
  }, [tab, data])

  // Load dependency graph + lockfile when deps tab is first opened
  useEffect(() => {
    if (tab !== 'deps' || !data) return
    const ver = data.artifact?.latest_version || 'latest'
    if (graph === null) {
      setGraphLoading(true)
      fetchArtifactGraph(kind, namespace, name, ver)
        .then(d => setGraph(d))
        .catch(() => setGraph({ nodes: [], edges: [] }))
        .finally(() => setGraphLoading(false))
    }
    if (lockfile === null) {
      setLockfileLoading(true)
      fetchLockfile(kind, namespace, name, ver)
        .then(d => setLockfile(d))
        .catch(() => setLockfile({ content: null }))
        .finally(() => setLockfileLoading(false))
    }
  }, [tab, data])

  const doYank = async (version, yanked) => {
    const ok = await confirm(
      `${yanked ? 'Un-yank' : 'Yank'} version ${version}? ${yanked ? 'It will become installable again.' : 'It will be hidden from install resolution.'}`,
      { confirmLabel: yanked ? 'Un-yank' : 'Yank', danger: !yanked }
    )
    if (!ok) return
    try {
      if (yanked) await unyankVersion(namespace, name, version)
      else await yankVersion(namespace, name, version)
      const updated = await fetchArtifactDetails(kind, namespace, name)
      setData(updated)
      toast(`Version ${version} ${yanked ? 'un-yanked' : 'yanked'}`, 'success')
    } catch (err) { toast(err.message, 'error') }
  }

  const doDeprecate = async (version) => {
    const ok = await confirm(
      `Deprecate version ${version}? Users will see a deprecation warning when installing.`,
      { confirmLabel: 'Deprecate', danger: true }
    )
    if (!ok) return
    try {
      await deprecateVersion(namespace, name, version, '')
      const updated = await fetchArtifactDetails(kind, namespace, name)
      setData(updated)
      toast(`Version ${version} deprecated`, 'success')
    } catch (err) { toast(err.message, 'error') }
  }

  const doSetLatest = async (version) => {
    const ok = await confirm(
      `Set ${version} as the "latest" tag? This is what users get when installing without specifying a version.`,
      { confirmLabel: 'Set as latest' }
    )
    if (!ok) return
    try {
      await setDistTag(namespace, name, 'latest', version)
      const updated = await fetchArtifactDetails(kind, namespace, name)
      setData(updated)
      toast(`Latest tag updated to ${version}`, 'success')
    } catch (err) { toast(err.message, 'error') }
  }

  const doDeleteVersion = async (version) => {
    const ok = await confirm(
      `Permanently delete version ${version}? This cannot be undone and will break any existing lockfiles pinned to this version.`,
      { confirmLabel: 'Delete version', danger: true }
    )
    if (!ok) return
    try {
      await deleteVersion(namespace, name, version)
      const updated = await fetchArtifactDetails(kind, namespace, name)
      setData(updated)
      toast(`Version ${version} deleted`, 'success')
    } catch (err) { toast(err.message, 'error') }
  }

  const loadAttestations = async (version) => {
    if (expandedAttestation === version) {
      setExpandedAttestation(null)
      return
    }
    if (attestationData[version]) {
      setExpandedAttestation(version)
      return
    }
    setAttestationLoading(version)
    try {
      const d = await fetchAttestations(kind, namespace, name, version)
      setAttestationData(prev => ({ ...prev, [version]: d.attestations || d || [] }))
      setExpandedAttestation(version)
    } catch {
      setAttestationData(prev => ({ ...prev, [version]: [] }))
      setExpandedAttestation(version)
    } finally {
      setAttestationLoading(null)
    }
  }

  // Keyboard shortcuts: ?, g+o/t/d/a/s, c
  useEffect(() => {
    let gPressed = false, gTimer = null
    const handler = (e) => {
      if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.isContentEditable) return
      if (e.key === '?') { setShowShortcuts(v => !v); return }
      if (e.key === 'g') {
        gPressed = true
        clearTimeout(gTimer)
        gTimer = setTimeout(() => { gPressed = false }, 1000)
        return
      }
      if (gPressed) {
        gPressed = false
        clearTimeout(gTimer)
        if (e.key === 'o') setTab('overview')
        else if (e.key === 't') setTab('tags')
        else if (e.key === 'd') setTab('deps')
        else if (e.key === 'a') setTab('activity')
        else if (e.key === 's') setTab('settings')
        return
      }
      if (e.key === 'c' && data) {
        const cmd = `skforge install ${namespace}/${name}@${data.artifact?.latest_version || 'latest'}`
        navigator.clipboard.writeText(cmd).then(() => {
          setInstallCopied(true)
          toast('Install command copied', 'success')
          setTimeout(() => setInstallCopied(false), 2000)
        })
      }
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [data, namespace, name])

  const doSaveSettings = async () => {
    setSettingsSaving(true)
    try {
      const updated = await patchArtifact(kind, namespace, name, {
        description: settingsForm.description,
        tags: settingsForm.tags,
        visibility: settingsForm.visibility,
      })
      setData(updated)
      toast('Settings saved', 'success')
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setSettingsSaving(false)
    }
  }

  const startEditReadme = () => {
    setReadmeDraft(data?.artifact?.readme || data?.artifact?.description || '')
    setReadmePreview(false)
    setReadmeEditing(true)
  }

  const doSaveReadme = async () => {
    setReadmeSaving(true)
    try {
      const updated = await patchArtifact(kind, namespace, name, { readme: readmeDraft })
      setData(updated)
      setReadmeEditing(false)
      toast('README updated', 'success')
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setReadmeSaving(false)
    }
  }

  const shareUrl = () => {
    navigator.clipboard.writeText(window.location.href).then(() => {
      toast('Link copied to clipboard', 'success')
    })
  }

  if (error) return <div style={{ padding: 32 }}><div className="error-state">{error}</div></div>
  if (!data) return (
    <div style={{ maxWidth: 1280, margin: '0 auto', padding: '24px 24px 48px' }}>
      <SkeletonDetail />
    </div>
  )

  const { artifact, versions = [], dist_tags: distTags = {} } = data
  const latest = versions.find(v => v.version === artifact.latest_version) || versions[0]
  const color = avatarColor(namespace)
  const installCmd = `skforge install ${namespace}/${name}`
  const installCmdLatest = latest ? `skforge install ${namespace}/${name}@${latest.version}` : installCmd

  const filteredVersions = tagFilter.trim()
    ? versions.filter(v => v.version.toLowerCase().includes(tagFilter.toLowerCase()))
    : versions

  return (
    <>
      {showShortcuts && <ShortcutsModal onClose={() => setShowShortcuts(false)} />}
      {showBadge && <BadgeModal kind={kind} namespace={namespace} name={name} onClose={() => setShowBadge(false)} />}
      {showCollectionModal && (
        <CollectionPickerModal
          kind={kind} namespace={namespace} name={name}
          collections={userCollections}
          busy={collectionBusy}
          onToggle={async (col, inCol) => {
            setCollectionBusy(b => ({ ...b, [col.slug]: true }))
            try {
              const fn = inCol ? removeFromCollection : addToCollection
              const updated = await fn(loggedInUser, col.slug, { kind, namespace, name })
              setUserCollections(cs => cs.map(c => c.slug === col.slug ? updated : c))
            } catch {}
            setCollectionBusy(b => ({ ...b, [col.slug]: false }))
          }}
          onClose={() => setShowCollectionModal(false)}
        />
      )}
      {showCompare && compareSelected.length === 2 && (
        <CompareModal
          a={versions.find(v => v.version === compareSelected[0])}
          b={versions.find(v => v.version === compareSelected[1])}
          onClose={() => { setShowCompare(false); setCompareSelected([]) }}
        />
      )}

      {/* Breadcrumb */}
      <div className="breadcrumb-bar">
        <div className="breadcrumb-inner">
          <Link to="/">Explore</Link>
          <span className="bc-sep">›</span>
          <Link to={`/namespace/${namespace}`}>{namespace}</Link>
          <span className="bc-sep">›</span>
          <span>{name}</span>
        </div>
        <button className="shortcuts-hint-btn" onClick={() => setShowShortcuts(true)} title="Keyboard shortcuts (?)">
          <kbd>?</kbd>
        </button>
      </div>

      {/* Repository hero */}
      <div className="repo-hero">
        <div className="repo-hero-inner">
          <div className={`repo-hero-avatar ${kindClass(kind)}`} style={{ background: color }}>
            {namespace[0].toUpperCase()}
          </div>
          <div className="repo-hero-body">
            <div className="repo-hero-title-row">
              <h1 className="repo-hero-name">{name}</h1>
              <span className="kind-chip">{kind}</span>
              {artifact.visibility === 'private' && (
                <span className="visibility-chip">Private</span>
              )}
            </div>
            <div className="repo-hero-meta">
              By{' '}
              <Link to={`/namespace/${namespace}`} className="repo-hero-ns">{namespace}</Link>
              {nsData?.verified && (
                <span className="ns-verified-badge" title="Verified publisher">✓</span>
              )}
              {artifact.latest_version && (
                <>
                  <span className="repo-hero-sep">·</span>
                  <code className="repo-hero-ver">v{artifact.latest_version}</code>
                </>
              )}
            </div>
            {artifact.description && (
              <p className="repo-hero-desc">{artifact.description}</p>
            )}
            <div className="repo-hero-stats">
              <span className="repo-stat-pill pulls-stat">
                <strong>{fmtNumber(artifact.downloads || 0)}</strong> pulls
              </span>
              <span className="repo-stat-sep">·</span>
              <span className="repo-stat-pill">
                <strong>{versions.length}</strong>{' '}
                {versions.length === 1 ? 'version' : 'versions'}
              </span>
              {artifact.updated_at && (
                <>
                  <span className="repo-stat-sep">·</span>
                  <span className="repo-stat-pill">Updated {timeAgo(artifact.updated_at)}</span>
                </>
              )}
            </div>
            {(artifact.tags || []).length > 0 && (
              <div className="repo-hero-tags">
                {(artifact.tags || []).map(t => (
                  <span key={t} className="tag tag-clickable" onClick={() => navigate(`/search?q=${encodeURIComponent(t)}`)}>{t}</span>
                ))}
              </div>
            )}
          </div>
          <div className="repo-hero-actions">
            <button
              className={`repo-star-btn ${serverStarred || starredState ? 'starred' : ''}`}
              onClick={async () => {
                const next = toggleStar({ kind, namespace, name, description: artifact.description, latest_version: artifact.latest_version, updated_at: artifact.updated_at })
                setStarredState(next)
                if (loggedInUser) {
                  try {
                    const res = serverStarred
                      ? await unstarArtifact(kind, namespace, name)
                      : await starArtifact(kind, namespace, name)
                    setServerStars(res.stars || 0)
                    setServerStarred(res.starred || false)
                  } catch {}
                }
              }}
              title={serverStarred || starredState ? 'Remove star' : 'Star this artifact'}
            >
              {serverStarred || starredState ? '★' : '☆'} {serverStars > 0 ? fmtNumber(serverStars) : 'Star'}
            </button>
            <CopyButton
              text={`skforge pull ${namespace}/${name}${artifact.latest_version ? ':' + artifact.latest_version : ''}`}
              label="⬇ Pull"
            />
            <button className="repo-share-btn" onClick={shareUrl} title="Copy link">
              Share
            </button>
            <button className="repo-badge-btn" onClick={() => setShowBadge(true)} title="Embed badge">
              Badge
            </button>
            {loggedInUser && (
              <button
                className="repo-collect-btn"
                title="Add to collection"
                onClick={async () => {
                  if (!userCollections.length) {
                    const d = await fetchCollections(loggedInUser).catch(() => ({ collections: [] }))
                    setUserCollections(d.collections || [])
                  }
                  setShowCollectionModal(true)
                }}
              >
                ☰ Collect
              </button>
            )}
          </div>
        </div>
      </div>

      <div className="detail-layout">
        {/* Main column — ALL tab panels live inside here */}
        <div className="detail-main">
          {/* Tabs */}
          <div className="tabs">
            <button className={`tab-btn ${tab === 'overview' ? 'active' : ''}`} onClick={() => setTab('overview')}>
              Overview
            </button>
            <button className={`tab-btn ${tab === 'tags' ? 'active' : ''}`} onClick={() => setTab('tags')}>
              Tags
              <span className="tab-count">{versions.length}</span>
            </button>
            <button className={`tab-btn ${tab === 'deps' ? 'active' : ''}`} onClick={() => setTab('deps')}>
              Dependencies
            </button>
            <button className={`tab-btn ${tab === 'activity' ? 'active' : ''}`} onClick={() => setTab('activity')}>
              Activity
              {promotions.length > 0 && <span className="tab-count">{promotions.length}</span>}
            </button>
            <button className={`tab-btn ${tab === 'security' ? 'active' : ''}`} onClick={() => setTab('security')}>
              Security
              {scanSummary && (scanSummary.critical > 0 || scanSummary.high > 0) && (
                <span className="tab-count tab-count-danger">{(scanSummary.critical || 0) + (scanSummary.high || 0)}</span>
              )}
            </button>
            <button className={`tab-btn ${tab === 'comments' ? 'active' : ''}`} onClick={() => setTab('comments')}>
              Discussion
              {comments.length > 0 && <span className="tab-count">{comments.length}</span>}
            </button>
            {isOwner && (
              <button
                className={`tab-btn ${tab === 'settings' ? 'active' : ''}`}
                onClick={() => {
                  setSettingsForm({
                    description: artifact.description || '',
                    tags: [...(artifact.tags || [])],
                    visibility: artifact.visibility || 'public',
                  })
                  setTagInput('')
                  setTab('settings')
                }}
              >
                ⚙ Settings
              </button>
            )}
          </div>

          {/* ── Overview ── */}
          {tab === 'overview' && (
            <div className="overview-panel">
              {/* Yanked/deprecated warning */}
              {latest && (latest.yanked || latest.deprecated) && (
                <div className={`version-warning-banner ${latest.yanked ? 'warn-yanked' : 'warn-deprecated'}`}>
                  <span className="warn-icon">{latest.yanked ? '⚠' : '⚠'}</span>
                  <div>
                    <strong>
                      {latest.yanked ? 'Latest version is yanked' : 'Latest version is deprecated'}
                    </strong>
                    <p>
                      {latest.yanked
                        ? `Version ${latest.version} has been yanked and may not be installable. Consider pinning an earlier version.`
                        : `Version ${latest.version} is deprecated. The maintainer recommends upgrading to a newer version.`}
                    </p>
                  </div>
                </div>
              )}
              {/* Quick Reference */}
              <div className="quick-ref">
                <h3 className="quick-ref-title">Quick Reference</h3>
                <div className="quick-ref-grid">
                  <div className="quick-ref-item">
                    <span className="quick-ref-label">Maintained by</span>
                    <Link to={`/namespace/${namespace}`} className="quick-ref-link">{namespace}</Link>
                  </div>
                  <div className="quick-ref-item">
                    <span className="quick-ref-label">Kind</span>
                    <span className="kind-chip kind-chip-sm">{kind}</span>
                  </div>
                  {artifact.latest_version && (
                    <div className="quick-ref-item">
                      <span className="quick-ref-label">Latest version</span>
                      <code className="quick-ref-mono">{artifact.latest_version}</code>
                    </div>
                  )}
                  <div className="quick-ref-item">
                    <span className="quick-ref-label">Total versions</span>
                    <span className="quick-ref-val">{versions.length}</span>
                  </div>
                  {artifact.updated_at && (
                    <div className="quick-ref-item">
                      <span className="quick-ref-label">Last updated</span>
                      <span className="quick-ref-val">{timeAgo(artifact.updated_at)}</span>
                    </div>
                  )}
                  <div className="quick-ref-item">
                    <span className="quick-ref-label">Where to get help</span>
                    <a href="/api-docs" className="quick-ref-link" target="_blank" rel="noopener noreferrer">
                      API docs
                    </a>
                  </div>
                  <div className="quick-ref-item">
                    <span className="quick-ref-label">Install</span>
                    <code className="quick-ref-mono">{installCmd}</code>
                  </div>
                  {Object.keys(distTags).length > 0 && (
                    <div className="quick-ref-item quick-ref-item-wide">
                      <span className="quick-ref-label">Distribution channels</span>
                      <div className="quick-ref-channels">
                        {Object.entries(distTags).map(([t, ver]) => (
                          <span key={t} className="quick-ref-channel">
                            <span className="tag">{t}</span>
                            <span className="quick-ref-arrow">→</span>
                            <code className="quick-ref-mono">{ver}</code>
                          </span>
                        ))}
                      </div>
                    </div>
                  )}
                  {(artifact.owners || []).length > 0 && (
                    <div className="quick-ref-item quick-ref-item-wide">
                      <span className="quick-ref-label">Maintainers</span>
                      <span className="quick-ref-val">{artifact.owners.join(', ')}</span>
                    </div>
                  )}
                </div>
              </div>

              {/* Supported Tags */}
              {versions.length > 0 && (
                <>
                  <div className="overview-divider" />
                  <div className="supported-tags-section">
                    <h3 className="supported-tags-title">Supported tags</h3>
                    <div className="supported-tags-list">
                      {versions.map(v => {
                        const isLatest = v.version === artifact.latest_version
                        const channelTags = Object.entries(distTags)
                          .filter(([, ver]) => ver === v.version)
                          .map(([t]) => t)
                        return (
                          <span
                            key={v.id}
                            className={`supported-tag-item ${isLatest ? 'supported-tag-latest' : ''}`}
                            onClick={() => setTab('tags')}
                            title={`skforge install ${namespace}/${name}@${v.version}`}
                          >
                            {v.version}
                            {isLatest && <span className="st-badge">latest</span>}
                            {channelTags.filter(t => t !== 'latest').map(t => (
                              <span key={t} className="st-channel">{t}</span>
                            ))}
                          </span>
                        )
                      })}
                    </div>
                  </div>
                </>
              )}

              {/* Compatibility */}
              {(() => {
                const latestV = versions.find(v => v.version === artifact.latest_version) || versions[0]
                const runtime = latestV?.manifest?.spec?.runtime
                const deps = latestV?.manifest?.spec?.dependencies || []
                if (!runtime && deps.length === 0) return null
                return (
                  <>
                    <div className="overview-divider" />
                    <div className="compat-section">
                      <h3 className="compat-title">Compatibility</h3>
                      <div className="compat-badges">
                        {runtime && (
                          <span className="compat-badge compat-runtime" title="Runtime requirement">
                            <span className="compat-badge-icon">⚙</span>
                            {runtime}
                          </span>
                        )}
                        {kind === 'agent' && (
                          <span className="compat-badge compat-model" title="Works with AI models">
                            <span className="compat-badge-icon">⬡</span>
                            Claude / OpenAI
                          </span>
                        )}
                        {(artifact.tags || []).filter(t => ['python', 'node', 'nodejs', 'typescript', 'go', 'rust', 'java'].includes(t)).map(t => (
                          <span key={t} className="compat-badge compat-lang">
                            <span className="compat-badge-icon">◈</span>
                            {t}
                          </span>
                        ))}
                        {deps.length > 0 && (
                          <span className="compat-badge compat-deps" title="Number of dependencies">
                            <span className="compat-badge-icon">⬟</span>
                            {deps.length} {deps.length === 1 ? 'dependency' : 'dependencies'}
                          </span>
                        )}
                      </div>
                    </div>
                  </>
                )
              })()}

              <div className="overview-divider" />

              {/* Install snippets */}
              <div className="snippets-section">
                <h3 className="snippets-title">Usage</h3>
                <div className="snippets-tabs">
                  {[
                    { id: 'cli', label: 'CLI' },
                    { id: 'yaml', label: 'skill.yaml' },
                    { id: 'python', label: 'Python' },
                    { id: 'curl', label: 'curl' },
                  ].map(t => (
                    <button
                      key={t.id}
                      className={`snippet-tab ${snippetLang === t.id ? 'active' : ''}`}
                      onClick={() => setSnippetLang(t.id)}
                    >
                      {t.label}
                    </button>
                  ))}
                </div>
                <div className="snippet-block">
                  {snippetLang === 'cli' && (
                    <MdCodeBlock className="language-bash" inline={false}>
                      {`# Install latest\nskforge install ${namespace}/${name}\n\n# Install specific version\nskforge install ${namespace}/${name}@${artifact.latest_version || '1.0.0'}`}
                    </MdCodeBlock>
                  )}
                  {snippetLang === 'yaml' && (
                    <MdCodeBlock className="language-yaml" inline={false}>
                      {`requires:\n  - ${namespace}/${name}@${artifact.latest_version || '1.0.0'}`}
                    </MdCodeBlock>
                  )}
                  {snippetLang === 'python' && (
                    <MdCodeBlock className="language-python" inline={false}>
                      {`from sklib import SkillRegistry\n\nregistry = SkillRegistry()\nskill = registry.load("${namespace}/${name}@${artifact.latest_version || '1.0.0'}")`}
                    </MdCodeBlock>
                  )}
                  {snippetLang === 'curl' && (
                    <MdCodeBlock className="language-bash" inline={false}>
                      {`# Download artifact tarball\ncurl -L -o ${name}.tar.gz \\\n  https://registry.example.com/api/v1/artifacts/${kind}/${namespace}/${name}/versions/${artifact.latest_version || 'latest'}/download`}
                    </MdCodeBlock>
                  )}
                </div>

                {/* Badge generator */}
                <div className="badge-gen">
                  <span className="badge-gen-label">README badge</span>
                  <CopyButton
                    text={`[![SkillForge](https://img.shields.io/badge/SkillForge-${encodeURIComponent(namespace + '/' + name)}-6366f1?logo=data:image/svg+xml;base64,PHN2Zy8+)](${window.location.href})`}
                    label="Copy badge markdown"
                  />
                </div>
              </div>

              <div className="overview-divider" />

              {/* README / description */}
              {/* ── Agent-specific panel ── */}
              {kind === 'agent' && latest?.manifest?.spec?.agent && (() => {
                const a = latest.manifest.spec.agent
                return (
                  <div className="kind-spec-panel">
                    <div className="kind-spec-title">🤖 Agent configuration</div>
                    <div className="kind-spec-grid">
                      {(a.model || a.model_family) && (
                        <div className="kind-spec-row">
                          <span className="kind-spec-label">Model</span>
                          <code className="kind-spec-value">{a.model || a.model_family}</code>
                        </div>
                      )}
                      {a.max_steps > 0 && (
                        <div className="kind-spec-row">
                          <span className="kind-spec-label">Max steps</span>
                          <span className="kind-spec-value">{a.max_steps}</span>
                        </div>
                      )}
                      {a.memory && (
                        <div className="kind-spec-row">
                          <span className="kind-spec-label">Memory</span>
                          <span className="kind-spec-value">
                            {a.memory.type}
                            {a.memory.backend ? ` (${a.memory.backend})` : ''}
                          </span>
                        </div>
                      )}
                      {a.capabilities?.length > 0 && (
                        <div className="kind-spec-row">
                          <span className="kind-spec-label">Capabilities</span>
                          <div className="kind-spec-chips">
                            {a.capabilities.map(c => <span key={c} className="kind-spec-chip">{c}</span>)}
                          </div>
                        </div>
                      )}
                    </div>
                    {a.system_prompt && (
                      <div className="agent-system-prompt">
                        <div className="agent-system-prompt-label">System prompt</div>
                        <pre className="agent-system-prompt-body">{a.system_prompt}</pre>
                      </div>
                    )}
                    {a.tools?.length > 0 && (
                      <div style={{ marginTop: 14 }}>
                        <div className="kind-spec-label" style={{ marginBottom: 6 }}>Tools used</div>
                        <div className="kind-spec-chips">
                          {a.tools.map(t => <span key={t} className="kind-spec-chip kind-spec-chip-tool">{t}</span>)}
                        </div>
                      </div>
                    )}
                  </div>
                )
              })()}

              {/* ── Flow-specific panel ── */}
              {kind === 'flow' && latest?.manifest?.spec?.steps?.length > 0 && (() => {
                const steps = latest.manifest.spec.steps
                return (
                  <div className="kind-spec-panel">
                    <div className="kind-spec-title">⚡ Flow steps</div>
                    <div className="flow-dag">
                      {steps.map((step, i) => (
                        <div key={step.id} className="flow-step-card">
                          <div className="flow-step-num">{i + 1}</div>
                          <div className="flow-step-body">
                            <div className="flow-step-id">{step.id}</div>
                            <code className="flow-step-uses">{step.uses}</code>
                            {step.needs?.length > 0 && (
                              <div className="flow-step-needs">
                                needs: {step.needs.map(n => (
                                  <span key={n} className="flow-step-need-chip">{n}</span>
                                ))}
                              </div>
                            )}
                            {step.if && (
                              <div className="flow-step-cond">if: <code>{step.if}</code></div>
                            )}
                          </div>
                          {i < steps.length - 1 && step.needs?.length === 0 && (
                            <div className="flow-step-arrow">↓</div>
                          )}
                        </div>
                      ))}
                    </div>
                  </div>
                )
              })()}

              {/* ── MCP-specific panel ── */}
              {kind === 'mcp' && latest?.manifest?.spec?.mcp && (() => {
                const mcp = latest.manifest.spec.mcp
                const claudeConfig = JSON.stringify({
                  mcpServers: {
                    [name]: {
                      command: mcp.command?.[0] || 'skforge',
                      args: [...(mcp.command?.slice(1) || ['run', `${namespace}/${name}`]), ...(mcp.args || [])],
                      env: mcp.env || {},
                    }
                  }
                }, null, 2)
                return (
                  <div className="kind-spec-panel">
                    <div className="kind-spec-title">🔌 MCP server</div>
                    <div className="kind-spec-grid">
                      <div className="kind-spec-row">
                        <span className="kind-spec-label">Transport</span>
                        <span className={`mcp-transport-badge mcp-transport-${mcp.transport}`}>{mcp.transport}</span>
                      </div>
                      {mcp.tools?.length > 0 && (
                        <div className="kind-spec-row">
                          <span className="kind-spec-label">Tools</span>
                          <span className="kind-spec-value">{mcp.tools.length}</span>
                        </div>
                      )}
                      {mcp.resources?.length > 0 && (
                        <div className="kind-spec-row">
                          <span className="kind-spec-label">Resources</span>
                          <span className="kind-spec-value">{mcp.resources.length}</span>
                        </div>
                      )}
                      {mcp.prompts?.length > 0 && (
                        <div className="kind-spec-row">
                          <span className="kind-spec-label">Prompts</span>
                          <span className="kind-spec-value">{mcp.prompts.length}</span>
                        </div>
                      )}
                    </div>

                    {mcp.tools?.length > 0 && (
                      <div className="mcp-tools-list">
                        <div className="kind-spec-label" style={{ marginBottom: 8 }}>Available tools</div>
                        {mcp.tools.map(t => (
                          <div key={t.name} className="mcp-tool-row">
                            <code className="mcp-tool-name">{t.name}</code>
                            {t.description && <span className="mcp-tool-desc">{t.description}</span>}
                          </div>
                        ))}
                      </div>
                    )}

                    {mcp.resources?.length > 0 && (
                      <div className="mcp-tools-list" style={{ marginTop: 12 }}>
                        <div className="kind-spec-label" style={{ marginBottom: 8 }}>Resources</div>
                        {mcp.resources.map(res => (
                          <div key={res.uri} className="mcp-tool-row">
                            <code className="mcp-tool-name">{res.uri}</code>
                            {res.description && <span className="mcp-tool-desc">{res.description}</span>}
                          </div>
                        ))}
                      </div>
                    )}

                    <div className="mcp-connect-section">
                      <div className="mcp-connect-label">
                        Claude Desktop config
                        <button
                          className={`copy-btn ${mcpCopied ? 'copied' : ''}`}
                          onClick={() => { navigator.clipboard.writeText(claudeConfig); setMcpCopied(true); setTimeout(() => setMcpCopied(false), 1500) }}
                        >{mcpCopied ? '✓ Copied' : 'Copy'}</button>
                      </div>
                      <pre className="mcp-connect-code">{claudeConfig}</pre>
                      <a
                        href={`/api/v1/artifacts/mcp/${namespace}/${name}/mcp.json`}
                        className="mcp-manifest-link"
                        target="_blank"
                        rel="noopener noreferrer"
                      >View full manifest →</a>
                    </div>
                  </div>
                )
              })()}

              <div className="readme-section">
                <div className="readme-section-header">
                  <span className="readme-section-title">README</span>
                  {isOwner && !readmeEditing && (
                    <button className="readme-edit-btn" onClick={startEditReadme}>✎ Edit</button>
                  )}
                </div>
                {readmeEditing ? (
                  <div className="readme-editor">
                    <div className="readme-editor-toolbar">
                      <button
                        className={`readme-tab-btn ${!readmePreview ? 'active' : ''}`}
                        onClick={() => setReadmePreview(false)}
                      >Edit</button>
                      <button
                        className={`readme-tab-btn ${readmePreview ? 'active' : ''}`}
                        onClick={() => setReadmePreview(true)}
                      >Preview</button>
                      <div style={{ flex: 1 }} />
                      <button className="btn-secondary" onClick={() => setReadmeEditing(false)} disabled={readmeSaving}>Cancel</button>
                      <button className="btn-primary" onClick={doSaveReadme} disabled={readmeSaving}>
                        {readmeSaving ? 'Saving…' : 'Save README'}
                      </button>
                    </div>
                    {readmePreview ? (
                      <div className="markdown-body readme-preview-body">
                        {readmeDraft ? (
                          <ReactMarkdown remarkPlugins={[remarkGfm]} components={{ code: MdCodeBlock }}>
                            {readmeDraft}
                          </ReactMarkdown>
                        ) : (
                          <p className="readme-empty-hint">Nothing to preview yet.</p>
                        )}
                      </div>
                    ) : (
                      <textarea
                        className="readme-textarea"
                        value={readmeDraft}
                        onChange={e => setReadmeDraft(e.target.value)}
                        placeholder="# My Skill&#10;&#10;Describe what your skill does, how to use it, and any configuration options."
                        rows={20}
                        autoFocus
                      />
                    )}
                  </div>
                ) : artifact.readme ? (
                  <div className="markdown-body">
                    <ReactMarkdown remarkPlugins={[remarkGfm]} components={{ code: MdCodeBlock }}>
                      {artifact.readme}
                    </ReactMarkdown>
                  </div>
                ) : artifact.description ? (
                  <div className="markdown-body">
                    <p>{artifact.description}</p>
                  </div>
                ) : (
                  <div className="no-readme">
                    <p>No README provided.</p>
                    {isOwner && (
                      <button className="btn-secondary" style={{ marginTop: 12 }} onClick={startEditReadme}>
                        + Add README
                      </button>
                    )}
                  </div>
                )}
              </div>
            </div>
          )}

          {/* ── Tags ── */}
          {tab === 'tags' && (
            <div className="versions-panel">
              <div className="tag-filter-bar">
                <input
                  className="tag-filter-input"
                  placeholder="Filter tags by name…"
                  value={tagFilter}
                  onChange={e => setTagFilter(e.target.value)}
                />
                <span className="tag-filter-count">
                  {filteredVersions.length === versions.length
                    ? `${versions.length} tag${versions.length !== 1 ? 's' : ''}`
                    : `${filteredVersions.length} of ${versions.length}`}
                </span>
              </div>

              {compareSelected.length > 0 && (
                <div className="compare-bar">
                  <span>{compareSelected.length === 1 ? 'Select one more version to compare' : `Comparing ${compareSelected[0]} ↔ ${compareSelected[1]}`}</span>
                  {compareSelected.length === 2 && (
                    <button className="btn-primary" style={{ padding: '4px 12px', fontSize: 13 }} onClick={() => setShowCompare(true)}>
                      Compare
                    </button>
                  )}
                  <button className="btn-secondary" style={{ padding: '4px 10px', fontSize: 12 }} onClick={() => setCompareSelected([])}>
                    Clear
                  </button>
                </div>
              )}

            {filteredVersions.length === 0 ? (
                <div className="empty-state" style={{ padding: '40px 20px' }}>
                  <p>No tags matching &ldquo;{tagFilter}&rdquo;</p>
                </div>
              ) : (
                <table className="versions-table">
                  <thead>
                    <tr>
                      <th style={{ width: 32 }}></th>
                      <th>Tag / Version</th>
                      <th>Digest</th>
                      <th>Size</th>
                      <th>Pulls</th>
                      <th>Published by</th>
                      <th>Status</th>
                      <th>Pushed</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredVersions.map(v => {
                      const isLatest = v.version === artifact.latest_version
                      const channelTags = Object.entries(distTags)
                        .filter(([, ver]) => ver === v.version)
                        .map(([t]) => t)
                      const pullCmd = `skforge install ${namespace}/${name}@${v.version}`
                      const attList = attestationData[v.version]
                      const isExpanded = expandedAttestation === v.version
                      const isLoadingAtt = attestationLoading === v.version
                      return (
                        <>
                          <tr key={v.id} className={isLatest ? 'version-row-latest' : ''}>
                            <td style={{ textAlign: 'center' }}>
                              <input
                                type="checkbox"
                                className="compare-checkbox"
                                checked={compareSelected.includes(v.version)}
                                onChange={() => {
                                  setCompareSelected(prev =>
                                    prev.includes(v.version)
                                      ? prev.filter(x => x !== v.version)
                                      : prev.length < 2 ? [...prev, v.version] : prev
                                  )
                                }}
                                title="Select to compare"
                              />
                            </td>
                            <td className="vtd-tag">
                              <div className="version-tags">
                                <span className="version-num">{v.version}</span>
                                {isLatest && <span className="tag tag-latest">latest</span>}
                                {channelTags.filter(t => t !== 'latest').map(t => (
                                  <span key={t} className="tag">{t}</span>
                                ))}
                              </div>
                              <div className="version-cmd-row">
                                <code className="version-cmd-code">{pullCmd}</code>
                                <CopyButton text={pullCmd} label="Copy" />
                              </div>
                            </td>
                            <td>
                              {v.digest_sha256 ? (
                                <div className="digest-cell">
                                  <span className="digest-mono" title={`sha256:${v.digest_sha256}`}>
                                    sha256:{v.digest_sha256.slice(0, 12)}…
                                  </span>
                                  <CopyButton text={`sha256:${v.digest_sha256}`} label="⎘" mini />
                                </div>
                              ) : (
                                <span className="text-muted">–</span>
                              )}
                            </td>
                            <td className="version-size">{fmtBytes(v.size_bytes)}</td>
                            <td className="version-pulls">{v.downloads > 0 ? fmtNumber(v.downloads) : '–'}</td>
                            <td className="version-publisher">
                              {v.created_by ? (
                                <Link to={`/namespace/${v.created_by}`} className="publisher-link">{v.created_by}</Link>
                              ) : '–'}
                            </td>
                            <td>
                              <div className="version-badges">
                                <StatusBadge status={v.validation_status} />
                                {v.scan_status === 'clean' && <span className="badge badge-scan-clean" title="Vulnerability scan: clean">✓ scan</span>}
                                {v.scan_status === 'vulnerable' && <span className="badge badge-scan-vuln" title="Vulnerability scan: issues found">⚠ vuln</span>}
                                {v.yanked && <span className="badge badge-unsigned">yanked</span>}
                                {v.deprecated && <span className="badge badge-pending">deprecated</span>}
                                {v.signature_status && v.signature_status !== 'unsigned' && (
                                  <StatusBadge status={v.signature_status} />
                                )}
                              </div>
                            </td>
                            <td className="version-date">{timeAgo(v.created_at)}</td>
                            <td>
                              <div className="version-actions" ref={isOwner ? menuRef : null}>
                                <button
                                  className={`att-toggle-btn ${expandedManifest === v.id ? 'active' : ''}`}
                                  onClick={() => setExpandedManifest(expandedManifest === v.id ? null : v.id)}
                                  title="View manifest"
                                >📄</button>
                                <button
                                  className={`att-toggle-btn ${isExpanded ? 'active' : ''}`}
                                  onClick={() => loadAttestations(v.version)}
                                  title="View attestations"
                                  disabled={isLoadingAtt}
                                >
                                  {isLoadingAtt ? '…' : '🔏'}
                                </button>
                                <a
                                  className="dl-btn"
                                  href={`/api/v1/artifacts/${kind}/${namespace}/${name}/versions/${v.version}/download`}
                                  title="Download tarball"
                                >↓</a>
                                <button
                                  className="dl-btn"
                                  title="Copy curl command"
                                  onClick={() => {
                                    const url = `${window.location.origin}/api/v1/artifacts/${kind}/${namespace}/${name}/versions/${v.version}/download`
                                    navigator.clipboard.writeText(`curl -L -o ${name}-${v.version}.tar.gz "${url}"`)
                                    toast('curl command copied', 'success')
                                  }}
                                >⎘</button>
                                {isOwner && (
                                  <div className="owner-actions-menu">
                                    <button
                                      className="owner-action-btn"
                                      title="Owner actions"
                                      onClick={e => {
                                        e.stopPropagation()
                                        setOpenMenu(openMenu === v.id ? null : v.id)
                                      }}
                                    >⋮</button>
                                    {openMenu === v.id && (
                                      <div className="owner-dropdown">
                                        <button onClick={() => { setOpenMenu(null); doYank(v.version, v.yanked) }}>
                                          {v.yanked ? 'Un-yank' : 'Yank'}
                                        </button>
                                        {!v.deprecated && (
                                          <button onClick={() => { setOpenMenu(null); doDeprecate(v.version) }}>
                                            Deprecate
                                          </button>
                                        )}
                                        {v.version !== artifact.latest_version && (
                                          <button onClick={() => { setOpenMenu(null); doSetLatest(v.version) }}>
                                            Set as latest
                                          </button>
                                        )}
                                        <div className="owner-dropdown-sep" />
                                        <button
                                          className="owner-dropdown-danger"
                                          onClick={() => { setOpenMenu(null); doDeleteVersion(v.version) }}
                                        >
                                          Delete version
                                        </button>
                                      </div>
                                    )}
                                  </div>
                                )}
                              </div>
                            </td>
                          </tr>
                          {expandedManifest === v.id && (
                            <tr key={`${v.id}-manifest`} className="att-row">
                              <td colSpan={9}>
                                <div className="manifest-panel">
                                  <div className="manifest-panel-header">
                                    <span className="manifest-panel-title">Manifest · {v.version}</span>
                                    {v.manifest && (
                                      <CopyButton
                                        text={JSON.stringify(v.manifest, null, 2)}
                                        label="Copy JSON"
                                      />
                                    )}
                                  </div>
                                  {v.manifest ? (
                                    <pre className="manifest-pre"><code>{JSON.stringify(v.manifest, null, 2)}</code></pre>
                                  ) : (
                                    <p className="att-none">No manifest recorded for this version.</p>
                                  )}
                                </div>
                              </td>
                            </tr>
                          )}
                          {isExpanded && (
                            <tr key={`${v.id}-att`} className="att-row">
                              <td colSpan={9}>
                                <div className="att-panel">
                                  <div className="att-panel-title">Attestations for {v.version}</div>
                                  {!attList || attList.length === 0 ? (
                                    <p className="att-none">No attestations recorded for this version.</p>
                                  ) : (
                                    <div className="att-list">
                                      {attList.map((a, i) => (
                                        <div key={i} className="att-item">
                                          <AttestationBadge type={a.type || a.predicate_type || 'unknown'} />
                                          <div className="att-item-body">
                                            <div className="att-item-type">{a.type || a.predicate_type}</div>
                                            {a.subject && (
                                              <div className="att-item-sub">Subject: <code>{a.subject}</code></div>
                                            )}
                                            {a.signed_by && (
                                              <div className="att-item-sub">Signed by: <code>{a.signed_by}</code></div>
                                            )}
                                            {a.created_at && (
                                              <div className="att-item-date">{timeAgo(a.created_at)}</div>
                                            )}
                                          </div>
                                          {a.verified && (
                                            <span className="att-verified">✓ Verified</span>
                                          )}
                                        </div>
                                      ))}
                                    </div>
                                  )}
                                </div>
                              </td>
                            </tr>
                          )}
                        </>
                      )
                    })}
                  </tbody>
                </table>
              )}
            </div>
          )}

          {/* ── Dependencies ── */}
          {tab === 'deps' && (
            <div className="deps-panel">
              <div className="deps-header">
                <h3 className="deps-title">Dependency graph</h3>
                <span className="deps-version-label">
                  {data.artifact?.latest_version ? `v${data.artifact.latest_version}` : 'latest'}
                </span>
              </div>
              {graphLoading ? (
                <div className="loading-state">Building dependency graph…</div>
              ) : (
                <DepGraph graph={graph} />
              )}

              {/* Lockfile */}
              <div className="lockfile-section">
                <div className="lockfile-header">
                  <span className="lockfile-title">Lockfile</span>
                  {lockfile?.content && (
                    <CopyButton text={typeof lockfile.content === 'string' ? lockfile.content : JSON.stringify(lockfile.content, null, 2)} label="Copy" />
                  )}
                </div>
                {lockfileLoading ? (
                  <div className="loading-state" style={{ padding: '16px 0' }}>Loading lockfile…</div>
                ) : lockfile === null ? null : !lockfile.content ? (
                  <div className="lockfile-empty">No lockfile found for this version.</div>
                ) : (
                  <pre className="lockfile-content">
                    <code>{typeof lockfile.content === 'string' ? lockfile.content : JSON.stringify(lockfile.content, null, 2)}</code>
                  </pre>
                )}
              </div>
            </div>
          )}

          {/* ── Activity ── */}
          {tab === 'activity' && (
            <div className="activity-panel">
              {/* 30-day download trend */}
              {dlTrend.length > 0 && (() => {
                const maxCount = Math.max(...dlTrend.map(d => d.count), 1)
                const total30 = dlTrend.reduce((s, d) => s + d.count, 0)
                return (
                  <div className="dl-chart-section">
                    <div className="dl-chart-header">
                      <span className="dl-chart-title">Downloads — last 30 days</span>
                      <span className="dl-chart-total">{fmtNumber(total30)} total</span>
                    </div>
                    <div className="dl-trend-chart">
                      {dlTrend.map(d => (
                        <div key={d.day} className="dl-trend-bar-wrap" title={`${d.day}: ${fmtNumber(d.count)}`}>
                          <div
                            className="dl-trend-bar"
                            style={{ height: `${Math.max(2, Math.round((d.count / maxCount) * 60))}px` }}
                          />
                        </div>
                      ))}
                    </div>
                    <div className="dl-trend-axis">
                      <span>{dlTrend[0]?.day}</span>
                      <span>{dlTrend[dlTrend.length - 1]?.day}</span>
                    </div>
                  </div>
                )
              })()}

              {/* Downloads by version bar chart */}
              {versions.some(v => v.downloads > 0) && (
                <div className="dl-chart-section">
                  <div className="dl-chart-title">Downloads by version</div>
                  <div className="dl-chart">
                    {(() => {
                      const max = Math.max(...versions.map(v => v.downloads || 0), 1)
                      return versions.slice(0, 10).map(v => (
                        <div key={v.id} className="dl-bar-row">
                          <div className="dl-bar-label" title={v.version}>{v.version}</div>
                          <div className="dl-bar-track">
                            <div
                              className="dl-bar-fill"
                              style={{ width: `${((v.downloads || 0) / max) * 100}%` }}
                            />
                          </div>
                          <div className="dl-bar-count">{fmtNumber(v.downloads || 0)}</div>
                        </div>
                      ))
                    })()}
                  </div>
                </div>
              )}
              <div className="activity-section-title">Version history</div>
              <div className="activity-timeline">
                {versions.map(v => (
                  <div key={v.id} className="activity-item">
                    <div className="activity-dot" />
                    <div className="activity-body">
                      <div className="activity-line">
                        <strong className="activity-version">{v.version}</strong>
                        {v.version === artifact.latest_version && (
                          <span className="tag tag-latest">latest</span>
                        )}
                        {v.yanked && <span className="badge badge-unsigned">yanked</span>}
                        {v.deprecated && <span className="badge badge-pending">deprecated</span>}
                      </div>
                      <div className="activity-meta">{timeAgo(v.created_at)}{v.created_by && ` · by ${v.created_by}`}</div>
                      {v.size_bytes > 0 && (
                        <div className="activity-detail">{fmtBytes(v.size_bytes)}</div>
                      )}
                      {v.release_notes ? (
                        <div className="activity-release-notes">{v.release_notes}</div>
                      ) : isOwner ? (
                        <button
                          className="activity-add-notes-btn"
                          onClick={() => { setReleaseNotesEditing(v.version); setReleaseNotesDraft(v.release_notes || '') }}
                        >+ Add release notes</button>
                      ) : null}
                      {releaseNotesEditing === v.version && (
                        <div className="release-notes-editor">
                          <textarea
                            className="release-notes-textarea"
                            value={releaseNotesDraft}
                            onChange={e => setReleaseNotesDraft(e.target.value)}
                            placeholder="What changed in this version?"
                            rows={3}
                            autoFocus
                          />
                          <div className="release-notes-actions">
                            <button
                              className="btn-primary"
                              style={{ fontSize: 12, padding: '4px 10px' }}
                              disabled={releaseNotesSaving}
                              onClick={async () => {
                                setReleaseNotesSaving(true)
                                try {
                                  await patchArtifactVersion(kind, namespace, name, v.version, { release_notes: releaseNotesDraft })
                                  setData(d => ({
                                    ...d,
                                    versions: d.versions.map(vv => vv.version === v.version ? { ...vv, release_notes: releaseNotesDraft } : vv)
                                  }))
                                  setReleaseNotesEditing(null)
                                  toast('Release notes saved', 'success')
                                } catch (err) {
                                  toast(err.message, 'error')
                                } finally {
                                  setReleaseNotesSaving(false)
                                }
                              }}
                            >{releaseNotesSaving ? 'Saving…' : 'Save'}</button>
                            <button
                              className="btn-secondary"
                              style={{ fontSize: 12, padding: '4px 10px' }}
                              onClick={() => setReleaseNotesEditing(null)}
                            >Cancel</button>
                          </div>
                        </div>
                      )}
                    </div>
                  </div>
                ))}
              </div>

              {promotions.length > 0 && (
                <>
                  <div className="activity-section-title" style={{ marginTop: 28 }}>
                    Channel promotions
                  </div>
                  <div className="activity-timeline">
                    {promotions.map((p, i) => (
                      <div key={i} className="activity-item">
                        <div className="activity-dot activity-dot-promo" />
                        <div className="activity-body">
                          <div className="activity-line">
                            <strong>{p.version}</strong>
                            <span className="activity-arrow">→</span>
                            <span className="tag">{p.channel || p.tag}</span>
                          </div>
                          <div className="activity-meta">
                            {p.promoted_by && `by ${p.promoted_by} · `}
                            {timeAgo(p.promoted_at || p.created_at)}
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                </>
              )}

              {versions.length === 0 && promotions.length === 0 && (
                <div className="empty-state" style={{ padding: '40px 0' }}>
                  <p>No activity yet.</p>
                </div>
              )}
            </div>
          )}
          {/* ── Settings (owner-only) ── */}
          {tab === 'settings' && isOwner && settingsForm && (
            <div className="settings-tab-panel">
              <div className="settings-tab-section">
                <h3 className="settings-tab-title">Artifact Settings</h3>

                <div className="field-group">
                  <label>Short description</label>
                  <input
                    type="text"
                    value={settingsForm.description}
                    onChange={e => setSettingsForm(f => ({ ...f, description: e.target.value }))}
                    placeholder="One-line description shown in search results"
                    maxLength={200}
                  />
                  <div className="field-hint">{settingsForm.description.length}/200 characters</div>
                </div>

                <div className="field-group">
                  <label>Tags</label>
                  <div className="tag-editor">
                    {settingsForm.tags.map(t => (
                      <span key={t} className="tag-editor-chip">
                        {t}
                        <button
                          className="tag-editor-remove"
                          onClick={() => setSettingsForm(f => ({ ...f, tags: f.tags.filter(x => x !== t) }))}
                        >×</button>
                      </span>
                    ))}
                    <input
                      className="tag-editor-input"
                      value={tagInput}
                      onChange={e => setTagInput(e.target.value)}
                      onKeyDown={e => {
                        if ((e.key === 'Enter' || e.key === ',') && tagInput.trim()) {
                          e.preventDefault()
                          const t = tagInput.trim().replace(/,$/, '').toLowerCase()
                          if (t && !settingsForm.tags.includes(t)) {
                            setSettingsForm(f => ({ ...f, tags: [...f.tags, t] }))
                          }
                          setTagInput('')
                        } else if (e.key === 'Backspace' && !tagInput && settingsForm.tags.length > 0) {
                          setSettingsForm(f => ({ ...f, tags: f.tags.slice(0, -1) }))
                        }
                      }}
                      placeholder={settingsForm.tags.length === 0 ? 'Add tags (press Enter)' : ''}
                    />
                  </div>
                  <div className="field-hint">Press Enter or comma to add a tag</div>
                </div>

                <div className="field-group">
                  <label>Visibility</label>
                  <div className="vis-radio-group">
                    {['public', 'private'].map(v => (
                      <label key={v} className="vis-radio-label">
                        <input
                          type="radio"
                          name="visibility"
                          value={v}
                          checked={settingsForm.visibility === v}
                          onChange={() => setSettingsForm(f => ({ ...f, visibility: v }))}
                        />
                        <span className="vis-radio-text">
                          <strong>{v.charAt(0).toUpperCase() + v.slice(1)}</strong>
                          <span className="vis-radio-hint">
                            {v === 'public' ? 'Anyone can discover and install this artifact' : 'Only namespace members can access this artifact'}
                          </span>
                        </span>
                      </label>
                    ))}
                  </div>
                </div>

                <div className="form-actions">
                  <button className="btn-primary" onClick={doSaveSettings} disabled={settingsSaving}>
                    {settingsSaving ? 'Saving…' : 'Save settings'}
                  </button>
                </div>
              </div>

              <div className="settings-tab-danger">
                <h3 className="settings-tab-danger-title">Danger zone</h3>
                <div className="danger-zone-row">
                  <div>
                    <div className="danger-zone-label">Deprecate all versions</div>
                    <div className="danger-zone-hint">Mark every version as deprecated. Users will see warnings when installing.</div>
                  </div>
                  <button
                    className="btn-danger"
                    onClick={async () => {
                      const ok = await confirm(`Deprecate all versions of ${name}?`, { confirmLabel: 'Deprecate all', danger: true })
                      if (!ok) return
                      try {
                        await Promise.all(versions.map(v => deprecateVersion(namespace, name, v.version, '')))
                        const updated = await fetchArtifactDetails(kind, namespace, name)
                        setData(updated)
                        toast('All versions deprecated', 'success')
                      } catch (err) { toast(err.message, 'error') }
                    }}
                  >Deprecate all</button>
                </div>
                <div className="danger-zone-row danger-zone-row-col">
                  <div>
                    <div className="danger-zone-label">Transfer artifact</div>
                    <div className="danger-zone-hint">Move this artifact to another namespace. All versions and download history will transfer. This cannot be undone.</div>
                  </div>
                  <div className="transfer-form">
                    <input
                      className="transfer-input"
                      placeholder="Target namespace"
                      value={transferTarget}
                      onChange={e => setTransferTarget(e.target.value)}
                    />
                    <button
                      className="btn-danger"
                      disabled={transferring || !transferTarget.trim()}
                      onClick={async () => {
                        const ok = await confirm(`Transfer ${name} to "${transferTarget}"? This cannot be undone.`, { confirmLabel: 'Transfer', danger: true })
                        if (!ok) return
                        setTransferring(true)
                        try {
                          await transferArtifact(kind, namespace, name, transferTarget.trim())
                          toast(`Transferred to ${transferTarget}`, 'success')
                          navigate(`/artifacts/${kind}/${transferTarget.trim()}/${name}`)
                        } catch (err) { toast(err.message, 'error') }
                        finally { setTransferring(false) }
                      }}
                    >{transferring ? 'Transferring…' : 'Transfer'}</button>
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* ── Security ── */}
          {tab === 'security' && (
            <div className="security-panel">
              <div className="security-header">
                <h3 className="security-title">Security scan</h3>
                <span className="security-sub">Results for latest version ({data?.artifact?.latest_version || 'unknown'})</span>
              </div>
              {scanResults === null ? (
                <div className="loading-state">Loading scan results…</div>
              ) : scanResults.length === 0 ? (
                <div className="security-clean">
                  <div className="security-clean-icon">✓</div>
                  <div className="security-clean-msg">No vulnerabilities found</div>
                  <div className="security-clean-sub">This version passed the security scan.</div>
                </div>
              ) : (
                <>
                  <div className="security-summary-row">
                    {['critical', 'high', 'medium', 'low'].map(sev => (
                      scanSummary?.[sev] > 0 && (
                        <span key={sev} className={`sec-sev-badge sec-sev-${sev}`}>
                          {scanSummary[sev]} {sev}
                        </span>
                      )
                    ))}
                  </div>
                  <table className="scan-table">
                    <thead>
                      <tr>
                        <th>Severity</th>
                        <th>CVE</th>
                        <th>Package</th>
                        <th>Description</th>
                        <th>Fixed in</th>
                      </tr>
                    </thead>
                    <tbody>
                      {scanResults.map(r => (
                        <tr key={r.id} className={`scan-row-${r.severity}`}>
                          <td><span className={`sec-sev-badge sec-sev-${r.severity}`}>{r.severity}</span></td>
                          <td><code className="cve-id">{r.cve_id || '—'}</code></td>
                          <td className="scan-pkg">{r.package_name || '—'}</td>
                          <td className="scan-desc">{r.description || '—'}</td>
                          <td className="scan-fix">{r.fixed_version || 'No fix'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </>
              )}
            </div>
          )}

          {/* ── Discussion ── */}
          {tab === 'comments' && (
            <div className="comments-panel">
              <h3 className="comments-title">Discussion</h3>
              {loggedInUser ? (
                <div className="comment-form">
                  <div className="comment-form-avatar" style={{ background: avatarColor(loggedInUser) }}>
                    {loggedInUser[0].toUpperCase()}
                  </div>
                  <div className="comment-form-body">
                    <textarea
                      className="comment-textarea"
                      placeholder="Leave a comment…"
                      value={commentDraft}
                      onChange={e => setCommentDraft(e.target.value)}
                      rows={3}
                      maxLength={4000}
                    />
                    <div className="comment-form-actions">
                      <span className="comment-char-count">{commentDraft.length}/4000</span>
                      <button
                        className="btn-primary"
                        disabled={commentPosting || !commentDraft.trim()}
                        onClick={async () => {
                          if (!commentDraft.trim()) return
                          setCommentPosting(true)
                          try {
                            const c = await addComment(kind, namespace, name, commentDraft.trim())
                            setComments(cs => [...cs, c])
                            setCommentDraft('')
                          } catch (err) { toast(err.message, 'error') }
                          finally { setCommentPosting(false) }
                        }}
                      >
                        {commentPosting ? 'Posting…' : 'Comment'}
                      </button>
                    </div>
                  </div>
                </div>
              ) : (
                <div className="comment-login-prompt">
                  <Link to="/login" className="btn-primary">Sign in to comment</Link>
                </div>
              )}
              {comments.length === 0 ? (
                <div className="comments-empty">
                  <p>No comments yet. Be the first to start the discussion.</p>
                </div>
              ) : (
                <div className="comments-list">
                  {comments.map(c => (
                    <div key={c.id} className="comment-item">
                      <div className="comment-avatar" style={{ background: avatarColor(c.username) }}>
                        {c.username[0].toUpperCase()}
                      </div>
                      <div className="comment-body">
                        <div className="comment-meta">
                          <Link to={`/namespace/${c.username}`} className="comment-author">{c.username}</Link>
                          <span className="comment-time">{timeAgo(c.created_at)}</span>
                          {c.updated_at !== c.created_at && <span className="comment-edited">(edited)</span>}
                        </div>
                        {editingComment === c.id ? (
                          <div className="comment-edit-form">
                            <textarea
                              className="comment-textarea"
                              value={editCommentDraft}
                              onChange={e => setEditCommentDraft(e.target.value)}
                              rows={3}
                              autoFocus
                            />
                            <div className="comment-form-actions">
                              <button className="btn-primary" onClick={async () => {
                                try {
                                  await updateComment(c.id, editCommentDraft.trim())
                                  setComments(cs => cs.map(x => x.id === c.id ? { ...x, body: editCommentDraft.trim(), updated_at: new Date().toISOString() } : x))
                                  setEditingComment(null)
                                } catch (err) { toast(err.message, 'error') }
                              }}>Save</button>
                              <button className="btn-secondary" onClick={() => setEditingComment(null)}>Cancel</button>
                            </div>
                          </div>
                        ) : (
                          <div className="comment-text">{c.body}</div>
                        )}
                        {loggedInUser === c.username && editingComment !== c.id && (
                          <div className="comment-actions">
                            <button className="comment-action-btn" onClick={() => { setEditingComment(c.id); setEditCommentDraft(c.body) }}>Edit</button>
                            <button className="comment-action-btn comment-action-delete" onClick={async () => {
                              const ok = await confirm('Delete this comment?', { confirmLabel: 'Delete', danger: true })
                              if (!ok) return
                              try {
                                await deleteComment(c.id)
                                setComments(cs => cs.filter(x => x.id !== c.id))
                              } catch (err) { toast(err.message, 'error') }
                            }}>Delete</button>
                          </div>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>

        {/* ── Sidebar ── */}
        <aside className="detail-aside">
          {/* Install command */}
          <div className="install-panel">
            <div className="install-panel-header">Install</div>
            <div className="install-cmd">
              <code>{installCmdLatest}</code>
              <CopyButton text={installCmdLatest} />
            </div>
            <div className="install-formats">
              {[
                { label: 'CLI', text: installCmdLatest },
                { label: 'YAML', text: `requires:\n  - ${namespace}/${name}@${artifact.latest_version || 'latest'}` },
                { label: 'Python', text: `registry.load("${namespace}/${name}@${artifact.latest_version || 'latest'}")` },
              ].map(f => (
                <CopyButton key={f.label} text={f.text} label={f.label} mini />
              ))}
            </div>
            <div className="install-hint">
              Using{' '}
              <a href="/api-docs" target="_blank" rel="noopener noreferrer">skforge CLI</a>
            </div>
          </div>

          {/* Share panel */}
          <div className="share-panel">
            <div className="share-panel-label">Share</div>
            <div className="share-panel-btns">
              <button
                className="share-btn"
                title="Copy link"
                onClick={shareUrl}
              >
                🔗 Copy link
              </button>
              <a
                className="share-btn"
                href={`https://twitter.com/intent/tweet?text=${encodeURIComponent(`Check out ${namespace}/${name} on SkillForge — ${artifact.description || 'an AI skill/agent'}`)}&url=${encodeURIComponent(window.location.href)}`}
                target="_blank"
                rel="noopener noreferrer"
              >
                𝕏 Post
              </a>
            </div>
          </div>

          {/* Latest version summary (Docker Hub sidebar tag card) */}
          {latest && (
            <div className="tag-summary-panel">
              <div className="tag-summary-header">
                <span className="info-panel-title" style={{ marginBottom: 0 }}>Latest version</span>
                <span className="tag tag-latest">latest</span>
              </div>
              <div className="tag-summary-version">{latest.version}</div>
              {latest.created_at && (
                <div className="tag-summary-pushed">
                  Last pushed {timeAgo(latest.created_at)}
                </div>
              )}
              <div className="tag-summary-divider" />
              <div className="tag-summary-rows">
                <div className="tag-summary-row">
                  <span className="tag-summary-label">Content type</span>
                  <span className="tag-summary-val">{kind}</span>
                </div>
                {latest.digest_sha256 && (
                  <div className="tag-summary-row">
                    <span className="tag-summary-label">Digest</span>
                    <div className="tag-summary-digest">
                      <span
                        className="digest-mono"
                        title={`sha256:${latest.digest_sha256}`}
                      >
                        sha256:{latest.digest_sha256.slice(0, 12)}…
                      </span>
                      <CopyButton
                        text={`sha256:${latest.digest_sha256}`}
                        label="⎘"
                        mini
                      />
                    </div>
                  </div>
                )}
                {latest.size_bytes > 0 && (
                  <div className="tag-summary-row">
                    <span className="tag-summary-label">Compressed size</span>
                    <span className="tag-summary-val">{fmtBytes(latest.size_bytes)}</span>
                  </div>
                )}
              </div>
            </div>
          )}

          {/* Stats */}
          <div className="info-panel">
            <div className="info-panel-title">Stats</div>
            <div className="pulls-stat-card">
              <div>
                <span className="pulls-big">{fmtNumber(artifact.downloads || 0)}</span>
                <span className="pulls-label">total pulls</span>
              </div>
              <Sparkline seed={artifact.downloads || 0} versions={versions.length} />
            </div>
            <div className="info-row">
              <span className="info-label">Versions</span>
              <span className="info-value">{versions.length}</span>
            </div>
            <div className="info-row">
              <span className="info-label">Visibility</span>
              <span className={`info-value vis-${artifact.visibility || 'public'}`}>
                {artifact.visibility || 'public'}
              </span>
            </div>
            {dependents.length > 0 && (
              <div className="info-row">
                <span className="info-label">Used by</span>
                <span className="info-value used-by-count">{dependents.length}</span>
              </div>
            )}
          </div>

          {/* Download */}
          {artifact.latest_version && (
            <div className="info-panel">
              <div className="info-panel-title">Download</div>
              <a
                href={`/api/v1/artifacts/${kind}/${namespace}/${name}/versions/${artifact.latest_version}/download`}
                className="dl-direct-btn"
                download
              >
                ⬇ Download latest ({artifact.latest_version})
              </a>
              {artifact.downloads > 0 && (
                <div className="dl-direct-note">⬇ {fmtNumber(artifact.downloads)} total pulls</div>
              )}
            </div>
          )}

          {/* Used by */}
          {dependents.length > 0 && (
            <div className="info-panel">
              <div className="info-panel-title-row">
                <span className="info-panel-title" style={{ marginBottom: 0 }}>Used by</span>
                <span className="info-panel-count">{dependents.length}</span>
              </div>
              <div className="more-from-list">
                {dependents.slice(0, 5).map(d => (
                  <Link
                    key={`${d.kind}/${d.namespace}/${d.name}`}
                    to={`/artifacts/${d.kind}/${d.namespace}/${d.name}`}
                    className="more-from-item"
                  >
                    <div className={`more-from-avatar ${kindClass(d.kind)}`} style={{ background: avatarColor(d.namespace) }}>
                      {d.namespace[0].toUpperCase()}
                    </div>
                    <div className="more-from-body">
                      <div className="more-from-name">{d.name}</div>
                      <div className="more-from-meta">
                        <span className="more-from-ns">{d.namespace}</span>
                        <span className="kind-chip kind-chip-sm">{d.kind}</span>
                      </div>
                    </div>
                  </Link>
                ))}
              </div>
            </div>
          )}

          {/* Publisher */}
          <div className="info-panel">
            <div className="info-panel-title">Publisher</div>
            <Link to={`/namespace/${namespace}`} className="publisher-link">
              <div className="publisher-card">
                <div className="publisher-avatar" style={{ background: color }}>
                  {namespace[0].toUpperCase()}
                </div>
                <div className="publisher-info">
                  <span className="publisher-name">{namespace}</span>
                  <span className="publisher-role">Publisher</span>
                </div>
              </div>
            </Link>
          </div>

          {/* Distribution channels */}
          {Object.keys(distTags).length > 0 && (
            <div className="info-panel">
              <div className="info-panel-title">Channels</div>
              {Object.entries(distTags).map(([tag, ver]) => (
                <div className="info-row" key={tag}>
                  <span className="tag">{tag}</span>
                  <span className="info-value info-mono">{ver}</span>
                </div>
              ))}
            </div>
          )}

          {/* Maintainers */}
          {(artifact.owners || []).length > 0 && (
            <div className="info-panel">
              <div className="info-panel-title">Maintainers</div>
              {(artifact.owners || []).map(o => (
                <div className="maintainer-row" key={o}>
                  <div className="maintainer-avatar" style={{ background: avatarColor(o) }}>
                    {o[0].toUpperCase()}
                  </div>
                  <span className="maintainer-name">{o}</span>
                </div>
              ))}
            </div>
          )}

          {/* More from this publisher */}
          {publisherArtifacts.length > 0 && (
            <div className="info-panel">
              <div className="info-panel-title-row">
                <span className="info-panel-title" style={{ marginBottom: 0 }}>More from {namespace}</span>
                <Link to={`/namespace/${namespace}`} className="info-panel-viewall">View all</Link>
              </div>
              <div className="more-from-list">
                {publisherArtifacts.map(a => (
                  <Link
                    key={`${a.kind}/${a.name}`}
                    to={`/artifacts/${a.kind}/${a.namespace}/${a.name}`}
                    className="more-from-item"
                  >
                    <div className={`more-from-avatar ${kindClass(a.kind)}`} style={{ background: avatarColor(a.namespace) }}>
                      {a.namespace[0].toUpperCase()}
                    </div>
                    <div className="more-from-body">
                      <div className="more-from-name">{a.name}</div>
                      <div className="more-from-meta">
                        <span className="kind-chip kind-chip-sm">{a.kind}</span>
                        {a.downloads > 0 && (
                          <span className="more-from-pulls">⬇ {fmtNumber(a.downloads)}</span>
                        )}
                      </div>
                    </div>
                  </Link>
                ))}
              </div>
            </div>
          )}
          {/* Similar artifacts */}
          {similarArtifacts.length > 0 && (
            <div className="info-panel">
              <div className="info-panel-title-row">
                <span className="info-panel-title" style={{ marginBottom: 0 }}>Similar {kind}s</span>
                <Link to={`/search?kind=${kind}`} className="info-panel-viewall">Browse all</Link>
              </div>
              <div className="more-from-list">
                {similarArtifacts.map(a => (
                  <Link
                    key={`${a.kind}/${a.namespace}/${a.name}`}
                    to={`/artifacts/${a.kind}/${a.namespace}/${a.name}`}
                    className="more-from-item"
                  >
                    <div className={`more-from-avatar ${kindClass(a.kind)}`} style={{ background: avatarColor(a.namespace) }}>
                      {a.namespace[0].toUpperCase()}
                    </div>
                    <div className="more-from-body">
                      <div className="more-from-name">{a.name}</div>
                      <div className="more-from-meta">
                        <span className="more-from-ns">{a.namespace}</span>
                        {a.downloads > 0 && (
                          <span className="more-from-pulls">⬇ {fmtNumber(a.downloads)}</span>
                        )}
                      </div>
                    </div>
                  </Link>
                ))}
              </div>
            </div>
          )}
        </aside>
      </div>
    </>
  )
}
