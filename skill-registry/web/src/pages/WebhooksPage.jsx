import { useEffect, useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { listWebhooks, createWebhook, deleteWebhook, fetchWebhookDeliveries } from '../api/client'
import { useDocumentTitle } from '../hooks/useDocumentTitle'
import { useToast } from '../contexts/ToastContext'
import { useConfirm } from '../contexts/ConfirmContext'
import { timeAgo } from '../utils'

const EVENTS = [
  { value: 'artifact.published', label: 'Artifact published' },
  { value: 'artifact.yanked', label: 'Artifact yanked' },
  { value: 'artifact.deprecated', label: 'Artifact deprecated' },
  { value: 'artifact.promoted', label: 'Artifact promoted' },
]

function WebhookItem({ w, namespace, isOwner, onDelete, toast }) {
  const [showDeliveries, setShowDeliveries] = useState(false)
  const [deliveries, setDeliveries] = useState([])
  const [dlLoading, setDlLoading] = useState(false)
  const [testing, setTesting] = useState(false)

  const loadDeliveries = async () => {
    setDlLoading(true)
    try {
      const d = await fetchWebhookDeliveries(namespace, w.id)
      setDeliveries(d.deliveries || [])
    } catch {
      setDeliveries([])
    } finally {
      setDlLoading(false)
    }
  }

  const handleToggleDeliveries = () => {
    const next = !showDeliveries
    setShowDeliveries(next)
    if (next && deliveries.length === 0) loadDeliveries()
  }

  const handleTest = async () => {
    setTesting(true)
    try {
      const res = await fetch(`/api/v1/namespaces/${namespace}/webhooks/${w.id}/test`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${localStorage.getItem('sf_token')}` },
      })
      const json = await res.json().catch(() => ({}))
      if (json.success) {
        toast(`Test delivered — ${json.status_code}`, 'success')
      } else {
        toast(`Test failed — ${json.status_code || 'no response'}`, 'error')
      }
    } catch {
      toast('Test delivery failed', 'error')
    } finally {
      setTesting(false)
      if (showDeliveries) loadDeliveries()
    }
  }

  return (
    <div className="wh-item">
      <div className="wh-item-url">{w.url}</div>
      <div className="wh-item-meta">
        <div className="wh-events-tags">
          {(w.events || []).map(ev => (
            <span key={ev} className="scope-tag">{ev}</span>
          ))}
        </div>
        <div className="wh-item-detail">
          {w.enabled === false
            ? <span className="badge badge-unsigned">disabled</span>
            : <span className="badge badge-valid">active</span>}
          {w.created_at && (
            <span className="wh-date">Added {timeAgo(w.created_at)}</span>
          )}
        </div>
      </div>
      <div className="wh-item-actions">
        <button
          className="btn-secondary"
          style={{ fontSize: 12, padding: '4px 10px' }}
          onClick={handleToggleDeliveries}
        >
          {showDeliveries ? 'Hide' : 'Deliveries'}
        </button>
        {isOwner && (
          <>
            <button
              className="btn-secondary"
              style={{ fontSize: 12, padding: '4px 10px' }}
              disabled={testing}
              onClick={handleTest}
            >
              {testing ? 'Sending…' : 'Test'}
            </button>
            <button className="revoke-btn" onClick={() => onDelete(w.id)}>
              Delete
            </button>
          </>
        )}
      </div>
      {showDeliveries && (
        <div className="wh-deliveries">
          {dlLoading ? (
            <div className="loading-state" style={{ padding: '12px 0' }}>Loading…</div>
          ) : deliveries.length === 0 ? (
            <div className="wh-delivery-empty">No deliveries yet. Click "Test" to send a test ping.</div>
          ) : (
            <table className="wh-delivery-table">
              <thead>
                <tr>
                  <th>Status</th>
                  <th>Event</th>
                  <th>Response</th>
                  <th>Duration</th>
                  <th>Time</th>
                </tr>
              </thead>
              <tbody>
                {deliveries.map(d => (
                  <tr key={d.id}>
                    <td>
                      {d.success
                        ? <span className="badge badge-valid">✓</span>
                        : <span className="badge badge-unsigned">✗</span>}
                    </td>
                    <td><span className="scope-tag">{d.event}</span></td>
                    <td className="audit-detail">{d.status_code || '—'}</td>
                    <td className="audit-detail">{d.duration_ms}ms</td>
                    <td className="audit-time">{new Date(d.delivered_at).toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  )
}

export default function WebhooksPage() {
  const { namespace } = useParams()
  useDocumentTitle(`Webhooks · ${namespace}`)

  const navigate = useNavigate()
  const loggedInUser = localStorage.getItem('sf_user')
  const isOwner = loggedInUser === namespace

  const [webhooks, setWebhooks] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ url: '', secret: '', events: ['artifact.published'] })
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState(null)
  const toast = useToast()
  const confirm = useConfirm()

  useEffect(() => {
    if (!isOwner) { navigate(`/namespace/${namespace}`); return }
    setLoading(true)
    listWebhooks(namespace)
      .then(d => setWebhooks(Array.isArray(d?.webhooks) ? d.webhooks : Array.isArray(d) ? d : []))
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }, [namespace, isOwner, navigate])

  const toggleEvent = (ev) => {
    setForm(f => ({
      ...f,
      events: f.events.includes(ev) ? f.events.filter(e => e !== ev) : [...f.events, ev],
    }))
  }

  const handleCreate = async (e) => {
    e.preventDefault()
    setSaveError(null)
    setSaving(true)
    try {
      const hook = await createWebhook(namespace, form)
      setWebhooks(prev => [...prev, hook])
      setShowForm(false)
      setForm({ url: '', secret: '', events: ['artifact.published'] })
      toast('Webhook created', 'success')
    } catch (err) {
      setSaveError(err.message)
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id) => {
    const ok = await confirm('Delete this webhook? This cannot be undone.', { confirmLabel: 'Delete', danger: true })
    if (!ok) return
    try {
      await deleteWebhook(namespace, id)
      setWebhooks(prev => prev.filter(w => w.id !== id))
      toast('Webhook deleted', 'success')
    } catch (err) {
      toast(err.message, 'error')
    }
  }

  return (
    <div className="webhooks-page">
      <div className="wh-header">
        <div>
          <div className="wh-breadcrumb">
            <Link to={`/namespace/${namespace}`}>{namespace}</Link>
            <span className="bc-sep">›</span>
            <span>Webhooks</span>
          </div>
          <h1 className="wh-title">Webhooks</h1>
          <p className="wh-sub">
            Receive HTTP POST notifications when events occur in <strong>{namespace}</strong>.
          </p>
        </div>
        {isOwner && (
          <button className="btn-primary" onClick={() => setShowForm(v => !v)}>
            {showForm ? 'Cancel' : '+ Add webhook'}
          </button>
        )}
      </div>

      {showForm && (
        <form className="wh-form" onSubmit={handleCreate}>
          <h3>New Webhook</h3>
          <div className="field-group">
            <label>Payload URL</label>
            <input
              type="url"
              placeholder="https://example.com/webhook"
              value={form.url}
              onChange={e => setForm(f => ({ ...f, url: e.target.value }))}
              required
            />
          </div>
          <div className="field-group">
            <label>Secret (optional)</label>
            <input
              type="text"
              placeholder="Used to sign the payload"
              value={form.secret}
              onChange={e => setForm(f => ({ ...f, secret: e.target.value }))}
            />
          </div>
          <div className="field-group">
            <label>Events to subscribe to</label>
            <div className="wh-events">
              {EVENTS.map(ev => (
                <label key={ev.value} className="scope-checkbox">
                  <input
                    type="checkbox"
                    checked={form.events.includes(ev.value)}
                    onChange={() => toggleEvent(ev.value)}
                  />
                  {ev.label}
                </label>
              ))}
            </div>
          </div>
          {saveError && <div className="auth-error">{saveError}</div>}
          <div className="form-actions">
            <button type="submit" className="btn-primary" disabled={saving}>
              {saving ? 'Saving…' : 'Create webhook'}
            </button>
            <button type="button" className="btn-secondary" onClick={() => setShowForm(false)}>
              Cancel
            </button>
          </div>
        </form>
      )}

      {error && <div className="error-state">{error}</div>}

      {loading ? (
        <div className="loading-state">Loading webhooks…</div>
      ) : webhooks.length === 0 ? (
        <div className="empty-state">
          <h3>No webhooks configured</h3>
          <p>Add a webhook to receive notifications when artifacts are published or changed.</p>
        </div>
      ) : (
        <div className="wh-list">
          {webhooks.map(w => (
            <WebhookItem
              key={w.id}
              w={w}
              namespace={namespace}
              isOwner={isOwner}
              onDelete={handleDelete}
              toast={toast}
            />
          ))}
        </div>
      )}
    </div>
  )
}
