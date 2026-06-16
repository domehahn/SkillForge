import { useEffect, useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { fetchNamespace, patchNamespace, updateProfile, listMembers, upsertMember, removeMember } from '../api/client'
import { useDocumentTitle } from '../hooks/useDocumentTitle'
import { useToast } from '../contexts/ToastContext'
import { useConfirm } from '../contexts/ConfirmContext'
import { avatarColor } from '../utils'

export default function NamespaceSettingsPage() {
  const { namespace } = useParams()
  useDocumentTitle(`Settings · ${namespace}`)

  const navigate = useNavigate()
  const toast = useToast()
  const confirm = useConfirm()

  const loggedInUser = localStorage.getItem('sf_user')
  const isOwner = loggedInUser === namespace

  const [data, setData] = useState(null)
  const [error, setError] = useState(null)

  const [displayName, setDisplayName] = useState('')
  const [displayNameDirty, setDisplayNameDirty] = useState(false)
  const [dnSaving, setDnSaving] = useState(false)

  const [bio, setBio] = useState('')
  const [bioDirty, setBioDirty] = useState(false)
  const [bioSaving, setBioSaving] = useState(false)

  const [visibility, setVisibility] = useState('public')
  const [visDirty, setVisDirty] = useState(false)
  const [visSaving, setVisSaving] = useState(false)

  const [members, setMembers] = useState([])
  const [memberLoading, setMemberLoading] = useState(false)
  const [newMemberUsername, setNewMemberUsername] = useState('')
  const [newMemberRole, setNewMemberRole] = useState('maintainer')
  const [addingMember, setAddingMember] = useState(false)

  useEffect(() => {
    if (!isOwner) { navigate(`/namespace/${namespace}`); return }
    fetchNamespace(namespace)
      .then(d => {
        setData(d)
        setDisplayName(d.display_name || '')
        setBio(d.bio || '')
        setVisibility(d.default_visibility || 'public')
      })
      .catch(err => setError(err.message))
    setMemberLoading(true)
    listMembers(namespace)
      .then(d => setMembers(d.members || []))
      .catch(() => setMembers([]))
      .finally(() => setMemberLoading(false))
  }, [namespace, isOwner, navigate])

  const saveDisplayName = async () => {
    setDnSaving(true)
    try {
      await updateProfile({ display_name: displayName })
      toast('Display name updated', 'success')
      setDisplayNameDirty(false)
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setDnSaving(false)
    }
  }

  const saveBio = async () => {
    setBioSaving(true)
    try {
      await patchNamespace(namespace, { bio })
      toast('Bio updated', 'success')
      setBioDirty(false)
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setBioSaving(false)
    }
  }

  const saveVisibility = async () => {
    setVisSaving(true)
    try {
      await patchNamespace(namespace, { default_visibility: visibility })
      toast('Default visibility updated', 'success')
      setVisDirty(false)
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setVisSaving(false)
    }
  }

  const addMember = async (e) => {
    e.preventDefault()
    if (!newMemberUsername.trim()) return
    setAddingMember(true)
    try {
      await upsertMember(namespace, newMemberUsername.trim(), newMemberRole)
      toast(`${newMemberUsername} added as ${newMemberRole}`, 'success')
      setNewMemberUsername('')
      const d = await listMembers(namespace)
      setMembers(d.members || [])
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setAddingMember(false)
    }
  }

  const handleRemoveMember = async (username) => {
    const ok = await confirm(`Remove ${username} from this namespace?`, { confirmLabel: 'Remove', danger: true })
    if (!ok) return
    try {
      await removeMember(namespace, username)
      setMembers(prev => prev.filter(m => m.username !== username))
      toast(`${username} removed`, 'success')
    } catch (err) {
      toast(err.message, 'error')
    }
  }

  if (!isOwner) return null

  const color = avatarColor(namespace)

  return (
    <div className="settings-page">
      <div className="settings-header">
        <div className="ns-settings-avatar" style={{ background: color }}>
          {namespace[0].toUpperCase()}
        </div>
        <div>
          <div className="ns-settings-breadcrumb">
            <Link to={`/namespace/${namespace}`}>{namespace}</Link>
            <span className="bc-sep">›</span>
            <span>Settings</span>
          </div>
          <h1 className="settings-username">Namespace settings</h1>
          <p className="settings-sub">{namespace}</p>
        </div>
      </div>

      {error && <div className="error-state">{error}</div>}

      <div className="settings-layout">
        <nav className="settings-nav">
          <a href="#profile" className="settings-nav-item active">Profile</a>
          <a href="#publishing" className="settings-nav-item">Publishing</a>
          <a href="#collaborators" className="settings-nav-item">Collaborators</a>
          <a href="#danger" className="settings-nav-item danger">Danger zone</a>
        </nav>

        <div className="settings-main">
          {/* Profile */}
          <section className="settings-section" id="profile">
            <h2 className="settings-section-title">Profile</h2>
            <div className="settings-card">
              <div className="settings-field">
                <div className="settings-field-label">Namespace</div>
                <div className="settings-field-value">
                  <span className="settings-field-text">{namespace}</span>
                  <span className="settings-field-note">Namespace cannot be changed.</span>
                </div>
              </div>

              <div className="settings-field">
                <div className="settings-field-label">Display name</div>
                <div className="settings-field-value">
                  <input
                    type="text"
                    className="ns-settings-input"
                    value={displayName}
                    onChange={e => { setDisplayName(e.target.value); setDisplayNameDirty(true) }}
                    placeholder="Your display name"
                    maxLength={100}
                  />
                  {displayNameDirty && (
                    <button className="btn-primary" onClick={saveDisplayName} disabled={dnSaving} style={{ marginTop: 8 }}>
                      {dnSaving ? 'Saving…' : 'Save'}
                    </button>
                  )}
                </div>
              </div>

              <div className="settings-field">
                <div className="settings-field-label">Bio</div>
                <div className="settings-field-value">
                  <textarea
                    className="ns-bio-textarea"
                    value={bio}
                    onChange={e => { setBio(e.target.value); setBioDirty(true) }}
                    placeholder="Tell people about this namespace…"
                    rows={3}
                    maxLength={300}
                  />
                  <div className="field-hint">{bio.length}/300 characters</div>
                  {bioDirty && (
                    <button className="btn-primary" onClick={saveBio} disabled={bioSaving} style={{ marginTop: 8 }}>
                      {bioSaving ? 'Saving…' : 'Save bio'}
                    </button>
                  )}
                </div>
              </div>
            </div>
          </section>

          {/* Publishing */}
          <section className="settings-section" id="publishing">
            <h2 className="settings-section-title">Publishing</h2>
            <div className="settings-card">
              <div className="settings-field">
                <div>
                  <div className="settings-field-label">Webhooks</div>
                  <div className="settings-field-note" style={{ marginTop: 4 }}>
                    Receive HTTP notifications when artifacts are published or changed.
                  </div>
                </div>
                <Link to={`/namespace/${namespace}/webhooks`} className="btn-secondary">
                  Manage webhooks
                </Link>
              </div>
              <div className="settings-field">
                <div>
                  <div className="settings-field-label">Access tokens</div>
                  <div className="settings-field-note" style={{ marginTop: 4 }}>
                    Tokens let CI/CD pipelines publish to this namespace.
                  </div>
                </div>
                <Link to="/account/tokens" className="btn-secondary">
                  Manage tokens
                </Link>
              </div>
              <div className="settings-field">
                <div>
                  <div className="settings-field-label">Default artifact visibility</div>
                  <div className="settings-field-note" style={{ marginTop: 4 }}>
                    Applied to newly published artifacts if not set in the manifest.
                  </div>
                </div>
                <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                  <select
                    className="sort-select"
                    value={visibility}
                    onChange={e => { setVisibility(e.target.value); setVisDirty(true) }}
                  >
                    <option value="public">Public</option>
                    <option value="private">Private</option>
                  </select>
                  {visDirty && (
                    <button className="btn-primary" onClick={saveVisibility} disabled={visSaving}>
                      {visSaving ? 'Saving…' : 'Save'}
                    </button>
                  )}
                </div>
              </div>
            </div>
          </section>

          {/* Collaborators */}
          <section className="settings-section" id="collaborators">
            <h2 className="settings-section-title">Collaborators</h2>
            <div className="settings-card">
              <p className="settings-field-note" style={{ marginBottom: 16 }}>
                Collaborators can publish and manage artifacts under this namespace. Owners can also add and remove collaborators.
              </p>

              {/* Add collaborator form */}
              <form className="collab-add-form" onSubmit={addMember}>
                <input
                  className="ns-settings-input"
                  placeholder="Username"
                  value={newMemberUsername}
                  onChange={e => setNewMemberUsername(e.target.value)}
                  style={{ flex: 1 }}
                />
                <select
                  className="sort-select"
                  value={newMemberRole}
                  onChange={e => setNewMemberRole(e.target.value)}
                >
                  <option value="reader">Reader</option>
                  <option value="maintainer">Maintainer</option>
                  <option value="owner">Owner</option>
                </select>
                <button type="submit" className="btn-primary" disabled={addingMember || !newMemberUsername.trim()}>
                  {addingMember ? 'Adding…' : 'Add'}
                </button>
              </form>

              {/* Members list */}
              {memberLoading ? (
                <div className="loading-state" style={{ padding: '12px 0' }}>Loading…</div>
              ) : members.length === 0 ? (
                <div className="collab-empty">No collaborators yet. This namespace is open to its owner only.</div>
              ) : (
                <div className="collab-list">
                  {members.map(m => (
                    <div key={m.username} className="collab-row">
                      <div className="collab-avatar" style={{ background: avatarColor(m.username) }}>
                        {m.username[0].toUpperCase()}
                      </div>
                      <div className="collab-info">
                        <div className="collab-name">{m.username}</div>
                        <div className="collab-since">since {new Date(m.created_at).toLocaleDateString()}</div>
                      </div>
                      <span className={`collab-role collab-role-${m.role}`}>{m.role}</span>
                      {m.username !== loggedInUser && (
                        <button
                          className="revoke-btn"
                          onClick={() => handleRemoveMember(m.username)}
                        >Remove</button>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          </section>

          {/* Danger zone */}
          <section className="settings-section" id="danger">
            <h2 className="settings-section-title settings-section-danger">Danger zone</h2>
            <div className="settings-card settings-card-danger">
              <div className="settings-field">
                <div>
                  <div className="settings-field-label">Transfer namespace</div>
                  <div className="settings-field-note" style={{ marginTop: 4 }}>
                    Transfer ownership of this namespace and all its artifacts to another user.
                  </div>
                </div>
                <button
                  className="btn-danger"
                  onClick={() => toast('Namespace transfer is not yet available via the web UI. Use the CLI: skforge namespace transfer', 'info')}
                >
                  Transfer
                </button>
              </div>
              <div className="settings-field">
                <div>
                  <div className="settings-field-label">Delete namespace</div>
                  <div className="settings-field-note" style={{ marginTop: 4 }}>
                    Permanently delete this namespace and all published artifacts.
                    This cannot be undone.
                  </div>
                </div>
                <button
                  className="btn-danger"
                  onClick={async () => {
                    const ok = await confirm(
                      `Delete namespace "${namespace}"? All artifacts will be permanently removed. This cannot be undone.`,
                      { confirmLabel: 'Delete namespace', danger: true }
                    )
                    if (ok) toast('Namespace deletion is not yet available via the web UI.', 'info')
                  }}
                >
                  Delete namespace
                </button>
              </div>
            </div>
          </section>
        </div>
      </div>
    </div>
  )
}
