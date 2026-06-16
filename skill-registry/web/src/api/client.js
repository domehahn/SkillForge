const API_BASE = '/api/v1'

function authHeaders() {
  const token = localStorage.getItem('sf_token')
  return token ? { Authorization: `Bearer ${token}` } : {}
}

async function apiFetch(url, options = {}) {
  const res = await fetch(url, {
    ...options,
    headers: { ...authHeaders(), ...(options.headers || {}) },
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err?.error?.message || `HTTP ${res.status}`)
  }
  return res.json()
}

export function login(username, password) {
  return apiFetch(`${API_BASE}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
}

export function listTokens() {
  return apiFetch(`${API_BASE}/tokens`)
}

export function createToken(name, scopes, expiresIn) {
  return apiFetch(`${API_BASE}/tokens`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, scopes, expires_in: expiresIn }),
  })
}

export function revokeToken(id) {
  return apiFetch(`${API_BASE}/tokens/${id}`, { method: 'DELETE' })
}

export function fetchArtifacts(params = {}) {
  const q = new URLSearchParams()
  if (params.q)         q.append('q', params.q)
  if (params.kind)      q.append('kind', params.kind)
  if (params.namespace) q.append('namespace', params.namespace)
  if (params.sort)      q.append('sort', params.sort)
  if (params.limit)     q.append('limit', params.limit)
  if (params.offset)    q.append('offset', params.offset)
  if (params.verified)  q.append('verified', 'true')
  return apiFetch(`${API_BASE}/artifacts?${q}`)
}

export async function addToCollection(owner, slug, artifact) {
  const col = await fetchCollection(owner, slug)
  const already = (col.artifacts || []).some(
    a => a.kind === artifact.kind && a.namespace === artifact.namespace && a.name === artifact.name
  )
  if (already) return col
  return updateCollection(owner, slug, {
    name: col.name, description: col.description, visibility: col.visibility,
    artifacts: [...(col.artifacts || []), artifact],
  })
}

export async function removeFromCollection(owner, slug, artifact) {
  const col = await fetchCollection(owner, slug)
  return updateCollection(owner, slug, {
    name: col.name, description: col.description, visibility: col.visibility,
    artifacts: (col.artifacts || []).filter(
      a => !(a.kind === artifact.kind && a.namespace === artifact.namespace && a.name === artifact.name)
    ),
  })
}

export function fetchArtifactDetails(kind, namespace, name) {
  return apiFetch(`${API_BASE}/artifacts/${kind}/${namespace}/${name}`)
}

export function fetchArtifactGraph(kind, namespace, name, version = 'latest') {
  return apiFetch(`${API_BASE}/artifacts/${kind}/${namespace}/${name}/versions/${version}/graph`)
}

export function fetchNamespace(namespace) {
  return apiFetch(`${API_BASE}/namespaces/${namespace}`)
}

export function fetchStats() {
  return apiFetch(`${API_BASE}/stats`)
}

export function fetchMetadata() {
  return apiFetch(`${API_BASE}/metadata`)
}

export function fetchFacets(q) {
  const params = q ? `?q=${encodeURIComponent(q)}` : ''
  return apiFetch(`${API_BASE}/artifacts/facets${params}`)
}

export function fetchPromotions(kind, namespace, name) {
  return apiFetch(`${API_BASE}/artifacts/${kind}/${namespace}/${name}/promotions`)
}

export function fetchAttestations(kind, namespace, name, version) {
  return apiFetch(`${API_BASE}/artifacts/${kind}/${namespace}/${name}/versions/${version}/attestations`)
}

export function listWebhooks(namespace) {
  return apiFetch(`${API_BASE}/namespaces/${namespace}/webhooks`)
}

export function createWebhook(namespace, data) {
  return apiFetch(`${API_BASE}/namespaces/${namespace}/webhooks`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}

export function deleteWebhook(namespace, id) {
  return apiFetch(`${API_BASE}/namespaces/${namespace}/webhooks/${id}`, { method: 'DELETE' })
}

export function listMembers(namespace) {
  return apiFetch(`${API_BASE}/namespaces/${namespace}/members`)
}

export function upsertMember(namespace, username, role) {
  return apiFetch(`${API_BASE}/namespaces/${namespace}/members`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, role }),
  })
}

export function removeMember(namespace, username) {
  return apiFetch(`${API_BASE}/namespaces/${namespace}/members/${username}`, { method: 'DELETE' })
}

export function yankVersion(namespace, name, version) {
  return apiFetch(`${API_BASE}/skills/${namespace}/${name}/versions/${version}/yank`, { method: 'POST' })
}

export function unyankVersion(namespace, name, version) {
  return apiFetch(`${API_BASE}/skills/${namespace}/${name}/versions/${version}/unyank`, { method: 'POST' })
}

export function deprecateVersion(namespace, name, version, reason = '') {
  return apiFetch(`${API_BASE}/skills/${namespace}/${name}/versions/${version}/deprecate`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message: reason }),
  })
}

export function deleteVersion(namespace, name, version) {
  return apiFetch(`${API_BASE}/skills/${namespace}/${name}/versions/${version}`, { method: 'DELETE' })
}

export function fetchLockfile(kind, namespace, name, version) {
  return apiFetch(`${API_BASE}/artifacts/${kind}/${namespace}/${name}/versions/${version}/lockfile`)
}

export function setDistTag(namespace, name, tag, version) {
  return apiFetch(`${API_BASE}/skills/${namespace}/${name}/dist-tags/${tag}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ version }),
  })
}

