const KEY = 'sf_recent'
const MAX = 8

export function getRecentlyViewed() {
  try { return JSON.parse(localStorage.getItem(KEY) || '[]') } catch { return [] }
}

export function trackRecentlyViewed(item) {
  // item: { kind, namespace, name, description, latest_version, updated_at }
  const prev = getRecentlyViewed()
  const filtered = prev.filter(r => !(r.kind === item.kind && r.namespace === item.namespace && r.name === item.name))
  const next = [item, ...filtered].slice(0, MAX)
  localStorage.setItem(KEY, JSON.stringify(next))
  window.dispatchEvent(new Event('sf_recent'))
}
