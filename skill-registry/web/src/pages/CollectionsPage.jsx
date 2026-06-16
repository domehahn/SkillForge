import { useState, useEffect } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import {
  fetchCollections, fetchCollection, createCollection,
  updateCollection, deleteCollection,
} from '../api/client'
import { avatarColor, timeAgo } from '../utils'
import { useDocumentTitle } from '../hooks/useDocumentTitle'
import { useToast } from '../contexts/ToastContext'
import { useConfirm } from '../contexts/ConfirmContext'

export function CollectionDetailPage() {
  const { owner, slug } = useParams()
  useDocumentTitle(`${slug} · ${owner}'s collections`)
  const toast = useToast()
  const confirm = useConfirm()
  const navigate = useNavigate()

  const loggedInUser = localStorage.getItem('sf_user')
  const isOwner = loggedInUser === owner
  const [col, setCol] = useState(null)
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState(false)
  const [editForm, setEditForm] = useState({ name: '', description: '', visibility: 'public' })

  useEffect(() => {
    fetchCollection(owner, slug)
      .then(d => { setCol(d); setEditForm({ name: d.name, description: d.description, visibility: d.visibility }) })
      .catch(() => navigate(`/namespace/${owner}/collections`))
      .finally(() => setLoading(false))
  }, [owner, slug])

  const saveEdit = async () => {
    try {
      const updated = await updateCollection(owner, slug, {
        ...editForm,
        artifacts: col.artifacts,
      })
      setCol(updated)
      setEditing(false)
      toast('Collection updated', 'success')
    } catch (err) { toast(err.message, 'error') }
  }

  const removeArtifact = async (art) => {
    const next = col.artifacts.filter(a => !(a.kind === art.kind && a.namespace === art.namespace && a.name === art.name))
    try {
      const updated = await updateCollection(owner, slug, { ...editForm, artifacts: next })
      setCol(updated)
    } catch (err) { toast(err.message, 'error') }
  }

  const doDelete = async () => {
    const ok = await confirm(`Delete collection "${col.name}"? This cannot be undone.`, { confirmLabel: 'Delete', danger: true })
    if (!ok) return
    try {
      await deleteCollection(owner, slug)
      toast('Collection deleted', 'success')
      navigate(`/namespace/${owner}/collections`)
    } catch (err) { toast(err.message, 'error') }
  }

  if (loading) return <div className="loading-state">Loading…</div>
  if (!col) return null

  return (
    <div className="collection-detail-page">
      <div className="collection-breadcrumb">
        <Link to={`/namespace/${owner}`}>{owner}</Link>
        <span> › </span>
        <Link to={`/namespace/${owner}/collections`}>Collections</Link>
        <span> › </span>
        <span>{col.name}</span>
      </div>

      {editing ? (
        <div className="collection-edit-form">
          <h2>Edit collection</h2>
          <label>Name<input value={editForm.name} onChange={e => setEditForm(f => ({ ...f, name: e.target.value }))} maxLength={80} /></label>
          <label>Description<textarea value={editForm.description} onChange={e => setEditForm(f => ({ ...f, description: e.target.value }))} rows={2} maxLength={500} /></label>
          <label>Visibility
            <select value={editForm.visibility} onChange={e => setEditForm(f => ({ ...f, visibility: e.target.value }))}>
              <option value="public">Public</option>
              <option value="private">Private</option>
            </select>
          </label>
          <div className="collection-edit-actions">
            <button className="btn-primary" onClick={saveEdit}>Save</button>
            <button className="btn-secondary" onClick={() => setEditing(false)}>Cancel</button>
          </div>
        </div>
      ) : (
        <div className="collection-detail-header">
          <div>
            <h1 className="collection-detail-title">
              {col.name}
              {col.visibility === 'private' && <span className="visibility-chip" style={{ marginLeft: 8 }}>Private</span>}
            </h1>
            {col.description && <p className="collection-detail-desc">{col.description}</p>}
            <div className="collection-detail-meta">
              By <Link to={`/namespace/${col.owner}`}>{col.owner}</Link>
              {' · '}{col.artifacts?.length || 0} artifact{col.artifacts?.length !== 1 ? 's' : ''}
              {' · '}{timeAgo(col.created_at)}
            </div>
          </div>
          {isOwner && (
            <div style={{ display: 'flex', gap: 8 }}>
              <button className="btn-secondary" onClick={() => setEditing(true)}>Edit</button>
              <button className="btn-danger" onClick={doDelete}>Delete</button>
            </div>
          )}
        </div>
      )}

      {col.artifacts?.length === 0 ? (
        <div className="empty-state" style={{ marginTop: 32 }}>
          <h3>No artifacts yet</h3>
          <p>Add artifacts to this collection from their detail page.</p>
        </div>
      ) : (
        <div className="collection-artifacts-list">
          {(col.artifacts || []).map(a => (
            <div key={`${a.kind}/${a.namespace}/${a.name}`} className="collection-artifact-item">
              <div className="collection-art-avatar" style={{ background: avatarColor(a.namespace) }}>
                {a.namespace[0].toUpperCase()}
              </div>
              <div className="collection-art-info">
                <Link to={`/artifacts/${a.kind}/${a.namespace}/${a.name}`} className="collection-art-name">
                  {a.name}
                </Link>
                <div className="collection-art-meta">
                  <span className="kind-chip kind-chip-sm">{a.kind}</span>
                  <Link to={`/namespace/${a.namespace}`} className="collection-art-ns">{a.namespace}</Link>
                </div>
              </div>
              {isOwner && (
                <button className="collection-art-remove" onClick={() => removeArtifact(a)} title="Remove from collection">✕</button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

export default function CollectionsPage() {
  const { namespace } = useParams()
  useDocumentTitle(`Collections · ${namespace}`)
  const toast = useToast()
  const confirm = useConfirm()
  const navigate = useNavigate()

  const loggedInUser = localStorage.getItem('sf_user')
  const isOwner = loggedInUser === namespace
  const [cols, setCols] = useState([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [newForm, setNewForm] = useState({ name: '', description: '', visibility: 'public' })
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    fetchCollections(namespace)
      .then(d => setCols(d.collections || []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [namespace])

  const doCreate = async () => {
    if (!newForm.name.trim()) return
    setSaving(true)
    try {
      const col = await createCollection(newForm)
      setCols(cs => [col, ...cs])
      setCreating(false)
      setNewForm({ name: '', description: '', visibility: 'public' })
      toast('Collection created', 'success')
    } catch (err) { toast(err.message, 'error') }
    finally { setSaving(false) }
  }

  return (
    <div className="collections-page">
      <div className="collections-header">
        <div>
          <h1 className="collections-title">Collections</h1>
          <p className="collections-sub">Curated lists of artifacts by <Link to={`/namespace/${namespace}`}>{namespace}</Link></p>
        </div>
        {isOwner && (
          <button className="btn-primary" onClick={() => setCreating(true)}>+ New collection</button>
        )}
      </div>

      {creating && (
        <div className="collection-create-form">
          <h3>New collection</h3>
          <label>Name <input value={newForm.name} onChange={e => setNewForm(f => ({ ...f, name: e.target.value }))} placeholder="e.g. My Favorites" maxLength={80} autoFocus /></label>
          <label>Description <input value={newForm.description} onChange={e => setNewForm(f => ({ ...f, description: e.target.value }))} placeholder="Optional description" maxLength={500} /></label>
          <label>Visibility
            <select value={newForm.visibility} onChange={e => setNewForm(f => ({ ...f, visibility: e.target.value }))}>
              <option value="public">Public</option>
              <option value="private">Private</option>
            </select>
          </label>
          <div className="collection-edit-actions">
            <button className="btn-primary" onClick={doCreate} disabled={saving || !newForm.name.trim()}>
              {saving ? 'Creating…' : 'Create'}
            </button>
            <button className="btn-secondary" onClick={() => setCreating(false)}>Cancel</button>
          </div>
        </div>
      )}

      {loading ? (
        <div className="loading-state">Loading…</div>
      ) : cols.length === 0 ? (
        <div className="empty-state">
          <h3>No collections yet</h3>
          {isOwner
            ? <p>Create a collection to curate and share your favorite artifacts.</p>
            : <p>This publisher hasn't created any collections yet.</p>}
        </div>
      ) : (
        <div className="collections-grid">
          {cols.map(c => (
            <Link
              key={c.slug}
              to={`/namespace/${namespace}/collections/${c.slug}`}
              className="collection-card"
            >
              <div className="collection-card-name">
                {c.name}
                {c.visibility === 'private' && <span className="visibility-chip" style={{ marginLeft: 6, fontSize: 11 }}>Private</span>}
              </div>
              {c.description && <div className="collection-card-desc">{c.description}</div>}
              <div className="collection-card-meta">
                {c.artifacts?.length || 0} artifact{c.artifacts?.length !== 1 ? 's' : ''}
                {' · '}{timeAgo(c.updated_at)}
              </div>
              <div className="collection-card-preview">
                {(c.artifacts || []).slice(0, 4).map(a => (
                  <div key={`${a.kind}/${a.name}`} className="col-preview-dot" style={{ background: avatarColor(a.namespace) }} title={a.name} />
                ))}
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  )
}