export function adminListUsers() {
  return apiFetch(`${API_BASE}/admin/users`)
}

export function adminCreateUser(username, password, role) {
  return apiFetch(`${API_BASE}/admin/users`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password, role }),
  })
}

export function adminUpdateUserRole(username, role) {
  return apiFetch(`${API_BASE}/admin/users/${username}/role`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ role }),
  })
}

export function adminDeleteUser(username) {
  return apiFetch(`${API_BASE}/admin/users/${username}`, { method: 'DELETE' })
}

export function patchArtifact(kind, namespace, name, updates) {
  return apiFetch(`${API_BASE}/artifacts/${kind}/${namespace}/${name}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(updates),
  })
}

export function patchNamespace(namespace, updates) {
  return apiFetch(`${API_BASE}/namespaces/${namespace}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(updates),
  })
}

export function changePassword(currentPassword, newPassword) {
  return apiFetch(`${API_BASE}/account/password`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
  })
}

export function fetchAdminNamespaces() {
  return apiFetch(`${API_BASE}/admin/namespaces`)
}

export function adminVerifyNamespace(namespace, verified) {
  return apiFetch(`${API_BASE}/admin/namespaces/${namespace}/verify`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ verified }),
  })
}

export function updateProfile(updates) {
  return apiFetch(`${API_BASE}/account/profile`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(updates),
  })
}

export function fetchArtifactDependents(namespace, name) {
  return apiFetch(`${API_BASE}/artifacts/${namespace}/${name}/dependents`)
}

export function fetchTopPublishers(limit = 12) {
  return apiFetch(`${API_BASE}/publishers/top?limit=${limit}`)
}

export function fetchPinned(namespace) {
  return apiFetch(`${API_BASE}/namespaces/${namespace}/pinned`)
}

export function setPinned(namespace, pinned) {
  return apiFetch(`${API_BASE}/namespaces/${namespace}/pinned`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ pinned }),
  })
}

export function fetchArtifactStats(kind, namespace, name, days = 30) {
  return apiFetch(`${API_BASE}/artifacts/${kind}/${namespace}/${name}/stats?days=${days}`)
}

