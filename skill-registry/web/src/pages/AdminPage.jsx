import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  fetchAuditLog,
  adminListUsers, adminCreateUser, adminUpdateUserRole, adminDeleteUser,
  fetchAdminNamespaces, adminVerifyNamespace,
} from '../api/client'
import { useDocumentTitle } from '../hooks/useDocumentTitle'
import { useToast } from '../contexts/ToastContext'
import { useConfirm } from '../contexts/ConfirmContext'
import { avatarColor, timeAgo } from '../utils'

const PAGE_SIZE = 30

const ACTION_COLORS = {
  publish: 'badge-valid',
  delete: 'badge-unsigned',
  yank: 'badge-pending',
  unyank: 'badge-valid',
  deprecate: 'badge-pending',
  login: 'badge-signed',
  token_create: 'badge-signed',
  token_revoke: 'badge-unsigned',
  user_create: 'badge-valid',
  user_delete: 'badge-unsigned',
  user_role_update: 'badge-pending',
}

function actionBadge(action) {
  const cls = ACTION_COLORS[action] || 'badge-unsigned'
  return <span className={`badge ${cls}`}>{action || '–'}</span>
}

function UsersTab() {
  const [users, setUsers] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [showCreate, setShowCreate] = useState(false)
  const [form, setForm] = useState({ username: '', password: '', role: 'user' })
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState(null)
  const toast = useToast()
  const confirm = useConfirm()

  const load = () => {
    setLoading(true)
    adminListUsers()
      .then(d => setUsers(d.users || []))
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(() => { load() }, [])

  const handleCreate = async (e) => {
    e.preventDefault()
    setCreateError(null)
    setCreating(true)
    try {
      await adminCreateUser(form.username, form.password, form.role)
      setForm({ username: '', password: '', role: 'user' })
      setShowCreate(false)
      toast(`User "${form.username}" created`, 'success')
      load()
    } catch (err) {
      setCreateError(err.message)
    } finally {
      setCreating(false)
    }
  }

  const handleRoleChange = async (username, newRole) => {
    try {
      await adminUpdateUserRole(username, newRole)
      setUsers(prev => prev.map(u => u.username === username ? { ...u, role: newRole } : u))
      toast(`Role updated for ${username}`, 'success')
    } catch (err) {
      toast(err.message, 'error')
    }
  }

  const handleDelete = async (username) => {
    const ok = await confirm(
      `Delete user "${username}"? This will remove the account permanently and cannot be undone.`,
      { confirmLabel: 'Delete user', danger: true }
    )
    if (!ok) return
    try {
      await adminDeleteUser(username)
      setUsers(prev => prev.filter(u => u.username !== username))
      toast(`User "${username}" deleted`, 'success')
    } catch (err) {
      toast(err.message, 'error')
    }
  }

  return (
    <div className="admin-users-tab">
      <div className="admin-users-header">
        <div>
          <div className="admin-users-count">
            {loading ? '…' : `${users.length} user${users.length !== 1 ? 's' : ''}`}
          </div>
        </div>
        <button className="btn-primary" onClick={() => setShowCreate(v => !v)}>
          {showCreate ? 'Cancel' : '+ New user'}
        </button>
      </div>

      {showCreate && (
        <form className="create-token-form" style={{ marginBottom: 20 }} onSubmit={handleCreate}>
          <h2>Create user</h2>
          <div className="admin-user-form-row">
            <div className="field-group" style={{ flex: 1 }}>
              <label>Username</label>
              <input
                type="text"
                value={form.username}
                onChange={e => setForm(f => ({ ...f, username: e.target.value }))}
                required
                autoFocus
                placeholder="username"
              />
            </div>
            <div className="field-group" style={{ flex: 1 }}>
              <label>Password</label>
              <input
                type="password"
                value={form.password}
                onChange={e => setForm(f => ({ ...f, password: e.target.value }))}
                required
                placeholder="••••••••"
              />
            </div>
            <div className="field-group" style={{ flex: '0 0 140px' }}>
              <label>Role</label>
              <select
                className="sort-select"
                value={form.role}
                onChange={e => setForm(f => ({ ...f, role: e.target.value }))}
              >
                <option value="user">user</option>
                <option value="admin">admin</option>
              </select>
            </div>
          </div>
          {createError && <div className="auth-error">{createError}</div>}
          <div className="form-actions">
            <button type="submit" className="btn-primary" disabled={creating}>
              {creating ? 'Creating…' : 'Create user'}
            </button>
          </div>
        </form>
      )}

      {error && <div className="error-state">{error}</div>}

      {loading ? (
        <div className="loading-state">Loading users…</div>
      ) : (
        <table className="admin-table">
          <thead>
            <tr>
              <th>User</th>
              <th>Role</th>
              <th>Created</th>
              <th>Last updated</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {users.map(u => (
              <tr key={u.id}>
                <td>
                  <div className="admin-user-cell">
                    <div className="admin-user-avatar" style={{ background: avatarColor(u.username) }}>
                      {u.username[0].toUpperCase()}
                    </div>
                    <span className="admin-user-name">{u.username}</span>
                  </div>
                </td>
                <td>
                  <select
                    className="role-select"
                    value={u.role}
                    onChange={e => handleRoleChange(u.username, e.target.value)}
                  >
                    <option value="user">user</option>
                    <option value="admin">admin</option>
                  </select>
                </td>
                <td className="audit-time">{u.created_at ? new Date(u.created_at).toLocaleDateString() : '–'}</td>
                <td className="audit-time">{u.updated_at ? timeAgo(u.updated_at) : '–'}</td>
                <td>
                  <button
                    className="revoke-btn"
                    onClick={() => handleDelete(u.username)}
                    disabled={u.username === localStorage.getItem('sf_user')}
                    title={u.username === localStorage.getItem('sf_user') ? 'Cannot delete your own account' : ''}
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

function NamespacesTab() {
  const [namespaces, setNamespaces] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const toast = useToast()
  const navigate = useNavigate()

  const load = () => {
    setLoading(true)
    fetchAdminNamespaces()
      .then(d => setNamespaces(d.namespaces || []))
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(() => { load() }, [])

  const toggleVerified = async (ns) => {
    try {
      await adminVerifyNamespace(ns.namespace, !ns.verified)
      setNamespaces(prev => prev.map(n => n.namespace === ns.namespace ? { ...n, verified: !n.verified } : n))
      toast(`${ns.namespace} ${ns.verified ? 'unverified' : 'verified'}`, 'success')
    } catch (err) {
      toast(err.message, 'error')
    }
  }

  if (loading) return <div className="loading-state">Loading namespaces…</div>
  if (error) return <div className="error-state">{error}</div>
  if (namespaces.length === 0) return <div className="empty-state"><h3>No namespaces found</h3><p>Namespaces appear when artifacts are published.</p></div>

  return (
    <div className="admin-ns-tab">
      <div className="admin-users-header">
        <div className="admin-users-count">{namespaces.length} namespace{namespaces.length !== 1 ? 's' : ''}</div>
      </div>
      <table className="admin-table">
        <thead>
          <tr>
            <th>Namespace</th>
            <th>Display name</th>
            <th>Artifacts</th>
            <th>Verified</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {namespaces.map(ns => (
            <tr key={ns.namespace}>
              <td>
                <button
                  className="admin-ns-link"
                  onClick={() => navigate(`/namespace/${ns.namespace}`)}
                >
                  <div className="admin-user-avatar" style={{ background: avatarColor(ns.namespace) }}>
                    {ns.namespace[0].toUpperCase()}
                  </div>
                  <span className="admin-user-name">{ns.namespace}</span>
                </button>
              </td>
              <td className="audit-detail">{ns.display_name || <span className="text-muted">—</span>}</td>
              <td className="audit-detail">{ns.artifact_count}</td>
              <td>
                {ns.verified
                  ? <span className="badge badge-valid">✓ verified</span>
                  : <span className="badge badge-unsigned">unverified</span>}
              </td>
              <td>
                <button
                  className={`btn-secondary ${ns.verified ? '' : ''}`}
                  style={{ fontSize: 12, padding: '4px 10px' }}
                  onClick={() => toggleVerified(ns)}
                >
                  {ns.verified ? 'Unverify' : 'Verify'}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export default function AdminPage() {
  useDocumentTitle('Admin Panel')

  const navigate = useNavigate()
  const role = localStorage.getItem('sf_role')
  const [tab, setTab] = useState('audit')
  const [entries, setEntries] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [actionFilter, setActionFilter] = useState('')

  useEffect(() => {
    if (role !== 'admin') { navigate('/'); return }
  }, [role, navigate])

  useEffect(() => {
    if (tab !== 'audit') return
    setLoading(true)
    fetchAuditLog({ limit: PAGE_SIZE, offset: (page - 1) * PAGE_SIZE, action: actionFilter || undefined })
      .then(d => {
        setEntries(d.entries || d.audit_log || d || [])
        setTotal(d.total || (d.entries || d.audit_log || d || []).length)
      })
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }, [page, tab, actionFilter])

  if (role !== 'admin') return null

  const totalPages = Math.ceil(total / PAGE_SIZE)

  return (
    <div className="admin-page">
      <div className="admin-header">
        <div>
          <h1 className="admin-title">Admin Panel</h1>
          <p className="admin-sub">Registry management for administrators</p>
        </div>
      </div>

      <div className="admin-tabs">
        <button className={`admin-tab ${tab === 'audit' ? 'active' : ''}`} onClick={() => setTab('audit')}>
          Audit Log
        </button>
        <button className={`admin-tab ${tab === 'users' ? 'active' : ''}`} onClick={() => setTab('users')}>
          Users
        </button>
        <button className={`admin-tab ${tab === 'namespaces' ? 'active' : ''}`} onClick={() => setTab('namespaces')}>
          Namespaces
        </button>
      </div>

      {tab === 'users' && <UsersTab />}
      {tab === 'namespaces' && <NamespacesTab />}

      {tab === 'audit' && (
        <>
          <div className="audit-toolbar">
            <select
              className="sort-select"
              value={actionFilter}
              onChange={e => { setActionFilter(e.target.value); setPage(1) }}
            >
              <option value="">All actions</option>
              {Object.keys(ACTION_COLORS).map(a => (
                <option key={a} value={a}>{a}</option>
              ))}
            </select>
          </div>

          {error && <div className="error-state">{error}</div>}

          {loading ? (
            <div className="loading-state">Loading audit log…</div>
          ) : entries.length === 0 ? (
            <div className="empty-state">
              <h3>No audit events{actionFilter ? ` for "${actionFilter}"` : ''}</h3>
              <p>Events will appear here as users interact with the registry.</p>
            </div>
          ) : (
            <>
              <div className="admin-table-wrap">
                <table className="admin-table">
                  <thead>
                    <tr>
                      <th>Time</th>
                      <th>Action</th>
                      <th>User</th>
                      <th>Resource</th>
                      <th>Details</th>
                    </tr>
                  </thead>
                  <tbody>
                    {entries.map((e, i) => (
                      <tr key={e.id || i}>
                        <td className="audit-time">
                          {e.created_at || e.timestamp
                            ? new Date(e.created_at || e.timestamp).toLocaleString()
                            : '–'}
                        </td>
                        <td>{actionBadge(e.action)}</td>
                        <td className="audit-user">{e.user || e.username || '–'}</td>
                        <td className="audit-resource">
                          <span className="audit-resource-text">
                            {e.resource || [e.namespace, e.name].filter(Boolean).join('/') || '–'}
                          </span>
                        </td>
                        <td className="audit-detail">{e.message || e.details || '–'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {totalPages > 1 && (
                <div className="pagination" style={{ marginTop: 20 }}>
                  <button className="page-btn" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>← Prev</button>
                  <span className="page-info">Page {page} of {totalPages}</span>
                  <button className="page-btn" disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>Next →</button>
                </div>
              )}
            </>
          )}
        </>
      )}
    </div>
  )
}
