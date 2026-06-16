import { useEffect } from 'react'

export function useDocumentTitle(title) {
  useEffect(() => {
    document.title = title ? `${title} — SkillForge` : 'SkillForge Registry'
    return () => { document.title = 'SkillForge Registry' }
  }, [title])
}
