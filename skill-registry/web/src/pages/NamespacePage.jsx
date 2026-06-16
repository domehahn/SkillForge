import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { fetchNamespace, listMembers, upsertMember, removeMember, patchNamespace, fetchAuditLog, fetchPinned, setPinned, fetchFollowInfo, followNamespace, unfollowNamespace, fetchCollections } from '../api/client'
import { getStarred } from '../hooks/useStarred'
import ArtifactCard from '../components/ArtifactCard'
import { SkeletonCards } from '../components/Skeleton'
import { avatarColor, fmtNumber, timeAgo } from '../utils'
import { useDocumentTitle } from '../hooks/useDocumentTitle'
import { useToast } from '../contexts/ToastContext'
import { useConfirm } from '../contexts/ConfirmContext'

const PAGE_SIZE = 20

const ROLES = ['owner', 'member', 'viewer']

const NS_ACT_ICON = { publish: '⬆', delete: '🗑', yank: '⚠', unyank: '✓', deprecate: '⚠' }
const NS_ACT_VERB = { publish: 'published', delete: 'deleted', yank: 'yanked', unyank: 'un-yanked', deprecate: 'deprecated' }

function NsActivityTab({ namespace }) {
  const [entries, setEntries] = useState([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchAuditLog({ limit: 50, offset: 0 })
      .then(d => {
        const all = d.entries || []
        // Filter to entries where resource starts with namespace/
        const ns = all.filter(e => {
          const res = e.resource || [e.namespace, e.name].filter(Boolean).join('/')
          return res.startsWith(namespace + '/')
        })
        setEntries(ns)
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [namespace])

  if (loading) return <div className="loading-state">Loading activity…</div>
  if (entries.length === 0) return (
    <div className="empty-state">
      <h3>No activity yet</h3>
      <p>Publish an artifact to see activity here.</p>
    </div>
  )

  return (
    <div className="activity-feed-list">
      {entries.map((e, i) => {
        const resource = e.resource || [e.namespace, e.name].filter(Boolean).join('/')
        const parts = (resource || '').split('/')
        const artifactName = parts.slice(1).join('/') || resource
        return (
          <div key={e.id || i} className="activity-feed-item">
            <div className="activity-feed-icon">{NS_ACT_ICON[e.action] || '·'}</div>
            <div className="activity-feed-body">
              <div className="activity-feed-line">
                <span className="activity-feed-verb">{NS_ACT_VERB[e.action] || e.action}</span>
                <span className="activity-feed-artifact">{artifactName}</span>
                {e.version && <code className="activity-feed-version">@{e.version}</code>}
              </div>
              {e.message && <div className="activity-feed-detail">{e.message}</div>}
            </div>
            <div className="activity-feed-time">{(e.created_at || e.timestamp) ? timeAgo(e.created_at || e.timestamp) : ''}</div>
          </div>
        )
      })}
    </div>
  )
}

function MembersTab({ namespace, isOwner }) {
  const [members, setMembers] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [form, setForm] = useState({ username: '', role: 'member' })
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState(null)
  const toast = useToast()
  const confirm = useConfirm()

  useEffect(() => {
    setLoading(true)
    listMembers(namespace)
      .then(d => setMembers(d.members || d || []))
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }, [namespace])

  const handleAdd = async (e) => {
    e.preventDefault()
    setSaveError(null)
    setSaving(true)
    try {
      const m = await upsertMember(namespace, form.username, form.role)
      setMembers(prev => {
        const idx = prev.findIndex(x => x.username === form.username)
        if (idx >= 0) { const next = [...prev]; next[idx] = m; return next }
        return [...prev, m]
      })
      setForm({ username: '', role: 'member' })
      toast(`${form.username} added as ${form.role}`, 'success')
    } catch (err) {
      setSaveError(err.message)
    } finally {
      setSaving(false)
    }
  }

  const handleRemove = async (username) => {
    const ok = await confirm(`Remove ${username} from ${namespace}?`, { confirmLabel: 'Remove', danger: true })
    if (!ok) return
    try {
      await removeMember(namespace, username)
      setMembers(prev => prev.filter(m => m.username !== username))
      toast(`${username} removed`, 'success')
    } catch (err) {
      toast(err.message, 'error')
    }
  }

  return (
    <div className="ns-members-tab">
      {isOwner && (
        <form className="ns-member-form" onSubmit={handleAdd}>
          <input
            className="ns-member-input"
            placeholder="Username"
            value={form.username}
            onChange={e => setForm(f => ({ ...f, username: e.target.value }))}
            required
          />
          <select
            className="sort-select"
            value={form.role}
            onChange={e => setForm(f => ({ ...f, role: e.target.value }))}
          >
            {ROLES.map(r => <option key={r} value={r}>{r}</option>)}
          </select>
          <button type="submit" className="btn-primary" disabled={saving}>
            {saving ? 'Adding…' : 'Add member'}
          </button>
          {saveError && <div className="auth-error" style={{ marginTop: 8 }}>{saveError}</div>}
        </form>
      )}

      {error && <div className="error-state">{error}</div>}
      {loading ? (
        <div className="loading-state">Loading members…</div>
      ) : members.length === 0 ? (
        <div className="empty-state">
          <h3>No members yet</h3>
          <p>Add members to give them access to this namespace.</p>
        </div>
      ) : (
        <table className="members-table">
          <thead>
            <tr>
              <th>User</th>
              <th>Role</th>
              <th>Added</th>
              {isOwner && <th></th>}
            </tr>
          </thead>
          <tbody>
            {members.map(m => (
              <tr key={m.username}>
                <td>
                  <div className="member-row">
                    <div className="member-avatar" style={{ background: avatarColor(m.username) }}>
                      {m.username[0].toUpperCase()}
                    </div>
                    <Link to={`/namespace/${m.username}`} className="member-name">{m.username}</Link>
                  </div>
                </td>
                <td>
                  <span className={`role-badge role-${m.role}`}>{m.role}</span>
                </td>
                <td className="audit-time">{m.created_at ? new Date(m.created_at).toLocaleDateString() : '–'}</td>
                {isOwner && (
                  <td>
                    <button className="revoke-btn" onClick={() => handleRemove(m.username)}>
                      Remove
                    </button>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

export default function NamespacePage() {
  const { namespace } = useParams()
  useDocumentTitle(namespace)

  const [data, setData] = useState(null)
  const [error, setError] = useState(null)
  const [page, setPage] = useState(1)
  const [activeTab, setActiveTab] = useState('artifacts')
  const [sort, setSort] = useState('updated')
  const [nsSearch, setNsSearch] = useState('')
  const [kindFilter, setKindFilter] = useState('')

  const loggedInUser = localStorage.getItem('sf_user')
  const isOwner = loggedInUser === namespace
  const [bioEditing, setBioEditing] = useState(false)
  const [bioDraft, setBioDraft] = useState('')
  const [bioSaving, setBioSaving] = useState(false)
  const [starredItems, setStarredItems] = useState(() => getStarred())
  const [pinned, setPinnedState] = useState([])
  const [pinSaving, setPinSaving] = useState(false)
  const [followInfo, setFollowInfo] = useState({ following: false, followers: 0 })
  const [nsCols, setNsCols] = useState([])
  const [followBusy, setFollowBusy] = useState(false)
  const toast = useToast()

  useEffect(() => {
    const sync = () => setStarredItems(getStarred())
    window.addEventListener('sf_starred', sync)
    return () => window.removeEventListener('sf_starred', sync)
  }, [])

  useEffect(() => {
    setData(null)
    setPage(1)
    fetchNamespace(namespace)
      .then(setData)
      .catch(err => setError(err.message))
    fetchPinned(namespace)
      .then(d => setPinnedState(d.pinned || []))
      .catch(() => {})
    fetchFollowInfo(namespace)
      .then(d => setFollowInfo({ following: d.following || false, followers: d.followers || 0 }))
      .catch(() => {})
    fetchCollections(namespace)
      .then(d => setNsCols((d.collections || []).filter(c => c.visibility === 'public').slice(0, 3)))
      .catch(() => {})
  }, [namespace])

  const togglePin = async (artifactName) => {
    if (pinSaving) return
    const next = pinned.includes(artifactName)
      ? pinned.filter(n => n !== artifactName)
      : pinned.length >= 6
        ? (toast('You can pin up to 6 artifacts', 'error'), pinned)
        : [...pinned, artifactName]
    if (next === pinned) return
    setPinSaving(true)
    try {
      await setPinned(namespace, next)
      setPinnedState(next)
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setPinSaving(false)
    }
  }

  const toggleFollow = async () => {
    if (!loggedInUser) { toast('Sign in to follow publishers', 'error'); return }
    if (followBusy) return
    setFollowBusy(true)
    try {
      const fn = followInfo.following ? unfollowNamespace : followNamespace
      const d = await fn(namespace)
      setFollowInfo({ following: d.following, followers: d.followers })
    } catch (err) { toast(err.message, 'error') }
    finally { setFollowBusy(false) }
  }

  const bio = data?.bio || ''

  const saveBio = async () => {
    setBioSaving(true)
    try {
      await patchNamespace(namespace, { bio: bioDraft })
      setData(d => ({ ...d, bio: bioDraft }))
      setBioEditing(false)
      toast('Bio updated', 'success')
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setBioSaving(false)
    }
  }

  const color = avatarColor(namespace)
  const allArtifacts = data?.artifacts || []

  const filteredArtifacts = allArtifacts.filter(a => {
    if (kindFilter && a.kind !== kindFilter) return false
    if (nsSearch.trim()) {
      return a.name.toLowerCase().includes(nsSearch.toLowerCase()) ||
        (a.description || '').toLowerCase().includes(nsSearch.toLowerCase())
    }
    return true
  })

  const kindCounts = allArtifacts.reduce((acc, a) => {
    acc[a.kind] = (acc[a.kind] || 0) + 1
    return acc
  }, {})

  const sortedArtifacts = [...filteredArtifacts].sort((a, b) => {
    if (sort === 'pulls') return (b.downloads || 0) - (a.downloads || 0)
    if (sort === 'stars') return (b.stars || 0) - (a.stars || 0)
    if (sort === 'name') return a.name.localeCompare(b.name)
    return new Date(b.updated_at || 0) - new Date(a.updated_at || 0)
  })

  const totalPages = Math.ceil(sortedArtifacts.length / PAGE_SIZE)
  const pagedArtifacts = sortedArtifacts.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)

  return (
    <>
      {/* Profile hero */}
      <div className="namespace-hero">
        <div className="namespace-hero-inner">
          <div className="namespace-avatar" style={{ background: color }}>
            {namespace[0].toUpperCase()}
          </div>
          <div className="namespace-info">
            <h1>
              {data?.display_name || namespace}
              {data?.display_name && <span className="ns-username-sub">@{namespace}</span>}
              {data?.verified && (
                <span className="ns-verified-badge" title="Verified publisher">✓</span>
              )}
            </h1>
            {bioEditing ? (
              <div className="ns-bio-edit">
                <textarea
                  className="ns-bio-textarea"
                  value={bioDraft}
                  onChange={e => setBioDraft(e.target.value)}
                  placeholder="Tell people about this namespace…"
                  rows={3}
                  maxLength={300}
                  autoFocus
                />
                <div className="ns-bio-actions">
                  <button className="btn-primary" onClick={saveBio} disabled={bioSaving}>
                    {bioSaving ? 'Saving…' : 'Save'}
                  </button>
                  <button className="btn-secondary" onClick={() => setBioEditing(false)}>Cancel</button>
                </div>
              </div>
            ) : (
              <div className="ns-bio-row">
                <p className="ns-bio-text">{bio || (isOwner ? '' : 'No bio yet.')}</p>
                {isOwner && (
                  <button
                    className="ns-bio-edit-btn"
                    onClick={() => { setBioDraft(bio); setBioEditing(true) }}
                    title="Edit bio"
                  >✎</button>
                )}
              </div>
            )}
            {data && (
              <div className="namespace-stats">
                <span className="namespace-stat">
                  <strong>{fmtNumber(allArtifacts.length)}</strong> artifacts
                </span>
                <span className="ns-stat-sep">·</span>
                <span className="namespace-stat">
                  <strong>{fmtNumber(data.total_pulls || 0)}</strong> total pulls
                </span>
                {data.member_count > 0 && (
                  <>
                    <span className="ns-stat-sep">·</span>
                    <span className="namespace-stat">
                      <strong>{fmtNumber(data.member_count)}</strong> {data.member_count === 1 ? 'member' : 'members'}
                    </span>
                  </>
                )}
                {followInfo.followers > 0 && (
                  <>
                    <span className="ns-stat-sep">·</span>
                    <span className="namespace-stat">
                      <strong>{fmtNumber(followInfo.followers)}</strong> follower{followInfo.followers !== 1 ? 's' : ''}
                    </span>
                  </>
                )}
              </div>
            )}
            {data && (() => {
              const pulls = data.total_pulls || 0
              const milestones = [1000000, 500000, 100000, 50000, 10000, 1000]
              const ms = milestones.find(m => pulls >= m)
              if (!ms) return null
              const labels = { 1000000: '1M+', 500000: '500K+', 100000: '100K+', 50000: '50K+', 10000: '10K+', 1000: '1K+' }
              return <div className="ns-milestone-badge">🏅 {labels[ms]} pulls</div>
            })()}
          </div>
          {!isOwner && (
            <div className="namespace-actions">
              <button
                className={`follow-btn ${followInfo.following ? 'following' : ''}`}
                onClick={toggleFollow}
                disabled={followBusy}
              >
                {followInfo.following ? '✓ Following' : '+ Follow'}
              </button>
              {followInfo.followers > 0 && (
                <span className="follow-count">{fmtNumber(followInfo.followers)} follower{followInfo.followers !== 1 ? 's' : ''}</span>
              )}
            </div>
          )}
          {isOwner && (
            <div className="namespace-actions">
              <Link to={`/namespace/${namespace}/insights`} className="btn-secondary">Insights</Link>
              <Link to={`/namespace/${namespace}/collections`} className="btn-secondary">Collections</Link>
              <Link to={`/namespace/${namespace}/settings`} className="btn-secondary">Settings</Link>
              <Link to="/account/tokens" className="btn-secondary">Tokens</Link>
              <a
                href={`/api/v1/namespaces/${namespace}/feed`}
                className="btn-secondary ns-feed-btn"
                title="Atom feed for this namespace"
                target="_blank"
                rel="noopener noreferrer"
              >RSS</a>
            </div>
          )}
          {localStorage.getItem('sf_role') === 'admin' && !isOwner && (
            <div className="namespace-actions">
              <button
                className={`btn-secondary ${data?.verified ? 'btn-verified-active' : ''}`}
                title={data?.verified ? 'Remove verification' : 'Mark as verified publisher'}
                onClick={async () => {
                  try {
                    await patchNamespace(namespace, { verified: !data?.verified })
                    setData(d => ({ ...d, verified: !d.verified }))
                    toast(data?.verified ? 'Verification removed' : 'Namespace verified', 'success')
                  } catch (err) { toast(err.message, 'error') }
                }}
              >
                {data?.verified ? '✓ Verified' : 'Verify namespace'}
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="ns-tabs-bar">
        <div className="ns-tabs-inner">
          <button
            className={`ns-tab ${activeTab === 'artifacts' ? 'active' : ''}`}
            onClick={() => setActiveTab('artifacts')}
          >
            Artifacts
            {data && <span className="tab-count">{allArtifacts.length}</span>}
          </button>
          <button
            className={`ns-tab ${activeTab === 'members' ? 'active' : ''}`}
            onClick={() => setActiveTab('members')}
          >
            Members
          </button>
          <button
            className={`ns-tab ${activeTab === 'activity' ? 'active' : ''}`}
            onClick={() => setActiveTab('activity')}
          >
            Activity
          </button>
          {isOwner && (
            <>
              <button
                className={`ns-tab ${activeTab === 'starred' ? 'active' : ''}`}
                onClick={() => setActiveTab('starred')}
              >
                Starred
                {starredItems.length > 0 && <span className="tab-count">{starredItems.length}</span>}
              </button>
              <Link to="/account/tokens" className="ns-tab">
                Access Tokens
              </Link>
            </>
          )}
        </div>
      </div>

      <div className="namespace-content">
        {error && <div className="error-state">{error}</div>}
        {!data && !error && activeTab === 'artifacts' && <SkeletonCards count={4} />}

        {/* Artifacts tab */}
        {activeTab === 'artifacts' && data && (
          <>
            {/* Pinned artifacts */}
            {pinned.length > 0 && (
              <div className="pinned-section">
                <div className="pinned-header">
                  <span className="pinned-title">📌 Pinned</span>
                  {isOwner && <span className="pinned-hint">Click 📌 on any artifact to pin/unpin (max 6)</span>}
                </div>
                <div className="pinned-grid">
                  {pinned.map(name => {
                    const a = allArtifacts.find(x => x.name === name)
                    if (!a) return null
                    return (
                      <Link
                        key={name}
                        to={`/artifact/${a.kind}/${namespace}/${name}`}
                        className="pinned-card"
                      >
                        <div className="pinned-card-name">{name}</div>
                        <div className="pinned-card-desc">{a.description || 'No description'}</div>
                        <div className="pinned-card-meta">
                          <span className="kind-badge kind-{a.kind}">{a.kind}</span>
                          <span>{fmtNumber(a.downloads || 0)} pulls</span>
                        </div>
                      </Link>
                    )
                  })}
                </div>
              </div>
            )}

            {nsCols.length > 0 && (
              <div className="ns-collections-preview">
                <div className="ns-collections-header">
                  <span className="ns-collections-title">Collections</span>
                  <Link to={`/namespace/${namespace}/collections`} className="ns-collections-viewall">
                    View all →
                  </Link>
                </div>
                <div className="ns-collections-row">
                  {nsCols.map(c => (
                    <Link key={c.slug} to={`/namespace/${namespace}/collections/${c.slug}`} className="ns-col-chip">
                      <span className="ns-col-chip-name">{c.name}</span>
                      <span className="ns-col-chip-count">{c.artifacts?.length || 0}</span>
                    </Link>
                  ))}
                </div>
              </div>
            )}

            {Object.keys(kindCounts).length > 1 && (
              <div className="ns-kind-filter">
                <button
                  className={`ns-kind-btn ${!kindFilter ? 'active' : ''}`}
                  onClick={() => { setKindFilter(''); setPage(1) }}
                >
                  All <span className="ns-kind-count">{allArtifacts.length}</span>
                </button>
                {Object.entries(kindCounts).sort((a, b) => b[1] - a[1]).map(([k, c]) => (
                  <button
                    key={k}
                    className={`ns-kind-btn ${kindFilter === k ? 'active' : ''}`}
                    onClick={() => { setKindFilter(k); setPage(1) }}
                  >
                    {k} <span className="ns-kind-count">{c}</span>
                  </button>
                ))}
              </div>
            )}

            {allArtifacts.length > 0 && (
              <div className="content-toolbar" style={{ marginBottom: 16 }}>
                <input
                  className="ns-search-input"
                  placeholder={`Search in ${namespace}…`}
                  value={nsSearch}
                  onChange={e => { setNsSearch(e.target.value); setPage(1) }}
                />
                <span className="result-count" style={{ flexShrink: 0 }}>
                  {nsSearch
                    ? `${fmtNumber(filteredArtifacts.length)} of ${fmtNumber(allArtifacts.length)}`
                    : `${fmtNumber(allArtifacts.length)} artifact${allArtifacts.length !== 1 ? 's' : ''}`}
                </span>
                <select
                  className="sort-select"
                  value={sort}
                  onChange={e => { setSort(e.target.value); setPage(1) }}
                >
                  <option value="updated">Recently updated</option>
                  <option value="pulls">Most pulled</option>
                  <option value="stars">Most starred</option>
                  <option value="name">A–Z</option>
                </select>
              </div>
            )}

            {allArtifacts.length === 0 ? (
              <div className="empty-state ns-empty">
                {isOwner ? (
                  <>
                    <div className="ns-empty-icon">⬡</div>
                    <h3>You haven't published anything yet</h3>
                    <p>Use <code>skforge publish</code> to push your first skill or agent.</p>
                    <Link
                      to="/publish"
                      className="btn-primary"
                      style={{ display: 'inline-block', marginTop: 16 }}
                    >
                      View publishing guide
                    </Link>
                  </>
                ) : (
                  <>
                    <div className="ns-empty-icon">⬡</div>
                    <h3>No artifacts yet</h3>
                    <p>This publisher hasn't published anything yet.</p>
                  </>
                )}
              </div>
            ) : (
              <>
                <div className="artifacts-list">
                  {pagedArtifacts.map(a => (
                    <div key={`${a.kind}/${a.namespace}/${a.name}`} className="artifact-card-wrap">
                      <ArtifactCard artifact={a} verified={data?.verified} />
                      {isOwner && (
                        <button
                          className={`pin-btn ${pinned.includes(a.name) ? 'pinned' : ''}`}
                          title={pinned.includes(a.name) ? 'Unpin' : 'Pin'}
                          onClick={() => togglePin(a.name)}
                          disabled={pinSaving}
                        >📌</button>
                      )}
                    </div>
                  ))}
                </div>
                {totalPages > 1 && (
                  <div className="pagination">
                    <button className="page-btn" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>← Prev</button>
                    {Array.from({ length: totalPages }, (_, i) => (
                      <button key={i + 1} className={`page-btn ${page === i + 1 ? 'active' : ''}`} onClick={() => setPage(i + 1)}>
                        {i + 1}
                      </button>
                    ))}
                    <button className="page-btn" disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>Next →</button>
                  </div>
                )}
              </>
            )}
          </>
        )}

        {/* Members tab */}
        {activeTab === 'members' && (
          <MembersTab namespace={namespace} isOwner={isOwner} />
        )}

        {/* Activity tab */}
        {activeTab === 'activity' && (
          <NsActivityTab namespace={namespace} />
        )}

        {/* Starred tab (own profile only) */}
        {activeTab === 'starred' && isOwner && (
          starredItems.length === 0 ? (
            <div className="empty-state ns-empty">
              <div className="ns-empty-icon">☆</div>
              <h3>No starred artifacts yet</h3>
              <p>Star artifacts you want to revisit. Look for the ☆ button on any artifact page or card.</p>
            </div>
          ) : (
            <div className="artifacts-list">
              {starredItems.map(a => (
                <ArtifactCard key={`${a.kind}/${a.namespace}/${a.name}`} artifact={a} />
              ))}
            </div>
          )
        )}
      </div>
    </>
  )
}
