import { useEffect } from 'react'

export function useMetaTags({ title, description, url } = {}) {
  useEffect(() => {
    const set = (prop, content, isName = false) => {
      const attr = isName ? 'name' : 'property'
      let el = document.querySelector(`meta[${attr}="${prop}"]`)
      if (!el) {
        el = document.createElement('meta')
        el.setAttribute(attr, prop)
        document.head.appendChild(el)
      }
      el.setAttribute('content', content)
    }

    const fullTitle = title ? `${title} — SkillForge` : 'SkillForge Registry'
    if (title) {
      set('og:title', fullTitle)
      set('twitter:title', fullTitle, true)
    }
    if (description) {
      set('og:description', description)
      set('description', description, true)
      set('twitter:description', description, true)
    }
    if (url) set('og:url', url)
    set('og:type', 'website')
    set('og:site_name', 'SkillForge Registry')
    set('twitter:card', 'summary', true)
  }, [title, description, url])
}
