import { useEffect, useState } from 'react'
import { useLocation } from 'react-router-dom'

export default function LoadingBar() {
  const location = useLocation()
  const [width, setWidth] = useState(0)
  const [visible, setVisible] = useState(false)

  useEffect(() => {
    setVisible(true)
    setWidth(0)
    const t1 = setTimeout(() => setWidth(60), 40)
    const t2 = setTimeout(() => setWidth(85), 250)
    const t3 = setTimeout(() => setWidth(100), 450)
    const t4 = setTimeout(() => setVisible(false), 700)
    return () => [t1, t2, t3, t4].forEach(clearTimeout)
  }, [location.pathname, location.search])

  if (!visible && width === 0) return null
  return (
    <div
      className="loading-bar"
      style={{
        width: `${width}%`,
        opacity: width === 100 ? 0 : 1,
        transition: width === 0 ? 'none' : 'width 0.3s ease, opacity 0.3s ease',
      }}
    />
  )
}
