const KEY = 'sf_starred'

export function getStarred() {
  try { return JSON.parse(localStorage.getItem(KEY) || '[]') } catch { return [] }
}

export function isStarred(kind, namespace, name) {
  return getStarred().some(s => s.kind === kind && s.namespace === namespace && s.name === name)
}

export function toggleStar(item) {
  const prev = getStarred()
  const idx = prev.findIndex(s => s.kind === item.kind && s.namespace === item.namespace && s.name === item.name)
  if (idx >= 0) {
    localStorage.setItem(KEY, JSON.stringify(prev.filter((_, i) => i !== idx)))
    window.dispatchEvent(new Event('sf_starred'))
    return false
  }
  localStorage.setItem(KEY, JSON.stringify([{ ...item, starred_at: new Date().toISOString() }, ...prev]))
  window.dispatchEvent(new Event('sf_starred'))
  return true
}