export function patchArtifactVersion(kind, namespace, name, version, updates) {
  return apiFetch(`${API_BASE}/artifacts/${kind}/${namespace}/${name}/versions/${version}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(updates),
  })
}

export function fetchWebhookDeliveries(namespace, webhookId) {
  return apiFetch(`${API_BASE}/namespaces/${namespace}/webhooks/${webhookId}/deliveries`)
}

export function fetchNamespaceInsights(namespace, days = 30) {
  return apiFetch(`${API_BASE}/namespaces/${namespace}/insights?days=${days}`)
}

export function fetchCollections(owner) {
  return apiFetch(`${API_BASE}/namespaces/${owner}/collections`)
}

export function fetchCollection(owner, slug) {
  return apiFetch(`${API_BASE}/namespaces/${owner}/collections/${slug}`)
}

export function createCollection(data) {
  return apiFetch(`${API_BASE}/collections`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}

export function updateCollection(owner, slug, data) {
  return apiFetch(`${API_BASE}/namespaces/${owner}/collections/${slug}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}

export function deleteCollection(owner, slug) {
  return apiFetch(`${API_BASE}/namespaces/${owner}/collections/${slug}`, { method: 'DELETE' })
}

export function transferArtifact(kind, namespace, name, toNamespace) {
  return apiFetch(`${API_BASE}/artifacts/${kind}/${namespace}/${name}/transfer`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ to_namespace: toNamespace }),
  })
}

export function fetchArtifactStarInfo(kind, namespace, name) {
  return apiFetch(`${API_BASE}/artifacts/${kind}/${namespace}/${name}/stars`)
}

export function starArtifact(kind, namespace, name) {
  return apiFetch(`${API_BASE}/artifacts/${kind}/${namespace}/${name}/star`, { method: 'POST' })
}

export function unstarArtifact(kind, namespace, name) {
  return apiFetch(`${API_BASE}/artifacts/${kind}/${namespace}/${name}/star`, { method: 'DELETE' })
}

export function fetchScanResults(kind, namespace, name, version) {
  return apiFetch(`${API_BASE}/artifacts/${kind}/${namespace}/${name}/versions/${version}/scan`)
}

export function fetchComments(kind, namespace, name) {
  return apiFetch(`${API_BASE}/artifacts/${kind}/${namespace}/${name}/comments`)
}

export function addComment(kind, namespace, name, body) {
  return apiFetch(`${API_BASE}/artifacts/${kind}/${namespace}/${name}/comments`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ body }),
  })
}

export function updateComment(id, body) {
  return apiFetch(`${API_BASE}/comments/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ body }),
  })
}

export function deleteComment(id) {
  return apiFetch(`${API_BASE}/comments/${id}`, { method: 'DELETE' })
}

export function fetchNotifications() {
  return apiFetch(`${API_BASE}/notifications`)
}

export function markNotificationsRead() {
  return apiFetch(`${API_BASE}/notifications/read`, { method: 'PUT' })
}

export function deleteNotification(id) {
  return apiFetch(`${API_BASE}/notifications/${id}`, { method: 'DELETE' })
}

export function fetchFollowInfo(namespace) {
  return apiFetch(`${API_BASE}/namespaces/${namespace}/follow`)
}

export function followNamespace(namespace) {
  return apiFetch(`${API_BASE}/namespaces/${namespace}/follow`, { method: 'POST' })
}

export function unfollowNamespace(namespace) {
  return apiFetch(`${API_BASE}/namespaces/${namespace}/follow`, { method: 'DELETE' })
}

export function fetchFollowing() {
  return apiFetch(`${API_BASE}/account/following`)
}

export function testWebhook(namespace, id) {
  return apiFetch(`${API_BASE}/namespaces/${namespace}/webhooks/${id}/test`, { method: 'POST' })
}

export function fetchAuditLog(params = {}) {
  const q = new URLSearchParams()
  if (params.limit) q.append('limit', params.limit)
  if (params.offset) q.append('offset', params.offset)
  if (params.action) q.append('action', params.action)
  return apiFetch(`${API_BASE}/audit?${q}`)
}
